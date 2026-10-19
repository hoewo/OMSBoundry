package httpserver

import (
	"net/http"
	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/projects"
	"sort"
	"strconv"
)

func (s *Server) counterpartyNameSuggestions(w http.ResponseWriter, r *http.Request) {
	stripUntrustedIdentityHeaders(r.Header)
	p, err := s.authenticator.Authenticate(r)
	if err != nil {
		writeProblem(w, r, 401, "credential_required", "需要有效访问凭证", false, nil)
		return
	}
	if !p.Can(auth.ActionCounterpartyContextRead) {
		writeProblem(w, r, 403, "capability_denied", "缺少甲方资料读取能力", false, nil)
		return
	}
	if p.ScopeKind != "tenant" && p.ScopeKind != "resources" {
		writeProblem(w, r, 403, "scope_denied", "无效项目范围", false, nil)
		return
	}
	q := projects.CounterpartyNamesQuery{Query: r.URL.Query().Get("q"), Limit: 10, ScopeKind: p.ScopeKind}
	for key, values := range r.URL.Query() {
		if len(values) != 1 || (key != "q" && key != "limit") {
			writeProblem(w, r, 400, "invalid_request", "不支持或重复的查询参数", false, nil)
			return
		}
		if key == "limit" {
			q.Limit, err = strconv.Atoi(values[0])
			if err != nil || q.Limit < 1 || q.Limit > 50 {
				writeProblem(w, r, 400, "invalid_request", "无效名称数量", false, nil)
				return
			}
		}
	}
	if p.ScopeKind == "resources" {
		for id, allowed := range p.ResourceIDs {
			if allowed {
				q.ResourceIDs = append(q.ResourceIDs, id)
			}
		}
		sort.Strings(q.ResourceIDs)
	}
	q, err = projects.NormalizeCounterpartyNamesQuery(q)
	if err != nil {
		writeProblem(w, r, 400, "invalid_request", err.Error(), false, nil)
		return
	}
	result, err := s.useCases.CounterpartyNameSuggestions(r.Context(), p.TenantID, q)
	s.writeResult(w, r, 200, result, err)
}
