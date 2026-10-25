package apikey

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"omsbizfoundry/identity-access/internal/identity"
)

const (
	ActionUse         = "open-api.invoice-reimbursement.use"
	ActionKeyManage   = "open-api.key.manage"
	ActionSelfAudit   = "open-api.self-audit.read"
	ActionTenantAudit = "open-api.tenant-audit.read"
)

var (
	uuidPattern   = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	secretPattern = regexp.MustCompile(`^(bfk_[A-Za-z0-9_-]{10})_([A-Za-z0-9_-]{32,64})$`)
	actionPattern = regexp.MustCompile(`^open-api\.[a-z0-9.-]+$`)
)

type ErrorKind string

const (
	KindValidation   ErrorKind = "validation"
	KindUnauthorized ErrorKind = "unauthorized"
	KindForbidden    ErrorKind = "forbidden"
	KindNotFound     ErrorKind = "not_found"
	KindConflict     ErrorKind = "conflict"
	KindDependency   ErrorKind = "dependency"
)

type DomainError struct {
	Kind    ErrorKind
	Code    string
	Message string
	Err     error
}

func (e *DomainError) Error() string { return e.Message }
func (e *DomainError) Unwrap() error { return e.Err }

type PersonReference struct {
	ID            string `json:"id"`
	TenantID      string `json:"tenant_id"`
	Status        string `json:"status"`
	Revision      int64  `json:"revision"`
	ReferenceCode string `json:"reference_code,omitempty"`
	DisplayName   string `json:"display_name"`
	Digest        string `json:"digest"`
}

type PersonResolver interface {
	ResolveBySubject(context.Context, string, string) (PersonReference, error)
}

type Key struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"-"`
	SubjectID      string     `json:"-"`
	PersonID       string     `json:"person_id"`
	PersonRevision int64      `json:"person_revision"`
	Label          string     `json:"label"`
	Prefix         string     `json:"prefix"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
}

type CreatedKey struct {
	Key
	Secret string `json:"secret"`
}

type Usage struct {
	ID             string     `json:"id"`
	KeyID          string     `json:"key_id"`
	KeyPrefix      string     `json:"key_prefix"`
	SubjectID      string     `json:"subject_id"`
	PersonID       string     `json:"person_id"`
	Action         string     `json:"action"`
	RequestID      string     `json:"request_id"`
	SourceIPMasked string     `json:"source_ip_masked"`
	ResultCode     string     `json:"result_code"`
	HTTPStatus     int        `json:"http_status"`
	OccurredAt     time.Time  `json:"occurred_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

type UsagePage struct {
	Items      []Usage `json:"items"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

type PurgeResult struct {
	RunID        string    `json:"run_id"`
	Cutoff       time.Time `json:"cutoff"`
	DeletedCount int64     `json:"deleted_count"`
	OperatorID   string    `json:"operator_id"`
	OccurredAt   time.Time `json:"occurred_at"`
}

type Authentication struct {
	Credential      string    `json:"credential"`
	OwnerCredential string    `json:"owner_credential,omitempty"`
	OwnerAudience   string    `json:"owner_audience,omitempty"`
	ExpiresAt       time.Time `json:"expires_at"`
	TenantID        string    `json:"tenant_id"`
	SubjectID       string    `json:"subject_id"`
	PersonID        string    `json:"person_id"`
	KeyID           string    `json:"key_id"`
	KeyPrefix       string    `json:"key_prefix"`
	RequestID       string    `json:"request_id"`
}

type ownerCapabilityClaims struct {
	identity.CapabilityClaims
	Person PersonReference `json:"person"`
}

type cursor struct {
	OccurredAt time.Time `json:"occurred_at"`
	ID         string    `json:"id"`
}

type Service struct {
	pool       *pgxpool.Pool
	signer     identity.Signer
	people     PersonResolver
	issuer     string
	now        func() time.Time
	newID      func() (string, error)
	bcryptCost int
}

func New(pool *pgxpool.Pool, signer identity.Signer, people PersonResolver, issuer string) (*Service, error) {
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	if pool == nil || signer == nil || people == nil || issuer == "" {
		return nil, fmt.Errorf("API key database, signer and Person resolver are required")
	}
	return &Service{pool: pool, signer: signer, people: people, issuer: issuer, now: func() time.Time { return time.Now().UTC() }, newID: randomUUID, bcryptCost: bcrypt.DefaultCost}, nil
}

func (s *Service) Create(ctx context.Context, actor identity.Principal, label string, ttlDays int) (CreatedKey, error) {
	label = strings.TrimSpace(label)
	if !validActor(actor) || label == "" || len([]rune(label)) > 80 || !allowedTTL(ttlDays) {
		return CreatedKey{}, problem(KindValidation, "invalid_api_key_request", "API Key 名称或有效期无效", nil)
	}
	if err := s.requireGrant(ctx, actor.TenantID, actor.SubjectID, ActionKeyManage); err != nil {
		return CreatedKey{}, err
	}
	if err := s.requireGrant(ctx, actor.TenantID, actor.SubjectID, ActionUse); err != nil {
		return CreatedKey{}, err
	}
	person, err := s.people.ResolveBySubject(ctx, actor.TenantID, actor.SubjectID)
	if err != nil {
		return CreatedKey{}, problem(KindDependency, "person_reference_unavailable", "无法确认当前账号关联的成员", err)
	}
	if person.TenantID != actor.TenantID || !uuidPattern.MatchString(person.ID) || person.Revision < 1 || person.Status != "active" {
		return CreatedKey{}, problem(KindForbidden, "person_mapping_required", "当前账号未关联有效成员", nil)
	}
	secret, prefix, err := newSecret()
	if err != nil {
		return CreatedKey{}, err
	}
	verifier, err := bcrypt.GenerateFromPassword([]byte(secret), s.bcryptCost)
	if err != nil {
		return CreatedKey{}, err
	}
	id, eventID, err := twoIDs(s.newID)
	if err != nil {
		return CreatedKey{}, err
	}
	now := s.now()
	value := Key{ID: id, TenantID: actor.TenantID, SubjectID: actor.SubjectID, PersonID: person.ID, PersonRevision: person.Revision, Label: label, Prefix: prefix, Status: "active", CreatedAt: now, ExpiresAt: now.AddDate(0, 0, ttlDays)}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CreatedKey{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "open-api-key:"+actor.TenantID+":"+actor.SubjectID); err != nil {
		return CreatedKey{}, err
	}
	var active int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM api_keys WHERE tenant_id=$1 AND subject_id=$2 AND status='active' AND expires_at>$3`, actor.TenantID, actor.SubjectID, now).Scan(&active); err != nil {
		return CreatedKey{}, err
	}
	if active >= 5 {
		return CreatedKey{}, problem(KindConflict, "active_api_key_limit", "最多可同时持有 5 个有效 API Key", nil)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO api_keys(id,tenant_id,subject_id,person_id,person_revision,label,prefix,secret_verifier,status,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'active',$9,$10)`, value.ID, value.TenantID, value.SubjectID, value.PersonID, value.PersonRevision, value.Label, value.Prefix, string(verifier), value.CreatedAt, value.ExpiresAt); err != nil {
		return CreatedKey{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO api_key_events(id,tenant_id,api_key_id,event_kind,actor_subject_id,occurred_at,details) VALUES($1,$2,$3,'created',$4,$5,jsonb_build_object('ttl_days',$6::integer,'prefix',$7::text))`, eventID, value.TenantID, value.ID, actor.SubjectID, now, ttlDays, value.Prefix); err != nil {
		return CreatedKey{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CreatedKey{}, err
	}
	return CreatedKey{Key: value, Secret: secret}, nil
}

func (s *Service) List(ctx context.Context, actor identity.Principal) ([]Key, error) {
	if !validActor(actor) {
		return nil, problem(KindUnauthorized, "session_required", "需要有效登录会话", nil)
	}
	if err := s.requireGrant(ctx, actor.TenantID, actor.SubjectID, ActionKeyManage); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text,tenant_id::text,subject_id::text,person_id::text,person_revision,label,prefix,status,created_at,expires_at,last_used_at,revoked_at FROM api_keys WHERE tenant_id=$1 AND subject_id=$2 ORDER BY created_at DESC,id DESC`, actor.TenantID, actor.SubjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]Key, 0)
	for rows.Next() {
		value, err := scanKey(rows)
		if err != nil {
			return nil, err
		}
		if value.Status == "active" && !value.ExpiresAt.After(s.now()) {
			value.Status = "expired"
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Service) Revoke(ctx context.Context, actor identity.Principal, keyID string) (Key, error) {
	if !validActor(actor) || !uuidPattern.MatchString(keyID) {
		return Key{}, problem(KindValidation, "invalid_api_key_id", "API Key 标识无效", nil)
	}
	if err := s.requireGrant(ctx, actor.TenantID, actor.SubjectID, ActionKeyManage); err != nil {
		return Key{}, err
	}
	eventID, err := s.newID()
	if err != nil {
		return Key{}, err
	}
	now := s.now()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Key{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	row := tx.QueryRow(ctx, `UPDATE api_keys SET status='revoked',revoked_at=$4,revoked_by=$3 WHERE tenant_id=$1 AND subject_id=$2 AND id=$5 AND status='active' RETURNING id::text,tenant_id::text,subject_id::text,person_id::text,person_revision,label,prefix,status,created_at,expires_at,last_used_at,revoked_at`, actor.TenantID, actor.SubjectID, actor.SubjectID, now, keyID)
	value, err := scanKey(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Key{}, problem(KindNotFound, "active_api_key_not_found", "有效 API Key 不存在", nil)
	}
	if err != nil {
		return Key{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO api_key_events(id,tenant_id,api_key_id,event_kind,actor_subject_id,occurred_at,details) VALUES($1,$2,$3,'revoked',$4,$5,jsonb_build_object('prefix',$6::text))`, eventID, value.TenantID, value.ID, actor.SubjectID, now, value.Prefix); err != nil {
		return Key{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Key{}, err
	}
	return value, nil
}

func (s *Service) Authenticate(ctx context.Context, secret, requestID, action, sourceIPMasked string) (Authentication, error) {
	requestID, action, sourceIPMasked = strings.TrimSpace(requestID), strings.TrimSpace(action), strings.TrimSpace(sourceIPMasked)
	matches := secretPattern.FindStringSubmatch(strings.TrimSpace(secret))
	if len(matches) != 3 || len(requestID) < 8 || len(requestID) > 128 || !allowedRequestAction(action) || sourceIPMasked == "" || len(sourceIPMasked) > 80 {
		return Authentication{}, problem(KindUnauthorized, "invalid_api_key", "API Key 无效或不可用", nil)
	}
	var value Key
	var verifier string
	err := s.pool.QueryRow(ctx, `SELECT id::text,tenant_id::text,subject_id::text,person_id::text,person_revision,label,prefix,status,created_at,expires_at,last_used_at,revoked_at,secret_verifier FROM api_keys WHERE prefix=$1`, matches[1]).Scan(&value.ID, &value.TenantID, &value.SubjectID, &value.PersonID, &value.PersonRevision, &value.Label, &value.Prefix, &value.Status, &value.CreatedAt, &value.ExpiresAt, &value.LastUsedAt, &value.RevokedAt, &verifier)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && bcrypt.CompareHashAndPassword([]byte(verifier), []byte(secret)) != nil) {
		return Authentication{}, problem(KindUnauthorized, "invalid_api_key", "API Key 无效或不可用", nil)
	}
	if err != nil {
		return Authentication{}, err
	}
	now := s.now()
	if value.Status != "active" || !value.ExpiresAt.After(now) {
		return Authentication{}, problem(KindUnauthorized, "invalid_api_key", "API Key 无效或不可用", nil)
	}
	if err := s.requireGrant(ctx, value.TenantID, value.SubjectID, ActionUse); err != nil {
		return Authentication{}, problem(KindUnauthorized, "invalid_api_key", "API Key 无效或不可用", err)
	}
	person, err := s.people.ResolveBySubject(ctx, value.TenantID, value.SubjectID)
	if err != nil {
		return Authentication{}, problem(KindDependency, "person_reference_unavailable", "成员关联暂时无法验证", err)
	}
	if person.TenantID != value.TenantID || person.ID != value.PersonID || person.Status != "active" || person.Revision < 1 {
		return Authentication{}, problem(KindUnauthorized, "invalid_api_key", "API Key 无效或不可用", nil)
	}
	usageID, credentialID, err := twoIDs(s.newID)
	if err != nil {
		return Authentication{}, err
	}
	expires := now.Add(5 * time.Minute)
	if value.ExpiresAt.Before(expires) {
		expires = value.ExpiresAt
	}
	claims := identity.CapabilityClaims{Version: "1", Issuer: s.issuer, Subject: value.SubjectID, Tenant: value.TenantID, Audience: "open-api", Name: "API Key " + value.Prefix, Actions: []string{ActionUse}, Scope: identity.Scope{Kind: "resources", Resources: []identity.ResourceRef{{Type: "api_key", ID: value.ID}}}, IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: expires.Unix(), AuthTime: now.Unix(), ID: credentialID}
	credential, err := s.signer.Sign(claims)
	if err != nil {
		return Authentication{}, err
	}
	ownerAudience, ownerActions := ownerAccess(action)
	ownerCredential := ""
	if ownerAudience != "" {
		ownerID, err := s.newID()
		if err != nil {
			return Authentication{}, err
		}
		baseOwnerClaims := identity.CapabilityClaims{
			Version: "1", Issuer: s.issuer, Subject: value.SubjectID, Tenant: value.TenantID,
			Audience: ownerAudience, Name: person.DisplayName, Actions: ownerActions,
			Scope: identity.Scope{Kind: "tenant"}, IssuedAt: now.Unix(), NotBefore: now.Unix(),
			ExpiresAt: expires.Unix(), AuthTime: now.Unix(), ID: ownerID,
		}
		claimsToSign := any(baseOwnerClaims)
		if ownerAudience == "internal-finance-requests" {
			claimsToSign = ownerCapabilityClaims{CapabilityClaims: baseOwnerClaims, Person: person}
		}
		ownerCredential, err = s.signer.Sign(claimsToSign)
		if err != nil {
			return Authentication{}, err
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Authentication{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `UPDATE api_keys SET last_used_at=$2 WHERE id=$1 AND status='active' AND expires_at>$2`, value.ID, now); err != nil {
		return Authentication{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO open_api_usage_records(id,tenant_id,subject_id,person_id,api_key_id,key_prefix,action,request_id,source_ip_masked,result_code,http_status,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'authenticated',0,$10)`, usageID, value.TenantID, value.SubjectID, value.PersonID, value.ID, value.Prefix, action, requestID, sourceIPMasked, now); err != nil {
		return Authentication{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Authentication{}, err
	}
	return Authentication{Credential: credential, OwnerCredential: ownerCredential, OwnerAudience: ownerAudience, ExpiresAt: expires, TenantID: value.TenantID, SubjectID: value.SubjectID, PersonID: value.PersonID, KeyID: value.ID, KeyPrefix: value.Prefix, RequestID: requestID}, nil
}

func (s *Service) CompleteUsage(ctx context.Context, tenantID, subjectID, keyID, requestID, resultCode string, httpStatus int) error {
	if !uuidPattern.MatchString(tenantID) || !uuidPattern.MatchString(subjectID) || !uuidPattern.MatchString(keyID) || len(strings.TrimSpace(requestID)) < 8 || strings.TrimSpace(resultCode) == "" || httpStatus < 100 || httpStatus > 599 {
		return problem(KindValidation, "invalid_usage_result", "调用结果无效", nil)
	}
	result, err := s.pool.Exec(ctx, `UPDATE open_api_usage_records SET result_code=$5,http_status=$6,completed_at=$7 WHERE tenant_id=$1 AND subject_id=$2 AND api_key_id=$3 AND request_id=$4 AND completed_at IS NULL`, tenantID, subjectID, keyID, requestID, strings.TrimSpace(resultCode), httpStatus, s.now())
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return problem(KindNotFound, "usage_record_not_found", "调用记录不存在或已经完成", nil)
	}
	return nil
}

func (s *Service) ListUsage(ctx context.Context, actor identity.Principal, tenantWide bool, rawCursor string, limit int) (UsagePage, error) {
	if !validActor(actor) {
		return UsagePage{}, problem(KindUnauthorized, "session_required", "需要有效登录会话", nil)
	}
	action := ActionSelfAudit
	if tenantWide {
		action = ActionTenantAudit
	}
	if err := s.requireGrant(ctx, actor.TenantID, actor.SubjectID, action); err != nil {
		return UsagePage{}, err
	}
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return UsagePage{}, problem(KindValidation, "invalid_page_size", "每页数量必须在 1 到 100 之间", nil)
	}
	position, err := decodeCursor(rawCursor)
	if err != nil {
		return UsagePage{}, problem(KindValidation, "invalid_cursor", "分页游标无效", err)
	}
	args := []any{actor.TenantID, limit + 1}
	where := "tenant_id=$1"
	if !tenantWide {
		args = append(args, actor.SubjectID)
		where += fmt.Sprintf(" AND subject_id=$%d", len(args))
	}
	if !position.OccurredAt.IsZero() {
		args = append(args, position.OccurredAt, position.ID)
		where += fmt.Sprintf(" AND (occurred_at,id)<($%d,$%d::uuid)", len(args)-1, len(args))
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text,api_key_id::text,key_prefix,subject_id::text,person_id::text,action,request_id,source_ip_masked,result_code,http_status,occurred_at,completed_at FROM open_api_usage_records WHERE `+where+` ORDER BY occurred_at DESC,id DESC LIMIT $2`, args...)
	if err != nil {
		return UsagePage{}, err
	}
	defer rows.Close()
	items := make([]Usage, 0, limit+1)
	for rows.Next() {
		var value Usage
		if err := rows.Scan(&value.ID, &value.KeyID, &value.KeyPrefix, &value.SubjectID, &value.PersonID, &value.Action, &value.RequestID, &value.SourceIPMasked, &value.ResultCode, &value.HTTPStatus, &value.OccurredAt, &value.CompletedAt); err != nil {
			return UsagePage{}, err
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return UsagePage{}, err
	}
	page := UsagePage{Items: items}
	if len(items) > limit {
		last := items[limit-1]
		page.Items = items[:limit]
		page.NextCursor, err = encodeCursor(cursor{OccurredAt: last.OccurredAt, ID: last.ID})
		if err != nil {
			return UsagePage{}, err
		}
	}
	return page, nil
}

// PurgeUsageRecords enforces the fixed 180-day metadata retention window. The
// usage rows contain no request bodies or credentials, and every cleanup is
// itself preserved as an immutable operational audit record.
func PurgeUsageRecords(ctx context.Context, pool *pgxpool.Pool, now time.Time, operatorID string) (PurgeResult, error) {
	operatorID = strings.TrimSpace(operatorID)
	if pool == nil || operatorID == "" || len([]rune(operatorID)) > 128 {
		return PurgeResult{}, problem(KindValidation, "invalid_retention_operator", "调用记录清理操作人无效", nil)
	}
	now = now.UTC()
	cutoff := now.AddDate(0, 0, -180)
	runID, err := randomUUID()
	if err != nil {
		return PurgeResult{}, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return PurgeResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	result, err := tx.Exec(ctx, `DELETE FROM open_api_usage_records WHERE occurred_at<$1`, cutoff)
	if err != nil {
		return PurgeResult{}, err
	}
	value := PurgeResult{RunID: runID, Cutoff: cutoff, DeletedCount: result.RowsAffected(), OperatorID: operatorID, OccurredAt: now}
	if _, err := tx.Exec(ctx, `INSERT INTO open_api_usage_cleanup_runs(id,cutoff,deleted_count,operator_id,occurred_at) VALUES($1,$2,$3,$4,$5)`, value.RunID, value.Cutoff, value.DeletedCount, value.OperatorID, value.OccurredAt); err != nil {
		return PurgeResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PurgeResult{}, err
	}
	return value, nil
}

func (s *Service) requireGrant(ctx context.Context, tenantID, subjectID, action string) error {
	var allowed bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (
SELECT 1 FROM subjects s JOIN capability_grants g ON g.subject_id=s.id
WHERE s.tenant_id=$1 AND s.id=$2 AND s.status='active' AND g.audience='open-api' AND g.action=$3 AND g.scope_kind='tenant' AND g.revoked_at IS NULL
)`, tenantID, subjectID, action).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return problem(KindForbidden, "open_api_capability_denied", "当前账号没有开放 API 权限", nil)
	}
	return nil
}

func scanKey(row interface{ Scan(...any) error }) (Key, error) {
	var value Key
	err := row.Scan(&value.ID, &value.TenantID, &value.SubjectID, &value.PersonID, &value.PersonRevision, &value.Label, &value.Prefix, &value.Status, &value.CreatedAt, &value.ExpiresAt, &value.LastUsedAt, &value.RevokedAt)
	return value, err
}

func validActor(actor identity.Principal) bool {
	return uuidPattern.MatchString(actor.TenantID) && uuidPattern.MatchString(actor.SubjectID)
}

func allowedTTL(days int) bool { return days == 30 || days == 90 || days == 180 || days == 365 }

func allowedRequestAction(action string) bool {
	switch action {
	case "open-api.profile.read", "open-api.settings.read",
		"open-api.evidence.upload.create", "open-api.evidence.upload.complete", "open-api.evidence.upload.abort",
		"open-api.reimbursement.create", "open-api.reimbursement.update", "open-api.reimbursement.submit",
		"open-api.reimbursement.withdraw", "open-api.reimbursement.restore", "open-api.reimbursement.delete",
		"open-api.reimbursement.read", "open-api.reimbursement.list":
		return true
	default:
		return false
	}
}

func ownerAccess(action string) (string, []string) {
	switch {
	case strings.HasPrefix(action, "open-api.evidence.upload."):
		return "evidence-service", []string{"evidence-service.upload"}
	case strings.HasPrefix(action, "open-api.reimbursement."):
		if action == "open-api.reimbursement.read" || action == "open-api.reimbursement.list" {
			return "internal-finance-requests", []string{"internal-finance-requests.self.read"}
		}
		return "internal-finance-requests", []string{"internal-finance-requests.self.manage", "internal-finance-requests.self.read"}
	default:
		return "", nil
	}
}

func newSecret() (string, string, error) {
	prefixBytes := make([]byte, 8)
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(prefixBytes); err != nil {
		return "", "", err
	}
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", err
	}
	prefix := "bfk_" + base64.RawURLEncoding.EncodeToString(prefixBytes)[:10]
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(secretBytes), prefix, nil
}

func randomUUID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}

func twoIDs(newID func() (string, error)) (string, string, error) {
	left, err := newID()
	if err != nil {
		return "", "", err
	}
	right, err := newID()
	return left, right, err
}

func problem(kind ErrorKind, code, message string, err error) error {
	return &DomainError{Kind: kind, Code: code, Message: message, Err: err}
}

func decodeCursor(raw string) (cursor, error) {
	if strings.TrimSpace(raw) == "" {
		return cursor{}, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return cursor{}, err
	}
	var value cursor
	if err := json.Unmarshal(data, &value); err != nil || value.OccurredAt.IsZero() || !uuidPattern.MatchString(value.ID) {
		return cursor{}, fmt.Errorf("invalid cursor")
	}
	return value, nil
}

func encodeCursor(value cursor) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
