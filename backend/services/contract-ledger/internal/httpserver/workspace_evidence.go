package httpserver

import (
	"errors"
	"net/http"

	"omsbizfoundry/contract-ledger/internal/contractevidence"
)

func (s *Server) workspaceEvidence(w http.ResponseWriter, r *http.Request) {
	if s.contractEvidence == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "contract Evidence references are not configured"})
		return
	}
	p := mustPrincipal(r)
	value, found, err := s.contractEvidence.Current(r.Context(), p.TenantID, r.PathValue("contractID"))
	if err != nil {
		workspaceEvidenceError(w, err)
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "contract not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"schema_version": "1", "source_application": serviceName, "tenant_id": p.TenantID, "evidence": value})
}

func (s *Server) workspaceReplaceEvidence(w http.ResponseWriter, r *http.Request) {
	if s.contractEvidence == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "contract Evidence references are not configured"})
		return
	}
	var input struct {
		CommandID        string                 `json:"command_id"`
		ExpectedRevision int64                  `json:"expected_revision"`
		Reason           string                 `json:"reason"`
		EvidenceRefs     []contractevidence.Ref `json:"evidence_refs"`
	}
	if !workspaceDecode(w, r, &input) {
		return
	}
	p := mustPrincipal(r)
	result, err := s.contractEvidence.Replace(r.Context(), contractevidence.ReplaceInput{
		CommandID: input.CommandID, ExpectedRevision: input.ExpectedRevision, Reason: input.Reason,
		EvidenceRefs: input.EvidenceRefs, Actor: p.Actor(), TenantID: p.TenantID, ContractID: r.PathValue("contractID"),
	})
	if err != nil {
		workspaceEvidenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"schema_version": "1", "source_application": serviceName, "tenant_id": p.TenantID, "evidence": result.Snapshot, "replayed": result.Replayed})
}

func workspaceEvidenceError(w http.ResponseWriter, err error) {
	status, message := http.StatusServiceUnavailable, "contract Evidence references unavailable"
	switch {
	case errors.Is(err, contractevidence.ErrInvalid):
		status, message = http.StatusUnprocessableEntity, "contract Evidence input is invalid"
	case errors.Is(err, contractevidence.ErrNotFound):
		status, message = http.StatusNotFound, "contract not found"
	case errors.Is(err, contractevidence.ErrRevisionConflict), errors.Is(err, contractevidence.ErrIdempotencyConflict):
		status, message = http.StatusConflict, "contract Evidence revision or command conflict"
	case errors.Is(err, contractevidence.ErrEvidenceInvalid):
		status, message = http.StatusUnprocessableEntity, "Evidence file reference is invalid or stale"
	case errors.Is(err, contractevidence.ErrEvidenceUnavailable):
		status, message = http.StatusServiceUnavailable, "Evidence file verification is unavailable"
	}
	writeJSON(w, status, map[string]string{"error": message})
}
