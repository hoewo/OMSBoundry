package httpserver

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/identity"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
	"omsbizfoundry/money-flow-ledger/internal/partymigration"
)

var referenceTenantPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type resourceReferenceView struct {
	SchemaVersion     string               `json:"schema_version"`
	SourceApplication string               `json:"source_application"`
	ID                string               `json:"id"`
	TenantID          string               `json:"tenant_id"`
	ResourceType      string               `json:"resource_type"`
	DisplaySummary    string               `json:"display_summary"`
	Status            string               `json:"status"`
	Revision          int64                `json:"revision"`
	Direction         string               `json:"direction,omitempty"`
	TransactionType   string               `json:"transaction_type,omitempty"`
	Currency          string               `json:"currency,omitempty"`
	EffectiveAmount   string               `json:"effective_amount,omitempty"`
	AllocatedAmount   string               `json:"allocated_amount,omitempty"`
	RemittedAmount    string               `json:"remitted_amount,omitempty"`
	OutstandingAmount string               `json:"outstanding_amount,omitempty"`
	UnallocatedAmount string               `json:"unallocated_amount,omitempty"`
	BalanceSequence   int64                `json:"balance_sequence,omitempty"`
	OccurredOn        string               `json:"occurred_on,omitempty"`
	PartyIDs          []string             `json:"party_ids,omitempty"`
	PartyReferences   []partyReferenceView `json:"party_references,omitempty"`
}

type partyReferenceView struct {
	ID                     string  `json:"id"`
	Revision               int64   `json:"revision"`
	Kind                   string  `json:"kind"`
	LegalName              string  `json:"legal_name"`
	RegistrationIdentifier *string `json:"registration_identifier,omitempty"`
	Source                 string  `json:"source"`
	Role                   string  `json:"role,omitempty"`
}

func validateReferenceOptions(tenantID, accessToken string) error {
	if tenantID == "" && accessToken == "" {
		return nil
	}
	if !referenceTenantPattern.MatchString(tenantID) || len(accessToken) < 24 || strings.HasPrefix(accessToken, "replace-with-") {
		return fmt.Errorf("reference access requires a valid tenant id and replaced 24+ byte token")
	}
	return nil
}

func (s *Server) authenticateReference(permission identity.Permission, resourceType, pathParameter string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bearer credential required"})
			return
		}

		var principal identity.Principal
		if s.referenceAccessToken != "" {
			if subtle.ConstantTimeCompare([]byte(token), []byte(s.referenceAccessToken)) != 1 {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid bearer credential"})
				return
			}
			principal = identity.Principal{
				UserID: "reference-client", DisplayName: "Project Workspace reference client",
				Role: identity.RoleAdministrator, Provider: "reference-template", TenantID: s.referenceTenantID,
			}
		} else {
			resolved, found, err := s.identities.ResolveSession(r.Context(), token)
			if err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credential verification unavailable"})
				return
			}
			if !found {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid bearer credential"})
				return
			}
			principal = resolved
		}
		allowed := principal.Provider == "reference-template" || (principal.Can(permission) && principal.CanReference(resourceType, r.PathValue(pathParameter)))
		if principal.TenantID == "" || !allowed {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "reference read is not allowed"})
			return
		}
		ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) obligationReference(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bearer credential required"})
		return
	}
	details, found, err := s.obligations.GetDetails(r.Context(), r.PathValue("obligationID"))
	if err != nil {
		var notFound *obligation.ObligationNotFoundError
		if errors.As(err, &notFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "obligation not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "obligation reference unavailable"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "obligation not found"})
		return
	}
	revision := details.Obligation.CurrentRevision
	if revision < 1 {
		revision = details.Obligation.DraftVersion
	}
	partyIDs, partyReferences, err := s.resolveReferenceParties(r.Context(), principal.TenantID, []string{details.Obligation.Creditor.PartyID, details.Obligation.Debtor.PartyID})
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Party runtime reference unavailable"})
		return
	}
	if len(partyReferences) == 2 {
		partyReferences[0].Role = "creditor"
		partyReferences[1].Role = "debtor"
	}
	writeJSON(w, http.StatusOK, resourceReferenceView{
		SchemaVersion: "1", SourceApplication: "money-flow-ledger",
		ID: details.Obligation.ID, TenantID: principal.TenantID, ResourceType: "obligation",
		DisplaySummary: details.Obligation.Title, Status: details.Obligation.Status, Revision: revision,
		Direction: string(details.Obligation.Direction), Currency: details.Obligation.Currency,
		EffectiveAmount: details.Obligation.EffectiveAmount, AllocatedAmount: details.Obligation.AllocatedAmount,
		RemittedAmount: details.Obligation.RemittedAmount, OutstandingAmount: details.Obligation.OutstandingAmount,
		BalanceSequence: details.Obligation.BalanceSequence,
		PartyIDs:        partyIDs, PartyReferences: partyReferences,
	})
}

func (s *Server) transactionReference(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bearer credential required"})
		return
	}
	value, found, err := s.cashTransactions.Get(r.Context(), r.PathValue("transactionID"))
	if err != nil {
		var notFound *cashtransaction.TransactionNotFoundError
		if errors.As(err, &notFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "transaction not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "transaction reference unavailable"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "transaction not found"})
		return
	}
	partyIDs, partyReferences, err := s.resolveReferenceParties(r.Context(), principal.TenantID, []string{value.Payer.PartyID, value.Payee.PartyID})
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Party runtime reference unavailable"})
		return
	}
	if len(partyReferences) == 2 {
		partyReferences[0].Role = "payer"
		partyReferences[1].Role = "payee"
	}
	writeJSON(w, http.StatusOK, resourceReferenceView{
		SchemaVersion: "1", SourceApplication: "money-flow-ledger",
		ID: value.ID, TenantID: principal.TenantID, ResourceType: "transaction",
		DisplaySummary: value.Description, Status: value.Status, Revision: 1,
		TransactionType: string(value.Type), Currency: value.Currency, EffectiveAmount: value.EffectiveAmount,
		AllocatedAmount: value.AllocatedAmount, UnallocatedAmount: value.UnallocatedAmount, OccurredOn: value.OccurredOn,
		PartyIDs: partyIDs, PartyReferences: partyReferences,
	})
}

func (s *Server) resolveReferenceParties(ctx context.Context, tenantID string, localPartyIDs []string) ([]string, []partyReferenceView, error) {
	if s.partyRuntime == nil {
		return localPartyIDs, nil, nil
	}
	resolved, err := s.partyRuntime.ResolveReferences(ctx, tenantID, localPartyIDs)
	if err != nil {
		return nil, nil, err
	}
	partyIDs := make([]string, 0, len(resolved))
	views := make([]partyReferenceView, 0, len(resolved))
	for _, value := range resolved {
		partyIDs = append(partyIDs, value.PartyID)
		views = append(views, partyReferenceView{
			ID: value.PartyID, Revision: value.Revision, Kind: value.Kind,
			LegalName: value.LegalName, RegistrationIdentifier: value.RegistrationIdentifier, Source: value.Source,
		})
	}
	return partyIDs, views, nil
}

var _ PartyRuntime = (*partymigration.RuntimeRegistry)(nil)

func bearerToken(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) <= 7 || !strings.EqualFold(raw[:7], "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(raw[7:])
	return token, token != "" && !strings.ContainsAny(token, " \t\r\n")
}
