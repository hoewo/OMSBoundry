package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

const tenantID = "99999999-9999-4999-8999-999999999999"

type server struct {
	token string
	mu    sync.Mutex
	reads map[string]int
}

func main() {
	token := os.Getenv("REFERENCE_STUB_TOKEN")
	if len(token) < 24 {
		log.Fatal("REFERENCE_STUB_TOKEN must contain at least 24 bytes")
	}
	s := &server{token: token, reads: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"service": "design-reference-contract-stub", "status": "ready"})
	})
	mux.HandleFunc("GET /", s.reference)
	log.Fatal(http.ListenAndServe(":9090", mux))
}

func (s *server) reference(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+s.token {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "credential_required"})
		return
	}
	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(segments) == 5 && segments[0] == "api" && segments[1] == "v1" && segments[2] == "references" && segments[3] == "projects" {
		s.project(w, r, segments[4])
		return
	}
	if len(segments) == 7 && segments[0] == "api" && segments[1] == "v1" && segments[2] == "references" && segments[3] == "projects" && segments[5] == "assignments" {
		s.assignment(w, r, segments[4], segments[6])
		return
	}
	if len(segments) == 6 && segments[0] == "v1" && segments[1] == "references" && segments[2] == "files" && segments[4] == "versions" {
		s.evidence(w, r, segments[3], segments[5])
		return
	}
	http.NotFound(w, r)
}

func (s *server) project(w http.ResponseWriter, r *http.Request, id string) {
	switch firstSegment(id) {
	case "99999999":
		http.NotFound(w, r)
	case "33333333":
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "reference_unavailable"})
	case "44444444":
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "tenant_id": tenantID, "status": "draft", "revision": 4, "project_type_keys": []string{"design"}})
	case "88888888":
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "tenant_id": tenantID, "status": "active", "revision": 4, "project_type_keys": []string{"design"}})
	default:
		http.NotFound(w, r)
	}
}

func (s *server) assignment(w http.ResponseWriter, r *http.Request, projectID, id string) {
	switch firstSegment(id) {
	case "99999999":
		http.NotFound(w, r)
	case "33333333":
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "reference_unavailable"})
	case "44444444":
		writeJSON(w, http.StatusOK, assignmentValue(projectID, id, "participant", "方案设计师"))
	case "11111111":
		if s.readCount(r.URL.Path) > 1 {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "reference_unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, assignmentValue(projectID, id, "production_owner", "临时设计生产组"))
	case "22222222":
		writeJSON(w, http.StatusOK, assignmentValue(projectID, id, "production_owner", "青岚设计生产组"))
	default:
		http.NotFound(w, r)
	}
}

func assignmentValue(projectID, id, role, display string) map[string]any {
	return map[string]any{"id": id, "tenant_id": tenantID, "project_id": projectID, "role_key": role, "status": "active", "display_name": display, "revision": 3}
}

func (s *server) evidence(w http.ResponseWriter, r *http.Request, id, rawVersion string) {
	version, err := strconv.Atoi(rawVersion)
	if err != nil || version != 1 {
		http.NotFound(w, r)
		return
	}
	prefix := firstSegment(id)
	switch prefix {
	case "99999999":
		http.NotFound(w, r)
	case "77777777":
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "reference_unavailable"})
	case "66666666":
		if s.readCount(r.URL.Path) > 2 {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "reference_unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, evidenceValue(id, "施工配合成果.pdf"))
	default:
		if strings.HasPrefix(prefix, "5555555") {
			writeJSON(w, http.StatusOK, evidenceValue(id, "设计成果-"+prefix+".pdf"))
			return
		}
		http.NotFound(w, r)
	}
}

func evidenceValue(id, display string) map[string]any {
	return map[string]any{"file_id": id, "tenant_id": tenantID, "status": "available", "display_name": display, "version": 1, "revision": 1}
}

func (s *server) readCount(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads[path]++
	return s.reads[path]
}

func firstSegment(id string) string {
	if index := strings.IndexByte(id, '-'); index >= 0 {
		return id[:index]
	}
	return id
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
