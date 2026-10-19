package partymigration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RuntimeDirectoryProjector resolves one exact Party Directory revision to the
// immutable Money snapshot used by native ledger facts. A missing snapshot is
// projected only after directory authority has been established and the target
// revision has been verified through the versioned Party reference port.
type RuntimeDirectoryProjector struct {
	pool      *pgxpool.Pool
	validator TargetValidator
	now       func() time.Time
}

func NewRuntimeDirectoryProjector(pool *pgxpool.Pool, validator TargetValidator) *RuntimeDirectoryProjector {
	return &RuntimeDirectoryProjector{pool: pool, validator: validator, now: func() time.Time { return time.Now().UTC() }}
}

func (p *RuntimeDirectoryProjector) ResolveOrProject(
	ctx context.Context,
	tenantID string,
	partyID string,
	revision int64,
	reviewedBySubjectID string,
	reviewedByDisplayName string,
) (RuntimePartyReference, error) {
	if p == nil || p.pool == nil || p.validator == nil {
		return RuntimePartyReference{}, fmt.Errorf("Money Flow Party Directory projector is required")
	}
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	partyID = strings.ToLower(strings.TrimSpace(partyID))
	reviewedBySubjectID = strings.ToLower(strings.TrimSpace(reviewedBySubjectID))
	reviewedByDisplayName = strings.TrimSpace(reviewedByDisplayName)
	if !uuidPattern.MatchString(tenantID) || !uuidPattern.MatchString(partyID) || !uuidPattern.MatchString(reviewedBySubjectID) || revision < 1 || reviewedByDisplayName == "" || len([]rune(reviewedByDisplayName)) > 100 {
		return RuntimePartyReference{}, ErrTargetInvalid
	}
	state, err := NewRuntimeRegistry(p.pool).State(ctx, tenantID)
	if err != nil {
		return RuntimePartyReference{}, err
	}
	if state.Mode != RuntimeModeDirectory || !state.DirectoryAuthorityEstablished {
		return RuntimePartyReference{}, ErrRuntimeProjectionState
	}
	if value, found, err := loadDirectoryReferenceByTarget(ctx, p.pool, tenantID, state.MigrationBatchID, partyID, revision); err != nil {
		return RuntimePartyReference{}, err
	} else if found {
		return value, nil
	}
	commandID := deterministicRuntimeProjectionUUID(fmt.Sprintf("money-runtime-party:%s:%s:%d", tenantID, partyID, revision))
	result, err := ApplyRuntimeProjection(ctx, p.pool, p.validator, RuntimeProjectionRequest{
		Format: RuntimeProjectionRequestFormat, TenantID: tenantID, CommandID: commandID,
		TargetPartyID: partyID, TargetPartyRevision: revision,
		ReviewedBySubjectID: reviewedBySubjectID, ReviewedByDisplayName: reviewedByDisplayName,
		Reason: "实际资金登记需要冻结 Party Directory 精确 revision",
	}, p.now())
	if err != nil {
		// A concurrent request can commit the same target before this request
		// observes it. Resolve the immutable target once more before surfacing a
		// conflict; never create a second local authority or match by name.
		if value, found, loadErr := loadDirectoryReferenceByTarget(ctx, p.pool, tenantID, state.MigrationBatchID, partyID, revision); loadErr == nil && found {
			return value, nil
		}
		return RuntimePartyReference{}, err
	}
	return RuntimePartyReference{
		LocalPartyID: result.LocalPartyID, PartyID: result.Target.PartyID,
		Revision: result.Target.Revision, Kind: result.Target.Kind,
		LegalName: result.Target.LegalName, RegistrationIdentifier: result.Target.RegistrationIdentifier,
		Source: RuntimeModeDirectory,
	}, nil
}

func loadDirectoryReferenceByTarget(
	ctx context.Context,
	query interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	},
	tenantID string,
	migrationBatchID string,
	partyID string,
	revision int64,
) (RuntimePartyReference, bool, error) {
	var value RuntimePartyReference
	value.PartyID, value.Revision, value.Source = partyID, revision, RuntimeModeDirectory
	err := query.QueryRow(ctx, `
		SELECT local_party_id,local_party_revision,target_kind,target_legal_name,target_registration_identifier
		FROM (
			SELECT local_party_id::text,local_party_revision,target_kind,target_legal_name,target_registration_identifier,1 AS priority
			FROM party_migration_mappings
			WHERE tenant_id=$1 AND migration_batch_id=$2 AND target_party_id=$3 AND target_party_revision=$4
			UNION ALL
			SELECT local_party_id::text,local_party_revision,target_kind,target_legal_name,target_registration_identifier,2 AS priority
			FROM party_directory_runtime_projections
			WHERE tenant_id=$1 AND target_party_id=$3 AND target_party_revision=$4
		) resolved
		ORDER BY priority
		LIMIT 1
	`, tenantID, migrationBatchID, partyID, revision).Scan(
		&value.LocalPartyID, &value.Revision, &value.Kind, &value.LegalName, &value.RegistrationIdentifier,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return RuntimePartyReference{}, false, nil
	}
	if err != nil {
		return RuntimePartyReference{}, false, fmt.Errorf("read Party Directory Money runtime projection: %w", err)
	}
	// The selected row's local revision is intentionally not exposed as the
	// Party Directory revision. RuntimePartyReference.Revision remains the exact
	// target revision supplied by the caller.
	value.Revision = revision
	return value, true, nil
}
