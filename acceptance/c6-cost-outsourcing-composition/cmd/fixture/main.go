package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	tenantID       = "11111111-1111-4111-8111-111111111111"
	subjectID      = "22222222-2222-4222-8222-222222222222"
	projectID      = "33333333-3333-4333-8333-333333333333"
	wrongProjectID = "33333333-3333-4333-8333-333333333334"
	partyID        = "44444444-4444-4444-8444-444444444444"
	ownerPartyID   = "44444444-4444-4444-8444-444444444445"
	contractID     = "55555555-5555-4555-8555-555555555555"
	evidenceID     = "66666666-6666-4666-8666-666666666666"
	keyID          = "88888888-8888-4888-8888-888888888888"
	issuer         = "https://iam.c6.test"
)

type privateFile struct {
	PrivateKey string `json:"private_key"`
}

func main() {
	mode := flag.String("mode", "", "init, iam, project, party, contract, or evidence")
	address := flag.String("address", "127.0.0.1:19170", "listen address")
	privatePath := flag.String("private", "", "fixture private key path")
	publicPath := flag.String("public", "", "IAM public key set path")
	flag.Parse()
	if *mode == "init" {
		if err := initKeys(*privatePath, *publicPath); err != nil {
			log.Fatal(err)
		}
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]string{"status": "ok", "service": *mode})
	})
	switch *mode {
	case "iam":
		privateKey, err := readPrivate(*privatePath)
		if err != nil {
			log.Fatal(err)
		}
		mux.HandleFunc("POST /api/v1/service-credentials", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, map[string]any{"token": "reference-token", "expires_at": time.Now().UTC().Add(2 * time.Minute)})
		})
		mux.HandleFunc("GET /issue", func(w http.ResponseWriter, r *http.Request) {
			profile := r.URL.Query().Get("profile")
			actions := fullActions()
			scopeProject := projectID
			switch profile {
			case "redacted":
				actions = []string{"cost-entry.read", "cost-entry.history.read", "outsourcing-engagement.read", "outsourcing-engagement.history.read"}
			case "read-only":
				actions = []string{"cost-entry.read", "outsourcing-engagement.read"}
			case "wrong-project":
				scopeProject = wrongProjectID
			case "full", "":
			default:
				http.Error(w, "unknown profile", http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]string{"token": sign(privateKey, actions, scopeProject)})
		})
	case "project":
		mux.HandleFunc("GET /api/v1/references/projects/{id}", guarded(projectReference))
	case "party":
		mux.HandleFunc("GET /api/v1/references/parties/{id}", guarded(partyReference))
	case "contract":
		mux.HandleFunc("GET /api/v1/references/contracts/{id}", guarded(contractReference))
	case "evidence":
		mux.HandleFunc("GET /v1/references/files/{id}/versions/{version}", guarded(evidenceReference))
	default:
		log.Fatal("unsupported mode")
	}
	log.Printf("%s fixture listening on %s", *mode, *address)
	log.Fatal(http.ListenAndServe(*address, mux))
}

func initKeys(privatePath, publicPath string) error {
	if privatePath == "" || publicPath == "" {
		return fmt.Errorf("private and public paths are required")
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	privateData, _ := json.Marshal(privateFile{PrivateKey: base64.RawURLEncoding.EncodeToString(privateKey)})
	publicData, _ := json.Marshal(map[string]any{"keys": []map[string]any{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": keyID, "x": base64.RawURLEncoding.EncodeToString(publicKey), "status": "active"}}})
	if err = os.WriteFile(privatePath, privateData, 0o600); err != nil {
		return err
	}
	return os.WriteFile(publicPath, publicData, 0o644)
}

func readPrivate(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var value privateFile
	if err = json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	raw, err := base64.RawURLEncoding.DecodeString(value.PrivateKey)
	if err != nil || len(raw) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key")
	}
	return ed25519.PrivateKey(raw), nil
}

func fullActions() []string {
	return []string{"cost-entry.create", "cost-entry.read", "cost-entry.amount.read", "cost-entry.correct", "cost-entry.withdraw", "cost-entry.evidence.manage", "cost-entry.history.read", "outsourcing-engagement.create", "outsourcing-engagement.read", "outsourcing-engagement.amount.read", "outsourcing-engagement.correct", "outsourcing-engagement.withdraw", "outsourcing-engagement.assess", "outsourcing-engagement.evidence.manage", "outsourcing-engagement.history.read"}
}

func sign(privateKey ed25519.PrivateKey, actions []string, scopeProject string) string {
	now := time.Now().UTC().Unix()
	header, _ := json.Marshal(map[string]any{"alg": "EdDSA", "typ": "JWT", "kid": keyID})
	claims, _ := json.Marshal(map[string]any{"ver": "1", "iss": issuer, "sub": subjectID, "tid": tenantID, "aud": "cost-outsourcing", "name": "C6 composition tester", "actions": actions, "scope": map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "project", "id": scopeProject}}}, "iat": now, "nbf": now - 1, "exp": now + 240, "auth_time": now - 1, "jti": newUUID()})
	encoded := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	return encoded + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(encoded)))
}

func guarded(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		credential := r.Header.Get("Authorization")
		if credential != "Bearer reference-token" && credential != "Bearer reference-token-1234567890" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func projectReference(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("id") != projectID {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, map[string]any{"id": projectID, "tenant_id": tenantID, "resource_type": "project", "project_number": "P-C6-001", "display_summary": "C6 最小组合项目", "status": "active", "revision": 7, "resource_links": []map[string]any{{"target_service": "contract-ledger", "resource_type": "contract", "resource_id": contractID, "source_revision": 9}}})
}

func partyReference(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == ownerPartyID {
		writeJSON(w, map[string]any{"schema_version": "1", "source_application": "party-directory", "id": ownerPartyID, "tenant_id": tenantID, "kind": "organization", "legal_name": "C6 设计单位", "status": "active", "revision": 2})
		return
	}
	if id != partyID {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, map[string]any{"schema_version": "1", "source_application": "party-directory", "id": partyID, "tenant_id": tenantID, "kind": "organization", "legal_name": "C6 协作单位", "status": "active", "revision": 5})
}

func contractReference(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("id") != contractID {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, map[string]any{"schema_version": "1", "source_application": "contract-ledger", "id": contractID, "tenant_id": tenantID, "resource_type": "contract", "display_summary": "C6 外协合同", "status": "active", "revision": 9, "currency": "CNY", "current_effective_amount": "5000.00", "party_ids": []string{partyID}})
}

func evidenceReference(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("id") != evidenceID || r.PathValue("version") != "2" {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, map[string]any{"id": evidenceID, "file_id": evidenceID, "tenant_id": tenantID, "resource_type": "evidence-file", "status": "available", "version": 2, "revision": 2, "display_name": "C6凭证.pdf", "sha256": strings.Repeat("a", 64)})
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func newUUID() string {
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}
