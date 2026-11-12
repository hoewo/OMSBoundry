package evidence

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	contract "omsbizfoundry/invoice-recognition-contract"
)

func TestOpenRewritesPublicLocalGrantToInternalEvidenceOrigin(t *testing.T) {
	const (
		tenantID = "11111111-1111-4111-8111-111111111111"
		fileID   = "22222222-2222-4222-8222-222222222222"
		digest   = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	)
	content := []byte("invoice-image")
	var origin string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/references/files/"+fileID+"/versions/1":
			fmt.Fprintf(w, `{"file_id":%q,"tenant_id":%q,"status":"available","version":1,"size_bytes":%d,"media_type":"image/jpeg","sha256":%q,"security_status":"not_required"}`, fileID, tenantID, len(content), digest)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/files/"+fileID+"/versions/1/access-grants":
			fmt.Fprintf(w, `{"file_id":%q,"version":1,"url":"https://public.invalid/evidence/v1/local-files/%s/versions/1?token=signed","method":"GET"}`, fileID, fileID)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/local-files/"+fileID+"/versions/1" && r.URL.Query().Get("token") == "signed":
			_, _ = w.Write(content)
		default:
			http.Error(w, r.Method+" "+r.URL.String(), http.StatusNotFound)
		}
	}))
	defer server.Close()
	origin = server.URL
	client, err := NewHTTPClient(origin, "test-token", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := client.Open(context.Background(), tenantID, contract.EvidenceReference{FileID: fileID, Version: 1, ContentSHA256: "sha256:" + digest, MIMEType: "image/jpeg", SizeBytes: int64(len(content))})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	actual, err := io.ReadAll(stream)
	if err != nil || string(actual) != string(content) {
		t.Fatalf("content=%q err=%v", actual, err)
	}
}

func TestExternalObjectGrantKeepsProviderOrigin(t *testing.T) {
	const fileID = "22222222-2222-4222-8222-222222222222"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "access-grants") {
			fmt.Fprintf(w, `{"file_id":%q,"version":1,"url":"https://private-bucket.example/signed-object","method":"GET"}`, fileID)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, "test-token", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	value, err := client.createGrant(context.Background(), "11111111-1111-4111-8111-111111111111", contract.EvidenceReference{FileID: fileID, Version: 1}, "test-token", false)
	if err != nil || value != "https://private-bucket.example/signed-object" {
		t.Fatalf("url=%q err=%v", value, err)
	}
}
