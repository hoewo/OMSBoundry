package partymigration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const MappingRequestFormat = "omsbizfoundry.contract-party-mapping-review/v1"
const MappingResultFormat = "omsbizfoundry.contract-party-mapping-apply/v1"

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

var (
	ErrCommandConflict   = errors.New("Party mapping command conflicts with an immutable recorded command")
	ErrSourceDrift       = errors.New("local Party changed after migration preflight")
	ErrMappingConflict   = errors.New("local Party already has a mapping in this migration batch")
	ErrTargetInvalid     = errors.New("target Party reference is invalid")
	ErrTargetUnavailable = errors.New("target Party Directory is unavailable")
)

type MappingRequest struct {
	Format                string            `json:"format"`
	TenantID              string            `json:"tenant_id"`
	MigrationBatchID      string            `json:"migration_batch_id"`
	ReviewedBySubjectID   string            `json:"reviewed_by_subject_id"`
	ReviewedByDisplayName string            `json:"reviewed_by_display_name"`
	Mappings              []MappingDecision `json:"mappings"`
}

type MappingDecision struct {
	CommandID           string `json:"command_id"`
	LocalPartyID        string `json:"local_party_id"`
	LocalSnapshotSHA256 string `json:"local_snapshot_sha256"`
	Decision            string `json:"decision"`
	TargetPartyID       string `json:"target_party_id"`
	TargetPartyRevision int64  `json:"target_party_revision"`
	Reason              string `json:"reason"`
}

type TargetSnapshot struct {
	PartyID                string  `json:"party_id"`
	Revision               int64   `json:"revision"`
	Kind                   string  `json:"kind"`
	LegalName              string  `json:"legal_name"`
	RegistrationIdentifier *string `json:"registration_identifier,omitempty"`
}

type TargetValidator interface {
	VerifyParty(context.Context, string, string, int64) (TargetSnapshot, error)
}

type RecordedMapping struct {
	CommandID           string         `json:"command_id"`
	LocalPartyID        string         `json:"local_party_id"`
	LocalPartyRevision  int64          `json:"local_party_revision"`
	LocalSnapshotSHA256 string         `json:"local_snapshot_sha256"`
	Decision            string         `json:"decision"`
	Target              TargetSnapshot `json:"target"`
	RecordedAt          time.Time      `json:"recorded_at"`
	Replayed            bool           `json:"replayed"`
	RequestSHA256       string         `json:"request_sha256"`
}

type MappingResult struct {
	Format           string            `json:"format"`
	TenantID         string            `json:"tenant_id"`
	MigrationBatchID string            `json:"migration_batch_id"`
	Recorded         int               `json:"recorded"`
	Replayed         int               `json:"replayed"`
	ReadyForCutover  bool              `json:"ready_for_cutover"`
	Mappings         []RecordedMapping `json:"mappings"`
}

type preparedDecision struct {
	MappingDecision
	RequestSHA256 string
	Target        TargetSnapshot
	Existing      *RecordedMapping
}

func ApplyMappings(ctx context.Context, pool *pgxpool.Pool, validator TargetValidator, request MappingRequest, now time.Time) (MappingResult, error) {
	if pool == nil || validator == nil {
		return MappingResult{}, fmt.Errorf("Contract database pool and Party validator are required")
	}
	normalized, err := normalizeMappingRequest(request)
	if err != nil {
		return MappingResult{}, err
	}
	prepared := make([]preparedDecision, 0, len(normalized.Mappings))
	for _, decision := range normalized.Mappings {
		digest, err := mappingRequestDigest(normalized, decision)
		if err != nil {
			return MappingResult{}, err
		}
		existing, err := loadRecordedMapping(ctx, pool, normalized.TenantID, decision.CommandID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return MappingResult{}, fmt.Errorf("load recorded Party mapping command: %w", err)
		}
		entry := preparedDecision{MappingDecision: decision, RequestSHA256: digest}
		if err == nil {
			if existing.RequestSHA256 != digest {
				return MappingResult{}, ErrCommandConflict
			}
			existing.Replayed = true
			entry.Existing = &existing
			prepared = append(prepared, entry)
			continue
		}
		target, err := validator.VerifyParty(ctx, normalized.TenantID, decision.TargetPartyID, decision.TargetPartyRevision)
		if err != nil {
			return MappingResult{}, fmt.Errorf("verify target Party %s revision %d: %w", decision.TargetPartyID, decision.TargetPartyRevision, err)
		}
		entry.Target = target
		prepared = append(prepared, entry)
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return MappingResult{}, fmt.Errorf("begin Party mapping transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result := MappingResult{Format: MappingResultFormat, TenantID: normalized.TenantID, MigrationBatchID: normalized.MigrationBatchID, ReadyForCutover: false, Mappings: make([]RecordedMapping, 0, len(prepared))}
	for _, entry := range prepared {
		if entry.Existing != nil {
			result.Replayed++
			result.Mappings = append(result.Mappings, *entry.Existing)
			continue
		}
		local, err := lockLocalParty(ctx, tx, entry.LocalPartyID)
		if err != nil {
			return MappingResult{}, err
		}
		digest, err := snapshotDigest(local)
		if err != nil {
			return MappingResult{}, err
		}
		if digest != entry.LocalSnapshotSHA256 {
			return MappingResult{}, fmt.Errorf("%w: %s", ErrSourceDrift, entry.LocalPartyID)
		}
		recorded := RecordedMapping{
			CommandID: entry.CommandID, LocalPartyID: entry.LocalPartyID, LocalPartyRevision: local.Revision,
			LocalSnapshotSHA256: digest, Decision: entry.Decision, Target: entry.Target,
			RecordedAt: now.UTC(), RequestSHA256: entry.RequestSHA256,
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO party_migration_mappings (
				tenant_id,migration_batch_id,command_id,source_application,local_party_id,local_party_revision,
				local_snapshot_sha256,decision,target_party_id,target_party_revision,target_kind,target_legal_name,
				target_registration_identifier,reviewed_by_subject_id,reviewed_by_display_name,review_reason,
				target_verified_at,recorded_at,request_sha256
			) VALUES ($1,$2,$3,'contract-ledger',$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$16,$17)
		`, normalized.TenantID, normalized.MigrationBatchID, entry.CommandID, entry.LocalPartyID, local.Revision,
			digest, entry.Decision, entry.Target.PartyID, entry.Target.Revision, entry.Target.Kind, entry.Target.LegalName,
			entry.Target.RegistrationIdentifier, normalized.ReviewedBySubjectID, normalized.ReviewedByDisplayName, entry.Reason,
			now.UTC(), entry.RequestSHA256)
		if err != nil {
			return MappingResult{}, fmt.Errorf("%w: %v", ErrMappingConflict, err)
		}
		result.Recorded++
		result.Mappings = append(result.Mappings, recorded)
	}
	if err := tx.Commit(ctx); err != nil {
		return MappingResult{}, fmt.Errorf("commit Party mappings: %w", err)
	}
	return result, nil
}

func normalizeMappingRequest(request MappingRequest) (MappingRequest, error) {
	request.Format = strings.TrimSpace(request.Format)
	request.TenantID = strings.ToLower(strings.TrimSpace(request.TenantID))
	request.MigrationBatchID = strings.ToLower(strings.TrimSpace(request.MigrationBatchID))
	request.ReviewedBySubjectID = strings.ToLower(strings.TrimSpace(request.ReviewedBySubjectID))
	request.ReviewedByDisplayName = strings.TrimSpace(request.ReviewedByDisplayName)
	if request.Format != MappingRequestFormat {
		return MappingRequest{}, fmt.Errorf("mapping request format must be %s", MappingRequestFormat)
	}
	for name, value := range map[string]string{"tenant_id": request.TenantID, "migration_batch_id": request.MigrationBatchID, "reviewed_by_subject_id": request.ReviewedBySubjectID} {
		if !uuidPattern.MatchString(value) {
			return MappingRequest{}, fmt.Errorf("%s must be a lowercase RFC 4122 UUID", name)
		}
	}
	if request.ReviewedByDisplayName == "" || len([]rune(request.ReviewedByDisplayName)) > 100 || len(request.Mappings) == 0 {
		return MappingRequest{}, fmt.Errorf("reviewer display name and at least one mapping are required")
	}
	commands, locals := map[string]struct{}{}, map[string]struct{}{}
	for index := range request.Mappings {
		item := &request.Mappings[index]
		item.CommandID = strings.ToLower(strings.TrimSpace(item.CommandID))
		item.LocalPartyID = strings.ToLower(strings.TrimSpace(item.LocalPartyID))
		item.LocalSnapshotSHA256 = strings.ToLower(strings.TrimSpace(item.LocalSnapshotSHA256))
		item.Decision = strings.TrimSpace(item.Decision)
		item.TargetPartyID = strings.ToLower(strings.TrimSpace(item.TargetPartyID))
		item.Reason = strings.TrimSpace(item.Reason)
		if !uuidPattern.MatchString(item.CommandID) || !uuidPattern.MatchString(item.LocalPartyID) || !uuidPattern.MatchString(item.TargetPartyID) {
			return MappingRequest{}, fmt.Errorf("mapping %d contains an invalid UUID", index)
		}
		if !sha256Pattern.MatchString(item.LocalSnapshotSHA256) || item.TargetPartyRevision < 1 || (item.Decision != "create_directory_party" && item.Decision != "map_existing_directory_party") || item.Reason == "" || len([]rune(item.Reason)) > 1000 {
			return MappingRequest{}, fmt.Errorf("mapping %d contains an invalid snapshot, decision, target revision, or reason", index)
		}
		if _, duplicate := commands[item.CommandID]; duplicate {
			return MappingRequest{}, fmt.Errorf("mapping %d repeats command_id", index)
		}
		if _, duplicate := locals[item.LocalPartyID]; duplicate {
			return MappingRequest{}, fmt.Errorf("mapping %d repeats local_party_id", index)
		}
		commands[item.CommandID], locals[item.LocalPartyID] = struct{}{}, struct{}{}
	}
	return request, nil
}

func mappingRequestDigest(request MappingRequest, decision MappingDecision) (string, error) {
	payload, err := json.Marshal(struct {
		Format, TenantID, MigrationBatchID, ReviewedBySubjectID, ReviewedByDisplayName string
		Decision                                                                       MappingDecision
	}{request.Format, request.TenantID, request.MigrationBatchID, request.ReviewedBySubjectID, request.ReviewedByDisplayName, decision})
	if err != nil {
		return "", fmt.Errorf("encode mapping request: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func lockLocalParty(ctx context.Context, tx pgx.Tx, localPartyID string) (LegacyParty, error) {
	var value LegacyParty
	err := tx.QueryRow(ctx, `SELECT id::text,kind,legal_name,registration_identifier,revision,created_at,updated_at FROM parties WHERE id=$1 FOR UPDATE`, localPartyID).Scan(
		&value.ID, &value.Kind, &value.LegalName, &value.RegistrationIdentifier, &value.Revision, &value.CreatedAt, &value.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return LegacyParty{}, fmt.Errorf("local Party %s does not exist", localPartyID)
	}
	if err != nil {
		return LegacyParty{}, fmt.Errorf("lock local Party %s: %w", localPartyID, err)
	}
	return value, nil
}

func loadRecordedMapping(ctx context.Context, pool *pgxpool.Pool, tenantID, commandID string) (RecordedMapping, error) {
	var value RecordedMapping
	err := pool.QueryRow(ctx, `
		SELECT command_id::text,local_party_id::text,local_party_revision,local_snapshot_sha256,decision,
			target_party_id::text,target_party_revision,target_kind,target_legal_name,target_registration_identifier,
			recorded_at,request_sha256
		FROM party_migration_mappings WHERE tenant_id=$1 AND command_id=$2
	`, tenantID, commandID).Scan(&value.CommandID, &value.LocalPartyID, &value.LocalPartyRevision, &value.LocalSnapshotSHA256, &value.Decision,
		&value.Target.PartyID, &value.Target.Revision, &value.Target.Kind, &value.Target.LegalName, &value.Target.RegistrationIdentifier,
		&value.RecordedAt, &value.RequestSHA256)
	return value, err
}
