package partymigration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const RuntimeProjectionRequestFormat = "omsbizfoundry.contract-party-directory-runtime-projection/v1"
const RuntimeProjectionResultFormat = "omsbizfoundry.contract-party-directory-runtime-projection-result/v1"

var (
	ErrRuntimeProjectionCommandConflict = errors.New("Party Directory runtime projection command conflicts with an immutable recorded command")
	ErrRuntimeProjectionConflict        = errors.New("Party Directory revision already has a different Contract runtime projection")
	ErrRuntimeProjectionState           = errors.New("Party Directory runtime projection requires directory authority to be active")
)

type RuntimeProjectionRequest struct {
	Format                string `json:"format"`
	TenantID              string `json:"tenant_id"`
	CommandID             string `json:"command_id"`
	TargetPartyID         string `json:"target_party_id"`
	TargetPartyRevision   int64  `json:"target_party_revision"`
	ReviewedBySubjectID   string `json:"reviewed_by_subject_id"`
	ReviewedByDisplayName string `json:"reviewed_by_display_name"`
	Reason                string `json:"reason"`
}

type RuntimeProjectionResult struct {
	Format                    string         `json:"format"`
	TenantID                  string         `json:"tenant_id"`
	CommandID                 string         `json:"command_id"`
	RuntimeTransitionSequence int64          `json:"runtime_transition_sequence"`
	LocalPartyID              string         `json:"local_party_id"`
	LocalPartyRevision        int64          `json:"local_party_revision"`
	Target                    TargetSnapshot `json:"target"`
	ReviewedBySubjectID       string         `json:"reviewed_by_subject_id"`
	ReviewedByDisplayName     string         `json:"reviewed_by_display_name"`
	Reason                    string         `json:"reason"`
	TargetVerifiedAt          time.Time      `json:"target_verified_at"`
	RecordedAt                time.Time      `json:"recorded_at"`
	RequestSHA256             string         `json:"request_sha256"`
	Replayed                  bool           `json:"replayed"`
}

func ApplyRuntimeProjection(ctx context.Context, pool *pgxpool.Pool, validator TargetValidator, request RuntimeProjectionRequest, now time.Time) (RuntimeProjectionResult, error) {
	if pool == nil || validator == nil {
		return RuntimeProjectionResult{}, fmt.Errorf("Contract database pool and Party validator are required")
	}
	normalized, err := normalizeRuntimeProjectionRequest(request)
	if err != nil {
		return RuntimeProjectionResult{}, err
	}
	digest, err := runtimeProjectionRequestDigest(normalized)
	if err != nil {
		return RuntimeProjectionResult{}, err
	}
	if existing, loadErr := loadRuntimeProjection(ctx, pool, normalized.TenantID, normalized.CommandID); loadErr == nil {
		if existing.RequestSHA256 != digest {
			return RuntimeProjectionResult{}, ErrRuntimeProjectionCommandConflict
		}
		existing.Replayed = true
		return existing, nil
	} else if !errors.Is(loadErr, pgx.ErrNoRows) {
		return RuntimeProjectionResult{}, fmt.Errorf("load Party Directory runtime projection: %w", loadErr)
	}
	target, err := validator.VerifyParty(ctx, normalized.TenantID, normalized.TargetPartyID, normalized.TargetPartyRevision)
	if err != nil {
		return RuntimeProjectionResult{}, fmt.Errorf("verify Party Directory runtime projection target: %w", err)
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return RuntimeProjectionResult{}, fmt.Errorf("begin Party Directory runtime projection: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "contract-party-runtime:"+normalized.TenantID); err != nil {
		return RuntimeProjectionResult{}, fmt.Errorf("lock Party Directory runtime projection: %w", err)
	}
	if existing, loadErr := loadRuntimeProjection(ctx, tx, normalized.TenantID, normalized.CommandID); loadErr == nil {
		if existing.RequestSHA256 != digest {
			return RuntimeProjectionResult{}, ErrRuntimeProjectionCommandConflict
		}
		existing.Replayed = true
		if err := tx.Commit(ctx); err != nil {
			return RuntimeProjectionResult{}, fmt.Errorf("commit Party Directory runtime projection replay: %w", err)
		}
		return existing, nil
	} else if !errors.Is(loadErr, pgx.ErrNoRows) {
		return RuntimeProjectionResult{}, fmt.Errorf("reload Party Directory runtime projection: %w", loadErr)
	}
	state, err := loadRuntimeState(ctx, tx, normalized.TenantID)
	if err != nil {
		return RuntimeProjectionResult{}, err
	}
	if state.Mode != RuntimeModeDirectory || !state.DirectoryAuthorityEstablished {
		return RuntimeProjectionResult{}, ErrRuntimeProjectionState
	}
	var existingCommand string
	err = tx.QueryRow(ctx, `SELECT command_id::text FROM party_directory_runtime_projections WHERE tenant_id=$1 AND target_party_id=$2 AND target_party_revision=$3`, normalized.TenantID, target.PartyID, target.Revision).Scan(&existingCommand)
	if err == nil {
		return RuntimeProjectionResult{}, ErrRuntimeProjectionConflict
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return RuntimeProjectionResult{}, fmt.Errorf("check existing Party Directory runtime projection: %w", err)
	}

	localPartyID := deterministicRuntimeProjectionUUID(normalized.TenantID + ":" + target.PartyID + ":" + fmt.Sprint(target.Revision))
	auditID := deterministicRuntimeProjectionUUID(normalized.CommandID + ":audit")
	verifiedAt, recordedAt := now.UTC(), now.UTC()
	if _, err = tx.Exec(ctx, `
		INSERT INTO parties(id,kind,legal_name,registration_identifier,revision,created_at,updated_at)
		VALUES($1,$2,$3,$4,1,$5,$5)
	`, localPartyID, target.Kind, target.LegalName, target.RegistrationIdentifier, recordedAt); err != nil {
		return RuntimeProjectionResult{}, fmt.Errorf("insert projected Contract Party snapshot: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO party_directory_runtime_projections(
			tenant_id,command_id,runtime_transition_sequence,source_application,
			local_party_id,local_party_revision,target_party_id,target_party_revision,
			target_kind,target_legal_name,target_registration_identifier,
			reviewed_by_subject_id,reviewed_by_display_name,review_reason,
			target_verified_at,recorded_at,request_sha256
		) VALUES($1,$2,$3,'party-directory',$4,1,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13,$14)
	`, normalized.TenantID, normalized.CommandID, state.Sequence, localPartyID, target.PartyID, target.Revision,
		target.Kind, target.LegalName, target.RegistrationIdentifier, normalized.ReviewedBySubjectID,
		normalized.ReviewedByDisplayName, normalized.Reason, recordedAt, digest); err != nil {
		return RuntimeProjectionResult{}, fmt.Errorf("record immutable Party Directory runtime projection: %w", err)
	}
	details, err := json.Marshal(map[string]any{
		"source_application":     "party-directory",
		"target_party_id":        target.PartyID,
		"target_party_revision":  target.Revision,
		"target_kind":            target.Kind,
		"target_legal_name":      target.LegalName,
		"runtime_transition_seq": state.Sequence,
		"local_party_revision":   int64(1),
		"review_reason":          normalized.Reason,
	})
	if err != nil {
		return RuntimeProjectionResult{}, fmt.Errorf("encode Party Directory runtime projection audit: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO audit_entries(
			id,object_type,object_id,action,command_id,actor_kind,actor_id,actor_display_name,
			occurred_at,summary,details
		) VALUES($1,'party',$2,'party_directory_runtime_projected',$3,'iam_subject',$4,$5,$6,$7,$8::jsonb)
	`, auditID, localPartyID, normalized.CommandID, normalized.ReviewedBySubjectID,
		normalized.ReviewedByDisplayName, recordedAt, "投影 Party Directory 精确 revision 为 Contract 业务快照", details); err != nil {
		return RuntimeProjectionResult{}, fmt.Errorf("record Party Directory runtime projection audit: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return RuntimeProjectionResult{}, fmt.Errorf("commit Party Directory runtime projection: %w", err)
	}
	return RuntimeProjectionResult{
		Format: RuntimeProjectionResultFormat, TenantID: normalized.TenantID, CommandID: normalized.CommandID,
		RuntimeTransitionSequence: state.Sequence, LocalPartyID: localPartyID, LocalPartyRevision: 1,
		Target: target, ReviewedBySubjectID: normalized.ReviewedBySubjectID,
		ReviewedByDisplayName: normalized.ReviewedByDisplayName, Reason: normalized.Reason,
		TargetVerifiedAt: verifiedAt, RecordedAt: recordedAt, RequestSHA256: digest,
	}, nil
}

func normalizeRuntimeProjectionRequest(request RuntimeProjectionRequest) (RuntimeProjectionRequest, error) {
	request.Format = strings.TrimSpace(request.Format)
	request.TenantID = strings.ToLower(strings.TrimSpace(request.TenantID))
	request.CommandID = strings.ToLower(strings.TrimSpace(request.CommandID))
	request.TargetPartyID = strings.ToLower(strings.TrimSpace(request.TargetPartyID))
	request.ReviewedBySubjectID = strings.ToLower(strings.TrimSpace(request.ReviewedBySubjectID))
	request.ReviewedByDisplayName = strings.TrimSpace(request.ReviewedByDisplayName)
	request.Reason = strings.TrimSpace(request.Reason)
	if request.Format != RuntimeProjectionRequestFormat {
		return RuntimeProjectionRequest{}, fmt.Errorf("runtime projection format must be %s", RuntimeProjectionRequestFormat)
	}
	for name, value := range map[string]string{
		"tenant_id": request.TenantID, "command_id": request.CommandID,
		"target_party_id": request.TargetPartyID, "reviewed_by_subject_id": request.ReviewedBySubjectID,
	} {
		if !uuidPattern.MatchString(value) {
			return RuntimeProjectionRequest{}, fmt.Errorf("%s must be a lowercase RFC 4122 UUID", name)
		}
	}
	if request.TargetPartyRevision < 1 || request.ReviewedByDisplayName == "" || len([]rune(request.ReviewedByDisplayName)) > 100 || request.Reason == "" || len([]rune(request.Reason)) > 1000 {
		return RuntimeProjectionRequest{}, fmt.Errorf("target revision, reviewer display name and reason are required and must fit their limits")
	}
	return request, nil
}

func runtimeProjectionRequestDigest(request RuntimeProjectionRequest) (string, error) {
	raw, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("encode Party Directory runtime projection request: %w", err)
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func deterministicRuntimeProjectionUUID(seed string) string {
	digest := sha256.Sum256([]byte(seed))
	digest[6] = (digest[6] & 0x0f) | 0x50
	digest[8] = (digest[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", digest[0:4], digest[4:6], digest[6:8], digest[8:10], digest[10:16])
}

func loadRuntimeProjection(ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenantID, commandID string) (RuntimeProjectionResult, error) {
	var result RuntimeProjectionResult
	result.Format, result.TenantID = RuntimeProjectionResultFormat, tenantID
	err := query.QueryRow(ctx, `
		SELECT command_id::text,runtime_transition_sequence,local_party_id::text,local_party_revision,
			target_party_id::text,target_party_revision,target_kind,target_legal_name,target_registration_identifier,
			reviewed_by_subject_id::text,reviewed_by_display_name,review_reason,target_verified_at,recorded_at,request_sha256
		FROM party_directory_runtime_projections WHERE tenant_id=$1 AND command_id=$2
	`, tenantID, commandID).Scan(&result.CommandID, &result.RuntimeTransitionSequence, &result.LocalPartyID,
		&result.LocalPartyRevision, &result.Target.PartyID, &result.Target.Revision, &result.Target.Kind,
		&result.Target.LegalName, &result.Target.RegistrationIdentifier, &result.ReviewedBySubjectID,
		&result.ReviewedByDisplayName, &result.Reason, &result.TargetVerifiedAt, &result.RecordedAt,
		&result.RequestSHA256)
	return result, err
}
