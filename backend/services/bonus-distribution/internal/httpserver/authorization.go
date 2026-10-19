package httpserver

import (
	"context"
	"encoding/json"
	"net/http"

	"omsbizfoundry/bonus-distribution/internal/auth"
)

type principalKey struct{}

func ProtectAny(authenticator auth.Authenticator, actions []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stripUntrustedIdentityHeaders(r.Header)
		principal, err := authenticator.Authenticate(r)
		if err != nil {
			writeProblem(w, http.StatusUnauthorized, "UNAUTHENTICATED", false)
			return
		}
		allowed := len(actions) == 0
		for _, action := range actions {
			allowed = allowed || principal.Can(action)
		}
		if !allowed {
			writeProblem(w, http.StatusForbidden, "CAPABILITY_DENIED", false)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, principal)))
	})
}

func PrincipalFrom(r *http.Request) auth.Principal {
	value, _ := r.Context().Value(principalKey{}).(auth.Principal)
	return value
}

func stripUntrustedIdentityHeaders(header http.Header) {
	for _, name := range []string{"X-Actor-ID", "X-Actor-Name", "X-Tenant-ID", "X-IAM-Subject", "X-Allowed-Actions", "X-Capability-Scope"} {
		header.Del(name)
	}
}

func writeProblem(w http.ResponseWriter, status int, code string, retryable bool) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": code, "retryable": retryable}})
}
