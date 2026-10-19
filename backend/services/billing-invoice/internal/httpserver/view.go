package httpserver

import (
	"strconv"
	"time"

	"omsbizfoundry/billing-invoice/internal/auth"
	"omsbizfoundry/billing-invoice/internal/invoice"
)

type RegistrationView struct {
	ID                 string                   `json:"id"`
	TenantID           string                   `json:"tenant_id"`
	Status             string                   `json:"status"`
	IssuedOn           string                   `json:"issued_on"`
	AmountVisible      bool                     `json:"amount_visible"`
	AmountMinor        *int64                   `json:"amount_minor"`
	AmountMinorDecimal *string                  `json:"amount_minor_decimal"`
	Currency           *string                  `json:"currency"`
	IssuerParty        invoice.PartySnapshot    `json:"issuer_party"`
	RecipientParty     invoice.PartySnapshot    `json:"recipient_party"`
	SourceRefs         []invoice.SourceSnapshot `json:"source_refs"`
	EvidenceRefs       []invoice.EvidenceRef    `json:"evidence_refs"`
	Note               string                   `json:"note"`
	Revision           int64                    `json:"revision"`
	CreatedAt          time.Time                `json:"created_at"`
	UpdatedAt          time.Time                `json:"updated_at"`
	AllowedActions     []string                 `json:"allowed_actions"`
}

type RevisionView struct {
	InvoiceRegistrationID string           `json:"invoice_registration_id"`
	TenantID              string           `json:"tenant_id"`
	Revision              int64            `json:"revision"`
	EventKind             string           `json:"event_kind"`
	CommandID             string           `json:"command_id"`
	Snapshot              RegistrationView `json:"snapshot"`
	Reason                string           `json:"reason"`
	ActorRef              string           `json:"actor_ref"`
	OccurredAt            time.Time        `json:"occurred_at"`
}

func ViewRegistration(value invoice.Registration, principal auth.Principal) RegistrationView {
	inScope := CanAccess(value, principal)
	visible := inScope && principal.Can(auth.ActionAmountRead)
	var amount *int64
	var decimal *string
	var currency *string
	if visible {
		amountValue, currencyValue := value.AmountMinor, value.Currency
		amount, currency = &amountValue, &currencyValue
		decimalValue := strconv.FormatInt(amountValue, 10)
		decimal = &decimalValue
	}
	return RegistrationView{
		ID: value.ID, TenantID: value.TenantID, Status: value.Status, IssuedOn: value.IssuedOn,
		AmountVisible: visible, AmountMinor: amount, AmountMinorDecimal: decimal, Currency: currency,
		IssuerParty: value.IssuerParty, RecipientParty: value.RecipientParty,
		SourceRefs: value.SourceRefs, EvidenceRefs: value.EvidenceRefs, Note: value.Note,
		Revision: value.Revision, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		AllowedActions: allowedActions(value, principal),
	}
}

func ViewRegistrations(values []invoice.Registration, principal auth.Principal) []RegistrationView {
	result := make([]RegistrationView, 0, len(values))
	for _, value := range values {
		result = append(result, ViewRegistration(value, principal))
	}
	return result
}

func listAllowedActions(principal auth.Principal) []string {
	result := make([]string, 0, 2)
	if principal.Can(auth.ActionRead) {
		result = append(result, "read")
	}
	if principal.Can(auth.ActionRecord) && principal.Can(auth.ActionAmountRead) {
		result = append(result, "record")
	}
	return result
}

func ViewHistory(page invoice.HistoryPage, principal auth.Principal) []RevisionView {
	result := make([]RevisionView, 0, len(page.Items))
	for _, value := range page.Items {
		result = append(result, RevisionView{
			InvoiceRegistrationID: value.InvoiceRegistrationID, TenantID: value.TenantID,
			Revision: value.Revision, EventKind: value.EventKind, CommandID: value.CommandID,
			Snapshot: ViewRegistration(value.Snapshot, principal), Reason: value.Reason,
			ActorRef: value.ActorRef, OccurredAt: value.OccurredAt,
		})
	}
	return result
}

func ProjectIDs(value invoice.Registration) []string {
	seen := map[string]bool{}
	result := make([]string, 0)
	for _, source := range value.SourceRefs {
		if source.Kind == invoice.SourceProject && source.ResourceID != nil && !seen[*source.ResourceID] {
			seen[*source.ResourceID] = true
			result = append(result, *source.ResourceID)
		}
	}
	return result
}

func CanAccess(value invoice.Registration, principal auth.Principal) bool {
	return value.TenantID == principal.TenantID && principal.CanRegistration(value.ID, ProjectIDs(value))
}

func allowedActions(value invoice.Registration, principal auth.Principal) []string {
	result := make([]string, 0, 6)
	if !CanAccess(value, principal) {
		return result
	}
	appendIf := func(name string, action string, permitted bool) {
		if permitted && principal.Can(action) {
			result = append(result, name)
		}
	}
	appendIf("read", auth.ActionRead, true)
	appendIf("amount_read", auth.ActionAmountRead, true)
	mutable := value.Status == invoice.StatusRecorded
	appendIf("correct", auth.ActionCorrect, mutable)
	appendIf("registration_invalidate", auth.ActionRegistrationInvalidate, mutable)
	appendIf("evidence_manage", auth.ActionEvidenceManage, mutable)
	appendIf("history_read", auth.ActionHistoryRead, true)
	return result
}
