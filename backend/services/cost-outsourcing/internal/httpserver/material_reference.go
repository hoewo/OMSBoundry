package httpserver

import (
	"net/http"
	"omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
)

func (s *Server) costMaterials(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	// GetCost is the existing target-owned final resource authorization path.
	value, err := s.useCases.GetCost(r.Context(), actorFrom(p), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	if value.TenantID != p.TenantID || value.ID != r.PathValue("id") {
		s.fail(w, costoutsourcing.ErrReferenceInvalid)
		return
	}
	if !p.CanCost(value.ID, value.Project.ID) {
		s.fail(w, costoutsourcing.ErrForbidden)
		return
	}
	projection, err := costoutsourcing.CostMaterialReference(value)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projection)
}
func (s *Server) engagementMaterials(w http.ResponseWriter, r *http.Request) {
	p := PrincipalFrom(r)
	value, err := s.useCases.GetEngagement(r.Context(), actorFrom(p), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	if value.TenantID != p.TenantID || value.ID != r.PathValue("id") {
		s.fail(w, costoutsourcing.ErrReferenceInvalid)
		return
	}
	if !p.CanEngagement(value.ID, value.Project.ID) {
		s.fail(w, costoutsourcing.ErrForbidden)
		return
	}
	projection, err := costoutsourcing.EngagementMaterialReference(value)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projection)
}
