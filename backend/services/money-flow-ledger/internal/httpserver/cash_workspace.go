package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/cashevidence"
	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/identity"
	"omsbizfoundry/money-flow-ledger/internal/partymigration"
)

// The JSON ports call existing Money use cases, never Project or OA tables.
func (s *Server) registerCashWorkspace(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/cash-workspace", s.cashWorkspaceAuth(identity.PermissionView, false, true, http.HandlerFunc(s.cashWorkspaceContext)))
	mux.Handle("GET /api/v1/cash-transactions", s.cashWorkspaceAuth(identity.PermissionView, false, true, http.HandlerFunc(s.cashWorkspaceList)))
	mux.Handle("POST /api/v1/cash-transactions", s.cashWorkspaceAuth(identity.PermissionCashRecord, true, true, http.HandlerFunc(s.cashWorkspaceCreate)))
	mux.Handle("GET /api/v1/cash-transactions/{cashTransactionID}", s.cashWorkspaceAuth(identity.PermissionView, false, false, http.HandlerFunc(s.cashWorkspaceGet)))
	mux.Handle("GET /api/v1/cash-transactions/{cashTransactionID}/export-view", s.cashWorkspaceAuth(identity.PermissionExport, true, false, http.HandlerFunc(s.cashWorkspaceGet)))
	mux.Handle("POST /api/v1/cash-transactions/{cashTransactionID}/reversal", s.cashWorkspaceAuth(identity.PermissionCashReverse, true, false, http.HandlerFunc(s.cashWorkspaceReverse)))
	mux.Handle("GET /api/v1/cash-transactions/{cashTransactionID}/evidence", s.cashWorkspaceAuth(identity.PermissionView, false, false, http.HandlerFunc(s.cashWorkspaceEvidence)))
	mux.Handle("PUT /api/v1/cash-transactions/{cashTransactionID}/evidence", s.cashWorkspaceAuth(identity.PermissionCashEvidenceManage, false, false, http.HandlerFunc(s.cashWorkspaceReplaceEvidence)))
}

func (s *Server) cashWorkspaceList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	for key := range query {
		switch key {
		case "type", "status", "occurred_from", "occurred_to", "source_reference", "offset", "limit":
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported cash list query"})
			return
		}
	}
	transactionType := strings.TrimSpace(query.Get("type"))
	if transactionType != "" && transactionType != string(cashtransaction.TypeReceipt) && transactionType != string(cashtransaction.TypePayment) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash transaction type"})
		return
	}
	status := strings.TrimSpace(query.Get("status"))
	if status == "" {
		status = cashtransaction.StatusRecorded
	}
	if status != "all" && status != cashtransaction.StatusRecorded && status != cashtransaction.StatusReversed {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash transaction status"})
		return
	}
	from, ok := cashListDate(w, query.Get("occurred_from"))
	if !ok {
		return
	}
	to, ok := cashListDate(w, query.Get("occurred_to"))
	if !ok {
		return
	}
	if from != "" && to != "" && from > to {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cash date range is reversed"})
		return
	}
	offset, ok := cashListInteger(w, query.Get("offset"), 0, 0, 1_000_000)
	if !ok {
		return
	}
	limit, ok := cashListInteger(w, query.Get("limit"), 20, 1, 100)
	if !ok {
		return
	}
	values, err := s.cashTransactions.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "cash transaction list unavailable"})
		return
	}
	sourceReference := strings.TrimSpace(query.Get("source_reference"))
	filtered := make([]cashtransaction.Transaction, 0, len(values))
	for _, value := range values {
		if transactionType != "" && string(value.Type) != transactionType || status != "all" && value.Status != status || from != "" && value.OccurredOn < from || to != "" && value.OccurredOn > to || sourceReference != "" && value.ManualSourceReference != sourceReference {
			continue
		}
		filtered = append(filtered, value)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].OccurredOn != filtered[j].OccurredOn {
			return filtered[i].OccurredOn > filtered[j].OccurredOn
		}
		if !filtered[i].RecordedAt.Equal(filtered[j].RecordedAt) {
			return filtered[i].RecordedAt.After(filtered[j].RecordedAt)
		}
		return filtered[i].ID > filtered[j].ID
	})
	total := len(filtered)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	p, _ := principalFromContext(r.Context())
	items, err := s.cashViews(r.Context(), p, filtered[offset:end])
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Money Party references unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"schema_version":     "1",
		"source_application": serviceName,
		"tenant_id":          p.TenantID,
		"items":              items,
		"total":              total,
		"offset":             offset,
		"limit":              limit,
	})
}

func cashListDate(w http.ResponseWriter, raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", true
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash list date"})
		return "", false
	}
	return value, true
}

func cashListInteger(w http.ResponseWriter, raw string, fallback, minimum, maximum int) (int, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minimum || parsed > maximum {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash list pagination"})
		return 0, false
	}
	return parsed, true
}

func (s *Server) cashWorkspaceAuth(permission identity.Permission, amount, tenantOnly bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, header := range []string{"X-Actor-ID", "X-Actor-Name", "X-Tenant-ID", "X-IAM-Subject", "X-Allowed-Actions", "X-Capability-Scope"} {
			r.Header.Del(header)
		}
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			writeJSON(w, 401, map[string]string{"error": "bearer credential required"})
			return
		}
		p, found, err := s.identities.ResolveSession(r.Context(), token)
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "credential verification unavailable"})
			return
		}
		if !found {
			writeJSON(w, 401, map[string]string{"error": "invalid bearer credential"})
			return
		}
		id := r.PathValue("cashTransactionID")
		contextOnly := r.URL.Path == "/api/v1/cash-workspace"
		exact := p.ScopeKind == "resources" && len(p.Resources) == 1 && len(p.Resources["cash_transaction"]) == 1 && (p.Resources["cash_transaction"][id] || contextOnly)
		if p.Provider != "iam" || !referenceTenantPattern.MatchString(p.TenantID) || !p.Can(permission) || (amount && !p.Can(identity.PermissionAmountRead)) || (p.ScopeKind != "tenant" && (!exact || (tenantOnly && !contextOnly))) {
			writeJSON(w, 403, map[string]string{"error": "cash workspace scope or capability denied"})
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalContextKey{}, p)))
	})
}

func cashJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	typ, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || typ != "application/json" {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON request"})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err = d.Decode(value); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON request"})
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON request"})
		return false
	}
	return true
}

func (s *Server) cashWorkspaceContext(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFromContext(r.Context())
	ops := []string{}
	rows := []map[string]any{}
	runtimeMode := partymigration.RuntimeModeLocal
	directoryAuthority := false
	if s.partyRuntime != nil {
		state, err := s.partyRuntime.State(r.Context(), p.TenantID)
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "Money Party runtime unavailable"})
			return
		}
		runtimeMode = state.Mode
		directoryAuthority = state.DirectoryAuthorityEstablished
	}
	if p.ScopeKind == "tenant" {
		parties, err := s.parties.List(r.Context())
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "Money party directory unavailable"})
			return
		}
		for _, v := range parties {
			rows = append(rows, map[string]any{"id": v.ID, "revision": v.Revision, "kind": v.Kind, "legal_name": v.LegalName})
		}
		if p.Can(identity.PermissionCashRecord) && p.Can(identity.PermissionAmountRead) {
			ops = append(ops, "cash.record")
		}
	}
	directoryKind := "money_local"
	if runtimeMode == partymigration.RuntimeModeDirectory {
		directoryKind = "party_directory_projection"
	}
	writeJSON(w, 200, map[string]any{"schema_version": "1", "source_application": serviceName, "tenant_id": p.TenantID, "directory_kind": directoryKind, "party_runtime_mode": runtimeMode, "directory_authority_established": directoryAuthority, "parties": rows, "allowed_operations": ops})
}

func cashView(p identity.Principal, v cashtransaction.Transaction, references map[string]partymigration.RuntimePartyReference) map[string]any {
	party := func(v cashtransaction.PartySnapshot) map[string]any {
		value := map[string]any{"id": v.PartyID, "revision": v.PartyRevision, "kind": v.Kind, "legal_name": v.LegalName}
		if reference, ok := references[v.PartyID]; ok {
			value["party_reference"] = map[string]any{
				"party_id":       reference.PartyID,
				"revision":       reference.Revision,
				"authority":      reference.Source,
				"local_party_id": reference.LocalPartyID,
			}
		}
		return value
	}
	ops := []string{}
	zero := strings.Trim(v.AllocatedAmount, "0.") == ""
	if p.Can(identity.PermissionCashReverse) && p.Can(identity.PermissionAmountRead) && v.Status == cashtransaction.StatusRecorded && zero {
		ops = append(ops, "cash.reverse")
	}
	if p.Can(identity.PermissionCashEvidenceManage) {
		ops = append(ops, "cash.evidence.manage")
	}
	result := map[string]any{"id": v.ID, "type": v.Type, "status": v.Status, "revision": 1, "description": v.Description, "ledger_owner_party_id": v.LedgerOwnerPartyID, "payer": party(v.Payer), "payee": party(v.Payee), "occurred_on": v.OccurredOn, "account_reference": v.AccountReference, "manual_source_reference": v.ManualSourceReference, "external_transaction_reference": v.ExternalTransactionReference, "recorded_at": v.RecordedAt, "amount_visible": p.Can(identity.PermissionAmountRead), "amount": nil, "effective_amount": nil, "allocated_amount": nil, "unallocated_amount": nil, "currency": nil, "allowed_operations": ops, "reversal": nil}
	if p.Can(identity.PermissionAmountRead) {
		result["amount"] = v.Amount
		result["effective_amount"] = v.EffectiveAmount
		result["allocated_amount"] = v.AllocatedAmount
		result["unallocated_amount"] = v.UnallocatedAmount
		result["currency"] = v.Currency
	}
	if v.Reversal != nil {
		result["reversal"] = map[string]any{"id": v.Reversal.ID, "reason": v.Reversal.Reason, "reversed_at": v.Reversal.ReversedAt}
	}
	return result
}

// cashViews preserves the Money-local Party identifiers required by existing
// consumers and adds the authoritative Party Directory reference when the
// runtime cutover is active. The lookup is batched so list reads do not create
// a per-row cross-application call, and it fails closed if any local identity
// cannot be projected back to its authoritative reference.
func (s *Server) cashViews(ctx context.Context, p identity.Principal, values []cashtransaction.Transaction) ([]map[string]any, error) {
	references := map[string]partymigration.RuntimePartyReference{}
	if s.partyRuntime != nil && len(values) > 0 {
		ids := make([]string, 0, len(values)*2)
		seen := map[string]bool{}
		for _, value := range values {
			for _, id := range []string{value.Payer.PartyID, value.Payee.PartyID} {
				if id != "" && !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
		resolved, err := s.partyRuntime.ResolveReferences(ctx, p.TenantID, ids)
		if err != nil {
			return nil, err
		}
		for _, reference := range resolved {
			references[reference.LocalPartyID] = reference
		}
		if len(references) != len(ids) {
			return nil, errors.New("Money Party runtime references incomplete")
		}
	}
	items := make([]map[string]any, 0, len(values))
	for _, value := range values {
		items = append(items, cashView(p, value, references))
	}
	return items, nil
}

func (s *Server) cashWorkspaceEvidence(w http.ResponseWriter, r *http.Request) {
	if s.cashEvidence == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "cash evidence is not configured"})
		return
	}
	p, _ := principalFromContext(r.Context())
	value, found, err := s.cashEvidence.Current(r.Context(), r.PathValue("cashTransactionID"))
	if err != nil {
		cashEvidenceError(w, err)
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "cash transaction not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"schema_version": "1", "source_application": serviceName, "tenant_id": p.TenantID, "evidence": value})
}

func (s *Server) cashWorkspaceReplaceEvidence(w http.ResponseWriter, r *http.Request) {
	if s.cashEvidence == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "cash evidence is not configured"})
		return
	}
	var in struct {
		CommandID        string             `json:"command_id"`
		ExpectedRevision int64              `json:"expected_revision"`
		Reason           string             `json:"reason"`
		EvidenceRefs     []cashevidence.Ref `json:"evidence_refs"`
	}
	if !cashJSON(w, r, &in) {
		return
	}
	p, _ := principalFromContext(r.Context())
	result, err := s.cashEvidence.Replace(r.Context(), cashevidence.ReplaceInput{CommandID: in.CommandID, ExpectedRevision: in.ExpectedRevision, Reason: in.Reason, EvidenceRefs: in.EvidenceRefs, Actor: p.Actor(), TenantID: p.TenantID, CashTransactionID: r.PathValue("cashTransactionID")})
	if err != nil {
		cashEvidenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"schema_version": "1", "source_application": serviceName, "tenant_id": p.TenantID, "evidence": result.Snapshot, "replayed": result.Replayed})
}

func cashEvidenceError(w http.ResponseWriter, err error) {
	status, message := http.StatusServiceUnavailable, "cash evidence unavailable"
	switch {
	case errors.Is(err, cashevidence.ErrInvalid):
		status, message = http.StatusUnprocessableEntity, "cash evidence input is invalid"
	case errors.Is(err, cashevidence.ErrNotFound):
		status, message = http.StatusNotFound, "cash transaction not found"
	case errors.Is(err, cashevidence.ErrRevisionConflict), errors.Is(err, cashevidence.ErrIdempotencyConflict):
		status, message = http.StatusConflict, "cash evidence revision or command conflict"
	case errors.Is(err, cashevidence.ErrEvidenceInvalid):
		status, message = http.StatusUnprocessableEntity, "Evidence file reference is invalid or stale"
	case errors.Is(err, cashevidence.ErrEvidenceUnavailable):
		status, message = http.StatusServiceUnavailable, "Evidence file verification is unavailable"
	}
	writeJSON(w, status, map[string]string{"error": message})
}

func (s *Server) cashWorkspaceGet(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFromContext(r.Context())
	v, found, err := s.cashTransactions.Get(r.Context(), r.PathValue("cashTransactionID"))
	if err != nil {
		cashError(w, err)
		return
	}
	if !found {
		writeJSON(w, 404, map[string]string{"error": "cash transaction not found"})
		return
	}
	items, err := s.cashViews(r.Context(), p, []cashtransaction.Transaction{v})
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Money Party references unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"schema_version": "1", "source_application": serviceName, "tenant_id": p.TenantID, "transaction": items[0]})
}

type cashCreateRequest struct {
	CommandID                    string                       `json:"command_id"`
	Type                         string                       `json:"type"`
	LedgerOwnerPartyID           string                       `json:"ledger_owner_party_id"`
	CounterpartyPartyID          string                       `json:"counterparty_party_id"`
	LedgerOwnerPartyReference    *cashPartyDirectoryReference `json:"ledger_owner_party_reference"`
	CounterpartyPartyReference   *cashPartyDirectoryReference `json:"counterparty_party_reference"`
	Description                  string                       `json:"description"`
	Amount                       string                       `json:"amount"`
	Currency                     string                       `json:"currency"`
	OccurredOn                   string                       `json:"occurred_on"`
	AccountReference             string                       `json:"account_reference"`
	ManualSourceReference        string                       `json:"manual_source_reference"`
	ExternalTransactionReference string                       `json:"external_transaction_reference"`
}

type cashPartyDirectoryReference struct {
	PartyID  string `json:"party_id"`
	Revision int64  `json:"revision"`
}

func (s *Server) cashWorkspaceCreate(w http.ResponseWriter, r *http.Request) {
	var in cashCreateRequest
	if !cashJSON(w, r, &in) {
		return
	}
	p, _ := principalFromContext(r.Context())
	usesDirectoryParties := in.LedgerOwnerPartyReference != nil || in.CounterpartyPartyReference != nil
	if usesDirectoryParties {
		if in.LedgerOwnerPartyReference == nil || in.CounterpartyPartyReference == nil || strings.TrimSpace(in.LedgerOwnerPartyID) != "" || strings.TrimSpace(in.CounterpartyPartyID) != "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "both Party Directory references are required and cannot be mixed with Money-local Party IDs"})
			return
		}
		if s.directoryParties == nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "Party Directory runtime projection is not configured"})
			return
		}
		owner, err := s.directoryParties.ResolveOrProject(r.Context(), p.TenantID, in.LedgerOwnerPartyReference.PartyID, in.LedgerOwnerPartyReference.Revision, p.UserID, p.DisplayName)
		if err != nil {
			cashPartyProjectionError(w, err)
			return
		}
		counterparty, err := s.directoryParties.ResolveOrProject(r.Context(), p.TenantID, in.CounterpartyPartyReference.PartyID, in.CounterpartyPartyReference.Revision, p.UserID, p.DisplayName)
		if err != nil {
			cashPartyProjectionError(w, err)
			return
		}
		in.LedgerOwnerPartyID = owner.LocalPartyID
		in.CounterpartyPartyID = counterparty.LocalPartyID
	}
	v, err := s.cashTransactions.Create(r.Context(), cashtransaction.CreateInput{CommandID: in.CommandID, Type: in.Type, LedgerOwnerPartyID: in.LedgerOwnerPartyID, CounterpartyPartyID: in.CounterpartyPartyID, Description: in.Description, Amount: in.Amount, Currency: in.Currency, OccurredOn: in.OccurredOn, AccountReference: in.AccountReference, ManualSourceReference: in.ManualSourceReference, ExternalTransactionReference: in.ExternalTransactionReference, Actor: p.Actor()})
	if err != nil {
		cashError(w, err)
		return
	}
	items, err := s.cashViews(r.Context(), p, []cashtransaction.Transaction{v.Transaction})
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Money Party references unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"schema_version": "1", "source_application": serviceName, "tenant_id": p.TenantID, "transaction": items[0], "replayed": v.Replayed})
}

func cashPartyProjectionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, partymigration.ErrRuntimeProjectionState):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Money Party Directory runtime is not ready"})
	case errors.Is(err, partymigration.ErrTargetInvalid):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "Party Directory reference is invalid or stale"})
	case errors.Is(err, partymigration.ErrTargetUnavailable):
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Party Directory reference verification is unavailable"})
	default:
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Money Party Directory projection is unavailable"})
	}
}

func (s *Server) cashWorkspaceReverse(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CommandID string `json:"command_id"`
		Reason    string `json:"reason"`
	}
	if !cashJSON(w, r, &in) {
		return
	}
	p, _ := principalFromContext(r.Context())
	v, err := s.cashTransactions.Reverse(r.Context(), cashtransaction.ReverseInput{CommandID: in.CommandID, CashTransactionID: r.PathValue("cashTransactionID"), Reason: in.Reason, Actor: p.Actor()})
	if err != nil {
		cashError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"schema_version": "1", "source_application": serviceName, "tenant_id": p.TenantID, "receipt": map[string]any{"cash_transaction_id": v.Reversal.CashTransactionID, "reversal_id": v.Reversal.ID, "replayed": v.Replayed}})
}

func cashError(w http.ResponseWriter, err error) {
	status := 503
	message := "cash transaction unavailable"
	var invalid *cashtransaction.ValidationError
	var party *cashtransaction.PartyNotFoundError
	var missing *cashtransaction.TransactionNotFoundError
	var createConflict *cashtransaction.IdempotencyConflictError
	var external *cashtransaction.ExternalReferenceConflictError
	var reverseConflict *cashtransaction.ReversalIdempotencyConflictError
	var reversed *cashtransaction.TransactionAlreadyReversedError
	var allocated *cashtransaction.TransactionAllocatedConflictError
	switch {
	case errors.As(err, &invalid):
		status = 422
		message = invalid.Error()
	case errors.As(err, &party):
		status = 422
		message = party.Error()
	case errors.As(err, &missing):
		status = 404
		message = missing.Error()
	case errors.As(err, &createConflict):
		status = 409
		message = createConflict.Error()
	case errors.As(err, &external):
		status = 409
		message = external.Error()
	case errors.As(err, &reverseConflict):
		status = 409
		message = reverseConflict.Error()
	case errors.As(err, &reversed):
		status = 409
		message = reversed.Error()
	case errors.As(err, &allocated):
		status = 409
		message = allocated.Error()
	}
	writeJSON(w, status, map[string]string{"error": message})
}
