package httpserver

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"omsbizfoundry/money-flow-ledger/internal/financepayment"
	"omsbizfoundry/money-flow-ledger/internal/identity"
)

func (s *Server) authenticateInternalFinancePayment(next http.Handler) http.Handler {
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
			principal = identity.Principal{UserID: "internal-finance-payment-template", DisplayName: "Internal finance payment template client", Role: identity.RoleAdministrator, Provider: "reference-template", TenantID: s.referenceTenantID}
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
		if principal.TenantID == "" || !principal.Can(identity.PermissionInternalFinancePaymentRecord) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal finance payment action is not allowed"})
			return
		}
		next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), principal)))
	})
}
func (s *Server) recordInternalFinancePayment(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bearer credential required"})
		return
	}
	var input financepayment.Request
	if !decodeAPIJSON(w, r, &input) {
		return
	}
	commandID := strings.ToLower(strings.TrimSpace(input.CommandID))
	if strings.ToLower(strings.TrimSpace(input.TenantID)) != principal.TenantID || (principal.Provider != "reference-template" && principal.ScopeKind != "tenant" && !principal.CanReference("internal_finance_payment", commandID)) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal finance payment scope is not allowed"})
		return
	}
	result, err := s.internalFinancePayments.Record(r.Context(), principal.Actor(), input)
	if err != nil {
		switch {
		case errors.Is(err, financepayment.ErrInvalid):
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid internal finance payment"})
		case errors.Is(err, financepayment.ErrOwnerMissing):
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "internal finance ledger owner is not configured"})
		case errors.Is(err, financepayment.ErrPayeeMapping):
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "internal finance payee mapping is missing"})
		case errors.Is(err, financepayment.ErrCommandConflict):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "internal finance payment command conflict"})
		default:
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "internal finance payment could not be recorded"})
		}
		return
	}
	writeJSON(w, http.StatusOK, result)
}
