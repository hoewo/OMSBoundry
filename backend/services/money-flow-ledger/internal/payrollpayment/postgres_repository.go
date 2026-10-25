package payrollpayment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"omsbizfoundry/money-flow-ledger/internal/audit"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Replay(ctx context.Context, tenant, command, requestDigest string) (Result, bool, error) {
	var stored string
	var raw []byte
	err := r.pool.QueryRow(ctx, `SELECT request_digest,response_snapshot FROM payroll_payment_deliveries WHERE tenant_id=$1 AND command_id=$2`, tenant, command).Scan(&stored, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, err
	}
	if stored != requestDigest {
		return Result{}, false, ErrCommandConflict
	}
	var result Result
	if err = json.Unmarshal(raw, &result); err != nil {
		return Result{}, false, err
	}
	return result, true, nil
}

func (r *PostgresRepository) Resolve(ctx context.Context, in Request) (Resolution, error) {
	var result Resolution
	err := r.pool.QueryRow(ctx, `SELECT s.local_party_id::text FROM payroll_ledger_owner_setting_revisions s JOIN parties p ON p.id=s.local_party_id AND p.revision=s.local_party_revision AND p.kind=s.local_party_kind AND p.legal_name=s.local_party_legal_name WHERE s.tenant_id=$1 ORDER BY s.revision DESC LIMIT 1`, in.TenantID).Scan(&result.OwnerPartyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Resolution{}, ErrOwnerMissing
	}
	if err != nil {
		return Resolution{}, err
	}
	err = r.pool.QueryRow(ctx, `SELECT m.local_party_id::text FROM payable_party_source_mappings m JOIN parties p ON p.id=m.local_party_id AND p.revision=m.local_party_revision AND p.kind=m.local_party_kind AND p.legal_name=m.local_party_legal_name WHERE m.tenant_id=$1 AND m.source_application='people-groups' AND m.source_resource_type='person' AND m.source_resource_id=$2 AND m.source_revision=$3 AND m.source_digest=$4`, in.TenantID, in.PersonID, in.PersonRevision, in.PersonDigest).Scan(&result.PersonPartyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Resolution{}, ErrPersonMapping
	}
	if err != nil {
		return Resolution{}, err
	}
	var binding Binding
	err = r.pool.QueryRow(ctx, `SELECT obligation_id::text,obligation_revision,total_net_payable_minor,payroll_line_id::text FROM payroll_payment_obligation_bindings WHERE tenant_id=$1 AND payroll_line_id=$2`, in.TenantID, in.PayrollLineID).Scan(&binding.ObligationID, &binding.ObligationRevision, &binding.TotalNetPayableMinor, &binding.PayrollLineID)
	if err == nil {
		result.Binding = &binding
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Resolution{}, err
	}
	return result, nil
}

func (r *PostgresRepository) Bind(ctx context.Context, in Request, obligationID string, revision int64) (Binding, error) {
	_, err := r.pool.Exec(ctx, `INSERT INTO payroll_payment_obligation_bindings(tenant_id,payroll_line_id,payroll_run_id,person_id,total_net_payable_minor,source_line_revision,source_line_digest,obligation_id,obligation_revision,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(tenant_id,payroll_line_id) DO NOTHING`, in.TenantID, in.PayrollLineID, in.PayrollRunID, in.PersonID, in.TotalNetPayableMinor, in.PayrollLineRevision, in.PayrollLineDigest, obligationID, revision, time.Now().UTC())
	if err != nil {
		return Binding{}, err
	}
	var value Binding
	err = r.pool.QueryRow(ctx, `SELECT obligation_id::text,obligation_revision,total_net_payable_minor,payroll_line_id::text FROM payroll_payment_obligation_bindings WHERE tenant_id=$1 AND payroll_line_id=$2`, in.TenantID, in.PayrollLineID).Scan(&value.ObligationID, &value.ObligationRevision, &value.TotalNetPayableMinor, &value.PayrollLineID)
	if err != nil {
		return Binding{}, err
	}
	if value.ObligationID != obligationID || value.TotalNetPayableMinor != in.TotalNetPayableMinor {
		return Binding{}, fmt.Errorf("payroll obligation binding conflict")
	}
	return value, nil
}

func (r *PostgresRepository) Record(ctx context.Context, in Request, requestDigest string, result Result, actor audit.ActorRef) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	var allocation any
	if result.MoneyAllocationID != "" {
		allocation = result.MoneyAllocationID
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO payroll_payment_deliveries(tenant_id,command_id,request_digest,payroll_run_id,payroll_line_id,payroll_line_revision,person_id,actual_amount_minor,money_payment_id,money_allocation_id,allocated_amount_minor,overage_amount_minor,response_snapshot,actor_kind,actor_id,recorded_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, in.TenantID, in.CommandID, requestDigest, in.PayrollRunID, in.PayrollLineID, in.PayrollLineRevision, in.PersonID, in.ActualAmountMinor, result.MoneyPaymentID, allocation, result.AllocatedAmountMinor, result.OverageAmountMinor, raw, actor.Kind, actor.ID, time.Now().UTC())
	return err
}

var _ Repository = (*PostgresRepository)(nil)
