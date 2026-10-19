package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
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
		writeJSON(w, http.StatusOK, map[string]string{"service": "project-reference-contract-stub", "status": "ready"})
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
	if len(segments) == 4 && segments[0] == "api" && segments[1] == "v1" && segments[2] == "persons" {
		s.people(w, r, "person", segments[3])
		return
	}
	if len(segments) == 4 && segments[0] == "api" && segments[1] == "v1" && segments[2] == "groups" {
		s.people(w, r, "group", segments[3])
		return
	}
	if len(segments) == 5 && segments[0] == "api" && segments[1] == "v1" && segments[2] == "references" && segments[3] == "contracts" {
		s.contract(w, r, segments[4])
		return
	}
	if len(segments) == 4 && segments[0] == "v1" && segments[1] == "references" && segments[2] == "files" {
		s.evidence(w, segments[3])
		return
	}
	http.NotFound(w, r)
}

func (s *server) people(w http.ResponseWriter, r *http.Request, kind, id string) {
	prefix := firstSegment(id)
	switch prefix {
	case "99999999":
		http.NotFound(w, r)
		return
	case "33333333":
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "reference_unavailable"})
		return
	}
	if kind == "person" && prefix == "11111111" {
		if s.readCount(r.URL.Path) > 1 {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "reference_unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "tenant_id": tenantID, "display_name": "经营负责人 林清和", "status": "active", "revision": 1})
		return
	}
	if kind == "group" && prefix == "22222222" {
		writeJSON(w, http.StatusOK, map[string]any{"group": map[string]any{"id": id, "tenant_id": tenantID, "name": "青岚设计生产组", "status": "active", "revision": 2}})
		return
	}
	http.NotFound(w, r)
}

func (s *server) contract(w http.ResponseWriter, r *http.Request, id string) {
	switch firstSegment(id) {
	case "99999999":
		http.NotFound(w, r)
	case "66666666":
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "reference_unavailable"})
	case "55555555":
		if s.readCount(r.URL.Path) > 1 {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "reference_unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"schema_version":           "1",
			"source_application":       "contract-ledger",
			"id":                       id,
			"tenant_id":                tenantID,
			"resource_type":            "contract",
			"display_summary":          "青岚设计服务合同",
			"status":                   "active",
			"revision":                 3,
			"currency":                 "CNY",
			"current_effective_amount": "125000.00",
		})
	default:
		http.NotFound(w, r)
	}
}

func (s *server) evidence(w http.ResponseWriter, id string) {
	if firstSegment(id) != "77777777" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "tenant_id": tenantID, "resource_type": "evidence-file", "status": "available", "revision": 1})
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
