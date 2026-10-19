package partymigration

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

const (
	testBatchID   = "44444444-4444-4444-8444-444444444444"
	testSubjectID = "55555555-5555-4555-8555-555555555555"
	testCommandID = "66666666-6666-4666-8666-666666666666"
	testLocalID   = "77777777-7777-4777-8777-777777777777"
	testTargetID  = "88888888-8888-4888-8888-888888888888"
)

func validMappingRequest() MappingRequest {
	return MappingRequest{
		Format: MappingRequestFormat, TenantID: testTenantID, MigrationBatchID: testBatchID,
		ReviewedBySubjectID: testSubjectID, ReviewedByDisplayName: "迁移审核员",
		Mappings: []MappingDecision{{
			CommandID: testCommandID, LocalPartyID: testLocalID,
			LocalSnapshotSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Decision:            "map_existing_directory_party", TargetPartyID: testTargetID, TargetPartyRevision: 3, Reason: "人工核对统一社会信用代码和名称",
		}},
	}
}

func TestNormalizeMappingRequestAndDigestAreDeterministic(t *testing.T) {
	request := validMappingRequest()
	request.ReviewedByDisplayName = " 迁移审核员 "
	request.Mappings[0].Reason = " 人工核对统一社会信用代码和名称 "
	first, err := normalizeMappingRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := normalizeMappingRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	firstDigest, err := mappingRequestDigest(first, first.Mappings[0])
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := mappingRequestDigest(second, second.Mappings[0])
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest != secondDigest || len(firstDigest) != 64 || !reflect.DeepEqual(first, second) {
		t.Fatalf("normalization is not deterministic: first=%#v second=%#v", first, second)
	}
}

func TestNormalizeMappingRequestRejectsDuplicateLocalParty(t *testing.T) {
	request := validMappingRequest()
	duplicate := request.Mappings[0]
	duplicate.CommandID = "99999999-9999-4999-8999-999999999999"
	request.Mappings = append(request.Mappings, duplicate)
	if _, err := normalizeMappingRequest(request); err == nil {
		t.Fatal("duplicate local Party was accepted")
	}
}

func TestHTTPPartyValidatorRequiresExactActiveRevision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/references/parties/"+testTargetID || request.Header.Get("Authorization") != "Bearer exact-token" {
			http.Error(response, "bad request", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(response, `{"schema_version":"1","source_application":"party-directory","id":"%s","tenant_id":"%s","kind":"organization","legal_name":"目标公司","registration_identifier":"91330000TARGET","status":"active","revision":3}`, testTargetID, testTenantID)
	}))
	defer server.Close()
	validator, err := NewHTTPPartyValidator(2*time.Second, PartyEndpoint{BaseURL: server.URL, AccessToken: "exact-token"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := validator.VerifyParty(context.Background(), testTenantID, testTargetID, 3)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.PartyID != testTargetID || snapshot.Revision != 3 || snapshot.Kind != "organization" || snapshot.RegistrationIdentifier == nil {
		t.Fatalf("unexpected target snapshot: %#v", snapshot)
	}
	if _, err := validator.VerifyParty(context.Background(), testTenantID, testTargetID, 2); !errors.Is(err, ErrTargetInvalid) {
		t.Fatalf("stale target revision error = %v", err)
	}
}

func TestHTTPPartyValidatorFailsClosedWhenPartyIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		http.Error(response, "down", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	validator, err := NewHTTPPartyValidator(time.Second, PartyEndpoint{BaseURL: server.URL, AccessToken: "exact-token"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validator.VerifyParty(context.Background(), testTenantID, testTargetID, 3); !errors.Is(err, ErrTargetUnavailable) {
		t.Fatalf("unavailable target error = %v", err)
	}
}
