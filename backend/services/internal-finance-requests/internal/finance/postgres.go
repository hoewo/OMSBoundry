package finance

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) (*PostgresStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("pool is required")
	}
	return &PostgresStore{pool: pool}, nil
}
func (s *PostgresStore) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *PostgresStore) Create(ctx context.Context, actor Actor, input CreateInput, requestHash string) (Request, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return Request{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replay(ctx, tx, actor, input.CommandID, requestHash); replayErr != nil || found {
		return value, replayErr
	}
	now := time.Now().UTC()
	value := Request{ID: newUUID(), TenantID: actor.TenantID, ApplicantSubjectID: actor.ID, RequestType: input.RequestType, Title: input.Title, AmountMinor: input.AmountMinor, Currency: input.Currency, Status: "draft", Details: input.Details, ApprovalPolicy: "not_required", ApprovalStatus: "not_applicable", Revision: 1, CreatedAt: now, UpdatedAt: now}
	details, err := json.Marshal(value.Details)
	if err != nil {
		return Request{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO finance_requests(id,tenant_id,applicant_subject_id,request_type,title,amount_minor,currency,status,detail_snapshot,approval_policy,approval_status,paid_minor,allocated_minor,overage_minor,payment_unknown_count,revision,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,0,0,0,0,$12,$13,$14)`, value.ID, value.TenantID, value.ApplicantSubjectID, value.RequestType, value.Title, value.AmountMinor, value.Currency, value.Status, details, value.ApprovalPolicy, value.ApprovalStatus, value.Revision, value.CreatedAt, value.UpdatedAt)
	if err != nil {
		return Request{}, err
	}
	if err = insertRevision(ctx, tx, value, actor, input.CommandID, "created"); err != nil {
		return Request{}, err
	}
	if err = insertAudit(ctx, tx, value, actor, input.CommandID, 0, "finance_request_created"); err != nil {
		return Request{}, err
	}
	if err = record(ctx, tx, actor, input.CommandID, requestHash, value); err != nil {
		return Request{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Request{}, err
	}
	return value, nil
}

func (s *PostgresStore) Update(ctx context.Context, actor Actor, id string, input UpdateInput, requestHash string) (Request, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return Request{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replay(ctx, tx, actor, input.CommandID, requestHash); replayErr != nil || found {
		return value, replayErr
	}
	value, err := load(ctx, tx, actor.TenantID, id, "FOR UPDATE")
	if err != nil {
		return Request{}, err
	}
	if value.ApplicantSubjectID != actor.ID {
		return Request{}, ErrNotFound
	}
	if value.ArchivedAt != nil || value.DeletedAt != nil {
		return Request{}, ErrStateConflict
	}
	if value.Revision != input.ExpectedRevision {
		return Request{}, ErrRevisionConflict
	}
	if value.Status != "draft" && value.Status != "withdrawn" {
		return Request{}, ErrStateConflict
	}
	source := value.Revision
	value.Title, value.AmountMinor = input.Title, input.AmountMinor
	if len(input.Details.Lines) > 0 {
		value.Details = input.Details
	}
	value.Status = "draft"
	value.SubmittedAt = nil
	value.WithdrawnAt = nil
	value.WithdrawalReason = ""
	value.Revision++
	value.UpdatedAt = time.Now().UTC()
	details, _ := json.Marshal(value.Details)
	_, err = tx.Exec(ctx, `UPDATE finance_requests SET title=$3,amount_minor=$4,status=$5,detail_snapshot=$6,submitted_at=NULL,withdrawn_at=NULL,withdrawal_reason=NULL,revision=$7,updated_at=$8 WHERE tenant_id=$1 AND id=$2`, value.TenantID, value.ID, value.Title, value.AmountMinor, value.Status, details, value.Revision, value.UpdatedAt)
	if err != nil {
		return Request{}, err
	}
	if err = insertRevision(ctx, tx, value, actor, input.CommandID, "draft_updated"); err != nil {
		return Request{}, err
	}
	if err = insertAudit(ctx, tx, value, actor, input.CommandID, source, "finance_request_draft_updated"); err != nil {
		return Request{}, err
	}
	if err = record(ctx, tx, actor, input.CommandID, requestHash, value); err != nil {
		return Request{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Request{}, err
	}
	return value, nil
}

func (s *PostgresStore) Submit(ctx context.Context, actor Actor, id string, input SubmitInput, requestHash string) (Request, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return Request{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replay(ctx, tx, actor, input.CommandID, requestHash); replayErr != nil || found {
		return value, replayErr
	}
	value, err := load(ctx, tx, actor.TenantID, id, "FOR UPDATE")
	if err != nil {
		return Request{}, err
	}
	if value.ApplicantSubjectID != actor.ID {
		return Request{}, ErrNotFound
	}
	if value.ArchivedAt != nil || value.DeletedAt != nil {
		return Request{}, ErrStateConflict
	}
	if value.Revision != input.ExpectedRevision {
		return Request{}, ErrRevisionConflict
	}
	if value.Status != "draft" && value.Status != "withdrawn" {
		return Request{}, ErrStateConflict
	}
	source := value.Revision
	now := time.Now().UTC()
	if input.ValidatedDetails != nil {
		value.Details = *input.ValidatedDetails
	}
	assessment, err := assessInvoiceDuplicates(ctx, tx, value)
	if err != nil {
		return Request{}, err
	}
	value.Details.Duplicate = assessment
	if assessment.Status == "strong" {
		return Request{}, &DuplicateError{Strong: true, MatchedRequestIDs: assessment.MatchedRequestIDs}
	}
	if assessment.Status == "suspected" && !input.SuspectedDuplicateAcknowledged {
		return Request{}, &DuplicateError{MatchedRequestIDs: assessment.MatchedRequestIDs}
	}
	value.Details.MissingEvidenceAcknowledged = input.MissingEvidenceAcknowledged
	value.Details.Approval = ApprovalSnapshot{Policy: "not_required", Status: "not_applicable", Revision: 1, FrozenAt: now}
	value.ApprovalPolicy = "not_required"
	value.ApprovalStatus = "not_applicable"
	value.Status = "payable"
	value.SubmittedAt = &now
	value.WithdrawnAt = nil
	value.WithdrawalReason = ""
	value.Revision++
	value.UpdatedAt = now
	details, _ := json.Marshal(value.Details)
	_, err = tx.Exec(ctx, `UPDATE finance_requests SET status=$3,detail_snapshot=$4,approval_policy=$5,approval_status=$6,submitted_at=$7,withdrawn_at=NULL,withdrawal_reason=NULL,revision=$8,updated_at=$9 WHERE tenant_id=$1 AND id=$2`, value.TenantID, value.ID, value.Status, details, value.ApprovalPolicy, value.ApprovalStatus, value.SubmittedAt, value.Revision, value.UpdatedAt)
	if err != nil {
		return Request{}, err
	}
	if err = insertRevision(ctx, tx, value, actor, input.CommandID, "submitted"); err != nil {
		return Request{}, err
	}
	if err = insertInvoiceFingerprints(ctx, tx, value); err != nil {
		return Request{}, err
	}
	if err = insertAudit(ctx, tx, value, actor, input.CommandID, source, "finance_request_submitted"); err != nil {
		return Request{}, err
	}
	if err = record(ctx, tx, actor, input.CommandID, requestHash, value); err != nil {
		return Request{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Request{}, err
	}
	return value, nil
}

func assessInvoiceDuplicates(ctx context.Context, tx pgx.Tx, value Request) (DuplicateAssessment, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "invoice-duplicates:"+value.TenantID); err != nil {
		return DuplicateAssessment{}, err
	}
	return findInvoiceDuplicates(ctx, tx, value, time.Now().UTC())
}

func findInvoiceDuplicates(ctx context.Context, q querier, value Request, checkedAt time.Time) (DuplicateAssessment, error) {
	result := DuplicateAssessment{Status: "clear", CheckedAt: checkedAt}
	strong, suspected := map[string]bool{}, map[string]bool{}
	for _, line := range value.Details.Lines {
		if line.Invoice == nil {
			continue
		}
		for _, evidence := range line.Evidence {
			if evidence.SHA256 == "" {
				continue
			}
			rows, err := q.Query(ctx, `SELECT DISTINCT fingerprints.request_id::text FROM finance_invoice_fingerprints fingerprints JOIN finance_requests requests ON requests.tenant_id=fingerprints.tenant_id AND requests.id=fingerprints.request_id WHERE fingerprints.tenant_id=$1 AND fingerprints.request_id<>$2 AND fingerprints.evidence_sha256=$3 AND requests.deleted_at IS NULL`, value.TenantID, value.ID, evidence.SHA256)
			if err != nil {
				return DuplicateAssessment{}, err
			}
			if err = collectRequestIDs(rows, strong); err != nil {
				return DuplicateAssessment{}, err
			}
		}
		invoice := line.Invoice
		if invoice.Number != "" {
			rows, err := q.Query(ctx, `SELECT DISTINCT fingerprints.request_id::text FROM finance_invoice_fingerprints fingerprints JOIN finance_requests requests ON requests.tenant_id=fingerprints.tenant_id AND requests.id=fingerprints.request_id WHERE fingerprints.tenant_id=$1 AND fingerprints.request_id<>$2 AND fingerprints.invoice_code=$3 AND fingerprints.invoice_number=$4 AND requests.deleted_at IS NULL`, value.TenantID, value.ID, invoice.Code, invoice.Number)
			if err != nil {
				return DuplicateAssessment{}, err
			}
			if err = collectRequestIDs(rows, strong); err != nil {
				return DuplicateAssessment{}, err
			}
		}
		rows, err := q.Query(ctx, `SELECT DISTINCT fingerprints.request_id::text FROM finance_invoice_fingerprints fingerprints JOIN finance_requests requests ON requests.tenant_id=fingerprints.tenant_id AND requests.id=fingerprints.request_id WHERE fingerprints.tenant_id=$1 AND fingerprints.request_id<>$2 AND fingerprints.issuer_name=$3 AND fingerprints.invoice_date=$4::date AND fingerprints.amount_minor=$5 AND requests.deleted_at IS NULL`, value.TenantID, value.ID, line.Invoice.IssuerName, line.Invoice.InvoiceDate, line.Invoice.AmountMinor)
		if err != nil {
			return DuplicateAssessment{}, err
		}
		if err = collectRequestIDs(rows, suspected); err != nil {
			return DuplicateAssessment{}, err
		}
	}
	if len(strong) > 0 {
		result.Status, result.MatchedRequestIDs = "strong", sortedKeys(strong)
		return result, nil
	}
	if len(suspected) > 0 {
		result.Status, result.MatchedRequestIDs = "suspected", sortedKeys(suspected)
	}
	return result, nil
}

func projectInvoiceDuplicates(ctx context.Context, q querier, value *Request) error {
	if value.Status != "draft" && value.Status != "withdrawn" {
		return nil
	}
	assessment, err := findInvoiceDuplicates(ctx, q, *value, time.Time{})
	if err != nil {
		return err
	}
	value.Details.Duplicate = assessment
	return nil
}

func collectRequestIDs(rows pgx.Rows, target map[string]bool) error {
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		target[id] = true
	}
	return rows.Err()
}

func sortedKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}

func insertInvoiceFingerprints(ctx context.Context, tx pgx.Tx, value Request) error {
	for _, line := range value.Details.Lines {
		if line.Invoice == nil {
			continue
		}
		hashes := make([]string, 0, len(line.Evidence))
		for _, evidence := range line.Evidence {
			if evidence.SHA256 != "" {
				hashes = append(hashes, evidence.SHA256)
			}
		}
		if len(hashes) == 0 {
			hashes = append(hashes, "")
		}
		for _, digest := range hashes {
			_, err := tx.Exec(ctx, `INSERT INTO finance_invoice_fingerprints(tenant_id,request_id,request_revision,line_id,evidence_sha256,invoice_code,invoice_number,issuer_name,invoice_date,amount_minor,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::date,$10,$11)`, value.TenantID, value.ID, value.Revision, line.ID, digest, line.Invoice.Code, line.Invoice.Number, line.Invoice.IssuerName, line.Invoice.InvoiceDate, line.Invoice.AmountMinor, value.UpdatedAt)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *PostgresStore) Withdraw(ctx context.Context, actor Actor, id string, input WithdrawInput, requestHash string) (Request, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return Request{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replay(ctx, tx, actor, input.CommandID, requestHash); replayErr != nil || found {
		return value, replayErr
	}
	value, err := load(ctx, tx, actor.TenantID, id, "FOR UPDATE")
	if err != nil {
		return Request{}, err
	}
	if value.ApplicantSubjectID != actor.ID {
		return Request{}, ErrNotFound
	}
	if value.ArchivedAt != nil || value.DeletedAt != nil {
		return Request{}, ErrStateConflict
	}
	if value.Revision != input.ExpectedRevision {
		return Request{}, ErrRevisionConflict
	}
	if value.Status != "payable" || value.PaidMinor > 0 {
		return Request{}, ErrStateConflict
	}
	var blocked bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM finance_payment_deliveries WHERE tenant_id=$1 AND request_id=$2 AND status IN ('unknown','succeeded'))`, actor.TenantID, id).Scan(&blocked)
	if err != nil {
		return Request{}, err
	}
	if blocked {
		return Request{}, ErrStateConflict
	}
	source := value.Revision
	now := time.Now().UTC()
	value.Status = "withdrawn"
	value.WithdrawnAt = &now
	value.WithdrawalReason = input.Reason
	value.Revision++
	value.UpdatedAt = now
	_, err = tx.Exec(ctx, `UPDATE finance_requests SET status=$3,withdrawn_at=$4,withdrawal_reason=$5,revision=$6,updated_at=$7 WHERE tenant_id=$1 AND id=$2`, value.TenantID, value.ID, value.Status, value.WithdrawnAt, value.WithdrawalReason, value.Revision, value.UpdatedAt)
	if err != nil {
		return Request{}, err
	}
	if err = insertRevision(ctx, tx, value, actor, input.CommandID, "withdrawn"); err != nil {
		return Request{}, err
	}
	if err = insertAudit(ctx, tx, value, actor, input.CommandID, source, "finance_request_withdrawn"); err != nil {
		return Request{}, err
	}
	if err = record(ctx, tx, actor, input.CommandID, requestHash, value); err != nil {
		return Request{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Request{}, err
	}
	return value, nil
}

func (s *PostgresStore) Restore(ctx context.Context, actor Actor, id string, input RestoreInput, requestHash string) (Request, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return Request{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replay(ctx, tx, actor, input.CommandID, requestHash); replayErr != nil || found {
		return value, replayErr
	}
	value, err := load(ctx, tx, actor.TenantID, id, "FOR UPDATE")
	if err != nil {
		return Request{}, err
	}
	if value.ApplicantSubjectID != actor.ID {
		return Request{}, ErrNotFound
	}
	if value.ArchivedAt == nil || value.DeletedAt != nil {
		return Request{}, ErrStateConflict
	}
	source := value.Revision
	now := time.Now().UTC()
	value.ArchivedAt, value.UpdatedAt = nil, now
	value.Revision++
	if _, err = tx.Exec(ctx, `UPDATE finance_requests SET archived_at=NULL,revision=$3,updated_at=$4 WHERE tenant_id=$1 AND id=$2`, value.TenantID, value.ID, value.Revision, now); err != nil {
		return Request{}, err
	}
	if err = insertRevision(ctx, tx, value, actor, input.CommandID, "restored"); err != nil {
		return Request{}, err
	}
	if err = insertAudit(ctx, tx, value, actor, input.CommandID, source, "finance_request_draft_restored"); err != nil {
		return Request{}, err
	}
	if err = insertRetentionEvent(ctx, tx, value, actor, input.CommandID, "restored", `{}`); err != nil {
		return Request{}, err
	}
	if err = record(ctx, tx, actor, input.CommandID, requestHash, value); err != nil {
		return Request{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Request{}, err
	}
	return value, nil
}

func (s *PostgresStore) DeleteDraft(ctx context.Context, actor Actor, id string, input DeleteDraftInput, requestHash string) (Request, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return Request{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, replayErr := replay(ctx, tx, actor, input.CommandID, requestHash); replayErr != nil || found {
		return value, replayErr
	}
	value, err := load(ctx, tx, actor.TenantID, id, "FOR UPDATE")
	if err != nil {
		return Request{}, err
	}
	if !input.TenantWide && value.ApplicantSubjectID != actor.ID {
		return Request{}, ErrNotFound
	}
	if input.ExpectedRevision > 0 && value.Revision != input.ExpectedRevision {
		return Request{}, ErrRevisionConflict
	}
	var submitted, paymentStarted bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM finance_request_revisions WHERE tenant_id=$1 AND request_id=$2 AND event_kind='submitted'), EXISTS(SELECT 1 FROM finance_payment_deliveries WHERE tenant_id=$1 AND request_id=$2 AND status IN ('unknown','succeeded'))`, actor.TenantID, id).Scan(&submitted, &paymentStarted); err != nil {
		return Request{}, err
	}
	selfDraftDeletion := !input.TenantWide && value.Status == "draft" && !submitted
	tenantDeletion := input.TenantWide && (value.Status == "draft" || value.Status == "withdrawn" || value.Status == "payable")
	if value.DeletedAt != nil || value.PaidMinor != 0 || value.AllocatedMinor != 0 || value.OverageMinor != 0 || paymentStarted || (!selfDraftDeletion && !tenantDeletion) {
		return Request{}, ErrStateConflict
	}
	now := time.Now().UTC()
	value.DeletedAt, value.UpdatedAt = &now, now
	value.Deletion = &DeletionAudit{Reason: input.Reason, ActorSubjectID: actor.ID, CredentialID: actor.CredentialID, OccurredAt: now}
	if _, err = tx.Exec(ctx, `UPDATE finance_requests SET deleted_at=$3,updated_at=$3 WHERE tenant_id=$1 AND id=$2`, value.TenantID, value.ID, now); err != nil {
		return Request{}, err
	}
	details, err := json.Marshal(map[string]any{"confirmed": true, "reason": input.Reason})
	if err != nil {
		return Request{}, err
	}
	if err = insertRetentionEvent(ctx, tx, value, actor, input.CommandID, "deleted", string(details)); err != nil {
		return Request{}, err
	}
	if err = record(ctx, tx, actor, input.CommandID, requestHash, value); err != nil {
		return Request{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Request{}, err
	}
	return value, nil
}

func (s *PostgresStore) ArchiveStaleDrafts(ctx context.Context, before time.Time, limit int) (int, error) {
	rows, err := s.pool.Query(ctx, `SELECT tenant_id::text,id::text FROM finance_requests WHERE request_type='reimbursement' AND status='draft' AND archived_at IS NULL AND deleted_at IS NULL AND updated_at<$1 ORDER BY updated_at,id LIMIT $2`, before, limit)
	if err != nil {
		return 0, err
	}
	var targets [][2]string
	for rows.Next() {
		var target [2]string
		if err = rows.Scan(&target[0], &target[1]); err != nil {
			rows.Close()
			return 0, err
		}
		targets = append(targets, target)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	archived := 0
	for _, target := range targets {
		tx, beginErr := s.pool.Begin(ctx)
		if beginErr != nil {
			return archived, beginErr
		}
		now := time.Now().UTC()
		tag, updateErr := tx.Exec(ctx, `UPDATE finance_requests SET archived_at=$3,updated_at=$3 WHERE tenant_id=$1 AND id=$2 AND request_type='reimbursement' AND status='draft' AND archived_at IS NULL AND deleted_at IS NULL AND updated_at<$4`, target[0], target[1], now, before)
		if updateErr == nil && tag.RowsAffected() == 1 {
			_, updateErr = tx.Exec(ctx, `INSERT INTO finance_request_retention_events(id,tenant_id,request_id,action,command_id,actor_subject_id,credential_id,occurred_at,details) VALUES($1,$2,$3,'archived',NULL,NULL,'system:stale-draft-archive',$4,jsonb_build_object('stale_before',$5::timestamptz))`, newUUID(), target[0], target[1], now, before)
		}
		if updateErr != nil {
			_ = tx.Rollback(ctx)
			return archived, updateErr
		}
		if tag.RowsAffected() == 1 {
			archived++
		}
		if err = tx.Commit(ctx); err != nil {
			return archived, err
		}
	}
	return archived, nil
}

func (s *PostgresStore) PreparePayment(ctx context.Context, actor Actor, id string, input ConfirmPaymentInput, requestHash string) (Request, PaymentDelivery, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return Request{}, PaymentDelivery{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	value, err := load(ctx, tx, actor.TenantID, id, "FOR UPDATE")
	if err != nil {
		return Request{}, PaymentDelivery{}, err
	}
	delivery, found, err := loadDelivery(ctx, tx, actor.TenantID, input.CommandID)
	if err != nil {
		return Request{}, PaymentDelivery{}, err
	}
	now := time.Now().UTC()
	if found {
		var storedHash string
		err = tx.QueryRow(ctx, `SELECT request_hash FROM finance_payment_deliveries WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, input.CommandID).Scan(&storedHash)
		if err != nil {
			return Request{}, PaymentDelivery{}, err
		}
		if storedHash != requestHash {
			return Request{}, PaymentDelivery{}, ErrCommandConflict
		}
		if delivery.RequestID != id {
			return Request{}, PaymentDelivery{}, ErrCommandConflict
		}
		if delivery.Status == "succeeded" {
			return value, delivery, tx.Commit(ctx)
		}
		if delivery.Status == "failed" {
			return value, delivery, tx.Commit(ctx)
		}
		delivery.Status = "pending"
		delivery.AttemptCount++
		delivery.UpdatedAt = now
		_, err = tx.Exec(ctx, `UPDATE finance_payment_deliveries SET status='pending',attempt_count=$3,last_error_code='',updated_at=$4 WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, input.CommandID, delivery.AttemptCount, now)
		if err != nil {
			return Request{}, PaymentDelivery{}, err
		}
		if err = insertPaymentEvent(ctx, tx, actor, delivery, "pending", nil, ""); err != nil {
			return Request{}, PaymentDelivery{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return Request{}, PaymentDelivery{}, err
		}
		return value, delivery, nil
	}
	if value.Revision != input.ExpectedRevision {
		return Request{}, PaymentDelivery{}, ErrRevisionConflict
	}
	if value.PaymentUnknownCount > 0 {
		return Request{}, PaymentDelivery{}, ErrPaymentUnknown
	}
	if value.Status != "payable" && value.Status != "partially_paid" {
		return Request{}, PaymentDelivery{}, ErrStateConflict
	}
	remaining := value.AmountMinor - value.AllocatedMinor
	if input.ActualAmountMinor > remaining && (!input.OverageConfirmed || input.OverageReason == "") {
		return Request{}, PaymentDelivery{}, fmt.Errorf("invalid payment: overage requires confirmation and reason")
	}
	delivery = PaymentDelivery{CommandID: input.CommandID, RequestID: id, SourceRequestRevision: value.Revision, ActualAmountMinor: input.ActualAmountMinor, OccurredOn: input.OccurredOn, Note: input.Note, OverageReason: input.OverageReason, Status: "pending", AttemptCount: 1, OperatorSubjectID: actor.ID, CreatedAt: now, UpdatedAt: now}
	_, err = tx.Exec(ctx, `INSERT INTO finance_payment_deliveries(tenant_id,command_id,request_hash,request_id,source_request_revision,actual_amount_minor,occurred_on,note,overage_confirmed,overage_reason,status,attempt_count,actor_subject_id,credential_id,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'pending',1,$11,$12,$13,$13)`, actor.TenantID, input.CommandID, requestHash, id, value.Revision, input.ActualAmountMinor, input.OccurredOn, input.Note, input.OverageConfirmed, input.OverageReason, actor.ID, actor.CredentialID, now)
	if err != nil {
		return Request{}, PaymentDelivery{}, err
	}
	if err = insertPaymentEvent(ctx, tx, actor, delivery, "pending", nil, ""); err != nil {
		return Request{}, PaymentDelivery{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Request{}, PaymentDelivery{}, err
	}
	return value, delivery, nil
}

func (s *PostgresStore) MarkPaymentFailed(ctx context.Context, actor Actor, id, command, errorCode string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	now := time.Now().UTC()
	tag, err := tx.Exec(ctx, `UPDATE finance_payment_deliveries SET status='failed',last_error_code=$4,updated_at=$5 WHERE tenant_id=$1 AND command_id=$2 AND request_id=$3 AND status='pending'`, actor.TenantID, command, id, errorCode, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	delivery, _, err := loadDelivery(ctx, tx, actor.TenantID, command)
	if err != nil {
		return err
	}
	if err = insertPaymentEvent(ctx, tx, actor, delivery, "failed", nil, errorCode); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) MarkPaymentUnknown(ctx context.Context, actor Actor, id, command string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	now := time.Now().UTC()
	tag, err := tx.Exec(ctx, `UPDATE finance_payment_deliveries SET status='unknown',last_error_code='MONEY_OUTCOME_UNKNOWN',updated_at=$4 WHERE tenant_id=$1 AND command_id=$2 AND request_id=$3 AND status='pending'`, actor.TenantID, command, id, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	delivery, _, err := loadDelivery(ctx, tx, actor.TenantID, command)
	if err != nil {
		return err
	}
	if err = insertPaymentEvent(ctx, tx, actor, delivery, "unknown", nil, "MONEY_OUTCOME_UNKNOWN"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) CompletePayment(ctx context.Context, actor Actor, id, command string, result MoneyPaymentResult) (Request, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Request{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, actor.TenantID+":"+command)
	if err != nil {
		return Request{}, err
	}
	delivery, found, err := loadDelivery(ctx, tx, actor.TenantID, command)
	if err != nil {
		return Request{}, err
	}
	if !found || delivery.RequestID != id {
		return Request{}, ErrNotFound
	}
	value, err := load(ctx, tx, actor.TenantID, id, "FOR UPDATE")
	if err != nil {
		return Request{}, err
	}
	if delivery.Status == "succeeded" {
		return value, tx.Commit(ctx)
	}
	if result.Status != "recorded" || result.MoneyPaymentID == "" || result.MoneyObligationID == "" || result.AllocatedAmountMinor < 0 || result.OverageAmountMinor < 0 || result.AllocatedAmountMinor+result.OverageAmountMinor != delivery.ActualAmountMinor {
		return Request{}, fmt.Errorf("invalid Money payment result")
	}
	now := time.Now().UTC()
	delivery.Status = "succeeded"
	delivery.AllocatedAmountMinor = result.AllocatedAmountMinor
	delivery.OverageAmountMinor = result.OverageAmountMinor
	delivery.MoneyPaymentID = result.MoneyPaymentID
	delivery.MoneyPaymentRevision = result.MoneyPaymentRevision
	delivery.MoneyAllocationID = result.MoneyAllocationID
	delivery.MoneyAllocationRevision = result.MoneyAllocationRevision
	delivery.MoneyObligationID = result.MoneyObligationID
	delivery.MoneyObligationRevision = result.MoneyObligationRevision
	delivery.UpdatedAt = now
	_, err = tx.Exec(ctx, `UPDATE finance_payment_deliveries SET status='succeeded',money_payment_id=$3,money_payment_revision=$4,money_allocation_id=NULLIF($5,'')::uuid,money_allocation_revision=NULLIF($6,0),money_obligation_id=$7,money_obligation_revision=$8,allocated_amount_minor=$9,overage_amount_minor=$10,last_error_code='',updated_at=$11 WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, command, result.MoneyPaymentID, result.MoneyPaymentRevision, result.MoneyAllocationID, result.MoneyAllocationRevision, result.MoneyObligationID, result.MoneyObligationRevision, result.AllocatedAmountMinor, result.OverageAmountMinor, now)
	if err != nil {
		return Request{}, err
	}
	source := value.Revision
	value.PaidMinor += delivery.ActualAmountMinor
	value.AllocatedMinor += result.AllocatedAmountMinor
	value.OverageMinor += result.OverageAmountMinor
	if value.AllocatedMinor > value.AmountMinor {
		return Request{}, fmt.Errorf("Money allocation exceeds finance request")
	}
	if value.AllocatedMinor == value.AmountMinor {
		if value.OverageMinor > 0 {
			value.Status = "paid_with_overpayment"
		} else {
			value.Status = "paid"
		}
	} else {
		value.Status = "partially_paid"
	}
	value.Revision++
	value.UpdatedAt = now
	_, err = tx.Exec(ctx, `UPDATE finance_requests SET paid_minor=$3,allocated_minor=$4,overage_minor=$5,status=$6,revision=$7,updated_at=$8 WHERE tenant_id=$1 AND id=$2`, value.TenantID, value.ID, value.PaidMinor, value.AllocatedMinor, value.OverageMinor, value.Status, value.Revision, value.UpdatedAt)
	if err != nil {
		return Request{}, err
	}
	if err = insertRevision(ctx, tx, value, actor, command, "payment_recorded"); err != nil {
		return Request{}, err
	}
	if err = insertAudit(ctx, tx, value, actor, command, source, "finance_request_payment_recorded"); err != nil {
		return Request{}, err
	}
	raw, _ := json.Marshal(result)
	if err = insertPaymentEvent(ctx, tx, actor, delivery, "succeeded", raw, ""); err != nil {
		return Request{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Request{}, err
	}
	// Reload the current projection after commit. The value loaded before the
	// Money call still contains the pending delivery, so appending the completed
	// form would expose two apparent payments for one command.
	return load(ctx, s.pool, actor.TenantID, id, "")
}

func (s *PostgresStore) Get(ctx context.Context, tenant, id string) (Request, error) {
	value, err := load(ctx, s.pool, tenant, id, "")
	if err == nil && value.DeletedAt != nil {
		return Request{}, ErrNotFound
	}
	if err == nil {
		err = projectInvoiceDuplicates(ctx, s.pool, &value)
	}
	return value, err
}
func (s *PostgresStore) List(ctx context.Context, tenant string, filter ListFilter) ([]Request, error) {
	limit := filter.Limit
	if limit < 1 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, selectRequest+` WHERE tenant_id=$1
		AND ($2='' OR applicant_subject_id=NULLIF($2,'')::uuid)
		AND ($3='' OR request_type=$3)
		AND ($4='' OR status=$4)
		AND ($5='' OR submitted_at::date >= NULLIF($5,'')::date)
		AND ($6='' OR submitted_at::date <= NULLIF($6,'')::date)
		AND ($7::timestamptz IS NULL OR updated_at > $7::timestamptz)
		AND ($8::timestamptz IS NULL OR updated_at < $8::timestamptz OR (updated_at=$8::timestamptz AND id<NULLIF($9,'')::uuid))
		AND ($10 OR archived_at IS NULL)
		AND ($11 OR deleted_at IS NULL)
		ORDER BY updated_at DESC,id DESC LIMIT $12`, tenant, filter.ApplicantSubjectID, filter.RequestType, filter.Status, filter.SubmittedFrom, filter.SubmittedTo, filter.UpdatedSince, filter.BeforeUpdatedAt, filter.BeforeID, filter.IncludeArchived, filter.IncludeDeleted, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []Request
	for rows.Next() {
		value, scanErr := scan(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		if scanErr = loadPayments(ctx, s.pool, &value); scanErr != nil {
			return nil, scanErr
		}
		if scanErr = loadDeletion(ctx, s.pool, &value); scanErr != nil {
			return nil, scanErr
		}
		if scanErr = projectInvoiceDuplicates(ctx, s.pool, &value); scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *PostgresStore) RecordSensitiveAccess(ctx context.Context, actor Actor, id, purpose, result string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO finance_sensitive_access_audit_events(id,tenant_id,request_id,actor_subject_id,credential_id,purpose,result,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, newUUID(), actor.TenantID, id, actor.ID, actor.CredentialID, purpose, result, time.Now().UTC())
	return err
}

func (s *PostgresStore) IsEvidenceReferenced(ctx context.Context, tenant, fileID string, version int) (bool, error) {
	var referenced bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1
		FROM finance_request_revisions r
		CROSS JOIN LATERAL jsonb_array_elements(COALESCE(r.detail_snapshot->'lines','[]'::jsonb)) line
		CROSS JOIN LATERAL jsonb_array_elements(COALESCE(line->'evidence','[]'::jsonb)) evidence
		WHERE r.tenant_id=$1
		  AND evidence->>'file_id'=$2
		  AND CASE WHEN evidence->>'version' ~ '^[0-9]+$' THEN (evidence->>'version')::integer=$3 ELSE false END
	)`, tenant, fileID, version).Scan(&referenced)
	return referenced, err
}

func (s *PostgresStore) CreateCategory(ctx context.Context, actor Actor, id string, input CategoryCreateInput, requestHash string) (ExpenseCategory, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return ExpenseCategory{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if replayed, found, replayErr := replayCategory(ctx, tx, actor.TenantID, input.CommandID, requestHash); replayErr != nil || found {
		if replayErr == nil {
			replayErr = tx.Commit(ctx)
		}
		return replayed, replayErr
	}
	value := ExpenseCategory{ID: id, Key: input.Key, DisplayName: input.DisplayName, Source: "tenant", Enabled: true, Revision: 1, UpdatedAt: time.Now().UTC()}
	_, err = tx.Exec(ctx, `INSERT INTO finance_expense_categories(tenant_id,id,category_key,display_name,source,enabled,revision,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, actor.TenantID, value.ID, value.Key, value.DisplayName, value.Source, value.Enabled, value.Revision, value.UpdatedAt)
	if err != nil {
		return ExpenseCategory{}, err
	}
	if err = insertCategoryRevision(ctx, tx, actor, value, input.CommandID, input.Reason, requestHash); err != nil {
		return ExpenseCategory{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ExpenseCategory{}, err
	}
	return value, nil
}

func (s *PostgresStore) UpdateCategory(ctx context.Context, actor Actor, base ExpenseCategory, input CategoryUpdateInput, requestHash string) (ExpenseCategory, error) {
	tx, err := s.begin(ctx, actor.TenantID, input.CommandID)
	if err != nil {
		return ExpenseCategory{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if replayed, found, replayErr := replayCategory(ctx, tx, actor.TenantID, input.CommandID, requestHash); replayErr != nil || found {
		if replayErr == nil {
			replayErr = tx.Commit(ctx)
		}
		return replayed, replayErr
	}
	current, found, err := loadCategory(ctx, tx, actor.TenantID, base.ID, "FOR UPDATE")
	if err != nil {
		return ExpenseCategory{}, err
	}
	if !found {
		if base.Source != "platform" || input.ExpectedRevision != base.Revision {
			return ExpenseCategory{}, ErrNotFound
		}
		current = base
		current.DisplayName = input.DisplayName
		current.Enabled = input.Enabled
		current.Revision++
		current.UpdatedAt = time.Now().UTC()
		_, err = tx.Exec(ctx, `INSERT INTO finance_expense_categories(tenant_id,id,category_key,display_name,source,enabled,revision,updated_at) VALUES($1,$2,$3,$4,'platform',$5,$6,$7)`, actor.TenantID, current.ID, current.Key, current.DisplayName, current.Enabled, current.Revision, current.UpdatedAt)
	} else {
		if current.Revision != input.ExpectedRevision {
			return ExpenseCategory{}, ErrRevisionConflict
		}
		if current.Source == "platform" && current.DisplayName != input.DisplayName {
			return ExpenseCategory{}, fmt.Errorf("invalid expense category: platform name cannot be changed")
		}
		current.DisplayName, current.Enabled, current.Revision, current.UpdatedAt = input.DisplayName, input.Enabled, current.Revision+1, time.Now().UTC()
		_, err = tx.Exec(ctx, `UPDATE finance_expense_categories SET display_name=$3,enabled=$4,revision=$5,updated_at=$6 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, current.ID, current.DisplayName, current.Enabled, current.Revision, current.UpdatedAt)
	}
	if err != nil {
		return ExpenseCategory{}, err
	}
	if err = insertCategoryRevision(ctx, tx, actor, current, input.CommandID, input.Reason, requestHash); err != nil {
		return ExpenseCategory{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ExpenseCategory{}, err
	}
	return current, nil
}

func (s *PostgresStore) ListCategories(ctx context.Context, tenant string) ([]ExpenseCategory, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,category_key,display_name,source,enabled,revision,updated_at FROM finance_expense_categories WHERE tenant_id=$1 ORDER BY category_key,id`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []ExpenseCategory
	for rows.Next() {
		var value ExpenseCategory
		if err = rows.Scan(&value.ID, &value.Key, &value.DisplayName, &value.Source, &value.Enabled, &value.Revision, &value.UpdatedAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func replayCategory(ctx context.Context, tx pgx.Tx, tenant, command, requestHash string) (ExpenseCategory, bool, error) {
	var stored, id string
	err := tx.QueryRow(ctx, `SELECT request_hash,category_id::text FROM finance_expense_category_revisions WHERE tenant_id=$1 AND command_id=$2`, tenant, command).Scan(&stored, &id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ExpenseCategory{}, false, nil
	}
	if err != nil {
		return ExpenseCategory{}, false, err
	}
	if stored != requestHash {
		return ExpenseCategory{}, false, ErrCommandConflict
	}
	value, found, err := loadCategory(ctx, tx, tenant, id, "")
	return value, found, err
}

func loadCategory(ctx context.Context, tx pgx.Tx, tenant, id, suffix string) (ExpenseCategory, bool, error) {
	var value ExpenseCategory
	err := tx.QueryRow(ctx, `SELECT id::text,category_key,display_name,source,enabled,revision,updated_at FROM finance_expense_categories WHERE tenant_id=$1 AND id=$2 `+suffix, tenant, id).Scan(&value.ID, &value.Key, &value.DisplayName, &value.Source, &value.Enabled, &value.Revision, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ExpenseCategory{}, false, nil
	}
	return value, err == nil, err
}

func insertCategoryRevision(ctx context.Context, tx pgx.Tx, actor Actor, value ExpenseCategory, command, reason, requestHash string) error {
	_, err := tx.Exec(ctx, `INSERT INTO finance_expense_category_revisions(tenant_id,category_id,revision,command_id,category_key,display_name,enabled,reason,request_hash,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, actor.TenantID, value.ID, value.Revision, command, value.Key, value.DisplayName, value.Enabled, reason, requestHash, actor.ID, actor.CredentialID, value.UpdatedAt)
	return err
}

func (s *PostgresStore) begin(ctx context.Context, tenant, command string) (pgx.Tx, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, tenant+":"+command); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, err
	}
	return tx, nil
}

func replay(ctx context.Context, tx pgx.Tx, actor Actor, command, requestHash string) (Request, bool, error) {
	var stored, storedActor string
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT request_hash,actor_subject_id::text,response_json FROM finance_request_commands WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, command).Scan(&stored, &storedActor, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, false, nil
	}
	if err != nil {
		return Request{}, false, err
	}
	if stored != requestHash || storedActor != actor.ID {
		return Request{}, false, ErrCommandConflict
	}
	var value Request
	if err = json.Unmarshal(raw, &value); err != nil {
		return Request{}, false, err
	}
	return value, true, nil
}
func record(ctx context.Context, tx pgx.Tx, actor Actor, command, requestHash string, value Request) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO finance_request_commands(tenant_id,command_id,request_hash,actor_subject_id,response_json,created_at) VALUES($1,$2,$3,$4,$5,$6)`, actor.TenantID, command, requestHash, actor.ID, raw, value.UpdatedAt)
	return err
}
func insertRevision(ctx context.Context, tx pgx.Tx, value Request, actor Actor, command, event string) error {
	details, err := json.Marshal(value.Details)
	if err != nil {
		return err
	}
	// Unknown delivery count is a live projection over the delivery table rather
	// than a business revision fact. Keep the persisted current/revision value at
	// zero so resolving a different delivery cannot violate revision consistency.
	_, err = tx.Exec(ctx, `INSERT INTO finance_request_revisions(tenant_id,request_id,revision,command_id,event_kind,applicant_subject_id,request_type,title,amount_minor,currency,status,detail_snapshot,approval_policy,approval_status,paid_minor,allocated_minor,overage_minor,payment_unknown_count,submitted_at,withdrawn_at,withdrawal_reason,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,0,$18,$19,$20,$21,$22,$23)`, value.TenantID, value.ID, value.Revision, command, event, value.ApplicantSubjectID, value.RequestType, value.Title, value.AmountMinor, value.Currency, value.Status, details, value.ApprovalPolicy, value.ApprovalStatus, value.PaidMinor, value.AllocatedMinor, value.OverageMinor, value.SubmittedAt, value.WithdrawnAt, nullString(value.WithdrawalReason), actor.ID, actor.CredentialID, value.UpdatedAt)
	return err
}
func insertAudit(ctx context.Context, tx pgx.Tx, value Request, actor Actor, command string, source int64, action string) error {
	_, err := tx.Exec(ctx, `INSERT INTO finance_request_audit_events(id,tenant_id,request_id,command_id,source_revision,result_revision,action,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, newUUID(), value.TenantID, value.ID, command, source, value.Revision, action, actor.ID, actor.CredentialID, value.UpdatedAt)
	return err
}
func insertPaymentEvent(ctx context.Context, tx pgx.Tx, actor Actor, d PaymentDelivery, status string, response []byte, errorCode string) error {
	if len(response) == 0 {
		response = []byte(`{}`)
	}
	_, err := tx.Exec(ctx, `INSERT INTO finance_payment_delivery_events(tenant_id,command_id,event_sequence,status,response_snapshot,error_code,actor_subject_id,credential_id,occurred_at) SELECT $1,$2,COALESCE(max(event_sequence),0)+1,$3,$4,$5,$6,$7,$8 FROM finance_payment_delivery_events WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, d.CommandID, status, response, errorCode, actor.ID, actor.CredentialID, time.Now().UTC())
	return err
}

func insertRetentionEvent(ctx context.Context, tx pgx.Tx, value Request, actor Actor, command, action, details string) error {
	_, err := tx.Exec(ctx, `INSERT INTO finance_request_retention_events(id,tenant_id,request_id,action,command_id,actor_subject_id,credential_id,occurred_at,details) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb)`, newUUID(), value.TenantID, value.ID, action, command, actor.ID, actor.CredentialID, value.UpdatedAt, details)
	return err
}

const selectRequest = `SELECT id,tenant_id,applicant_subject_id,request_type,title,amount_minor,currency,status,detail_snapshot,approval_policy,approval_status,paid_minor,allocated_minor,overage_minor,payment_unknown_count,submitted_at,withdrawn_at,COALESCE(withdrawal_reason,''),revision,created_at,updated_at,archived_at,deleted_at FROM finance_requests`

type scanner interface{ Scan(...any) error }

func scan(row scanner) (Request, error) {
	var value Request
	var details []byte
	err := row.Scan(&value.ID, &value.TenantID, &value.ApplicantSubjectID, &value.RequestType, &value.Title, &value.AmountMinor, &value.Currency, &value.Status, &details, &value.ApprovalPolicy, &value.ApprovalStatus, &value.PaidMinor, &value.AllocatedMinor, &value.OverageMinor, &value.PaymentUnknownCount, &value.SubmittedAt, &value.WithdrawnAt, &value.WithdrawalReason, &value.Revision, &value.CreatedAt, &value.UpdatedAt, &value.ArchivedAt, &value.DeletedAt)
	if err == nil {
		err = json.Unmarshal(details, &value.Details)
	}
	return value, err
}

type querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func load(ctx context.Context, q querier, tenant, id, suffix string) (Request, error) {
	value, err := scan(q.QueryRow(ctx, selectRequest+` WHERE tenant_id=$1 AND id=$2 `+suffix, tenant, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, ErrNotFound
	}
	if err != nil {
		return Request{}, err
	}
	if err = loadPayments(ctx, q, &value); err != nil {
		return Request{}, err
	}
	if err = loadDeletion(ctx, q, &value); err != nil {
		return Request{}, err
	}
	return value, nil
}
func loadPayments(ctx context.Context, q querier, value *Request) error {
	rows, err := q.Query(ctx, `SELECT command_id::text,request_id::text,source_request_revision,actual_amount_minor,allocated_amount_minor,overage_amount_minor,occurred_on::text,note,overage_reason,status,attempt_count,COALESCE(money_payment_id::text,''),COALESCE(money_payment_revision,0),COALESCE(money_allocation_id::text,''),COALESCE(money_allocation_revision,0),COALESCE(money_obligation_id::text,''),COALESCE(money_obligation_revision,0),last_error_code,actor_subject_id::text,created_at,updated_at FROM finance_payment_deliveries WHERE tenant_id=$1 AND request_id=$2 ORDER BY created_at,command_id`, value.TenantID, value.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	value.PaymentUnknownCount = 0
	for rows.Next() {
		var d PaymentDelivery
		if err = rows.Scan(&d.CommandID, &d.RequestID, &d.SourceRequestRevision, &d.ActualAmountMinor, &d.AllocatedAmountMinor, &d.OverageAmountMinor, &d.OccurredOn, &d.Note, &d.OverageReason, &d.Status, &d.AttemptCount, &d.MoneyPaymentID, &d.MoneyPaymentRevision, &d.MoneyAllocationID, &d.MoneyAllocationRevision, &d.MoneyObligationID, &d.MoneyObligationRevision, &d.LastErrorCode, &d.OperatorSubjectID, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return err
		}
		if d.Status == "unknown" {
			value.PaymentUnknownCount++
		}
		value.Payments = append(value.Payments, d)
	}
	return rows.Err()
}

func loadDeletion(ctx context.Context, q querier, value *Request) error {
	if value.DeletedAt == nil {
		return nil
	}
	var deletion DeletionAudit
	err := q.QueryRow(ctx, `SELECT COALESCE(details->>'reason',''),actor_subject_id::text,credential_id,occurred_at FROM finance_request_retention_events WHERE tenant_id=$1 AND request_id=$2 AND action='deleted' ORDER BY occurred_at DESC,id DESC LIMIT 1`, value.TenantID, value.ID).Scan(&deletion.Reason, &deletion.ActorSubjectID, &deletion.CredentialID, &deletion.OccurredAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err == nil {
		value.Deletion = &deletion
	}
	return err
}
func loadDelivery(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenant, command string) (PaymentDelivery, bool, error) {
	var d PaymentDelivery
	err := q.QueryRow(ctx, `SELECT command_id::text,request_id::text,source_request_revision,actual_amount_minor,allocated_amount_minor,overage_amount_minor,occurred_on::text,note,overage_reason,status,attempt_count,COALESCE(money_payment_id::text,''),COALESCE(money_payment_revision,0),COALESCE(money_allocation_id::text,''),COALESCE(money_allocation_revision,0),COALESCE(money_obligation_id::text,''),COALESCE(money_obligation_revision,0),last_error_code,actor_subject_id::text,created_at,updated_at FROM finance_payment_deliveries WHERE tenant_id=$1 AND command_id=$2`, tenant, command).Scan(&d.CommandID, &d.RequestID, &d.SourceRequestRevision, &d.ActualAmountMinor, &d.AllocatedAmountMinor, &d.OverageAmountMinor, &d.OccurredOn, &d.Note, &d.OverageReason, &d.Status, &d.AttemptCount, &d.MoneyPaymentID, &d.MoneyPaymentRevision, &d.MoneyAllocationID, &d.MoneyAllocationRevision, &d.MoneyObligationID, &d.MoneyObligationRevision, &d.LastErrorCode, &d.OperatorSubjectID, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PaymentDelivery{}, false, nil
	}
	return d, err == nil, err
}
func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func newUUID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}

var _ Store = (*PostgresStore)(nil)
