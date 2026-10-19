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

const RuntimeTransitionRequestFormat = "omsbizfoundry.money-flow-party-runtime-transition/v1"
const RuntimeTransitionResultFormat = "omsbizfoundry.money-flow-party-runtime-transition-result/v1"

const (
	RuntimeModeLocal              = "local"
	RuntimeModeDirectory          = "directory"
	RuntimeModeLocalCompatibility = "local_compatibility"
	RuntimeActionCutover          = "cutover"
	RuntimeActionRollback         = "rollback"
)

var (
	ErrRuntimeCommandConflict = errors.New("Party runtime transition command conflicts with an immutable recorded command")
	ErrRuntimeTransition      = errors.New("Party runtime transition is not allowed from the current state")
	ErrRuntimeMappingMissing  = errors.New("Party runtime mapping is missing")
	ErrRuntimeMappingConflict = errors.New("Party runtime target mapping is ambiguous")
)

type RuntimeTransitionRequest struct {
	Format                string `json:"format"`
	TenantID              string `json:"tenant_id"`
	CommandID             string `json:"command_id"`
	Action                string `json:"action"`
	MigrationBatchID      string `json:"migration_batch_id"`
	ReviewedBySubjectID   string `json:"reviewed_by_subject_id"`
	ReviewedByDisplayName string `json:"reviewed_by_display_name"`
	Reason                string `json:"reason"`
}

type RuntimeState struct {
	Mode                          string    `json:"mode"`
	DirectoryAuthorityEstablished bool      `json:"directory_authority_established"`
	Sequence                      int64     `json:"sequence,omitempty"`
	Action                        string    `json:"action,omitempty"`
	MigrationBatchID              string    `json:"migration_batch_id,omitempty"`
	CreatedAt                     time.Time `json:"created_at,omitempty"`
}

type RuntimePartyReference struct {
	LocalPartyID           string  `json:"local_party_id"`
	PartyID                string  `json:"party_id"`
	Revision               int64   `json:"revision"`
	Kind                   string  `json:"kind"`
	LegalName              string  `json:"legal_name"`
	RegistrationIdentifier *string `json:"registration_identifier,omitempty"`
	Source                 string  `json:"source"`
}

type RuntimeTransitionResult struct {
	Format               string       `json:"format"`
	TenantID             string       `json:"tenant_id"`
	CommandID            string       `json:"command_id"`
	Action               string       `json:"action"`
	MigrationBatchID     string       `json:"migration_batch_id"`
	Sequence             int64        `json:"sequence"`
	MappedPartyCount     int          `json:"mapped_party_count"`
	ReferencedPartyCount int          `json:"referenced_party_count"`
	Replayed             bool         `json:"replayed"`
	State                RuntimeState `json:"state"`
	RequestSHA256        string       `json:"request_sha256"`
}

type runtimeMapping struct {
	LocalPartyID        string
	LocalPartyRevision  int64
	LocalSnapshotSHA256 string
	Target              TargetSnapshot
}

type RuntimeRegistry struct {
	pool *pgxpool.Pool
}

func NewRuntimeRegistry(pool *pgxpool.Pool) *RuntimeRegistry {
	return &RuntimeRegistry{pool: pool}
}

func (r *RuntimeRegistry) State(ctx context.Context, tenantID string) (RuntimeState, error) {
	if r == nil || r.pool == nil {
		return RuntimeState{}, fmt.Errorf("Money Flow Party runtime registry is required")
	}
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	if !uuidPattern.MatchString(tenantID) {
		return RuntimeState{}, fmt.Errorf("tenant ID must be a lowercase RFC 4122 UUID")
	}
	return loadRuntimeState(ctx, r.pool, tenantID)
}

func (r *RuntimeRegistry) LocalPartyWritesAllowed(ctx context.Context, tenantID string) (bool, error) {
	state, err := r.State(ctx, tenantID)
	if err != nil {
		return false, err
	}
	return !state.DirectoryAuthorityEstablished, nil
}

func (r *RuntimeRegistry) ResolveReferences(ctx context.Context, tenantID string, localPartyIDs []string) ([]RuntimePartyReference, error) {
	state, err := r.State(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	values := make([]RuntimePartyReference, 0, len(localPartyIDs))
	for _, localPartyID := range localPartyIDs {
		localPartyID = strings.ToLower(strings.TrimSpace(localPartyID))
		if !uuidPattern.MatchString(localPartyID) {
			return nil, fmt.Errorf("invalid local Party ID %q", localPartyID)
		}
		value, err := r.resolveReference(ctx, tenantID, state, localPartyID)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func (r *RuntimeRegistry) resolveReference(ctx context.Context, tenantID string, state RuntimeState, localPartyID string) (RuntimePartyReference, error) {
	if state.Mode != RuntimeModeDirectory {
		var value RuntimePartyReference
		value.LocalPartyID, value.PartyID, value.Source = localPartyID, localPartyID, RuntimeModeLocal
		err := r.pool.QueryRow(ctx, `SELECT revision,kind,legal_name,registration_identifier FROM parties WHERE id=$1`, localPartyID).Scan(
			&value.Revision, &value.Kind, &value.LegalName, &value.RegistrationIdentifier,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return RuntimePartyReference{}, fmt.Errorf("%w: %s", ErrRuntimeMappingMissing, localPartyID)
		}
		if err != nil {
			return RuntimePartyReference{}, fmt.Errorf("read local Party runtime reference: %w", err)
		}
		return value, nil
	}
	var value RuntimePartyReference
	value.LocalPartyID, value.Source = localPartyID, RuntimeModeDirectory
	err := r.pool.QueryRow(ctx, `
		SELECT target_party_id,target_party_revision,target_kind,target_legal_name,target_registration_identifier
		FROM (
			SELECT target_party_id::text,target_party_revision,target_kind,target_legal_name,target_registration_identifier,1 AS priority
			FROM party_migration_mappings
			WHERE tenant_id=$1 AND migration_batch_id=$2 AND local_party_id=$3
			UNION ALL
			SELECT target_party_id::text,target_party_revision,target_kind,target_legal_name,target_registration_identifier,2 AS priority
			FROM party_directory_runtime_projections
			WHERE tenant_id=$1 AND local_party_id=$3
		) resolved
		ORDER BY priority
		LIMIT 1
	`, tenantID, state.MigrationBatchID, localPartyID).Scan(
		&value.PartyID, &value.Revision, &value.Kind, &value.LegalName, &value.RegistrationIdentifier,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return RuntimePartyReference{}, fmt.Errorf("%w: %s", ErrRuntimeMappingMissing, localPartyID)
	}
	if err != nil {
		return RuntimePartyReference{}, fmt.Errorf("read Directory Party runtime reference: %w", err)
	}
	return value, nil
}

func ApplyRuntimeTransition(ctx context.Context, pool *pgxpool.Pool, validator TargetValidator, request RuntimeTransitionRequest, now time.Time) (RuntimeTransitionResult, error) {
	if pool == nil {
		return RuntimeTransitionResult{}, fmt.Errorf("Contract database pool is required")
	}
	normalized, err := normalizeRuntimeTransitionRequest(request)
	if err != nil {
		return RuntimeTransitionResult{}, err
	}
	digest, err := runtimeTransitionDigest(normalized)
	if err != nil {
		return RuntimeTransitionResult{}, err
	}
	if existing, err := loadRuntimeTransition(ctx, pool, normalized.TenantID, normalized.CommandID); err == nil {
		if existing.RequestSHA256 != digest {
			return RuntimeTransitionResult{}, ErrRuntimeCommandConflict
		}
		existing.Replayed = true
		return existing, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return RuntimeTransitionResult{}, err
	}

	var mappings []runtimeMapping
	var validatedMappingsSHA256 string
	if normalized.Action == RuntimeActionCutover {
		if validator == nil {
			return RuntimeTransitionResult{}, fmt.Errorf("Party validator is required for cutover")
		}
		mappings, err = loadRuntimeMappings(ctx, pool, normalized.TenantID, normalized.MigrationBatchID)
		if err != nil {
			return RuntimeTransitionResult{}, err
		}
		if len(mappings) == 0 {
			return RuntimeTransitionResult{}, fmt.Errorf("%w: migration batch has no reviewed mappings", ErrRuntimeMappingMissing)
		}
		validatedMappingsSHA256, err = runtimeMappingsDigest(mappings)
		if err != nil {
			return RuntimeTransitionResult{}, err
		}
		targets := make(map[string]struct{}, len(mappings))
		for _, mapping := range mappings {
			if _, duplicate := targets[mapping.Target.PartyID]; duplicate {
				return RuntimeTransitionResult{}, fmt.Errorf("%w: target Party %s", ErrRuntimeMappingConflict, mapping.Target.PartyID)
			}
			targets[mapping.Target.PartyID] = struct{}{}
			verified, verifyErr := validator.VerifyParty(ctx, normalized.TenantID, mapping.Target.PartyID, mapping.Target.Revision)
			if verifyErr != nil {
				return RuntimeTransitionResult{}, verifyErr
			}
			if !sameTargetSnapshot(verified, mapping.Target) {
				return RuntimeTransitionResult{}, fmt.Errorf("%w: target Party %s changed after review", ErrTargetInvalid, mapping.Target.PartyID)
			}
		}
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return RuntimeTransitionResult{}, fmt.Errorf("begin Party runtime transition: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "money-flow-party-runtime:"+normalized.TenantID); err != nil {
		return RuntimeTransitionResult{}, fmt.Errorf("lock Party runtime transition: %w", err)
	}
	if existing, err := loadRuntimeTransition(ctx, tx, normalized.TenantID, normalized.CommandID); err == nil {
		if existing.RequestSHA256 != digest {
			return RuntimeTransitionResult{}, ErrRuntimeCommandConflict
		}
		existing.Replayed = true
		return existing, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return RuntimeTransitionResult{}, err
	}
	current, err := loadRuntimeState(ctx, tx, normalized.TenantID)
	if err != nil {
		return RuntimeTransitionResult{}, err
	}
	if normalized.Action == RuntimeActionCutover {
		if current.Mode == RuntimeModeDirectory || (current.DirectoryAuthorityEstablished && current.MigrationBatchID != normalized.MigrationBatchID) {
			return RuntimeTransitionResult{}, ErrRuntimeTransition
		}
		mappings, err = loadRuntimeMappings(ctx, tx, normalized.TenantID, normalized.MigrationBatchID)
		if err != nil {
			return RuntimeTransitionResult{}, err
		}
		if len(mappings) == 0 {
			return RuntimeTransitionResult{}, fmt.Errorf("%w: migration batch has no reviewed mappings", ErrRuntimeMappingMissing)
		}
		transactionMappingsSHA256, digestErr := runtimeMappingsDigest(mappings)
		if digestErr != nil {
			return RuntimeTransitionResult{}, digestErr
		}
		if transactionMappingsSHA256 != validatedMappingsSHA256 {
			return RuntimeTransitionResult{}, fmt.Errorf("%w: migration batch changed during cutover validation", ErrRuntimeMappingConflict)
		}
		mapped := make(map[string]struct{}, len(mappings))
		for _, mapping := range mappings {
			local, lockErr := lockLocalParty(ctx, tx, mapping.LocalPartyID)
			if lockErr != nil {
				return RuntimeTransitionResult{}, lockErr
			}
			localDigest, digestErr := snapshotDigest(local)
			if digestErr != nil {
				return RuntimeTransitionResult{}, digestErr
			}
			if local.Revision != mapping.LocalPartyRevision || localDigest != mapping.LocalSnapshotSHA256 {
				return RuntimeTransitionResult{}, fmt.Errorf("%w: %s", ErrSourceDrift, mapping.LocalPartyID)
			}
			mapped[mapping.LocalPartyID] = struct{}{}
		}
		referenced, referenceErr := moneyReferencedPartyIDs(ctx, tx)
		if referenceErr != nil {
			return RuntimeTransitionResult{}, referenceErr
		}
		for _, localPartyID := range referenced {
			if _, ok := mapped[localPartyID]; !ok {
				return RuntimeTransitionResult{}, fmt.Errorf("%w: referenced local Party %s", ErrRuntimeMappingMissing, localPartyID)
			}
		}
		result, insertErr := insertRuntimeTransition(ctx, tx, normalized, digest, current, len(mappings), len(referenced), now)
		if insertErr != nil {
			return RuntimeTransitionResult{}, insertErr
		}
		if err := tx.Commit(ctx); err != nil {
			return RuntimeTransitionResult{}, fmt.Errorf("commit Party runtime cutover: %w", err)
		}
		return result, nil
	}
	if current.Mode != RuntimeModeDirectory || current.MigrationBatchID != normalized.MigrationBatchID {
		return RuntimeTransitionResult{}, ErrRuntimeTransition
	}
	mappedCount, err := currentMappingCount(ctx, tx, normalized.TenantID, normalized.MigrationBatchID)
	if err != nil {
		return RuntimeTransitionResult{}, err
	}
	referencedCount, err := currentReferencedCount(ctx, tx)
	if err != nil {
		return RuntimeTransitionResult{}, err
	}
	result, err := insertRuntimeTransition(ctx, tx, normalized, digest, current, mappedCount, referencedCount, now)
	if err != nil {
		return RuntimeTransitionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RuntimeTransitionResult{}, fmt.Errorf("commit Party runtime rollback: %w", err)
	}
	return result, nil
}

func normalizeRuntimeTransitionRequest(request RuntimeTransitionRequest) (RuntimeTransitionRequest, error) {
	request.Format = strings.TrimSpace(request.Format)
	request.TenantID = strings.ToLower(strings.TrimSpace(request.TenantID))
	request.CommandID = strings.ToLower(strings.TrimSpace(request.CommandID))
	request.Action = strings.ToLower(strings.TrimSpace(request.Action))
	request.MigrationBatchID = strings.ToLower(strings.TrimSpace(request.MigrationBatchID))
	request.ReviewedBySubjectID = strings.ToLower(strings.TrimSpace(request.ReviewedBySubjectID))
	request.ReviewedByDisplayName = strings.TrimSpace(request.ReviewedByDisplayName)
	request.Reason = strings.TrimSpace(request.Reason)
	if request.Format != RuntimeTransitionRequestFormat {
		return RuntimeTransitionRequest{}, fmt.Errorf("runtime transition format must be %s", RuntimeTransitionRequestFormat)
	}
	for name, value := range map[string]string{"tenant_id": request.TenantID, "command_id": request.CommandID, "migration_batch_id": request.MigrationBatchID, "reviewed_by_subject_id": request.ReviewedBySubjectID} {
		if !uuidPattern.MatchString(value) {
			return RuntimeTransitionRequest{}, fmt.Errorf("%s must be a lowercase RFC 4122 UUID", name)
		}
	}
	if request.Action != RuntimeActionCutover && request.Action != RuntimeActionRollback {
		return RuntimeTransitionRequest{}, fmt.Errorf("action must be cutover or rollback")
	}
	if request.ReviewedByDisplayName == "" || len([]rune(request.ReviewedByDisplayName)) > 100 || request.Reason == "" || len([]rune(request.Reason)) > 1000 {
		return RuntimeTransitionRequest{}, fmt.Errorf("reviewer display name and reason are required and must fit their limits")
	}
	return request, nil
}

func runtimeTransitionDigest(request RuntimeTransitionRequest) (string, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("encode Party runtime transition: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func runtimeMappingsDigest(mappings []runtimeMapping) (string, error) {
	payload, err := json.Marshal(mappings)
	if err != nil {
		return "", fmt.Errorf("encode Party runtime mappings: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func loadRuntimeMappings(ctx context.Context, query interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, tenantID, batchID string) ([]runtimeMapping, error) {
	rows, err := query.Query(ctx, `
		SELECT local_party_id::text,local_party_revision,local_snapshot_sha256,
			target_party_id::text,target_party_revision,target_kind,target_legal_name,target_registration_identifier
		FROM party_migration_mappings
		WHERE tenant_id=$1 AND migration_batch_id=$2
		ORDER BY local_party_id
	`, tenantID, batchID)
	if err != nil {
		return nil, fmt.Errorf("query Party runtime mappings: %w", err)
	}
	defer rows.Close()
	values := make([]runtimeMapping, 0)
	for rows.Next() {
		var value runtimeMapping
		if err := rows.Scan(&value.LocalPartyID, &value.LocalPartyRevision, &value.LocalSnapshotSHA256, &value.Target.PartyID, &value.Target.Revision, &value.Target.Kind, &value.Target.LegalName, &value.Target.RegistrationIdentifier); err != nil {
			return nil, fmt.Errorf("scan Party runtime mapping: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Party runtime mappings: %w", err)
	}
	return values, nil
}

func moneyReferencedPartyIDs(ctx context.Context, query interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}) ([]string, error) {
	rows, err := query.Query(ctx, `
		SELECT ledger_owner_party_id::text FROM obligations
		UNION SELECT creditor_party_id::text FROM obligations
		UNION SELECT debtor_party_id::text FROM obligations
		UNION SELECT ledger_owner_party_id::text FROM obligation_revisions
		UNION SELECT creditor_party_id::text FROM obligation_revisions
		UNION SELECT debtor_party_id::text FROM obligation_revisions
		UNION SELECT ledger_owner_party_id::text FROM cash_transactions
		UNION SELECT payer_party_id::text FROM cash_transactions
		UNION SELECT payee_party_id::text FROM cash_transactions
		UNION SELECT ledger_owner_party_id::text FROM internal_transfers
		ORDER BY 1
	`)
	if err != nil {
		return nil, fmt.Errorf("query referenced Money Flow Parties: %w", err)
	}
	defer rows.Close()
	values := make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, fmt.Errorf("scan referenced Money Flow Party: %w", err)
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func loadRuntimeState(ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenantID string) (RuntimeState, error) {
	var state RuntimeState
	err := query.QueryRow(ctx, `
		SELECT sequence,action,migration_batch_id::text,created_at
		FROM party_runtime_transitions WHERE tenant_id=$1 ORDER BY sequence DESC LIMIT 1
	`, tenantID).Scan(&state.Sequence, &state.Action, &state.MigrationBatchID, &state.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return RuntimeState{Mode: RuntimeModeLocal}, nil
	}
	if err != nil {
		return RuntimeState{}, fmt.Errorf("read Party runtime state: %w", err)
	}
	state.DirectoryAuthorityEstablished = true
	if state.Action == RuntimeActionCutover {
		state.Mode = RuntimeModeDirectory
	} else {
		state.Mode = RuntimeModeLocalCompatibility
	}
	return state, nil
}

func insertRuntimeTransition(ctx context.Context, tx pgx.Tx, request RuntimeTransitionRequest, digest string, current RuntimeState, mappedCount, referencedCount int, now time.Time) (RuntimeTransitionResult, error) {
	var prior any
	if current.Sequence > 0 {
		prior = current.Sequence
	}
	var sequence int64
	err := tx.QueryRow(ctx, `
		INSERT INTO party_runtime_transitions (
			tenant_id,command_id,action,migration_batch_id,prior_sequence,reviewed_by_subject_id,
			reviewed_by_display_name,reason,mapped_party_count,referenced_party_count,request_sha256,created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING sequence
	`, request.TenantID, request.CommandID, request.Action, request.MigrationBatchID, prior, request.ReviewedBySubjectID,
		request.ReviewedByDisplayName, request.Reason, mappedCount, referencedCount, digest, now.UTC()).Scan(&sequence)
	if err != nil {
		return RuntimeTransitionResult{}, fmt.Errorf("append Party runtime transition: %w", err)
	}
	mode := RuntimeModeDirectory
	if request.Action == RuntimeActionRollback {
		mode = RuntimeModeLocalCompatibility
	}
	state := RuntimeState{Mode: mode, DirectoryAuthorityEstablished: true, Sequence: sequence, Action: request.Action, MigrationBatchID: request.MigrationBatchID, CreatedAt: now.UTC()}
	return RuntimeTransitionResult{Format: RuntimeTransitionResultFormat, TenantID: request.TenantID, CommandID: request.CommandID, Action: request.Action, MigrationBatchID: request.MigrationBatchID, Sequence: sequence, MappedPartyCount: mappedCount, ReferencedPartyCount: referencedCount, State: state, RequestSHA256: digest}, nil
}

func loadRuntimeTransition(ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenantID, commandID string) (RuntimeTransitionResult, error) {
	var result RuntimeTransitionResult
	var createdAt time.Time
	err := query.QueryRow(ctx, `
		SELECT command_id::text,action,migration_batch_id::text,sequence,mapped_party_count,referenced_party_count,request_sha256,created_at
		FROM party_runtime_transitions WHERE tenant_id=$1 AND command_id=$2
	`, tenantID, commandID).Scan(&result.CommandID, &result.Action, &result.MigrationBatchID, &result.Sequence, &result.MappedPartyCount, &result.ReferencedPartyCount, &result.RequestSHA256, &createdAt)
	if err != nil {
		return RuntimeTransitionResult{}, err
	}
	result.Format, result.TenantID = RuntimeTransitionResultFormat, tenantID
	mode := RuntimeModeDirectory
	if result.Action == RuntimeActionRollback {
		mode = RuntimeModeLocalCompatibility
	}
	result.State = RuntimeState{Mode: mode, DirectoryAuthorityEstablished: true, Sequence: result.Sequence, Action: result.Action, MigrationBatchID: result.MigrationBatchID, CreatedAt: createdAt}
	return result, nil
}

func currentMappingCount(ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenantID, batchID string) (int, error) {
	var count int
	if err := query.QueryRow(ctx, `SELECT count(*) FROM party_migration_mappings WHERE tenant_id=$1 AND migration_batch_id=$2`, tenantID, batchID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count Party runtime mappings: %w", err)
	}
	return count, nil
}

func currentReferencedCount(ctx context.Context, query interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}) (int, error) {
	values, err := moneyReferencedPartyIDs(ctx, query)
	if err != nil {
		return 0, err
	}
	return len(values), nil
}

func sameTargetSnapshot(left, right TargetSnapshot) bool {
	if left.PartyID != right.PartyID || left.Revision != right.Revision || left.Kind != right.Kind || left.LegalName != right.LegalName {
		return false
	}
	if left.RegistrationIdentifier == nil || right.RegistrationIdentifier == nil {
		return left.RegistrationIdentifier == nil && right.RegistrationIdentifier == nil
	}
	return *left.RegistrationIdentifier == *right.RegistrationIdentifier
}
