package httpserver

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"omsbizfoundry/money-flow-ledger/internal/identity"
	"omsbizfoundry/money-flow-ledger/internal/payablesuggestion"
	contract "omsbizfoundry/payable-suggestion-contract"
)

func (s *Server) authenticatePayableSuggestion(permission identity.Permission, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bearer credential required"})
			return
		}
		var principal identity.Principal
		if s.referenceAccessToken != "" {
			if subtleToken(token, s.referenceAccessToken) == false {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid bearer credential"})
				return
			}
			principal = identity.Principal{UserID: "payable-suggestion-template", DisplayName: "Payable Suggestion template client", Role: identity.RoleAdministrator, Provider: "reference-template", TenantID: s.referenceTenantID}
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
		if principal.TenantID == "" || !principal.Can(permission) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "payable suggestion action is not allowed"})
			return
		}
		next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), principal)))
	})
}

func (s *Server) submitPayableSuggestion(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bearer credential required"})
		return
	}
	var input contract.Submission
	if !decodeAPIJSON(w, r, &input) {
		return
	}
	if strings.ToLower(strings.TrimSpace(input.TenantID)) != principal.TenantID || !canSuggestionResource(principal, input.SuggestionID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "payable suggestion scope is not allowed"})
		return
	}
	result, err := s.payableSuggestions.Submit(r.Context(), principal.Actor(), input)
	if err != nil {
		writePayableSuggestionError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) getPayableSuggestion(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bearer credential required"})
		return
	}
	id := strings.ToLower(strings.TrimSpace(r.PathValue("suggestionID")))
	if !canSuggestionResource(principal, id) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "payable suggestion scope is not allowed"})
		return
	}
	value, err := s.payableSuggestions.Get(r.Context(), principal.TenantID, id)
	if err != nil {
		writePayableSuggestionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) payableSuggestionHistory(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bearer credential required"})
		return
	}
	id := strings.ToLower(strings.TrimSpace(r.PathValue("suggestionID")))
	if !canSuggestionResource(principal, id) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "payable suggestion scope is not allowed"})
		return
	}
	value, err := s.payableSuggestions.History(r.Context(), principal.TenantID, id)
	if err != nil {
		writePayableSuggestionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": value})
}

func (s *Server) decidePayableSuggestion(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bearer credential required"})
		return
	}
	id := strings.ToLower(strings.TrimSpace(r.PathValue("suggestionID")))
	if !canSuggestionResource(principal, id) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "payable suggestion scope is not allowed"})
		return
	}
	var input contract.Decision
	if !decodeAPIJSON(w, r, &input) {
		return
	}
	value, err := s.payableSuggestions.Decide(r.Context(), principal.TenantID, id, principal.Actor(), input)
	if err != nil {
		writePayableSuggestionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) withdrawPayableSuggestion(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bearer credential required"})
		return
	}
	id := strings.ToLower(strings.TrimSpace(r.PathValue("suggestionID")))
	if !canSuggestionResource(principal, id) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "payable suggestion scope is not allowed"})
		return
	}
	var input contract.Withdrawal
	if !decodeAPIJSON(w, r, &input) {
		return
	}
	value, err := s.payableSuggestions.Withdraw(r.Context(), principal.TenantID, id, principal.Actor(), input)
	if err != nil {
		writePayableSuggestionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func canSuggestionResource(principal identity.Principal, id string) bool {
	return principal.Provider == "reference-template" || principal.ScopeKind == "tenant" || principal.CanReference("payable_suggestion", strings.ToLower(strings.TrimSpace(id)))
}

func decodeAPIJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON request"})
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid trailing JSON"})
		return false
	}
	return true
}

func writePayableSuggestionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, payablesuggestion.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "payable suggestion not found"})
	case errors.Is(err, payablesuggestion.ErrInvalid):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid payable suggestion"})
	case errors.Is(err, payablesuggestion.ErrCommandConflict), errors.Is(err, payablesuggestion.ErrStateConflict), errors.Is(err, payablesuggestion.ErrRevisionConflict):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	case errors.Is(err, payablesuggestion.ErrPartyMapping):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "Party mapping unavailable"})
	default:
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "payable suggestion service unavailable"})
	}
}

func withPrincipal(ctx context.Context, principal identity.Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func subtleToken(first, second string) bool {
	return subtle.ConstantTimeCompare([]byte(first), []byte(second)) == 1
}
