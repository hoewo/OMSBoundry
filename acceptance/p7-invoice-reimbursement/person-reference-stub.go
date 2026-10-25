package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	tenant, person := os.Getenv("TENANT_ID"), os.Getenv("PERSON_ID")
	if tenant == "" || person == "" {
		log.Fatal("TENANT_ID and PERSON_ID are required")
	}
	http.HandleFunc("/health/ready", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})
	http.HandleFunc("/api/v1/references/iam-subjects/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || strings.TrimPrefix(r.URL.Path, "/api/v1/references/iam-subjects/") == "" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"schema_version": "1", "source_application": "people-groups", "resource_type": "person",
			"id": person, "tenant_id": tenant, "status": "active", "revision": 1,
			"reference_code": "P7-EMP-001", "display_name": "P7 测试员工",
		})
	})
	log.Fatal(http.ListenAndServe("127.0.0.1:19432", nil))
}
