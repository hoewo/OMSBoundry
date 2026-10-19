package referenceoa

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/contract"
)

type Store struct {
	pool     *pgxpool.Pool
	evidence EvidenceValidator
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }
func (s *Store) Read(ctx context.Context, tenant, id string) (*Draft, error) {
	if !uuid.MatchString(tenant) || !uuid.MatchString(id) {
		return nil, ErrInvalid
	}
	var body []byte
	var owner string
	err := s.pool.QueryRow(ctx, `SELECT tenant_id::text,snapshot FROM reference_oa_contract_drafts WHERE contract_id=$1`, id).Scan(&owner, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if owner != tenant {
		return nil, ErrScope
	}
	var value Draft
	if err = json.Unmarshal(body, &value); err != nil {
		return nil, err
	}
	return &value, nil
}
func (s *Store) Save(ctx context.Context, in SaveInput) (Result, error) {
	in, total, err := normalize(in)
	if err != nil {
		return Result{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(context.Background())
	// Same ordering as core commands: command lock then contract row lock.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('reference-oa:'||$1))`, in.CommandID); err != nil {
		return Result{}, err
	}
	hash := intentHash(in)
	var previousHash string
	var previous []byte
	err = tx.QueryRow(ctx, `SELECT intent_sha256,snapshot FROM reference_oa_contract_draft_commands WHERE command_id=$1`, in.CommandID).Scan(&previousHash, &previous)
	if err == nil {
		if previousHash != hash {
			return Result{}, ErrConflict
		}
		var result Draft
		if err = json.Unmarshal(previous, &result); err != nil {
			return Result{}, err
		}
		return Result{result, true}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Result{}, err
	}
	var status, currency, amount string
	var draftVersion int64
	err = tx.QueryRow(ctx, `SELECT status,currency,COALESCE(total_amount::text,''),draft_version FROM contracts WHERE id=$1 FOR UPDATE`, in.ContractID).Scan(&status, &currency, &amount, &draftVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, ErrScope
	}
	if err != nil {
		return Result{}, err
	}
	if (status != "draft" && status != "active") || currency != "CNY" {
		return Result{}, ErrState
	}
	// Core amount is NUMERIC(24,6); compare rationally, never by floating point.
	if draftVersion != in.ExpectedDraftVersion || !equalAmounts(total, amount) {
		return Result{}, ErrConflict
	}
	var oldBody []byte
	err = tx.QueryRow(ctx, `SELECT snapshot FROM reference_oa_contract_drafts WHERE contract_id=$1 FOR UPDATE`, in.ContractID).Scan(&oldBody)
	if err == nil {
		var old Draft
		if err = json.Unmarshal(oldBody, &old); err != nil {
			return Result{}, err
		}
		if old.TenantID != in.TenantID || old.ProjectID != in.ProjectID || old.ChainType != in.ChainType {
			return Result{}, ErrScope
		}
		if old.Revision != in.ExpectedRevision {
			return Result{}, ErrConflict
		}
		if status == "active" {
			var currentRevision int64
			if err = tx.QueryRow(ctx, `SELECT current_revision FROM contracts WHERE id=$1`, in.ContractID).Scan(&currentRevision); err != nil {
				return Result{}, err
			}
			if in.ExpectedContractRevision != currentRevision || old.Fees != in.Fees || !reflect.DeepEqual(old.RateBPS, in.RateBPS) {
				return Result{}, ErrConflict
			}
			confirmed, e := confirmedAmendmentIDs(ctx, tx, in.ContractID)
			if e != nil {
				return Result{}, e
			}
			for _, prior := range old.Amendments {
				if !confirmed[prior.ID] {
					continue
				}
				found := false
				for _, next := range in.Amendments {
					if next.ID == prior.ID {
						found = true
						if !reflect.DeepEqual(prior, next) {
							return Result{}, ErrState
						}
					}
				}
				if !found {
					return Result{}, ErrInvalid
				}
			}
			// Confirmed intentions form a prefix in the original fixed order.
			seenUnconfirmed := false
			for _, row := range orderedAmendments(in.Amendments) {
				if row.Cancelled {
					continue
				}
				if !confirmed[row.ID] {
					seenUnconfirmed = true
				} else if seenUnconfirmed {
					return Result{}, ErrConflict
				}
			}
		}
		// Removal is a tombstone, not history deletion; ended intention stays ended.
		for _, prior := range old.Amendments {
			found := false
			for _, next := range in.Amendments {
				if prior.ID == next.ID {
					found = true
					if prior.Cancelled && !next.Cancelled {
						return Result{}, ErrState
					}
				}
			}
			if !found {
				return Result{}, ErrInvalid
			}
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Result{}, err
	} else if in.ExpectedRevision != 0 || status != "draft" {
		return Result{}, ErrConflict
	}
	value := Draft{"1", in.ContractID, in.TenantID, in.ProjectID, in.ChainType, in.ExpectedRevision + 1, in.Fees, in.RateBPS, in.SettlementFollows, in.SettlementAmount, in.Amendments}
	body, err := json.Marshal(value)
	if err != nil {
		return Result{}, err
	}
	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `INSERT INTO reference_oa_contract_drafts(contract_id,tenant_id,project_id,chain_type,revision,snapshot,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(contract_id) DO UPDATE SET revision=EXCLUDED.revision,snapshot=EXCLUDED.snapshot,updated_at=EXCLUDED.updated_at`, in.ContractID, in.TenantID, in.ProjectID, in.ChainType, value.Revision, body, now)
	if err != nil {
		return Result{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO reference_oa_contract_draft_commands(command_id,contract_id,tenant_id,actor_id,intent_sha256,revision,snapshot,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, in.CommandID, in.ContractID, in.TenantID, in.Actor.ID, hash, value.Revision, body, now)
	if err != nil {
		return Result{}, err
	}
	auditID, err := contract.NewCommandID()
	if err != nil {
		return Result{}, err
	}
	if err = audit.Insert(ctx, tx, audit.Entry{ID: auditID, ObjectType: audit.ObjectContract, ObjectID: in.ContractID, Action: "reference_oa_draft_saved", CommandID: in.CommandID, Actor: in.Actor, OccurredAt: now, Summary: "保存参考 OA 合同客户扩展草稿", Details: map[string]any{"overlay_version": "1", "overlay_revision": value.Revision, "project_id": in.ProjectID, "chain_type": in.ChainType, "confirmed": false}}); err != nil {
		return Result{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return Result{value, false}, nil
}
func equalAmounts(a, b string) bool {
	left, lok := new(big.Rat).SetString(a)
	right, rok := new(big.Rat).SetString(b)
	return lok && rok && left.Cmp(right) == 0
}
