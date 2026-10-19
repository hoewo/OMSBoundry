package httpserver

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"omsbizfoundry/contract-ledger/internal/contract"
	"omsbizfoundry/contract-ledger/internal/identity"
	"omsbizfoundry/contract-ledger/internal/partymigration"
)

var referenceTenantPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type contractReferenceView struct {
	SchemaVersion          string               `json:"schema_version"`
	SourceApplication      string               `json:"source_application"`
	ID                     string               `json:"id"`
	TenantID               string               `json:"tenant_id"`
	ResourceType           string               `json:"resource_type"`
	DisplaySummary         string               `json:"display_summary"`
	Status                 string               `json:"status"`
	Revision               int64                `json:"revision"`
	Category               string               `json:"category,omitempty"`
	ContractID             string               `json:"contract_id,omitempty"`
	Currency               string               `json:"currency,omitempty"`
	CurrentEffectiveAmount *string              `json:"current_effective_amount,omitempty"`
	PartyIDs               []string             `json:"party_ids,omitempty"`
	PartyReferences        []partyReferenceView `json:"party_references,omitempty"`
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

func (s *Server) authenticateReference(resourceType, pathParameter string, next http.Handler) http.Handler {
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
				Role: identity.RoleViewer, Provider: "reference-template", TenantID: s.referenceTenantID,
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
		allowed := principal.Provider == "reference-template" || (principal.Can(identity.PermissionReferenceRead) && principal.CanReference(resourceType, r.PathValue(pathParameter)))
		if principal.TenantID == "" || !allowed {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "reference read is not allowed"})
			return
		}
		ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) contractReference(w http.ResponseWriter, r *http.Request) {
	principal, ok := currentPrincipal(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bearer credential required"})
		return
	}
	details, err := s.contracts.GetDraft(r.Context(), r.PathValue("contractID"))
	if err != nil {
		var notFound *contract.ContractNotFoundError
		if errors.As(err, &notFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "contract not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "contract reference unavailable"})
		return
	}
	revision := details.Contract.CurrentRevision
	if revision < 1 {
		revision = details.Contract.DraftVersion
	}
	var currentEffectiveAmount *string
	if details.Contract.TotalAmount != "" {
		value := details.Contract.TotalAmount
		currentEffectiveAmount = &value
	}
	partyIDs := contractReferencePartyIDs(details)
	partyIDs, partyReferences, err := s.resolveReferenceParties(r.Context(), principal.TenantID, partyIDs, contractReferencePartyRoles(details))
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Party runtime reference unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, contractReferenceView{
		SchemaVersion: "1", SourceApplication: "contract-ledger",
		ID: details.Contract.ID, TenantID: principal.TenantID, ResourceType: "contract",
		DisplaySummary: details.Contract.Title, Status: string(details.Contract.Status), Revision: revision,
		Category: details.Contract.Category, Currency: details.Contract.Currency, CurrentEffectiveAmount: currentEffectiveAmount,
		PartyIDs: partyIDs, PartyReferences: partyReferences,
	})
}

func contractReferencePartyRoles(details contract.DraftDetails) map[string]string {
	roles := map[string]string{}
	if details.Contract.CurrentRevision > 0 {
		for _, snapshot := range details.PartySnapshots {
			if snapshot.Revision == details.Contract.CurrentRevision {
				roles[snapshot.SourcePartyID] = snapshot.Role
			}
		}
		return roles
	}
	for _, linked := range details.Parties {
		if linked.Active() {
			roles[linked.PartyID] = linked.Role
		}
	}
	return roles
}

func contractReferencePartyIDs(details contract.DraftDetails) []string {
	if details.Contract.CurrentRevision > 0 {
		partyIDs := make([]string, 0, len(details.PartySnapshots))
		for _, snapshot := range details.PartySnapshots {
			if snapshot.Revision == details.Contract.CurrentRevision {
				partyIDs = append(partyIDs, snapshot.SourcePartyID)
			}
		}
		return partyIDs
	}
	partyIDs := make([]string, 0, len(details.Parties))
	for _, linked := range details.Parties {
		if linked.Active() {
			partyIDs = append(partyIDs, linked.PartyID)
		}
	}
	return partyIDs
}

func (s *Server) settlementReference(w http.ResponseWriter, r *http.Request) {
	principal, ok := currentPrincipal(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bearer credential required"})
		return
	}
	settlement, err := s.contracts.GetSettlement(r.Context(), r.PathValue("settlementID"))
	if err != nil {
		var notFound *contract.ContractSettlementNotFoundError
		if errors.As(err, &notFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "settlement not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "settlement reference unavailable"})
		return
	}
	details, err := s.contracts.GetDraft(r.Context(), settlement.ContractID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "settlement contract reference unavailable"})
		return
	}
	amount := settlement.Amount
	partyIDs := make([]string, 0, len(settlement.ConfirmingParties))
	for _, linked := range settlement.ConfirmingParties {
		partyIDs = append(partyIDs, linked.SourcePartyID)
	}
	roles := map[string]string{}
	for _, linked := range settlement.ConfirmingParties {
		roles[linked.SourcePartyID] = linked.Role
	}
	partyIDs, partyReferences, err := s.resolveReferenceParties(r.Context(), principal.TenantID, partyIDs, roles)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Party runtime reference unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, contractReferenceView{
		SchemaVersion: "1", SourceApplication: "contract-ledger",
		ID: settlement.ID, TenantID: principal.TenantID, ResourceType: "settlement",
		DisplaySummary: settlement.Title, Status: "recorded", Revision: 1,
		Category: details.Contract.Category, ContractID: settlement.ContractID,
		Currency: settlement.Currency, CurrentEffectiveAmount: &amount,
		PartyIDs: partyIDs, PartyReferences: partyReferences,
	})
}

func (s *Server) resolveReferenceParties(ctx context.Context, tenantID string, localPartyIDs []string, roles map[string]string) ([]string, []partyReferenceView, error) {
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
			Role: roles[value.LocalPartyID],
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
