package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"omsbizfoundry/money-flow-ledger/internal/identity"
	"omsbizfoundry/money-flow-ledger/internal/receiptset"
)

func (s *Server) authenticateReceiptSetCommand(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bearer credential required"})
			return
		}
		principal, found, err := s.identities.ResolveSession(r.Context(), token)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credential verification unavailable"})
			return
		}
		if !found {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid bearer credential"})
			return
		}
		if principal.Provider != "iam" || principal.TenantID == "" || principal.ScopeKind != "tenant" || !principal.Can(identity.PermissionReceiptSetConfirm) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "receipt set confirmation is not allowed"})
			return
		}
		ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) confirmReceiptSet(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bearer credential required"})
		return
	}
	var input receiptset.ConfirmInput
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") || decoder.Decode(&input) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid receipt set request"})
		return
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid receipt set request"})
		return
	}
	input.TenantID, input.Actor = principal.TenantID, principal.Actor()
	result, err := s.receiptSets.Confirm(r.Context(), input)
	if err != nil {
		writeReceiptSetError(w, err)
		return
	}
	status := http.StatusCreated
	if result.Replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
}

func (s *Server) receiptSetReference(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bearer credential required"})
		return
	}
	value, err := s.receiptSets.Get(r.Context(), principal.TenantID, r.PathValue("receiptSetID"))
	if err != nil {
		writeReceiptSetError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		SchemaVersion     string `json:"schema_version"`
		SourceApplication string `json:"source_application"`
		ResourceType      string `json:"resource_type"`
		receiptset.Set
	}{SchemaVersion: "1", SourceApplication: serviceName, ResourceType: "receipt-set", Set: value})
}

func writeReceiptSetError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, receiptset.ErrInvalid):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "receipt_set_invalid"})
	case errors.Is(err, receiptset.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "receipt_set_not_found"})
	case errors.Is(err, receiptset.ErrStateConflict):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "receipt_set_not_confirmable"})
	case errors.Is(err, receiptset.ErrCommandConflict):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "command_conflict"})
	default:
		slog.Error("receipt set request failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "receipt_set_unavailable"})
	}
}
