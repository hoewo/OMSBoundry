package evidence

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/evidence-service/internal/storage"
)

var (
	shaPattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

type Service struct {
	pool           *pgxpool.Pool
	provider       storage.Provider
	uploadTTL      time.Duration
	maxUploadBytes int64
	now            func() time.Time
}

func NewService(pool *pgxpool.Pool, provider storage.Provider, uploadTTL time.Duration, maxUploadBytes int64) (*Service, error) {
	if pool == nil || provider == nil || uploadTTL <= 0 || maxUploadBytes <= 0 {
		return nil, fmt.Errorf("evidence service configuration is invalid")
	}
	return &Service{pool: pool, provider: provider, uploadTTL: uploadTTL, maxUploadBytes: maxUploadBytes, now: func() time.Time { return time.Now().UTC() }}, nil
}
func (s *Service) Ping(ctx context.Context) error {
	if err := s.pool.Ping(ctx); err != nil {
		return err
	}
	return s.provider.Ready(ctx)
}

func (s *Service) ListVersions(ctx context.Context, tenantID string, limit int) (VersionPage, error) {
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return VersionPage{}, ErrInvalid
	}
	rows, err := s.pool.Query(ctx, `SELECT file_id,version,status,file_name,size_bytes,media_type,sha256,created_at FROM file_versions WHERE tenant_id=$1 ORDER BY created_at DESC,file_id DESC LIMIT $2`, tenantID, limit)
	if err != nil {
		return VersionPage{}, err
	}
	defer rows.Close()
	page := VersionPage{Items: make([]Version, 0, limit)}
	for rows.Next() {
		var item Version
		if err := rows.Scan(&item.FileID, &item.Version, &item.Status, &item.FileName, &item.SizeBytes, &item.MediaType, &item.SHA256, &item.CreatedAt); err != nil {
			return VersionPage{}, err
		}
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		return VersionPage{}, err
	}
	return page, nil
}

func (s *Service) CreateUploadSession(ctx context.Context, actor Actor, key string, input CreateUploadInput) (UploadSession, error) {
	input = normalizeCreate(input)
	if err := validateCreate(key, input, s.maxUploadBytes); err != nil {
		return UploadSession{}, err
	}
	fingerprint, err := fingerprint(input)
	if err != nil {
		return UploadSession{}, err
	}
	now := s.now()
	sessionID, err := newUUID()
	if err != nil {
		return UploadSession{}, err
	}
	fileID, err := newUUID()
	if err != nil {
		return UploadSession{}, err
	}
	auditID, err := newUUID()
	if err != nil {
		return UploadSession{}, err
	}
	objectKey := fmt.Sprintf("tenants/%s/files/%s/versions/1/%s", actor.TenantID, fileID, sessionID)
	expires := now.Add(s.uploadTTL)
	clientContext, _ := json.Marshal(input.ClientContext)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return UploadSession{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, actor.TenantID+":"+actor.SubjectID+":upload.create:"+key); err != nil {
		return UploadSession{}, err
	}
	var existingHash []byte
	var existingSession string
	err = tx.QueryRow(ctx, `SELECT payload_hash, resource_id::text FROM idempotency_commands WHERE tenant_id=$1 AND subject_id=$2 AND action='upload.create' AND idempotency_key=$3 FOR UPDATE`, actor.TenantID, actor.SubjectID, key).Scan(&existingHash, &existingSession)
	if err == nil {
		if !equalHash(existingHash, fingerprint) {
			return UploadSession{}, ErrIdempotencyConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return UploadSession{}, err
		}
		return s.authorizedSession(ctx, actor.TenantID, existingSession)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return UploadSession{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO files(tenant_id,id,created_at,created_by_subject) VALUES($1,$2,$3,$4)`, actor.TenantID, fileID, now, actor.SubjectID)
	if err != nil {
		return UploadSession{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO upload_sessions(tenant_id,id,file_id,version,provider,bucket,object_key,file_name,expected_size,declared_media_type,declared_sha256,client_context,status,expires_at,created_at,updated_at,created_by_subject,created_by_name) VALUES($1,$2,$3,1,$4,$5,$6,$7,$8,$9,NULLIF($10,''),$11,'pending',$12,$13,$13,$14,$15)`, actor.TenantID, sessionID, fileID, s.provider.Name(), providerBucket(s.provider), objectKey, input.FileName, input.SizeBytes, input.MediaType, input.SHA256, clientContext, expires, now, actor.SubjectID, actor.DisplayName)
	if err != nil {
		return UploadSession{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO idempotency_commands(tenant_id,subject_id,action,idempotency_key,payload_hash,resource_id,created_at) VALUES($1,$2,'upload.create',$3,$4,$5,$6)`, actor.TenantID, actor.SubjectID, key, fingerprint, sessionID, now)
	if err != nil {
		return UploadSession{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO evidence_audit_entries(id,tenant_id,action,object_type,object_id,actor_kind,actor_id,actor_display_name,outcome,provider,occurred_at,details) VALUES($1,$2,'upload_session.created','upload_session',$3,$4,$5,$6,'succeeded',$7,$8,jsonb_build_object('file_id',$9::text,'size_bytes',$10::bigint))`, auditID, actor.TenantID, sessionID, actor.Kind, actor.SubjectID, actor.DisplayName, s.provider.Name(), now, fileID, input.SizeBytes)
	if err != nil {
		return UploadSession{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return UploadSession{}, err
	}
	return s.authorizedSession(ctx, actor.TenantID, sessionID)
}

func (s *Service) CreateVersionUploadSession(ctx context.Context, actor Actor, fileID, key string, input CreateUploadInput) (UploadSession, error) {
	fileID = strings.ToLower(strings.TrimSpace(fileID))
	input = normalizeCreate(input)
	if !uuidPattern.MatchString(fileID) {
		return UploadSession{}, ErrInvalid
	}
	if err := validateCreate(key, input, s.maxUploadBytes); err != nil {
		return UploadSession{}, err
	}
	fingerprint, err := fingerprint(struct {
		FileID string            `json:"file_id"`
		Input  CreateUploadInput `json:"input"`
	}{FileID: fileID, Input: input})
	if err != nil {
		return UploadSession{}, err
	}
	now := s.now()
	sessionID, err := newUUID()
	if err != nil {
		return UploadSession{}, err
	}
	auditID, err := newUUID()
	if err != nil {
		return UploadSession{}, err
	}
	expires := now.Add(s.uploadTTL)
	clientContext, _ := json.Marshal(input.ClientContext)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return UploadSession{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, actor.TenantID+":"+actor.SubjectID+":upload.version.create:"+key); err != nil {
		return UploadSession{}, err
	}
	var existingHash []byte
	var existingSession string
	err = tx.QueryRow(ctx, `SELECT payload_hash,resource_id::text FROM idempotency_commands WHERE tenant_id=$1 AND subject_id=$2 AND action='upload.version.create' AND idempotency_key=$3 FOR UPDATE`, actor.TenantID, actor.SubjectID, key).Scan(&existingHash, &existingSession)
	if err == nil {
		if !equalHash(existingHash, fingerprint) {
			return UploadSession{}, ErrIdempotencyConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return UploadSession{}, err
		}
		return s.authorizedSession(ctx, actor.TenantID, existingSession)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return UploadSession{}, err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, actor.TenantID+":"+fileID+":upload.version.allocate"); err != nil {
		return UploadSession{}, err
	}
	var latestVersion int
	err = tx.QueryRow(ctx, `SELECT COALESCE(max(version),0) FROM file_versions WHERE tenant_id=$1 AND file_id=$2`, actor.TenantID, fileID).Scan(&latestVersion)
	if err != nil {
		return UploadSession{}, err
	}
	if latestVersion < 1 {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM files WHERE tenant_id=$1 AND id=$2)`, actor.TenantID, fileID).Scan(&exists); err != nil {
			return UploadSession{}, err
		}
		if !exists {
			return UploadSession{}, ErrNotFound
		}
		return UploadSession{}, ErrConflict
	}
	var active bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM upload_sessions WHERE tenant_id=$1 AND file_id=$2 AND version>$3 AND status IN ('pending','completing','cleanup_pending'))`, actor.TenantID, fileID, latestVersion).Scan(&active); err != nil {
		return UploadSession{}, err
	}
	if active {
		return UploadSession{}, ErrConflict
	}
	var allocatedVersion int
	if err := tx.QueryRow(ctx, `SELECT GREATEST($3,COALESCE(max(version),0))+1 FROM upload_sessions WHERE tenant_id=$1 AND file_id=$2`, actor.TenantID, fileID, latestVersion).Scan(&allocatedVersion); err != nil {
		return UploadSession{}, err
	}
	objectKey := fmt.Sprintf("tenants/%s/files/%s/versions/%d/%s", actor.TenantID, fileID, allocatedVersion, sessionID)
	_, err = tx.Exec(ctx, `INSERT INTO upload_sessions(tenant_id,id,file_id,version,provider,bucket,object_key,file_name,expected_size,declared_media_type,declared_sha256,client_context,status,expires_at,created_at,updated_at,created_by_subject,created_by_name) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NULLIF($11,''),$12,'pending',$13,$14,$14,$15,$16)`, actor.TenantID, sessionID, fileID, allocatedVersion, s.provider.Name(), providerBucket(s.provider), objectKey, input.FileName, input.SizeBytes, input.MediaType, input.SHA256, clientContext, expires, now, actor.SubjectID, actor.DisplayName)
	if err != nil {
		return UploadSession{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO idempotency_commands(tenant_id,subject_id,action,idempotency_key,payload_hash,resource_id,created_at) VALUES($1,$2,'upload.version.create',$3,$4,$5,$6)`, actor.TenantID, actor.SubjectID, key, fingerprint, sessionID, now)
	if err != nil {
		return UploadSession{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO evidence_audit_entries(id,tenant_id,action,object_type,object_id,actor_kind,actor_id,actor_display_name,outcome,provider,occurred_at,details) VALUES($1,$2,'file_version.upload_session.created','upload_session',$3,$4,$5,$6,'succeeded',$7,$8,jsonb_build_object('file_id',$9::text,'version',$10::integer,'size_bytes',$11::bigint))`, auditID, actor.TenantID, sessionID, actor.Kind, actor.SubjectID, actor.DisplayName, s.provider.Name(), now, fileID, allocatedVersion, input.SizeBytes)
	if err != nil {
		return UploadSession{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return UploadSession{}, err
	}
	return s.authorizedSession(ctx, actor.TenantID, sessionID)
}

func (s *Service) authorizedSession(ctx context.Context, tenantID, sessionID string) (UploadSession, error) {
	intent, status, err := s.loadUploadIntent(ctx, tenantID, sessionID, false)
	if err != nil {
		return UploadSession{}, err
	}
	if status != "pending" {
		return UploadSession{}, ErrConflict
	}
	if !intent.ExpiresAt.After(s.now()) {
		_, _ = s.pool.Exec(ctx, `UPDATE upload_sessions SET status='expired',updated_at=$3 WHERE tenant_id=$1 AND id=$2 AND status='pending'`, tenantID, sessionID, s.now())
		return UploadSession{}, ErrExpired
	}
	authorization, err := s.provider.AuthorizeUpload(ctx, intent)
	if err != nil {
		return UploadSession{}, err
	}
	return UploadSession{SessionID: intent.SessionID, FileID: intent.FileID, Version: intent.Version, Status: status, ExpiresAt: intent.ExpiresAt, Authorization: authorization}, nil
}

func (s *Service) CompleteUpload(ctx context.Context, actor Actor, sessionID, key string, input CompleteUploadInput) (Version, error) {
	input = normalizeComplete(input)
	if err := validateComplete(sessionID, key, input); err != nil {
		return Version{}, err
	}
	fp, err := fingerprint(input)
	if err != nil {
		return Version{}, err
	}
	if version, replayed, err := s.completedCommand(ctx, actor, "upload.complete", sessionID, key, fp); err != nil || replayed {
		return version, err
	}
	if version, completed, err := s.recordCompletedSessionReplay(ctx, actor, sessionID, key, fp); err != nil || completed {
		return version, err
	}
	intent, err := s.claimCompletion(ctx, actor, sessionID)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			if version, completed, replayErr := s.recordCompletedSessionReplay(ctx, actor, sessionID, key, fp); replayErr != nil || completed {
				return version, replayErr
			}
		}
		return Version{}, err
	}
	actualSize, actualType, actualSHA, inspectErr := s.inspect(ctx, intent)
	if inspectErr != nil || actualSize != intent.ExpectedSize || input.ObservedSizeBytes != actualSize || (input.ObservedSHA256 != "" && input.ObservedSHA256 != actualSHA) {
		message := "object verification failed"
		if inspectErr != nil {
			message = inspectErr.Error()
		}
		_ = s.markCompletionFailed(ctx, actor, intent, message)
		if inspectErr != nil {
			return Version{}, inspectErr
		}
		return Version{}, ErrConflict
	}
	declared := strings.ToLower(strings.TrimSpace(intent.ExpectedSHA256))
	if declared != "" && declared != actualSHA {
		_ = s.markCompletionFailed(ctx, actor, intent, "declared sha256 mismatch")
		return Version{}, ErrConflict
	}
	return s.finalizeCompletion(ctx, actor, intent, key, fp, actualSize, actualType, actualSHA)
}

func (s *Service) completedCommand(ctx context.Context, actor Actor, action, resourceID, key string, fp []byte) (Version, bool, error) {
	var storedHash []byte
	var raw []byte
	var storedResource string
	err := s.pool.QueryRow(ctx, `SELECT payload_hash, resource_id::text, result_json FROM idempotency_commands WHERE tenant_id=$1 AND subject_id=$2 AND action=$3 AND idempotency_key=$4`, actor.TenantID, actor.SubjectID, action, key).Scan(&storedHash, &storedResource, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Version{}, false, nil
	}
	if err != nil {
		return Version{}, false, err
	}
	if storedResource != resourceID || !equalHash(storedHash, fp) {
		return Version{}, true, ErrIdempotencyConflict
	}
	var version Version
	if err := json.Unmarshal(raw, &version); err != nil {
		return Version{}, true, err
	}
	return version, true, nil
}

func (s *Service) recordCompletedSessionReplay(ctx context.Context, actor Actor, sessionID, key string, fp []byte) (Version, bool, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Version{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, actor.TenantID+":"+actor.SubjectID+":upload.complete:"+key); err != nil {
		return Version{}, false, err
	}
	var storedHash []byte
	var storedResource string
	var storedResult []byte
	err = tx.QueryRow(ctx, `SELECT payload_hash,resource_id::text,result_json FROM idempotency_commands WHERE tenant_id=$1 AND subject_id=$2 AND action='upload.complete' AND idempotency_key=$3 FOR UPDATE`, actor.TenantID, actor.SubjectID, key).Scan(&storedHash, &storedResource, &storedResult)
	if err == nil {
		if storedResource != sessionID || !equalHash(storedHash, fp) {
			return Version{}, true, ErrIdempotencyConflict
		}
		var value Version
		if err := json.Unmarshal(storedResult, &value); err != nil {
			return Version{}, true, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Version{}, true, err
		}
		return value, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Version{}, false, err
	}
	var status string
	var fileID string
	var versionNumber int
	err = tx.QueryRow(ctx, `SELECT status,file_id::text,version FROM upload_sessions WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, actor.TenantID, sessionID).Scan(&status, &fileID, &versionNumber)
	if errors.Is(err, pgx.ErrNoRows) {
		return Version{}, false, ErrNotFound
	}
	if err != nil {
		return Version{}, false, err
	}
	if status != "completed" {
		if err := tx.Commit(ctx); err != nil {
			return Version{}, false, err
		}
		return Version{}, false, nil
	}
	var value Version
	var provider, bucket, objectKey string
	err = tx.QueryRow(ctx, `SELECT file_id::text,version,status,file_name,size_bytes,media_type,sha256,created_at,provider,bucket,object_key FROM file_versions WHERE tenant_id=$1 AND file_id=$2 AND version=$3`, actor.TenantID, fileID, versionNumber).Scan(&value.FileID, &value.Version, &value.Status, &value.FileName, &value.SizeBytes, &value.MediaType, &value.SHA256, &value.CreatedAt, &provider, &bucket, &objectKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return Version{}, false, fmt.Errorf("completed upload session has no file version")
	}
	if err != nil {
		return Version{}, false, err
	}
	value.Object = storage.ObjectRef{TenantID: actor.TenantID, Provider: provider, Bucket: bucket, ObjectKey: objectKey}
	result, err := json.Marshal(value)
	if err != nil {
		return Version{}, true, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO idempotency_commands(tenant_id,subject_id,action,idempotency_key,payload_hash,resource_id,result_json,created_at) VALUES($1,$2,'upload.complete',$3,$4,$5,$6,$7)`, actor.TenantID, actor.SubjectID, key, fp, sessionID, result, s.now()); err != nil {
		return Version{}, true, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Version{}, true, err
	}
	return value, true, nil
}

func (s *Service) claimCompletion(ctx context.Context, actor Actor, sessionID string) (storage.UploadIntent, error) {
	now := s.now()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return storage.UploadIntent{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	intent, status, err := s.loadUploadIntentTx(ctx, tx, actor.TenantID, sessionID, false)
	if err != nil {
		return storage.UploadIntent{}, err
	}
	if status == "completed" {
		return storage.UploadIntent{}, ErrConflict
	}
	if status != "pending" {
		return storage.UploadIntent{}, ErrConflict
	}
	if !intent.ExpiresAt.After(now) {
		_, _ = tx.Exec(ctx, `UPDATE upload_sessions SET status='expired',updated_at=$3 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, sessionID, now)
		if err := tx.Commit(ctx); err != nil {
			return storage.UploadIntent{}, err
		}
		return storage.UploadIntent{}, ErrExpired
	}
	command, err := tx.Exec(ctx, `UPDATE upload_sessions SET status='completing',updated_at=$3 WHERE tenant_id=$1 AND id=$2 AND status='pending'`, actor.TenantID, sessionID, now)
	if err != nil {
		return storage.UploadIntent{}, err
	}
	if command.RowsAffected() != 1 {
		return storage.UploadIntent{}, ErrConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return storage.UploadIntent{}, err
	}
	return intent, nil
}

func (s *Service) inspect(ctx context.Context, intent storage.UploadIntent) (int64, string, string, error) {
	object, err := s.provider.Open(ctx, intent.Object)
	if err != nil {
		return 0, "", "", err
	}
	defer object.Reader.Close()
	hash := sha256.New()
	header := make([]byte, 512)
	count, readErr := io.ReadFull(object.Reader, header)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return 0, "", "", readErr
	}
	header = header[:count]
	if len(header) == 0 {
		return 0, "", "", fmt.Errorf("object is empty")
	}
	_, _ = hash.Write(header)
	total := int64(count)
	copied, err := io.Copy(hash, io.LimitReader(object.Reader, s.maxUploadBytes+1-total))
	if err != nil {
		return 0, "", "", err
	}
	total += copied
	if total > s.maxUploadBytes || total != object.Metadata.SizeBytes {
		return 0, "", "", fmt.Errorf("object size is inconsistent")
	}
	return total, http.DetectContentType(header), hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *Service) finalizeCompletion(ctx context.Context, actor Actor, intent storage.UploadIntent, key string, fp []byte, size int64, mediaType, digest string) (Version, error) {
	now := s.now()
	version := Version{FileID: intent.FileID, Version: intent.Version, Status: "available", FileName: intent.FileName, SizeBytes: size, MediaType: mediaType, SHA256: digest, CreatedAt: now, Object: intent.Object}
	result, _ := json.Marshal(version)
	auditID, _ := newUUID()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Version{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM upload_sessions WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, actor.TenantID, intent.SessionID).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
		return Version{}, ErrNotFound
	} else if err != nil {
		return Version{}, err
	}
	if status != "completing" {
		return Version{}, ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO file_versions(tenant_id,file_id,version,status,file_name,size_bytes,media_type,sha256,provider,bucket,object_key,created_at,created_by_subject,created_by_name) VALUES($1,$2,$3,'available',$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, actor.TenantID, intent.FileID, intent.Version, intent.FileName, size, mediaType, digest, intent.Object.Provider, intent.Object.Bucket, intent.Object.ObjectKey, now, actor.SubjectID, actor.DisplayName)
	if err != nil {
		return Version{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE upload_sessions SET status='completed',completed_at=$3,updated_at=$3 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, intent.SessionID, now)
	if err != nil {
		return Version{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO idempotency_commands(tenant_id,subject_id,action,idempotency_key,payload_hash,resource_id,result_json,created_at) VALUES($1,$2,'upload.complete',$3,$4,$5,$6,$7)`, actor.TenantID, actor.SubjectID, key, fp, intent.SessionID, result, now)
	if err != nil {
		return Version{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO evidence_audit_entries(id,tenant_id,action,object_type,object_id,actor_kind,actor_id,actor_display_name,outcome,provider,occurred_at,details) VALUES($1,$2,'upload_session.completed','file_version',$3,$4,$5,$6,'succeeded',$7,$8,jsonb_build_object('version',$9::integer,'size_bytes',$10::bigint,'sha256',$11::text))`, auditID, actor.TenantID, intent.FileID, actor.Kind, actor.SubjectID, actor.DisplayName, intent.Object.Provider, now, intent.Version, size, digest)
	if err != nil {
		return Version{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Version{}, err
	}
	return version, nil
}

func (s *Service) markCompletionFailed(ctx context.Context, actor Actor, intent storage.UploadIntent, message string) error {
	if len(message) > 2000 {
		message = message[:2000]
	}
	now := s.now()
	auditID, _ := newUUID()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = tx.Exec(ctx, `UPDATE upload_sessions SET status='cleanup_pending',updated_at=$3,last_cleanup_error=$4 WHERE tenant_id=$1 AND id=$2 AND status='completing'`, actor.TenantID, intent.SessionID, now, message)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO evidence_audit_entries(id,tenant_id,action,object_type,object_id,actor_kind,actor_id,actor_display_name,outcome,provider,occurred_at,details) VALUES($1,$2,'upload_session.complete_failed','upload_session',$3,$4,$5,$6,'failed',$7,$8,jsonb_build_object('error',$9::text))`, auditID, actor.TenantID, intent.SessionID, actor.Kind, actor.SubjectID, actor.DisplayName, intent.Object.Provider, now, message)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) AbortUpload(ctx context.Context, actor Actor, sessionID, key string, input AbortUploadInput) error {
	input.Reason = strings.TrimSpace(input.Reason)
	if !uuidPattern.MatchString(sessionID) || !validKey(key) || (input.Reason != "user_cancelled" && input.Reason != "transfer_failed" && input.Reason != "client_shutdown") {
		return ErrInvalid
	}
	fp, _ := fingerprint(input)
	now := s.now()
	auditID, _ := newUUID()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, actor.TenantID+":"+actor.SubjectID+":upload.abort:"+key); err != nil {
		return err
	}
	var stored []byte
	var storedResource string
	err = tx.QueryRow(ctx, `SELECT payload_hash,resource_id::text FROM idempotency_commands WHERE tenant_id=$1 AND subject_id=$2 AND action='upload.abort' AND idempotency_key=$3 FOR UPDATE`, actor.TenantID, actor.SubjectID, key).Scan(&stored, &storedResource)
	if err == nil {
		if storedResource != sessionID || !equalHash(stored, fp) {
			return ErrIdempotencyConflict
		}
		return tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM upload_sessions WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, actor.TenantID, sessionID).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if status == "completed" || status == "cleaned" {
		return ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE upload_sessions SET status='aborted',aborted_at=COALESCE(aborted_at,$3),abort_reason=CASE WHEN abort_reason='' THEN $4 ELSE abort_reason END,updated_at=$3 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, sessionID, now, input.Reason)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO idempotency_commands(tenant_id,subject_id,action,idempotency_key,payload_hash,resource_id,created_at) VALUES($1,$2,'upload.abort',$3,$4,$5,$6)`, actor.TenantID, actor.SubjectID, key, fp, sessionID, now)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO evidence_audit_entries(id,tenant_id,action,object_type,object_id,actor_kind,actor_id,actor_display_name,outcome,provider,occurred_at) SELECT $1,tenant_id,'upload_session.aborted','upload_session',id::text,$3,$4,$5,'succeeded',provider,$6 FROM upload_sessions WHERE tenant_id=$2 AND id=$7`, auditID, actor.TenantID, actor.Kind, actor.SubjectID, actor.DisplayName, now, sessionID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) CreateAccessGrant(ctx context.Context, actor Actor, fileID string, version int, input AccessInput) (AccessGrant, error) {
	input.Disposition = strings.ToLower(strings.TrimSpace(input.Disposition))
	input.DeliveryMode = strings.ToLower(strings.TrimSpace(input.DeliveryMode))
	input.FileName = strings.TrimSpace(input.FileName)
	if !uuidPattern.MatchString(fileID) || version < 1 || (input.Disposition != "inline" && input.Disposition != "attachment") || (input.DeliveryMode != "blob" && input.DeliveryMode != "direct") || len([]rune(input.FileName)) > 512 || strings.ContainsAny(input.FileName, "/\\") {
		return AccessGrant{}, ErrInvalid
	}
	value, err := s.GetVersion(ctx, actor.TenantID, fileID, version)
	if err != nil {
		return AccessGrant{}, err
	}
	fileName := value.FileName
	if input.FileName != "" {
		fileName = input.FileName
	}
	grant, err := s.provider.AuthorizeAccess(ctx, storage.AccessIntent{FileID: fileID, Version: version, FileName: fileName, Disposition: input.Disposition, Object: value.Object})
	if err != nil {
		return AccessGrant{}, err
	}
	auditID, _ := newUUID()
	_, err = s.pool.Exec(ctx, `INSERT INTO evidence_audit_entries(id,tenant_id,action,object_type,object_id,actor_kind,actor_id,actor_display_name,outcome,provider,occurred_at,details) VALUES($1,$2,'file_version.access_granted','file_version',$3,$4,$5,$6,'succeeded',$7,$8,jsonb_build_object('version',$9::integer,'disposition',$10::text,'delivery_mode',$11::text))`, auditID, actor.TenantID, fileID, actor.Kind, actor.SubjectID, actor.DisplayName, value.Object.Provider, s.now(), version, input.Disposition, input.DeliveryMode)
	if err != nil {
		return AccessGrant{}, err
	}
	return AccessGrant{FileID: fileID, Version: version, URL: grant.URL, Method: "GET", Disposition: input.Disposition, DeliveryMode: input.DeliveryMode, ExpiresAt: grant.ExpiresAt, RefreshAfter: grant.RefreshAfter}, nil
}

func (s *Service) GetVersion(ctx context.Context, tenantID, fileID string, version int) (Version, error) {
	if !uuidPattern.MatchString(strings.ToLower(strings.TrimSpace(tenantID))) || !uuidPattern.MatchString(strings.ToLower(strings.TrimSpace(fileID))) || version < 1 {
		return Version{}, ErrInvalid
	}
	var value Version
	var provider, bucket, objectKey string
	err := s.pool.QueryRow(ctx, `SELECT file_id::text,version,status,file_name,size_bytes,media_type,sha256,created_at,provider,bucket,object_key FROM file_versions WHERE tenant_id=$1 AND file_id=$2 AND version=$3`, tenantID, fileID, version).Scan(&value.FileID, &value.Version, &value.Status, &value.FileName, &value.SizeBytes, &value.MediaType, &value.SHA256, &value.CreatedAt, &provider, &bucket, &objectKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return Version{}, ErrNotFound
	}
	if err != nil {
		return Version{}, err
	}
	value.Object = storage.ObjectRef{TenantID: tenantID, Provider: provider, Bucket: bucket, ObjectKey: objectKey}
	return value, nil
}

func (s *Service) GetLatestVersion(ctx context.Context, tenantID, fileID string) (Version, error) {
	if !uuidPattern.MatchString(strings.ToLower(strings.TrimSpace(tenantID))) || !uuidPattern.MatchString(strings.ToLower(strings.TrimSpace(fileID))) {
		return Version{}, ErrInvalid
	}
	var value Version
	var provider, bucket, objectKey string
	err := s.pool.QueryRow(ctx, `SELECT file_id::text,version,status,file_name,size_bytes,media_type,sha256,created_at,provider,bucket,object_key FROM file_versions WHERE tenant_id=$1 AND file_id=$2 ORDER BY version DESC LIMIT 1`, tenantID, fileID).Scan(&value.FileID, &value.Version, &value.Status, &value.FileName, &value.SizeBytes, &value.MediaType, &value.SHA256, &value.CreatedAt, &provider, &bucket, &objectKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return Version{}, ErrNotFound
	}
	if err != nil {
		return Version{}, err
	}
	value.Object = storage.ObjectRef{TenantID: tenantID, Provider: provider, Bucket: bucket, ObjectKey: objectKey}
	return value, nil
}
func (s *Service) GetVersionByGlobalID(ctx context.Context, fileID string, version int) (Version, error) {
	if !uuidPattern.MatchString(fileID) || version < 1 {
		return Version{}, ErrInvalid
	}
	var tenant string
	err := s.pool.QueryRow(ctx, `SELECT tenant_id::text FROM files WHERE id=$1`, fileID).Scan(&tenant)
	if errors.Is(err, pgx.ErrNoRows) {
		return Version{}, ErrNotFound
	}
	if err != nil {
		return Version{}, err
	}
	return s.GetVersion(ctx, tenant, fileID, version)
}
func (s *Service) GetUploadIntentByGlobalID(ctx context.Context, sessionID string) (storage.UploadIntent, string, error) {
	if !uuidPattern.MatchString(sessionID) {
		return storage.UploadIntent{}, "", ErrInvalid
	}
	return s.loadUploadIntent(ctx, "", sessionID, true)
}

func (s *Service) loadUploadIntent(ctx context.Context, tenantID, sessionID string, global bool) (storage.UploadIntent, string, error) {
	return s.loadUploadIntentTx(ctx, s.pool, tenantID, sessionID, false || global)
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (s *Service) loadUploadIntentTx(ctx context.Context, q rowQuerier, tenantID, sessionID string, global bool) (storage.UploadIntent, string, error) {
	query := `SELECT tenant_id::text,id::text,file_id::text,version,file_name,declared_media_type,expected_size,expires_at,provider,bucket,object_key,COALESCE(declared_sha256,''),status FROM upload_sessions WHERE tenant_id=$1 AND id=$2 FOR UPDATE`
	args := []any{tenantID, sessionID}
	if global {
		query = `SELECT tenant_id::text,id::text,file_id::text,version,file_name,declared_media_type,expected_size,expires_at,provider,bucket,object_key,COALESCE(declared_sha256,''),status FROM upload_sessions WHERE id=$1 FOR UPDATE`
		args = []any{sessionID}
	}
	var intent storage.UploadIntent
	var provider, bucket, objectKey, status string
	err := q.QueryRow(ctx, query, args...).Scan(&intent.Object.TenantID, &intent.SessionID, &intent.FileID, &intent.Version, &intent.FileName, &intent.MediaType, &intent.ExpectedSize, &intent.ExpiresAt, &provider, &bucket, &objectKey, &intent.ExpectedSHA256, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return storage.UploadIntent{}, "", ErrNotFound
	}
	if err != nil {
		return storage.UploadIntent{}, "", err
	}
	intent.Object.Provider = provider
	intent.Object.Bucket = bucket
	intent.Object.ObjectKey = objectKey
	return intent, status, nil
}

func (s *Service) Cleanup(ctx context.Context, limit int, staleBefore time.Time) (CleanupResult, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text FROM upload_sessions WHERE status IN ('aborted','expired','cleanup_pending') OR (status='pending' AND expires_at <= $1) OR (status='completing' AND updated_at <= $2) ORDER BY updated_at,id LIMIT $3`, s.now(), staleBefore, limit)
	if err != nil {
		return CleanupResult{}, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return CleanupResult{}, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return CleanupResult{}, err
	}
	result := CleanupResult{Examined: len(ids)}
	for _, id := range ids {
		protected, err := s.cleanupOne(ctx, id)
		if protected {
			result.Protected++
			continue
		}
		if err != nil {
			result.Failed++
		} else {
			result.Cleaned++
		}
	}
	return result, nil
}
func (s *Service) cleanupOne(ctx context.Context, sessionID string) (bool, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Release()
	lockName := "evidence-cleanup:" + sessionID
	var acquired bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, lockName).Scan(&acquired); err != nil {
		return false, err
	}
	if !acquired {
		return true, nil
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1,0))`, lockName)
	}()
	return s.cleanupOneLocked(ctx, sessionID)
}

func (s *Service) cleanupOneLocked(ctx context.Context, sessionID string) (bool, error) {
	now := s.now()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	intent, status, err := s.loadUploadIntentTx(ctx, tx, "", sessionID, true)
	if err != nil {
		return false, err
	}
	var protected bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_versions WHERE tenant_id=$1 AND provider=$2 AND bucket=$3 AND object_key=$4)`, intent.Object.TenantID, intent.Object.Provider, intent.Object.Bucket, intent.Object.ObjectKey).Scan(&protected); err != nil {
		return false, err
	}
	if protected || status == "completed" || status == "cleaned" {
		return true, nil
	}
	tag, err := tx.Exec(ctx, `UPDATE upload_sessions SET status='cleanup_pending',updated_at=$2,cleanup_attempts=cleanup_attempts+1 WHERE id=$1 AND status IN ('pending','completing','aborted','expired','cleanup_pending')`, sessionID, now)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() != 1 {
		return true, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	deleteErr := s.provider.Delete(ctx, intent.Object)
	auditID, _ := newUUID()
	if deleteErr != nil {
		message := deleteErr.Error()
		if len(message) > 2000 {
			message = message[:2000]
		}
		_, _ = s.pool.Exec(ctx, `UPDATE upload_sessions SET last_cleanup_error=$2,updated_at=$3 WHERE id=$1 AND status='cleanup_pending'`, sessionID, message, now)
		_, _ = s.pool.Exec(ctx, `INSERT INTO evidence_audit_entries(id,tenant_id,action,object_type,object_id,actor_kind,actor_display_name,outcome,provider,occurred_at,details) VALUES($1,$2,'upload_session.cleanup','upload_session',$3,'system','evidence-cleanup','failed',$4,$5,jsonb_build_object('error',$6::text))`, auditID, intent.Object.TenantID, sessionID, intent.Object.Provider, now, message)
		return false, deleteErr
	}
	tx, err = s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = tx.Exec(ctx, `UPDATE upload_sessions SET status='cleaned',last_cleanup_error='',updated_at=$2 WHERE id=$1 AND status='cleanup_pending'`, sessionID, now)
	if err != nil {
		return false, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO evidence_audit_entries(id,tenant_id,action,object_type,object_id,actor_kind,actor_display_name,outcome,provider,occurred_at) VALUES($1,$2,'upload_session.cleanup','upload_session',$3,'system','evidence-cleanup','succeeded',$4,$5)`, auditID, intent.Object.TenantID, sessionID, intent.Object.Provider, now)
	if err != nil {
		return false, err
	}
	return false, tx.Commit(ctx)
}

func normalizeCreate(input CreateUploadInput) CreateUploadInput {
	input.FileName = strings.TrimSpace(input.FileName)
	input.MediaType = strings.TrimSpace(input.MediaType)
	input.SHA256 = strings.ToLower(strings.TrimSpace(input.SHA256))
	input.ClientContext.OwnerRef = strings.TrimSpace(input.ClientContext.OwnerRef)
	input.ClientContext.Purpose = strings.TrimSpace(input.ClientContext.Purpose)
	return input
}
func validateCreate(key string, input CreateUploadInput, max int64) error {
	if !validKey(key) || input.FileName == "" || len([]rune(input.FileName)) > 512 || strings.ContainsAny(input.FileName, "/\\") || input.SizeBytes <= 0 || input.SizeBytes > max || len([]rune(input.MediaType)) > 255 || (input.SHA256 != "" && !shaPattern.MatchString(input.SHA256)) || len([]rune(input.ClientContext.OwnerRef)) > 512 || len([]rune(input.ClientContext.Purpose)) > 128 || len(input.ClientContext.Metadata) > 32 {
		return ErrInvalid
	}
	for k, v := range input.ClientContext.Metadata {
		if strings.TrimSpace(k) == "" || len([]rune(k)) > 128 || len([]rune(v)) > 1024 {
			return ErrInvalid
		}
	}
	return nil
}
func normalizeComplete(input CompleteUploadInput) CompleteUploadInput {
	input.ObservedSHA256 = strings.ToLower(strings.TrimSpace(input.ObservedSHA256))
	input.ETag = strings.Trim(strings.TrimSpace(input.ETag), "\"")
	sort.Slice(input.Parts, func(i, j int) bool { return input.Parts[i].PartNumber < input.Parts[j].PartNumber })
	return input
}
func validateComplete(sessionID, key string, input CompleteUploadInput) error {
	if !uuidPattern.MatchString(sessionID) || !validKey(key) || input.ObservedSizeBytes <= 0 || (input.ObservedSHA256 != "" && !shaPattern.MatchString(input.ObservedSHA256)) || len(input.ETag) > 512 || len(input.Parts) > 10000 {
		return ErrInvalid
	}
	last := 0
	for _, part := range input.Parts {
		if part.PartNumber <= last || strings.TrimSpace(part.ETag) == "" || len(part.ETag) > 512 || part.SizeBytes < 0 {
			return ErrInvalid
		}
		last = part.PartNumber
	}
	return nil
}
func validKey(value string) bool {
	return value == strings.TrimSpace(value) && len(value) >= 1 && len(value) <= 128
}
func fingerprint(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	return sum[:], nil
}
func equalHash(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var difference byte
	for i := range left {
		difference |= left[i] ^ right[i]
	}
	return difference == 0
}
func providerBucket(provider storage.Provider) string {
	if named, ok := provider.(interface{ BucketName() string }); ok {
		return named.BucketName()
	}
	return ""
}
func newUUID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}
