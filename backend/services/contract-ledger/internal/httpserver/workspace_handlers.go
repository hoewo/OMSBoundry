package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"omsbizfoundry/contract-ledger/internal/contract"
	"omsbizfoundry/contract-ledger/internal/identity"
	"omsbizfoundry/contract-ledger/internal/partymigration"
	"omsbizfoundry/contract-ledger/internal/referenceoa"
)

// Native command ports reuse the same Contract use cases as the independent
// application. Neither Project nor the workbench becomes a fact owner.
func (s *Server) registerWorkspace(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/contracts/{contractID}/workspace", s.workspaceAuth(identity.PermissionBusinessView, false, http.HandlerFunc(s.workspaceRead)))
	mux.Handle("GET /api/v1/contracts/{contractID}/evidence", s.workspaceAuth(identity.PermissionBusinessView, false, http.HandlerFunc(s.workspaceEvidence)))
	mux.Handle("PUT /api/v1/contracts/{contractID}/evidence", s.workspaceAuth(identity.PermissionEvidenceManage, false, http.HandlerFunc(s.workspaceReplaceEvidence)))
	mux.Handle("POST /api/v1/contracts", s.workspaceAuth(identity.PermissionDraftManage, true, http.HandlerFunc(s.workspaceCreate)))
	mux.Handle("GET /api/v1/contracts/workspace-context", s.workspaceContextAuth(http.HandlerFunc(s.workspaceContext)))
	mux.Handle("GET /api/v1/contracts/party-candidates", s.workspaceAuth(identity.PermissionBusinessView, true, http.HandlerFunc(s.workspacePartyCandidates)))
	for _, route := range []struct {
		path       string
		permission identity.Permission
		handler    http.HandlerFunc
	}{
		{"POST /api/v1/contracts/{contractID}", identity.PermissionDraftManage, s.workspaceUpdate},
		{"POST /api/v1/contracts/{contractID}/cancel", identity.PermissionDraftManage, s.workspaceCancel},
		{"POST /api/v1/contracts/{contractID}/parties", identity.PermissionDraftManage, s.workspaceAddParty},
		{"POST /api/v1/contracts/{contractID}/parties/{partyLinkID}/remove", identity.PermissionDraftManage, s.workspaceRemoveParty},
		{"POST /api/v1/contracts/{contractID}/activate", identity.PermissionLifecycleManage, s.workspaceActivate},
		{"POST /api/v1/contracts/{contractID}/changes", identity.PermissionLifecycleManage, s.workspaceChange},
		{"POST /api/v1/contracts/{contractID}/settlements", identity.PermissionLifecycleManage, s.workspaceSettlement},
		{"POST /api/v1/contracts/{contractID}/reference-oa-draft", identity.PermissionDraftManage, s.workspaceSaveReferenceOA},
		{"POST /api/v1/contracts/{contractID}/reference-oa-amendments/{amendmentID}/confirm", identity.PermissionLifecycleManage, s.workspaceConfirmReferenceOA},
		{"POST /api/v1/contracts/{contractID}/reference-oa-materials", identity.PermissionEvidenceManage, s.workspaceMaterial},
	} {
		mux.Handle(route.path, s.workspaceAuth(route.permission, false, route.handler))
	}
}

func (s *Server) workspacePartyCandidates(w http.ResponseWriter, r *http.Request) {
	values, err := s.parties.List(r.Context())
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "contract party directory unavailable"})
		return
	}
	rows := []map[string]any{}
	for _, p := range values {
		rows = append(rows, map[string]any{"id": p.ID, "kind": p.Kind, "legal_name": p.LegalName, "revision": p.Revision})
	}
	// These are the existing local Contract Party identities, not an assertion
	// that the independent Party Directory migration has already completed.
	writeJSON(w, 200, map[string]any{"schema_version": "1", "source_application": "contract-ledger", "tenant_id": mustPrincipal(r).TenantID, "directory_kind": "contract_local", "parties": rows})
}
func (s *Server) workspaceContext(w http.ResponseWriter, r *http.Request) {
	p := mustPrincipal(r)
	operations := []string{}
	if p.ScopeKind == "tenant" && p.Can(identity.PermissionDraftManage) {
		operations = append(operations, "draft.create")
	}
	if p.ScopeKind == "tenant" && p.Can(identity.PermissionEvidenceManage) {
		operations = append(operations, "evidence.manage")
	}
	runtimeMode := partymigration.RuntimeModeLocal
	directoryAuthorityEstablished := false
	if s.partyRuntime != nil {
		if state, err := s.partyRuntime.State(r.Context(), p.TenantID); err == nil {
			runtimeMode = state.Mode
			directoryAuthorityEstablished = state.DirectoryAuthorityEstablished
		}
	}
	writeJSON(w, 200, map[string]any{
		"schema_version": "1", "source_application": "contract-ledger", "tenant_id": p.TenantID,
		"allowed_operations": operations, "party_runtime_mode": runtimeMode,
		"directory_authority_established": directoryAuthorityEstablished,
	})
}

func (s *Server) workspaceSaveReferenceOA(w http.ResponseWriter, r *http.Request) {
	var in referenceoa.SaveInput
	if !workspaceDecode(w, r, &in) {
		return
	}
	if s.referenceOADrafts == nil {
		writeJSON(w, 503, map[string]string{"error": "reference OA draft store unavailable"})
		return
	}
	p := mustPrincipal(r)
	in.ContractID = r.PathValue("contractID")
	in.TenantID = p.TenantID
	in.Actor = p.Actor()
	result, err := s.referenceOADrafts.Save(r.Context(), in)
	if err != nil {
		status := 500
		switch {
		case errors.Is(err, referenceoa.ErrInvalid):
			status = 422
		case errors.Is(err, referenceoa.ErrScope):
			status = 403
		case errors.Is(err, referenceoa.ErrConflict) || errors.Is(err, referenceoa.ErrState):
			status = 409
		}
		message := "reference OA draft unavailable"
		if status != 500 {
			message = err.Error()
		}
		writeJSON(w, status, map[string]string{"error": message})
		return
	}
	writeJSON(w, 200, map[string]any{"schema_version": "1", "source_application": "contract-ledger", "tenant_id": p.TenantID, "result": result})
}

func (s *Server) workspaceConfirmReferenceOA(w http.ResponseWriter, r *http.Request) {
	var in referenceoa.ConfirmInput
	if !workspaceDecode(w, r, &in) {
		return
	}
	if s.referenceOADrafts == nil {
		writeJSON(w, 503, map[string]string{"error": "reference OA confirmation unavailable"})
		return
	}
	p := mustPrincipal(r)
	in.ContractID = r.PathValue("contractID")
	in.AmendmentID = r.PathValue("amendmentID")
	in.TenantID = p.TenantID
	in.Actor = p.Actor()
	result, err := s.referenceOADrafts.Confirm(r.Context(), in)
	if err != nil {
		status := 500
		switch {
		case errors.Is(err, referenceoa.ErrInvalid):
			status = 422
		case errors.Is(err, referenceoa.ErrScope):
			status = 403
		case errors.Is(err, referenceoa.ErrState) || errors.Is(err, referenceoa.ErrConflict):
			status = 409
		default:
			if code, message, ok := contractChangeHTTPError(err); ok {
				writeJSON(w, code, map[string]string{"error": message})
				return
			}
		}
		message := "reference OA confirmation unavailable"
		if status != 500 {
			message = err.Error()
		}
		writeJSON(w, status, map[string]string{"error": message})
		return
	}
	writeJSON(w, 200, map[string]any{"schema_version": "1", "source_application": "contract-ledger", "tenant_id": p.TenantID, "confirmation": result})
}

func (s *Server) workspaceAuth(permission identity.Permission, create bool, next http.Handler) http.Handler {
	return s.workspaceAuthWhere(permission, func(p identity.Principal, r *http.Request) bool {
		return p.ScopeKind == "tenant" || (!create && p.CanReference("contract", r.PathValue("contractID")))
	}, next)
}

// Context contains no Contract facts. Exact Contract readers need to load this
// metadata without acquiring tenant-wide create/list permissions; each later
// workspace read still checks its concrete resource independently.
func (s *Server) workspaceContextAuth(next http.Handler) http.Handler {
	return s.workspaceAuthWhere(identity.PermissionBusinessView, func(p identity.Principal, _ *http.Request) bool {
		if p.ScopeKind == "tenant" {
			return true
		}
		for id := range p.Resources["contract"] {
			if p.CanReference("contract", id) {
				return true
			}
		}
		return false
	}, next)
}

func (s *Server) workspaceAuthWhere(permission identity.Permission, scope func(identity.Principal, *http.Request) bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
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
		// Reference-template and local sessions are not signed native capabilities.
		// Their existing read-only reference / independent HTML routes stay intact.
		if p.Provider != "iam" || p.TenantID == "" || !p.Can(permission) || !scope(p, r) {
			writeJSON(w, 403, map[string]string{"error": "native capability or resource scope denied"})
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalContextKey{}, p)))
	})
}

func workspaceDecode(w http.ResponseWriter, r *http.Request, input any) bool {
	typeName, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || typeName != "application/json" {
		writeJSON(w, 415, map[string]string{"error": "application/json required"})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid or oversized command"})
		return false
	}
	if !strings.HasPrefix(strings.TrimSpace(string(body)), "{") {
		writeJSON(w, 400, map[string]string{"error": "command object required"})
		return false
	}
	d := json.NewDecoder(strings.NewReader(string(body)))
	d.DisallowUnknownFields()
	if err := d.Decode(input); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid command fields"})
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		writeJSON(w, 400, map[string]string{"error": "one command required"})
		return false
	}
	return true
}

type workspaceContract struct {
	ID              string          `json:"id"`
	CommandID       string          `json:"command_id"`
	NumberNamespace string          `json:"number_namespace"`
	ContractNumber  string          `json:"contract_number"`
	Title           string          `json:"title"`
	Category        string          `json:"category"`
	Currency        string          `json:"currency"`
	TotalAmount     string          `json:"total_amount"`
	SignedOn        string          `json:"signed_on"`
	EffectiveOn     string          `json:"effective_on"`
	Status          contract.Status `json:"status"`
	DraftVersion    int64           `json:"draft_version"`
	CurrentRevision int64           `json:"current_revision"`
}

func contractWorkspaceValue(c contract.Contract) workspaceContract {
	return workspaceContract{c.ID, c.CommandID, c.NumberNamespace, c.ContractNumber, c.Title, c.Category, c.Currency, c.TotalAmount, c.SignedOn, c.EffectiveOn, c.Status, c.DraftVersion, c.CurrentRevision}
}

type workspaceDraftFields struct {
	CommandID       string `json:"command_id"`
	NumberNamespace string `json:"number_namespace"`
	ContractNumber  string `json:"contract_number"`
	Title           string `json:"title"`
	Category        string `json:"category"`
	Currency        string `json:"currency"`
	TotalAmount     string `json:"total_amount"`
	SignedOn        string `json:"signed_on"`
	EffectiveOn     string `json:"effective_on"`
}

func (s *Server) workspaceCreate(w http.ResponseWriter, r *http.Request) {
	var in workspaceDraftFields
	if !workspaceDecode(w, r, &in) {
		return
	}
	value, err := s.contracts.CreateDraft(r.Context(), contract.CreateDraftInput{CommandID: in.CommandID, NumberNamespace: in.NumberNamespace, ContractNumber: in.ContractNumber, Title: in.Title, Category: in.Category, Currency: in.Currency, TotalAmount: in.TotalAmount, SignedOn: in.SignedOn, EffectiveOn: in.EffectiveOn, Actor: mustPrincipal(r).Actor()})
	if err != nil {
		workspaceError(w, err, draftChangeHTTPError)
		return
	}
	workspaceReceipt(w, r, map[string]any{"contract_id": value.ID, "draft_version": value.DraftVersion, "status": value.Status})
}

func (s *Server) workspaceUpdate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		workspaceDraftFields
		ExpectedDraftVersion int64 `json:"expected_draft_version"`
	}
	if !workspaceDecode(w, r, &in) {
		return
	}
	value, err := s.contracts.UpdateDraft(r.Context(), contract.UpdateDraftInput{CommandID: in.CommandID, ContractID: r.PathValue("contractID"), ExpectedDraftVersion: in.ExpectedDraftVersion, NumberNamespace: in.NumberNamespace, ContractNumber: in.ContractNumber, Title: in.Title, Category: in.Category, Currency: in.Currency, TotalAmount: in.TotalAmount, SignedOn: in.SignedOn, EffectiveOn: in.EffectiveOn, Actor: mustPrincipal(r).Actor()})
	if err != nil {
		workspaceError(w, err, draftChangeHTTPError)
		return
	}
	workspaceReceipt(w, r, map[string]any{"contract_id": value.ContractID, "draft_version": value.DraftVersion, "status": value.Status, "replayed": value.Replayed})
}

func (s *Server) workspaceCancel(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CommandID            string `json:"command_id"`
		ExpectedDraftVersion int64  `json:"expected_draft_version"`
		Reason               string `json:"reason"`
	}
	if !workspaceDecode(w, r, &in) {
		return
	}
	value, err := s.contracts.CancelDraft(r.Context(), contract.CancelDraftInput{CommandID: in.CommandID, ContractID: r.PathValue("contractID"), ExpectedDraftVersion: in.ExpectedDraftVersion, Reason: in.Reason, Actor: mustPrincipal(r).Actor()})
	if err != nil {
		workspaceError(w, err, draftChangeHTTPError)
		return
	}
	workspaceReceipt(w, r, map[string]any{"contract_id": value.ContractID, "draft_version": value.DraftVersion, "status": value.Status, "replayed": value.Replayed})
}

func (s *Server) workspaceActivate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CommandID            string `json:"command_id"`
		ExpectedDraftVersion int64  `json:"expected_draft_version"`
	}
	if !workspaceDecode(w, r, &in) {
		return
	}
	value, err := s.contracts.Activate(r.Context(), contract.ActivateContractInput{CommandID: in.CommandID, ContractID: r.PathValue("contractID"), ExpectedDraftVersion: in.ExpectedDraftVersion, Actor: mustPrincipal(r).Actor()})
	if err != nil {
		workspaceError(w, err, activationHTTPError)
		return
	}
	workspaceReceipt(w, r, map[string]any{"contract_id": value.ContractID, "revision": value.Revision, "replayed": value.Replayed})
}

func (s *Server) workspaceAddParty(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CommandID            string `json:"command_id"`
		ExpectedDraftVersion int64  `json:"expected_draft_version"`
		PartyID              string `json:"party_id,omitempty"`
		PartyReference       *struct {
			PartyID  string `json:"party_id"`
			Revision int64  `json:"revision"`
		} `json:"party_reference,omitempty"`
		Role string `json:"role"`
	}
	if !workspaceDecode(w, r, &in) {
		return
	}
	p := mustPrincipal(r)
	if in.PartyReference != nil {
		if strings.TrimSpace(in.PartyID) != "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "party_id and party_reference cannot be combined"})
			return
		}
		if s.directoryParties == nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "Party Directory runtime projection is not configured"})
			return
		}
		resolved, err := s.directoryParties.ResolveOrProject(r.Context(), p.TenantID, in.PartyReference.PartyID, in.PartyReference.Revision, p.UserID, p.DisplayName)
		if err != nil {
			contractPartyProjectionError(w, err)
			return
		}
		in.PartyID = resolved.LocalPartyID
	}
	value, err := s.contracts.AddDraftParty(r.Context(), contract.AddDraftPartyInput{CommandID: in.CommandID, ContractID: r.PathValue("contractID"), ExpectedDraftVersion: in.ExpectedDraftVersion, PartyID: in.PartyID, Role: in.Role, Actor: p.Actor()})
	if err != nil {
		workspaceError(w, err, draftPartyHTTPError)
		return
	}
	workspaceReceipt(w, r, map[string]any{"contract_id": value.Party.ContractID, "party_link_id": value.Party.ID, "draft_version": value.DraftVersion, "replayed": value.Replayed})
}

func contractPartyProjectionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, partymigration.ErrRuntimeProjectionState):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Contract Party Directory runtime is not ready"})
	case errors.Is(err, partymigration.ErrTargetInvalid):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "Party Directory reference is invalid or stale"})
	case errors.Is(err, partymigration.ErrTargetUnavailable):
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Party Directory reference verification is unavailable"})
	default:
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Contract Party Directory projection is unavailable"})
	}
}

func (s *Server) workspaceRemoveParty(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CommandID            string `json:"command_id"`
		ExpectedDraftVersion int64  `json:"expected_draft_version"`
	}
	if !workspaceDecode(w, r, &in) {
		return
	}
	value, err := s.contracts.RemoveDraftParty(r.Context(), contract.RemoveDraftPartyInput{CommandID: in.CommandID, ContractID: r.PathValue("contractID"), ExpectedDraftVersion: in.ExpectedDraftVersion, PartyLinkID: r.PathValue("partyLinkID"), Actor: mustPrincipal(r).Actor()})
	if err != nil {
		workspaceError(w, err, draftPartyHTTPError)
		return
	}
	workspaceReceipt(w, r, map[string]any{"contract_id": r.PathValue("contractID"), "party_link_id": value.Party.ID, "draft_version": value.DraftVersion, "replayed": value.Replayed})
}

func (s *Server) workspaceChange(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CommandID        string `json:"command_id"`
		ExpectedRevision int64  `json:"expected_revision"`
		InstrumentNumber string `json:"instrument_number"`
		Title            string `json:"title"`
		ChangeSummary    string `json:"change_summary"`
		SignedOn         string `json:"signed_on"`
		EffectiveOn      string `json:"effective_on"`
		EffectiveOrder   int64  `json:"effective_order"`
		AmountEffect     string `json:"amount_effect"`
		AmountValue      string `json:"amount_value"`
	}
	if !workspaceDecode(w, r, &in) {
		return
	}
	value, err := s.contracts.AppendChange(r.Context(), contract.AppendContractChangeInput{CommandID: in.CommandID, ContractID: r.PathValue("contractID"), ExpectedRevision: in.ExpectedRevision, InstrumentNumber: in.InstrumentNumber, Title: in.Title, ChangeSummary: in.ChangeSummary, SignedOn: in.SignedOn, EffectiveOn: in.EffectiveOn, EffectiveOrder: in.EffectiveOrder, AmountEffect: in.AmountEffect, AmountValue: in.AmountValue, Actor: mustPrincipal(r).Actor()})
	if err != nil {
		workspaceError(w, err, contractChangeHTTPError)
		return
	}
	workspaceReceipt(w, r, map[string]any{"contract_id": value.ContractID, "instrument_id": value.InstrumentID, "revision": value.Revision, "replayed": value.Replayed})
}

func (s *Server) workspaceSettlement(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CommandID                  string  `json:"command_id"`
		ExpectedContractRevision   int64   `json:"expected_contract_revision"`
		ExpectedSettlementSequence int64   `json:"expected_settlement_sequence"`
		Kind                       string  `json:"kind"`
		StatementNumber            string  `json:"statement_number"`
		Title                      string  `json:"title"`
		Amount                     string  `json:"amount"`
		ConfirmedOn                string  `json:"confirmed_on"`
		Basis                      string  `json:"basis"`
		ConfirmingPartyPositions   []int64 `json:"confirming_party_positions"`
	}
	if !workspaceDecode(w, r, &in) {
		return
	}
	value, err := s.contracts.RecordSettlement(r.Context(), contract.RecordContractSettlementInput{CommandID: in.CommandID, ContractID: r.PathValue("contractID"), ExpectedContractRevision: in.ExpectedContractRevision, ExpectedSettlementSequence: in.ExpectedSettlementSequence, Kind: in.Kind, StatementNumber: in.StatementNumber, Title: in.Title, Amount: in.Amount, ConfirmedOn: in.ConfirmedOn, Basis: in.Basis, ConfirmingPartyPositions: in.ConfirmingPartyPositions, Actor: mustPrincipal(r).Actor()})
	if err != nil {
		workspaceError(w, err, settlementHTTPError)
		return
	}
	workspaceReceipt(w, r, map[string]any{"contract_id": value.Settlement.ContractID, "settlement_id": value.Settlement.ID, "sequence": value.Settlement.Sequence, "replayed": value.Replayed})
}

func workspaceReceipt(w http.ResponseWriter, r *http.Request, receipt map[string]any) {
	writeJSON(w, 200, map[string]any{"schema_version": "1", "source_application": "contract-ledger", "tenant_id": mustPrincipal(r).TenantID, "receipt": receipt})
}

func workspaceError(w http.ResponseWriter, err error, mapper func(error) (int, string, bool)) {
	if status, message, ok := mapper(err); ok {
		writeJSON(w, status, map[string]string{"error": message})
		return
	}
	writeJSON(w, 500, map[string]string{"error": "contract operation unavailable"})
}

func (s *Server) workspaceRead(w http.ResponseWriter, r *http.Request) {
	details, err := s.contracts.GetDraft(r.Context(), r.PathValue("contractID"))
	if err != nil {
		workspaceError(w, err, draftPartyHTTPError)
		return
	}
	p := mustPrincipal(r)
	if details.Contract.ID != r.PathValue("contractID") {
		writeJSON(w, 503, map[string]string{"error": "contract reference mismatch"})
		return
	}
	runtimeParties := make(map[string]partymigration.RuntimePartyReference, len(details.Parties))
	if s.partyRuntime != nil && len(details.Parties) > 0 {
		localPartyIDs := make([]string, 0, len(details.Parties))
		for _, value := range details.Parties {
			localPartyIDs = append(localPartyIDs, value.PartyID)
		}
		resolved, resolveErr := s.partyRuntime.ResolveReferences(r.Context(), p.TenantID, localPartyIDs)
		if resolveErr != nil || len(resolved) != len(localPartyIDs) {
			writeJSON(w, 503, map[string]string{"error": "contract Party references unavailable"})
			return
		}
		for _, value := range resolved {
			runtimeParties[value.LocalPartyID] = value
		}
		if len(runtimeParties) != len(localPartyIDs) {
			writeJSON(w, 503, map[string]string{"error": "contract Party references incomplete"})
			return
		}
	}
	operations := []string{}
	actions := []string{"contract-ledger.view"}
	if details.Contract.Status == contract.StatusDraft && p.Can(identity.PermissionDraftManage) {
		actions = append(actions, "contract-ledger.draft.manage")
		operations = append(operations, "draft.update", "draft.cancel", "draft.party.add", "draft.party.remove")
	}
	if details.Contract.Status == contract.StatusActive && p.Can(identity.PermissionDraftManage) {
		operations = append(operations, "reference-oa.amendment.manage")
	}
	if p.Can(identity.PermissionLifecycleManage) {
		if details.Contract.Status == contract.StatusDraft {
			operations = append(operations, "contract.activate")
		}
		if details.Contract.Status == contract.StatusActive {
			operations = append(operations, "contract.change.append")
			operations = append(operations, "reference-oa.amendment.confirm")
		}
		final := false
		for _, v := range details.Settlements {
			if v.Kind == contract.SettlementKindFinal {
				final = true
			}
		}
		if !final && (details.Contract.Status == contract.StatusActive || details.Contract.Status == contract.StatusTerminated) {
			operations = append(operations, "contract.settlement.record")
		}
		if len(operations) > 0 && details.Contract.Status != contract.StatusCancelled && details.Contract.Status != contract.StatusClosed {
			actions = append(actions, "contract-ledger.lifecycle.manage")
		}
	}
	// Read DTO deliberately does not serialize domain structs or audit credentials.
	parties := []map[string]any{}
	for _, v := range details.Parties {
		value := map[string]any{"id": v.ID, "party_id": v.PartyID, "role": v.Role, "legal_name": v.PartyLegalName, "source_revision": v.PartyRevision, "active": v.Active()}
		if reference, ok := runtimeParties[v.PartyID]; ok {
			value["party_reference"] = map[string]any{
				"party_id":       reference.PartyID,
				"revision":       reference.Revision,
				"authority":      reference.Source,
				"local_party_id": reference.LocalPartyID,
			}
		}
		parties = append(parties, value)
	}
	instruments := []map[string]any{}
	for _, v := range details.Instruments {
		instruments = append(instruments, map[string]any{"id": v.ID, "instrument_number": v.InstrumentNumber, "instrument_type": v.InstrumentType, "revision": v.ResultingRevision, "signed_on": v.SignedOn, "effective_on": v.EffectiveOn, "effective_order": v.EffectiveOrder, "title": v.Title, "change_summary": v.ChangeSummary, "amount_effect": v.AmountEffect, "amount_value": v.AmountValue})
	}
	settlements := []map[string]any{}
	for _, v := range details.Settlements {
		confirmedParties := []map[string]any{}
		for _, party := range v.ConfirmingParties {
			confirmedParties = append(confirmedParties, map[string]any{"position": party.Position, "source_party_id": party.SourcePartyID, "source_party_revision": party.SourcePartyRevision, "legal_name": party.LegalName, "role": party.Role})
		}
		settlements = append(settlements, map[string]any{"id": v.ID, "source_revision": v.SourceRevision, "sequence": v.Sequence, "kind": v.Kind, "statement_number": v.StatementNumber, "title": v.Title, "amount": v.Amount, "currency": v.Currency, "confirmed_on": v.ConfirmedOn, "basis": v.Basis, "confirming_parties": confirmedParties})
	}
	snapshots := []map[string]any{}
	for _, v := range details.PartySnapshots {
		snapshots = append(snapshots, map[string]any{"revision": v.Revision, "position": v.Position, "source_party_id": v.SourcePartyID, "source_party_revision": v.SourcePartyRevision, "legal_name": v.LegalName, "role": v.Role})
	}
	var overlay *referenceoa.Draft
	confirmations := []referenceoa.Confirmation{}
	materials := referenceoa.Materials{ContractID: details.Contract.ID, TenantID: p.TenantID, Items: []referenceoa.Material{}}
	if s.referenceOADrafts != nil {
		overlay, err = s.referenceOADrafts.Read(r.Context(), p.TenantID, details.Contract.ID)
		if err != nil {
			status := 503
			if errors.Is(err, referenceoa.ErrScope) {
				status = 403
			}
			writeJSON(w, status, map[string]string{"error": "reference OA draft context unavailable"})
			return
		}
		confirmations, err = s.referenceOADrafts.Confirmations(r.Context(), p.TenantID, details.Contract.ID)
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "reference OA confirmations unavailable"})
			return
		}
		materials, err = s.referenceOADrafts.ReadMaterials(r.Context(), p.TenantID, details.Contract.ID)
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "contract materials unavailable"})
			return
		}
		if s.referenceOADrafts.MaterialsAvailable() && overlay != nil && p.Can(identity.PermissionEvidenceManage) && details.Contract.Status != contract.StatusCancelled && details.Contract.Status != contract.StatusClosed {
			operations = append(operations, "reference-oa.material.manage")
			actions = append(actions, "contract-ledger.evidence.manage")
		}
	}
	writeJSON(w, 200, map[string]any{"schema_version": "1", "source_application": "contract-ledger", "tenant_id": p.TenantID, "contract": contractWorkspaceValue(details.Contract), "parties": parties, "party_snapshots": snapshots, "instruments": instruments, "settlements": settlements, "reference_oa_draft": overlay, "reference_oa_confirmations": confirmations, "reference_oa_materials": materials, "allowed_actions": actions, "allowed_operations": operations})
}

func (s *Server) workspaceMaterial(w http.ResponseWriter, r *http.Request) {
	if s.referenceOADrafts == nil {
		writeJSON(w, 503, map[string]string{"error": "contract materials unavailable"})
		return
	}
	var in referenceoa.MaterialInput
	if !workspaceDecode(w, r, &in) {
		return
	}
	p := mustPrincipal(r)
	in.ContractID = r.PathValue("contractID")
	in.TenantID = p.TenantID
	in.Actor = p.Actor()
	result, e := s.referenceOADrafts.MutateMaterial(r.Context(), in)
	if e != nil {
		status := 500
		switch {
		case errors.Is(e, referenceoa.ErrInvalid):
			status = 422
		case errors.Is(e, referenceoa.ErrScope):
			status = 403
		case errors.Is(e, referenceoa.ErrConflict), errors.Is(e, referenceoa.ErrState):
			status = 409
		case errors.Is(e, referenceoa.ErrEvidenceUnavailable):
			status = 503
		}
		writeJSON(w, status, map[string]string{"error": "contract material operation not confirmed"})
		return
	}
	writeJSON(w, 200, map[string]any{"schema_version": "1", "source_application": "contract-ledger", "tenant_id": p.TenantID, "result": result})
}
