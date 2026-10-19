package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// This explicitly synthetic, opt-in UI probe never connects to IAM, PostgreSQL
// or any deployment. It proves browser wiring and geometry, not authorization
// or real persistence. The fixture proxy is test-only and cannot ship in server.
func TestReferenceOAInitiationBrowserProbe(t *testing.T) {
	if os.Getenv("OA_INITIATION_BROWSER_PROBE") != "1" {
		t.Skip("opt-in synthetic browser probe")
	}
	handler := initiationWorkbench(t)
	const tenant = "11111111-1111-4111-8111-111111111111"
	const business = "55555555-5555-4555-8555-555555555555"
	const production = "66666666-6666-4666-8666-666666666666"
	people := []map[string]any{
		{"id": business, "tenant_id": tenant, "revision": 2, "status": "active", "display_name": "经营负责人（隔离测试）"},
		{"id": production, "tenant_id": tenant, "revision": 3, "status": "active", "display_name": "生产负责人（隔离测试）"},
	}
	actions := []string{"project-workspace.read", "project-workspace.create", "project-workspace.update", "project-workspace.profile.correct", "project-workspace.hierarchy.manage", "project-workspace.initiation.complete", "project-workspace.assignment.manage"}
	actions = append(actions, "project-workspace.counterparty-context.read", "project-workspace.counterparty-context.manage")
	clientReadOnly := os.Getenv("OA_CLIENT_BROWSER_READ_ONLY") == "1"
	clientNameReadFail := os.Getenv("OA_CLIENT_NAME_READ_FAIL") == "1"
	completionFailure := os.Getenv("OA_INITIATION_COMPLETION_FAILURE")
	if completionFailure != "" && completionFailure != "activation-unknown-once" {
		t.Fatal("unknown synthetic completion failure")
	}
	activationReplyLost := false
	if clientReadOnly {
		actions = []string{"project-workspace.read", "project-workspace.counterparty-context.read"}
	}
	contexts := map[string]map[string]any{}
	clientReadFailures := 0
	basicInfoRole := os.Getenv("OA_BASIC_INFO_BROWSER_ROLE")
	switch basicInfoRole {
	case "":
	case "profile-only":
		actions = []string{"project-workspace.read", "project-workspace.profile.correct"}
	case "business-only":
		actions = []string{"project-workspace.read", "project-workspace.assignment.business-owner.manage"}
	case "production-only":
		actions = []string{"project-workspace.read", "project-workspace.assignment.production-owner.manage"}
	default:
		t.Fatal("unknown synthetic basic info role")
	}
	projects := map[string]map[string]any{}
	assignments := map[string][]map[string]any{}
	assignmentSequence := 0
	newAssignmentID := func() string {
		assignmentSequence++
		return fmt.Sprintf("a0000000-0000-4000-8000-%012d", assignmentSequence)
	}
	currentAssignments := func(id string) []map[string]any {
		current := []map[string]any{}
		for _, assignment := range assignments[id] {
			if assignment["ended_at"] == nil {
				current = append(current, assignment)
			}
		}
		return current
	}
	if os.Getenv("OA_DIRECTORY_BROWSER_PROBE") == "1" {
		seed := func(sequence int, name, number, node string, parent any) string {
			id := fmt.Sprintf("33333333-3333-4333-8333-%012d", sequence)
			project := map[string]any{"id": id, "tenant_id": tenant, "name": name, "node_type": node, "project_number": number, "project_type_keys": []string{"design"}, "summary": "显式隔离浏览器替身", "revision": float64(3), "status": "active", "created_at": "2026-10-02T00:00:00Z", "planned_start_date": "2030-01-01T00:00:00Z"}
			if parent != nil {
				project["parent_project_id"] = parent
			}
			projects[id] = project
			assignments[id] = []map[string]any{{"id": newAssignmentID(), "project_id": id, "tenant_id": tenant, "role_key": "business_owner", "subject_kind": "person", "subject_id": business, "source_revision": 2, "display_snapshot": people[0]["display_name"]}}
			return id
		}
		for index := 1; index <= 25; index++ {
			year := 2026
			if index <= 10 {
				year = 2025
			}
			seed(1000+index, fmt.Sprintf("目录项目%02d", index), fmt.Sprintf("HNJH%d-%03d", year, index), "project", nil)
		}
		parent := seed(1030, "北环项目包（隔离）", "", "package", nil)
		seed(1031, "北环子项甲（隔离）", "HNJH2025-009J01", "project", parent)
		seed(1032, "北环子项乙（隔离）", "HNJH2025-009J02", "project", parent)
	}
	counts := map[string]int{"dispatch": 0, "update": 0, "complete": 0, "production_assignment": 0, "profile_correction": 0, "basic_info_change": 0}
	replays := map[string]any{}
	var mutex sync.Mutex
	finished := make(chan struct{})
	var finish sync.Once
	write := func(w http.ResponseWriter, status int, value any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(value)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:18114")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		defer mutex.Unlock()
		if r.URL.Path == "/__probe/finish" && r.Method == http.MethodPost {
			write(w, 200, counts)
			if flush, ok := w.(http.Flusher); ok {
				flush.Flush()
			}
			finish.Do(func() { close(finished) })
			return
		}
		if r.URL.Path == "/__probe/state" {
			write(w, 200, map[string]any{"synthetic_ui_only": true, "basic_info_role": basicInfoRole, "client_name_read_fail": clientNameReadFail, "completion_failure": completionFailure, "activation_reply_lost": activationReplyLost, "counts": counts, "projects": projects, "assignments": assignments, "counterparty_contexts": contexts})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/gateway/authorize/") {
			write(w, 200, map[string]any{"synthetic_ui_only": true})
			return
		}
		if r.URL.Path == "/configured-iam/api/v1/csrf" && r.Method == http.MethodGet {
			const token = "synthetic-ui-probe-not-a-credential"
			http.SetCookie(w, &http.Cookie{Name: "oms_iam_csrf", Value: token, Path: "/", SameSite: http.SameSiteStrictMode, MaxAge: 3600})
			write(w, 200, map[string]string{"csrf_token": token})
			return
		}
		if r.URL.Path == "/configured-people/api/v1/persons" {
			write(w, 200, people)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/configured-project")
		if path == "/api/v1/counterparty-context/name-suggestions" && r.Method == http.MethodGet {
			counts["client_name_suggestions_read"]++
			if clientNameReadFail {
				write(w, http.StatusServiceUnavailable, map[string]any{"error": "synthetic client name suggestion failure"})
				return
			}
			query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
			names := []string{}
			for _, name := range []string{"历史甲方单位", "甲方单位", "示例甲方（历史）"} {
				if query != "" && strings.Contains(strings.ToLower(name), query) {
					names = append(names, name)
				}
			}
			write(w, 200, map[string]any{"schema_version": "project-counterparty-name-suggestions-v1", "names": names})
			return
		}
		if path == "/api/v1/project-directory" && r.Method == http.MethodGet {
			counts["directory_read"]++
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			if limit < 1 {
				limit = 100
			}
			keyword := strings.ToLower(r.URL.Query().Get("keyword"))
			ids := []string{}
			for id, project := range projects {
				if project["status"] != "active" || project["parent_project_id"] != nil {
					continue
				}
				matches := strings.Contains(strings.ToLower(fmt.Sprint(project["name"])+" "+fmt.Sprint(project["project_number"])), keyword)
				for _, child := range projects {
					if child["parent_project_id"] == id && child["status"] == "active" && strings.Contains(strings.ToLower(fmt.Sprint(child["name"])+" "+fmt.Sprint(child["project_number"])), keyword) {
						matches = true
					}
				}
				if matches {
					ids = append(ids, id)
				}
			}
			sort.Sort(sort.Reverse(sort.StringSlice(ids)))
			total := len(ids)
			end := offset + limit
			if end > total {
				end = total
			}
			if offset > total {
				offset = total
			}
			values := []map[string]any{}
			for _, id := range ids[offset:end] {
				entry := cloneProbeMap(projects[id])
				entry["assignments"] = currentAssignments(id)
				children := []map[string]any{}
				for childID, child := range projects {
					if child["parent_project_id"] == id && child["status"] == "active" {
						nested := cloneProbeMap(child)
						nested["assignments"] = currentAssignments(childID)
						nested["children"] = []any{}
						children = append(children, nested)
					}
				}
				sort.Slice(children, func(i, j int) bool { return fmt.Sprint(children[i]["id"]) < fmt.Sprint(children[j]["id"]) })
				entry["children"] = children
				values = append(values, entry)
			}
			write(w, 200, map[string]any{"schema_version": "1", "items": values, "total": total, "offset": offset, "limit": limit, "has_more": end < total, "allowed_actions": actions})
			return
		}
		if path == "/api/v1/projects" {
			values := []map[string]any{}
			for _, project := range projects {
				values = append(values, project)
			}
			write(w, 200, map[string]any{"items": values, "allowed_actions": actions})
			return
		}
		var input map[string]any
		if strings.HasPrefix(path, "/api/") && r.Method != http.MethodGet {
			if strings.HasSuffix(path, "/complete-initiation") {
				counts["activation_requests"]++
			}
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				write(w, 400, map[string]any{"error": map[string]any{"message": "invalid probe input"}})
				return
			}
			if cached, ok := replays[fmt.Sprint(input["command_id"])]; ok {
				write(w, 200, cached)
				return
			}
		}
		if path == "/api/v1/initiation-dispatches" && r.Method == http.MethodPost {
			owner, ok := input["business_owner"].(map[string]any)
			if !ok || len(input) != 4 || owner["person_id"] != business || owner["source_revision"] != float64(2) || input["name"] == "" {
				write(w, 422, map[string]any{"error": map[string]any{"message": "dispatch must contain name and exact versioned business owner only"}})
				return
			}
			id := fmt.Sprintf("33333333-3333-4333-8333-%012d", len(projects)+1)
			project := map[string]any{"id": id, "tenant_id": tenant, "name": input["name"], "node_type": input["node_type"], "project_number": "", "project_type_keys": []string{}, "summary": "", "revision": float64(1), "status": "draft", "created_at": "2026-10-02T00:00:00Z"}
			projects[id] = project
			assignments[id] = []map[string]any{{"id": newAssignmentID(), "project_id": id, "tenant_id": tenant, "role_key": "business_owner", "subject_kind": "person", "subject_id": business, "source_revision": 2, "display_snapshot": people[0]["display_name"]}}
			counts["dispatch"]++
			replays[fmt.Sprint(input["command_id"])] = cloneProbeMap(project)
			write(w, 201, project)
			return
		}
		if strings.HasPrefix(path, "/api/v1/projects/") {
			parts := strings.Split(strings.TrimPrefix(path, "/api/v1/projects/"), "/")
			id := parts[0]
			project, exists := projects[id]
			if !exists {
				write(w, 404, map[string]any{"error": map[string]any{"message": "unknown synthetic project"}})
				return
			}
			if len(parts) == 2 && parts[1] == "counterparty-context" {
				if r.Method == http.MethodGet {
					counts["client_context_read"]++
					if clientReadFailures > 0 {
						clientReadFailures--
						write(w, 503, map[string]any{"error": map[string]any{"message": "synthetic client read unavailable after successful save"}})
						return
					}
					allowed := []string{"project-workspace.counterparty-context.read"}
					if !clientReadOnly && project["status"] == "active" {
						allowed = append(allowed, "project-workspace.counterparty-context.manage")
					}
					write(w, 200, map[string]any{"project_id": id, "project_revision": project["revision"], "project_status": project["status"], "context": contexts[id], "allowed_actions": allowed})
					return
				}
				if clientReadOnly {
					write(w, 403, map[string]any{"error": map[string]any{"message": "synthetic view only"}})
					return
				}
				if r.Method != http.MethodPost || input["expected_revision"] != project["revision"] || project["status"] != "active" {
					write(w, 409, map[string]any{"error": map[string]any{"message": "synthetic client revision conflict"}})
					return
				}
				next := map[string]any{"schema_version": "project-counterparty-context-v1", "role": "client", "name": input["name"], "contact_name": input["contact_name"], "contact_phone": input["contact_phone"]}
				changed := contexts[id] == nil
				for _, key := range []string{"name", "contact_name", "contact_phone"} {
					if contexts[id][key] != next[key] {
						changed = true
					}
				}
				if changed {
					project["revision"] = project["revision"].(float64) + 1
					next["recorded_project_revision"] = project["revision"]
					next["recorded_at"] = time.Now().UTC().Format(time.RFC3339)
					contexts[id] = next
				}
				counts["client_context_change"]++
				result := map[string]any{"project_id": id, "project_revision": project["revision"], "project_status": project["status"], "context": contexts[id]}
				replays[fmt.Sprint(input["command_id"])] = cloneProbeMap(result)
				if os.Getenv("OA_CLIENT_READ_FAIL_AFTER_SAVE") == "1" && counts["client_context_change"] == 1 {
					clientReadFailures = 1
				}
				write(w, 200, result)
				return
			}
			if r.Method == http.MethodGet {
				allowed := append([]string{}, actions...)
				if project["status"] == "active" && basicInfoRole == "" && !clientReadOnly {
					allowed = []string{"project-workspace.read", "project-workspace.assignment.manage", "project-workspace.profile.correct"}
					allowed = append(allowed, "project-workspace.counterparty-context.read", "project-workspace.counterparty-context.manage")
				}
				detailAssignments := currentAssignments(id)
				if r.URL.Query().Get("include_inactive") == "true" {
					detailAssignments = assignments[id]
				}
				details := map[string]any{"project": project, "assignments": detailAssignments, "allowed_actions": allowed}
				children := []map[string]any{}
				if parentID, ok := project["parent_project_id"].(string); ok {
					details["parent"] = projects[parentID]
				}
				for _, child := range projects {
					if child["parent_project_id"] == id {
						children = append(children, child)
					}
				}
				details["children"] = children
				write(w, 200, details)
				return
			}
			if input["expected_revision"] != project["revision"] {
				write(w, 409, map[string]any{"error": map[string]any{"message": "synthetic revision conflict"}})
				return
			}
			switch {
			case len(parts) == 2 && (parts[1] == "basic-info-changes" || parts[1] == "initiation-info-changes") && r.Method == http.MethodPost:
				validState := project["status"] == "active"
				if parts[1] == "initiation-info-changes" {
					validState = project["status"] == "draft" || project["status"] == "pending_completion"
				}
				if !validState || input["reason"] == "" {
					write(w, 409, map[string]any{"error": map[string]any{"message": "synthetic basic-info state or reason"}})
					return
				}
				profile, _ := input["profile"].(map[string]any)
				owners, _ := input["owners"].([]any)
				for _, raw := range owners {
					v := raw.(map[string]any)
					if v["operation"] == "set" {
						person := v["person"].(map[string]any)
						valid := false
						for _, candidate := range people {
							if candidate["id"] == person["person_id"] && float64(candidate["revision"].(int)) == person["source_revision"] {
								valid = true
							}
						}
						if !valid {
							write(w, 422, map[string]any{"error": map[string]any{"message": "synthetic invalid person"}})
							return
						}
					}
				}
				changed := false
				ownerResults := []map[string]any{}
				if profile != nil {
					for _, key := range []string{"name", "project_number", "project_type_keys", "summary", "planned_start_date", "planned_end_date"} {
						previousJSON, _ := json.Marshal(project[key])
						nextJSON, _ := json.Marshal(profile[key])
						if string(previousJSON) != string(nextJSON) {
							changed = true
						}
						project[key] = profile[key]
					}
				}
				for _, raw := range owners {
					v := raw.(map[string]any)
					var old map[string]any
					for _, a := range assignments[id] {
						if a["role_key"] == v["role_key"] && a["ended_at"] == nil {
							old = a
						}
					}
					person, _ := v["person"].(map[string]any)
					different := (v["operation"] == "clear" && old != nil) || (v["operation"] == "set" && (old == nil || old["subject_id"] != person["person_id"]))
					entry := map[string]any{"role_key": v["role_key"], "changed": different}
					if old != nil {
						entry["previous"] = cloneProbeMap(old)
						entry["current"] = cloneProbeMap(old)
					}
					if different {
						changed = true
						delete(entry, "current")
						if old != nil {
							old["ended_at"] = time.Now().UTC().Format(time.RFC3339)
							entry["previous"] = cloneProbeMap(old)
						}
						if v["operation"] == "set" {
							var display any
							for _, candidate := range people {
								if candidate["id"] == person["person_id"] {
									display = candidate["display_name"]
								}
							}
							current := map[string]any{"id": newAssignmentID(), "project_id": id, "tenant_id": tenant, "role_key": v["role_key"], "subject_kind": "person", "subject_id": person["person_id"], "source_revision": person["source_revision"], "display_snapshot": display}
							assignments[id] = append(assignments[id], current)
							entry["current"] = cloneProbeMap(current)
						}
					}
					ownerResults = append(ownerResults, entry)
				}
				if changed {
					project["revision"] = project["revision"].(float64) + 1
				}
				counts["basic_info_change"]++
				response := map[string]any{"project": cloneProbeMap(project), "owners": ownerResults}
				replays[fmt.Sprint(input["command_id"])] = response
				write(w, 200, response)
				return
			case len(parts) == 2 && parts[1] == "profile-corrections" && r.Method == http.MethodPost:
				if project["status"] != "active" || input["correction_reason"] == "" {
					write(w, 409, map[string]any{"error": map[string]any{"message": "synthetic correction state or reason"}})
					return
				}
				for _, key := range []string{"name", "project_number", "project_type_keys", "summary", "planned_start_date", "planned_end_date"} {
					project[key] = input[key]
				}
				project["revision"] = project["revision"].(float64) + 1
				counts["profile_correction"]++
			case len(parts) == 1 && r.Method == http.MethodPut:
				for _, key := range []string{"name", "project_number", "project_type_keys", "summary", "planned_start_date", "planned_end_date"} {
					project[key] = input[key]
				}
				project["revision"] = project["revision"].(float64) + 1
				counts["update"]++
			case len(parts) == 2 && parts[1] == "complete-initiation":
				if input["initiation_profile"] != "business-owner-v1" || (project["node_type"] != "package" && project["project_number"] == "") {
					write(w, 422, map[string]any{"error": map[string]any{"message": "synthetic completion readiness"}})
					return
				}
				project["status"] = "active"
				project["revision"] = project["revision"].(float64) + 1
				counts["complete"]++
			case len(parts) == 2 && parts[1] == "assignments":
				if input["subject_id"] != production || input["source_revision"] != float64(3) || input["role_key"] != "production_owner" {
					write(w, 422, map[string]any{"error": map[string]any{"message": "incorrect synthetic production owner"}})
					return
				}
				assignment := map[string]any{"id": newAssignmentID(), "project_id": id, "tenant_id": tenant, "role_key": "production_owner", "subject_kind": "person", "subject_id": production, "source_revision": 3, "display_snapshot": people[1]["display_name"]}
				assignments[id] = append(assignments[id], assignment)
				project["revision"] = project["revision"].(float64) + 1
				counts["production_assignment"]++
				response := map[string]any{"project": cloneProbeMap(project), "assignment": assignment}
				replays[fmt.Sprint(input["command_id"])] = response
				write(w, 201, response)
				return
			default:
				write(w, 404, map[string]any{"error": map[string]any{"message": "unsupported synthetic route"}})
				return
			}
			replays[fmt.Sprint(input["command_id"])] = cloneProbeMap(project)
			// Simulate a proxy losing the acknowledgement after the owner has
			// committed. The cached receipt remains available for an explicit
			// retry. This fault cannot ship outside this opt-in synthetic server.
			if len(parts) == 2 && parts[1] == "complete-initiation" && completionFailure == "activation-unknown-once" && !activationReplyLost {
				activationReplyLost = true
				write(w, 503, map[string]any{"error": map[string]any{"message": "synthetic activation acknowledgement unavailable"}})
				return
			}
			write(w, 200, project)
			return
		}
		r.Header.Set("X-OMS-Subject-ID", "88888888-8888-4888-8888-888888888888")
		r.Header.Set("X-OMS-Display-Name", "隔离界面测试，不是真实 IAM")
		r.Header.Set("X-OMS-Administrator", "true")
		r.Header.Set("X-OMS-CSRF-Token", "synthetic-ui-probe-not-a-credential")
		handler.ServeHTTP(w, r)
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	fmt.Println("SYNTHETIC_UI_PROBE http://127.0.0.1:18114/ (not IAM or persistence acceptance)")
	select {
	case <-finished:
	case <-time.After(15 * time.Minute):
		t.Fatal("browser probe timed out")
	}
}

func cloneProbeMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}
