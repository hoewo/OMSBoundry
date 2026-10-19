package httpserver

import (
	"context"
	"net/http"
	"sort"
	"strconv"

	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/projects"
)

func (s *Server) directory(w http.ResponseWriter, r *http.Request) {
	stripUntrustedIdentityHeaders(r.Header)
	principal, err := s.authenticator.Authenticate(r)
	if err != nil {
		writeProblem(w, r, 401, "credential_required", "需要有效访问凭证", false, nil)
		return
	}
	if !principal.Can(auth.ActionRead) {
		writeProblem(w, r, 403, "capability_denied", "缺少读取 Project 的能力", false, nil)
		return
	}
	if principal.ScopeKind != "tenant" && principal.ScopeKind != "resources" {
		writeProblem(w, r, 403, "scope_denied", "无效目录范围", false, nil)
		return
	}
	query := projects.DirectoryQuery{Limit: 100, Keyword: r.URL.Query().Get("keyword"), Status: r.URL.Query().Get("status"), ScopeKind: principal.ScopeKind}
	if query.Status != "" && query.Status != "active" && query.Status != "all" {
		writeProblem(w, r, 400, "invalid_request", "无效项目状态范围", false, nil)
		return
	}
	for key, destination := range map[string]*int{"offset": &query.Offset, "limit": &query.Limit} {
		if raw, exists := r.URL.Query()[key]; exists {
			if len(raw) != 1 {
				writeProblem(w, r, 400, "invalid_request", "分页参数重复", false, nil)
				return
			}
			value, err := strconv.Atoi(raw[0])
			if err != nil || (key == "offset" && value < 0) || (key == "limit" && (value < 1 || value > 100)) {
				writeProblem(w, r, 400, "invalid_request", "无效分页参数", false, nil)
				return
			}
			*destination = value
		}
	}
	// No URL/header can override this signed scope; unknown query fields are rejected.
	for key := range r.URL.Query() {
		if key != "offset" && key != "limit" && key != "keyword" && key != "status" {
			writeProblem(w, r, 400, "invalid_request", "不支持的目录参数", false, nil)
			return
		}
	}
	if principal.ScopeKind == "resources" {
		for id, allowed := range principal.ResourceIDs {
			if allowed {
				query.ResourceIDs = append(query.ResourceIDs, id)
			}
		}
		sort.Strings(query.ResourceIDs)
	}
	r = r.WithContext(context.WithValue(r.Context(), principalKey{}, principal))
	page, err := s.useCases.Directory(r.Context(), principal.TenantID, query)
	s.writeResult(w, r, 200, struct {
		projects.DirectoryPage
		AllowedActions []string `json:"allowed_actions"`
	}{page, credentialActions(principal)}, err)
}
