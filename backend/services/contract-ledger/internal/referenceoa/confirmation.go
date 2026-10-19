package referenceoa

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/contract"
)

type ConfirmInput struct {
	CommandID                string         `json:"command_id"`
	ExpectedOverlayRevision  int64          `json:"expected_overlay_revision"`
	ExpectedContractRevision int64          `json:"expected_contract_revision"`
	EffectiveOn              string         `json:"effective_on"`
	Amendment                Amendment      `json:"amendment"`
	ContractID               string         `json:"-"`
	AmendmentID              string         `json:"-"`
	TenantID                 string         `json:"-"`
	Actor                    audit.ActorRef `json:"-"`
}
type Confirmation struct {
	CommandID        string `json:"command_id"`
	ContractID       string `json:"contract_id"`
	TenantID         string `json:"tenant_id"`
	AmendmentID      string `json:"amendment_id"`
	OverlayRevision  int64  `json:"overlay_revision"`
	ContractRevision int64  `json:"contract_revision"`
	InstrumentID     string `json:"instrument_id"`
	EffectiveAmount  string `json:"effective_amount"`
	EffectiveOn      string `json:"effective_on"`
	Replayed         bool   `json:"replayed"`
}

func normalizeConfirmation(in ConfirmInput) (ConfirmInput, string, string, error) {
	if !uuid.MatchString(in.AmendmentID) || in.Amendment.ID != in.AmendmentID || in.Amendment.Cancelled || in.ExpectedOverlayRevision < 1 || in.ExpectedContractRevision < 1 {
		return in, "", "", ErrInvalid
	}
	date, e := time.Parse("2006-01-02", in.EffectiveOn)
	if e != nil || date.Format("2006-01-02") != in.EffectiveOn {
		return in, "", "", ErrInvalid
	}
	// Reuse the typed customer's exact normalization; the zero base permits
	// negative adjustment inputs without wrongly treating them as preview.
	row := in.Amendment
	if row.ChangeType != "adjustment" && row.ChangeType != "revision" && row.ChangeType != "no_amount_change" {
		return in, "", "", ErrInvalid
	}
	normalized, total, e := NormalizeFees(row.Fees, row.ChangeType == "adjustment")
	if e != nil {
		return in, "", "", e
	}
	row.Fees = normalized
	row.Number = strings.TrimSpace(row.Number)
	row.Description = strings.TrimSpace(row.Description)
	signed, e := time.Parse("2006-01-02", row.SignedOn)
	if e != nil || signed.Format("2006-01-02") != row.SignedOn || row.Number == "" || len([]rune(row.Number)) > 100 || len([]rune(row.Description)) > 2000 || (row.RateBPS != nil && (*row.RateBPS < 0 || *row.RateBPS > 10000)) {
		return in, "", "", ErrInvalid
	}
	for _, id := range []string{in.CommandID, in.ContractID, in.TenantID} {
		if !uuid.MatchString(id) {
			return in, "", "", ErrInvalid
		}
	}
	actor, e := audit.NormalizeActor(in.Actor)
	if e != nil || actor.Kind != audit.ActorKindIAMSubject {
		return in, "", "", ErrInvalid
	}
	in.Actor = actor
	in.Amendment = row
	switch row.ChangeType {
	case "adjustment":
		if total == "0.00" {
			return in, "", "", ErrInvalid
		}
		return in, contract.AmountEffectAdjustment, total, nil
	case "revision":
		return in, contract.AmountEffectRestatement, total, nil
	default:
		if total != "0.00" {
			return in, "", "", ErrInvalid
		}
		return in, contract.AmountEffectUnchanged, "", nil
	}
}
func confirmationHash(in ConfirmInput) string {
	body, _ := json.Marshal(struct {
		Input                              ConfirmInput
		Contract, Tenant, Amendment, Actor string
	}{in, in.ContractID, in.TenantID, in.AmendmentID, in.Actor.ID})
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
func (s *Store) Confirmations(ctx context.Context, tenant, id string) ([]Confirmation, error) {
	if !uuid.MatchString(tenant) || !uuid.MatchString(id) {
		return nil, ErrInvalid
	}
	values := []Confirmation{}
	rows, e := s.pool.Query(ctx, `SELECT receipt FROM reference_oa_amendment_confirmations WHERE tenant_id=$1 AND contract_id=$2 ORDER BY contract_revision`, tenant, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var b []byte
		var v Confirmation
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(b, &v); e != nil {
			return nil, e
		}
		values = append(values, v)
	}
	return values, rows.Err()
}
func (s *Store) Confirm(ctx context.Context, input ConfirmInput) (Confirmation, error) {
	in, effect, amount, e := normalizeConfirmation(input)
	if e != nil {
		return Confirmation{}, e
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return Confirmation{}, e
	}
	defer tx.Rollback(context.Background())
	// Match native lock order: the same command lock precedes the contract row.
	// Taking it only after the row would deadlock a concurrent native replay.
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, in.CommandID); e != nil {
		return Confirmation{}, e
	}
	hash := confirmationHash(in)
	var priorHash string
	var prior []byte
	e = tx.QueryRow(ctx, `SELECT intent_sha256,receipt FROM reference_oa_amendment_confirmations WHERE command_id=$1`, in.CommandID).Scan(&priorHash, &prior)
	if e == nil {
		if priorHash != hash {
			return Confirmation{}, ErrConflict
		}
		var result Confirmation
		if e = json.Unmarshal(prior, &result); e != nil {
			return result, e
		}
		result.Replayed = true
		return result, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return Confirmation{}, e
	}
	var status, currency string
	var revision int64
	e = tx.QueryRow(ctx, `SELECT status,currency,current_revision FROM contracts WHERE id=$1 FOR UPDATE`, in.ContractID).Scan(&status, &currency, &revision)
	if errors.Is(e, pgx.ErrNoRows) {
		return Confirmation{}, ErrScope
	}
	if e != nil {
		return Confirmation{}, e
	}
	if status != "active" || currency != "CNY" {
		return Confirmation{}, ErrState
	}
	if revision != in.ExpectedContractRevision {
		return Confirmation{}, ErrConflict
	}
	var body []byte
	e = tx.QueryRow(ctx, `SELECT snapshot FROM reference_oa_contract_drafts WHERE contract_id=$1 FOR UPDATE`, in.ContractID).Scan(&body)
	if errors.Is(e, pgx.ErrNoRows) {
		return Confirmation{}, ErrScope
	}
	if e != nil {
		return Confirmation{}, e
	}
	var draft Draft
	if e = json.Unmarshal(body, &draft); e != nil {
		return Confirmation{}, e
	}
	if draft.TenantID != in.TenantID {
		return Confirmation{}, ErrScope
	}
	if draft.Revision != in.ExpectedOverlayRevision {
		return Confirmation{}, ErrConflict
	}
	confirmed, e := confirmedAmendmentIDs(ctx, tx, in.ContractID)
	if e != nil {
		return Confirmation{}, e
	}
	already := []Amendment{}
	found := false
	for _, row := range orderedAmendments(draft.Amendments) {
		if row.Cancelled {
			continue
		}
		if confirmed[row.ID] {
			already = append(already, row)
			continue
		}
		if row.ID != in.AmendmentID || !reflect.DeepEqual(row, in.Amendment) {
			return Confirmation{}, ErrConflict
		}
		found = true
		break
	}
	if !found {
		return Confirmation{}, ErrState
	}
	fees, e := PreviewFees(draft.Fees, already)
	if e != nil {
		return Confirmation{}, e
	}
	_, expectedAmount, e := NormalizeFees(fees, false)
	if e != nil {
		return Confirmation{}, e
	}
	var currentAmount string
	if e = tx.QueryRow(ctx, `SELECT COALESCE(total_amount::text,'') FROM contract_revisions WHERE contract_id=$1 AND revision=$2`, in.ContractID, revision).Scan(&currentAmount); e != nil {
		return Confirmation{}, e
	}
	if !equalAmounts(currentAmount, expectedAmount) {
		return Confirmation{}, ErrConflict
	}
	summary := in.Amendment.Description
	if summary == "" {
		summary = "明确确认参考 OA 补充协议"
	}
	// Native persistence runs in this same owner transaction. Its lifecycle,
	// date order, amount, snapshots, replay and audit rules remain unchanged.
	native, e := contract.NewService(contract.NewPostgresRepository(s.pool)).AppendChangeInTransaction(ctx, tx, contract.AppendContractChangeInput{CommandID: in.CommandID, ContractID: in.ContractID, ExpectedRevision: revision, InstrumentNumber: in.Amendment.Number, Title: in.Amendment.Number, ChangeSummary: summary, SignedOn: in.Amendment.SignedOn, EffectiveOn: in.EffectiveOn, EffectiveOrder: revision + 1, AmountEffect: effect, AmountValue: amount, Actor: in.Actor})
	if e != nil {
		return Confirmation{}, e
	}
	result := Confirmation{in.CommandID, in.ContractID, in.TenantID, in.AmendmentID, draft.Revision, native.Revision, native.InstrumentID, native.EffectiveAmount, in.EffectiveOn, false}
	receipt, e := json.Marshal(result)
	if e != nil {
		return result, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO reference_oa_amendment_confirmations(command_id,contract_id,tenant_id,amendment_id,overlay_revision,contract_revision,instrument_id,actor_id,intent_sha256,receipt,recorded_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, in.CommandID, in.ContractID, in.TenantID, in.AmendmentID, draft.Revision, native.Revision, native.InstrumentID, in.Actor.ID, hash, receipt, time.Now().UTC())
	if e != nil {
		return result, e
	}
	if e = tx.Commit(ctx); e != nil {
		return result, e
	}
	return result, nil
}
func confirmedAmendmentIDs(ctx context.Context, tx pgx.Tx, id string) (map[string]bool, error) {
	rows, e := tx.Query(ctx, `SELECT amendment_id::text FROM reference_oa_amendment_confirmations WHERE contract_id=$1`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	values := map[string]bool{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		values[id] = true
	}
	return values, rows.Err()
}
