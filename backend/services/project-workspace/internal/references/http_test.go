package references

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"omsbizfoundry/project-workspace/internal/projects"
)

const (
	tenantID   = "11111111-1111-4111-8111-111111111111"
	resourceID = "22222222-2222-4222-8222-222222222222"
)

type credentialSpy struct {
	audience, action, resourceType, resourceID string
}

func (s *credentialSpy) Credential(_ context.Context, audience, action, resourceType, resourceID string) (string, error) {
	s.audience, s.action, s.resourceType, s.resourceID = audience, action, resourceType, resourceID
	return "dynamic-token", nil
}
func (*credentialSpy) Invalidate(string, string, string, string) {}

func TestValidatePeopleAcceptsPersonAndGroupShapes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer reference-token" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/persons/"+resourceID {
			_, _ = w.Write([]byte(`{"id":"` + resourceID + `","tenant_id":"` + tenantID + `","display_name":"张设计","status":"active","revision":3}`))
			return
		}
		_, _ = w.Write([]byte(`{"group":{"id":"` + resourceID + `","tenant_id":"` + tenantID + `","name":"设计一组","status":"active","revision":4}}`))
	}))
	defer server.Close()
	validator, err := NewHTTPValidator(time.Second, Endpoint{BaseURL: server.URL, AccessToken: "reference-token"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	person, err := validator.ValidatePeople(context.Background(), tenantID, "person", resourceID, 3)
	if err != nil || person.DisplayName != "张设计" {
		t.Fatalf("person=%#v error=%v", person, err)
	}
	group, err := validator.ValidatePeople(context.Background(), tenantID, "group", resourceID, 4)
	if err != nil || group.DisplayName != "设计一组" {
		t.Fatalf("group=%#v error=%v", group, err)
	}
}

func TestValidatePeopleRejectsStaleOrArchivedReference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"` + resourceID + `","tenant_id":"` + tenantID + `","display_name":"旧人员","status":"archived","revision":2}`))
	}))
	defer server.Close()
	validator, _ := NewHTTPValidator(time.Second, Endpoint{BaseURL: server.URL, AccessToken: "token"}, nil)
	_, err := validator.ValidatePeople(context.Background(), tenantID, "person", resourceID, 3)
	if !errors.Is(err, projects.ErrReferenceInvalid) {
		t.Fatalf("error = %v", err)
	}
}

func TestValidatePeopleDistinguishesInvalidAndUnavailable(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   error
	}{
		{"not found", http.StatusNotFound, projects.ErrReferenceInvalid},
		{"upstream failure", http.StatusBadGateway, projects.ErrReferenceUnavailable},
		{"unauthorized upstream", http.StatusUnauthorized, projects.ErrReferenceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(test.status) }))
			defer server.Close()
			validator, _ := NewHTTPValidator(time.Second, Endpoint{BaseURL: server.URL, AccessToken: "token"}, nil)
			_, err := validator.ValidatePeople(context.Background(), tenantID, "person", resourceID, 1)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestValidateResourceChecksTypeTenantAndRevision(t *testing.T) {
	revision := int64(7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/references/contracts/"+resourceID {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"schema_version":"1","source_application":"contract-ledger","id":"` + resourceID + `","tenant_id":"` + tenantID + `","resource_type":"contract","display_summary":"设计合同","status":"active","revision":7,"currency":"CNY","current_effective_amount":"1200.00"}`))
	}))
	defer server.Close()
	validator, _ := NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{"contract-ledger": {BaseURL: server.URL, AccessToken: "token"}})
	if err := validator.ValidateResource(context.Background(), tenantID, "contract-ledger", "contract", resourceID, &revision); err != nil {
		t.Fatalf("ValidateResource() error = %v", err)
	}
	wrong := int64(6)
	if err := validator.ValidateResource(context.Background(), tenantID, "contract-ledger", "contract", resourceID, &wrong); !errors.Is(err, projects.ErrReferenceInvalid) {
		t.Fatalf("stale revision error = %v", err)
	}
}

func TestValidatePartyReturnsMinimumVerifiedSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/references/parties/"+resourceID {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"schema_version":"1","source_application":"party-directory","id":"` + resourceID + `","tenant_id":"` + tenantID + `","kind":"organization","legal_name":"甲方公司","registration_identifier":"ignored","status":"active","revision":5}`))
	}))
	defer server.Close()
	validator, _ := NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{"party-directory": {BaseURL: server.URL, AccessToken: "token"}})
	snapshot, err := validator.ValidateParty(context.Background(), tenantID, resourceID, 5)
	if err != nil || snapshot.PartyID != resourceID || snapshot.SourceRevision != 5 || snapshot.Kind != "organization" || snapshot.LegalName != "甲方公司" {
		t.Fatalf("snapshot=%#v error=%v", snapshot, err)
	}
}

func TestValidatePartyRequestsExactIAMCapabilityCredential(t *testing.T) {
	credentials := &credentialSpy{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer dynamic-token" {
			t.Fatalf("authorization=%q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"schema_version":"1","source_application":"party-directory","id":"` + resourceID + `","tenant_id":"` + tenantID + `","kind":"individual","legal_name":"张三","status":"active","revision":2}`))
	}))
	defer server.Close()
	validator, err := NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{
		"party-directory": {BaseURL: server.URL, Audience: "party-directory", Action: "party-directory.reference.read", Credentials: credentials},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validator.ValidateParty(context.Background(), tenantID, resourceID, 2); err != nil {
		t.Fatal(err)
	}
	if credentials.audience != "party-directory" || credentials.action != "party-directory.reference.read" || credentials.resourceType != "party" || credentials.resourceID != resourceID {
		t.Fatalf("credential request=%#v", credentials)
	}
}

func TestValidatePartyRejectsStaleCrossTenantOrMalformedProjection(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"stale", `{"schema_version":"1","source_application":"party-directory","id":"` + resourceID + `","tenant_id":"` + tenantID + `","kind":"organization","legal_name":"甲方","status":"active","revision":4}`},
		{"cross tenant", `{"schema_version":"1","source_application":"party-directory","id":"` + resourceID + `","tenant_id":"99999999-9999-4999-8999-999999999999","kind":"organization","legal_name":"甲方","status":"active","revision":5}`},
		{"inactive", `{"schema_version":"1","source_application":"party-directory","id":"` + resourceID + `","tenant_id":"` + tenantID + `","kind":"organization","legal_name":"甲方","status":"inactive","revision":5}`},
		{"wrong source", `{"schema_version":"1","source_application":"other","id":"` + resourceID + `","tenant_id":"` + tenantID + `","kind":"organization","legal_name":"甲方","status":"active","revision":5}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(test.body)) }))
			defer server.Close()
			validator, _ := NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{"party-directory": {BaseURL: server.URL, AccessToken: "token"}})
			if _, err := validator.ValidateParty(context.Background(), tenantID, resourceID, 5); !errors.Is(err, projects.ErrReferenceInvalid) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestValidateResourceAcceptsDesignStableReference(t *testing.T) {
	revision := int64(9)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/references/design-work-items/"+resourceID {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"` + resourceID + `","work_item_id":"` + resourceID + `","tenant_id":"` + tenantID + `","resource_type":"design-work-item","status":"completed","revision":9}`))
	}))
	defer server.Close()
	validator, _ := NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{"design-operations": {BaseURL: server.URL, AccessToken: "token"}})
	if err := validator.ValidateResource(context.Background(), tenantID, "design-operations", "design-work-item", resourceID, &revision); err != nil {
		t.Fatalf("ValidateResource() error = %v", err)
	}
}

func TestResolveCommercialReferencesRequestsExactIAMCredentialAndKeepsTypedFacts(t *testing.T) {
	credentials := &credentialSpy{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer dynamic-token" {
			t.Fatalf("authorization=%q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/api/v1/references/contracts/" + resourceID:
			_, _ = w.Write([]byte(`{"schema_version":"1","source_application":"contract-ledger","id":"` + resourceID + `","tenant_id":"` + tenantID + `","resource_type":"contract","display_summary":"施工协议","status":"active","revision":7,"currency":"CNY","current_effective_amount":"128000.00","party_references":[{"id":"77777777-7777-4777-8777-777777777777","revision":4,"kind":"organization","legal_name":"施工方公司","source":"directory","role":"施工方"}]}`))
		case "/api/v1/references/obligations/" + resourceID:
			_, _ = w.Write([]byte(`{"schema_version":"1","source_application":"money-flow-ledger","id":"` + resourceID + `","tenant_id":"` + tenantID + `","resource_type":"obligation","display_summary":"首期应收","status":"open","revision":8,"direction":"receivable","currency":"CNY","effective_amount":"50000.00","allocated_amount":"20000.00","remitted_amount":"10000.00","outstanding_amount":"20000.00","balance_sequence":3,"party_references":[{"id":"77777777-7777-4777-8777-777777777777","revision":4,"kind":"organization","legal_name":"甲方公司","source":"party-directory","role":"debtor"}]}`))
		case "/api/v1/references/transactions/" + resourceID:
			_, _ = w.Write([]byte(`{"schema_version":"1","source_application":"money-flow-ledger","id":"` + resourceID + `","tenant_id":"` + tenantID + `","resource_type":"transaction","display_summary":"首期回款","status":"recorded","revision":1,"transaction_type":"receipt","currency":"CNY","effective_amount":"20000.00","allocated_amount":"15000.00","unallocated_amount":"5000.00","occurred_on":"2026-09-27"}`))
		case "/api/v1/references/invoice-registrations/" + resourceID:
			_, _ = w.Write([]byte(`{"schema_version":"1","source_application":"billing-invoice","id":"` + resourceID + `","tenant_id":"` + tenantID + `","status":"registered","issued_on":"2026-09-27","amount_minor":1800000,"currency":"CNY","revision":2}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	endpoint := func(audience, action string) Endpoint {
		return Endpoint{BaseURL: server.URL, Audience: audience, Action: action, Credentials: credentials}
	}
	validator, err := NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{
		"contract-ledger":   endpoint("contract-ledger", "contract-ledger.reference.read"),
		"money-flow-ledger": endpoint("money-flow-ledger", "money-flow-ledger.reference.read"),
		"billing-invoice":   endpoint("billing-invoice", "billing-invoice.reference.read"),
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		service, resourceType, audience, action string
		assert                                  func(projects.ResourceReference) bool
	}{
		{"contract-ledger", "contract", "contract-ledger", "contract-ledger.reference.read", func(value projects.ResourceReference) bool {
			return value.Contract != nil && value.Contract.CurrentEffectiveAmount != nil && *value.Contract.CurrentEffectiveAmount == "128000.00" && len(value.Contract.Parties) == 1 && value.Contract.Parties[0].LegalName == "施工方公司" && value.Contract.Parties[0].Role == "施工方"
		}},
		{"money-flow-ledger", "obligation", "money-flow-ledger", "money-flow-ledger.reference.read", func(value projects.ResourceReference) bool {
			return value.Obligation != nil && value.Obligation.BalanceSequence == 3 && value.Obligation.OutstandingAmount == "20000.00" && len(value.Obligation.Parties) == 1 && value.Obligation.Parties[0].LegalName == "甲方公司" && value.Obligation.Parties[0].Role == "debtor"
		}},
		{"money-flow-ledger", "transaction", "money-flow-ledger", "money-flow-ledger.reference.read", func(value projects.ResourceReference) bool {
			return value.Transaction != nil && value.Transaction.UnallocatedAmount == "5000.00"
		}},
		{"billing-invoice", "invoice_registration", "billing-invoice", "billing-invoice.reference.read", func(value projects.ResourceReference) bool {
			return value.Invoice != nil && value.Invoice.AmountMinor == 1800000
		}},
	}
	for _, test := range tests {
		value, resolveErr := validator.ResolveResource(context.Background(), tenantID, test.service, test.resourceType, resourceID)
		if resolveErr != nil || !test.assert(value) {
			t.Fatalf("%s/%s value=%#v error=%v", test.service, test.resourceType, value, resolveErr)
		}
		if credentials.audience != test.audience || credentials.action != test.action || credentials.resourceType != test.resourceType || credentials.resourceID != resourceID {
			t.Fatalf("%s/%s credential request=%#v", test.service, test.resourceType, credentials)
		}
	}
}

func TestResolveEvidenceMetadataUsesExactCapabilityAndRejectsUnsafeSources(t *testing.T) {
	credentials := &credentialSpy{}
	body := `{"schema_version":"1","source_application":"evidence-service","id":"` + resourceID + `","tenant_id":"` + tenantID + `","resource_type":"evidence-file","display_summary":"成果.pdf","status":"available","version":3,"revision":3,"file_name":"成果.pdf","size_bytes":42,"media_type":"application/pdf","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/references/files/"+resourceID || r.Header.Get("Authorization") != "Bearer dynamic-token" {
			t.Fatalf("path=%q authorization=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	validator, err := NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{
		"evidence-service": {BaseURL: server.URL, Audience: "evidence-service", Action: "evidence-service.metadata.read", Credentials: credentials},
	})
	if err != nil {
		t.Fatal(err)
	}
	value, err := validator.ResolveResource(context.Background(), tenantID, "evidence-service", "evidence-file", resourceID)
	if err != nil || value.Evidence == nil || value.Evidence.Version != 3 || value.Evidence.SHA256 == "" {
		t.Fatalf("evidence=%#v error=%v", value, err)
	}
	if credentials.audience != "evidence-service" || credentials.action != "evidence-service.metadata.read" || credentials.resourceType != "file" || credentials.resourceID != resourceID {
		t.Fatalf("credential request=%#v", credentials)
	}

	wrongTenant := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Replace(body, tenantID, "99999999-9999-4999-8999-999999999999", 1)))
	}))
	defer wrongTenant.Close()
	validator, _ = NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{"evidence-service": {BaseURL: wrongTenant.URL, AccessToken: "token"}})
	if _, err := validator.ResolveResource(context.Background(), tenantID, "evidence-service", "evidence-file", resourceID); !errors.Is(err, projects.ErrReferenceInvalid) {
		t.Fatalf("wrong tenant error=%v", err)
	}

	forbidden := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer forbidden.Close()
	validator, _ = NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{"evidence-service": {BaseURL: forbidden.URL, AccessToken: "token"}})
	if _, err := validator.ResolveResource(context.Background(), tenantID, "evidence-service", "evidence-file", resourceID); !errors.Is(err, projects.ErrReferenceForbidden) {
		t.Fatalf("forbidden error=%v", err)
	}
}

func TestResolveBusinessEngagementMaterialsUsesExactCapabilityAndKeepsOnlyReferenceFacts(t *testing.T) {
	credentials := &credentialSpy{}
	fileID := "33333333-3333-4333-8333-333333333333"
	projectID := "44444444-4444-4444-8444-444444444444"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/references/business-engagements/"+resourceID+"/materials" || r.Header.Get("Authorization") != "Bearer dynamic-token" {
			t.Fatalf("path=%q authorization=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"schema_version":"1","source_application":"business-engagement","id":"` + resourceID + `","tenant_id":"` + tenantID + `","project_id":"` + projectID + `","resource_type":"business-engagement","status":"ready","revision":5,"digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","object_key":"must-not-cross-boundary","items":[{"relation_key":"award_document","file_id":"` + fileID + `","version":2,"evidence_source_revision":9,"display_name":"中标通知书.pdf"}]}`))
	}))
	defer server.Close()
	validator, err := NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{
		"business-engagement": {BaseURL: server.URL, Audience: "business-engagement", Action: "business-engagement.material-reference.read", Credentials: credentials},
	})
	if err != nil {
		t.Fatal(err)
	}
	value, err := validator.ResolveResource(context.Background(), tenantID, "business-engagement", "business-engagement", resourceID)
	if err != nil || value.MaterialSource == nil || value.MaterialSource.ProjectID != projectID || len(value.MaterialSource.Items) != 1 || value.MaterialSource.Items[0].FileID != fileID {
		t.Fatalf("reference=%#v error=%v", value, err)
	}
	if credentials.audience != "business-engagement" || credentials.action != "business-engagement.material-reference.read" || credentials.resourceType != "business-engagement" || credentials.resourceID != resourceID {
		t.Fatalf("credential request=%#v", credentials)
	}
}

func TestResolveBusinessEngagementMaterialsRejectsMalformedProjection(t *testing.T) {
	projectID := "44444444-4444-4444-8444-444444444444"
	valid := `{"schema_version":"1","source_application":"business-engagement","id":"` + resourceID + `","tenant_id":"` + tenantID + `","project_id":"` + projectID + `","resource_type":"business-engagement","status":"ready","revision":5,"digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","items":[]}`
	tests := []struct{ name, body string }{
		{"bad digest", strings.Replace(valid, strings.Repeat("b", 64), "too-short", 1)},
		{"wrong tenant", strings.Replace(valid, tenantID, "99999999-9999-4999-8999-999999999999", 1)},
		{"wrong source", strings.Replace(valid, "business-engagement\"", "other-source\"", 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(test.body)) }))
			defer server.Close()
			validator, _ := NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{"business-engagement": {BaseURL: server.URL, AccessToken: "token"}})
			if _, err := validator.ResolveResource(context.Background(), tenantID, "business-engagement", "business-engagement", resourceID); !errors.Is(err, projects.ErrReferenceInvalid) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestMissingEndpointFailsClosed(t *testing.T) {
	validator, _ := NewHTTPValidator(time.Second, Endpoint{}, nil)
	if _, err := validator.ValidatePeople(context.Background(), tenantID, "person", resourceID, 1); !errors.Is(err, projects.ErrReferenceUnavailable) {
		t.Fatalf("people error = %v", err)
	}
	if _, err := validator.ValidateParty(context.Background(), tenantID, resourceID, 1); !errors.Is(err, projects.ErrReferenceUnavailable) {
		t.Fatalf("party error = %v", err)
	}
	if err := validator.ValidateResource(context.Background(), tenantID, "contract-ledger", "contract", resourceID, nil); !errors.Is(err, projects.ErrReferenceUnavailable) {
		t.Fatalf("resource error = %v", err)
	}
}

func TestResolveCompletionResultsUsesExactProjectCredentialAndRejectsMalformedProjection(t *testing.T) {
	credentials := &credentialSpy{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/references/projects/"+resourceID+"/approval-audit-results" || r.Header.Get("Authorization") != "Bearer dynamic-token" {
			t.Fatalf("path=%q authorization=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"schema_version":"1","source_application":"approval-audit-results","project_id":"` + resourceID + `","items":[{"id":"33333333-3333-4333-8333-333333333333","tenant_id":"` + tenantID + `","project_id":"` + resourceID + `","resource_type":"approval_audit_result","kind":"approval","state":"recorded","revision":2,"digest":"approval-r2","display_summary":"批复 r2"}]}`))
	}))
	defer server.Close()
	validator, err := NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{
		"approval-audit-results": {BaseURL: server.URL, Audience: "approval-audit-results", Action: "approval-audit-result.read", Credentials: credentials},
	})
	if err != nil {
		t.Fatal(err)
	}
	values, err := validator.ResolveCompletionResults(context.Background(), tenantID, resourceID)
	if err != nil || len(values) != 1 || values[0].Kind != "approval" || values[0].Revision != 2 {
		t.Fatalf("values=%#v error=%v", values, err)
	}
	if credentials.audience != "approval-audit-results" || credentials.action != "approval-audit-result.read" || credentials.resourceType != "project" || credentials.resourceID != resourceID {
		t.Fatalf("credential request=%#v", credentials)
	}

	invalid := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"schema_version":"1","source_application":"approval-audit-results","project_id":"` + resourceID + `","items":[{"id":"33333333-3333-4333-8333-333333333333","tenant_id":"99999999-9999-4999-8999-999999999999","project_id":"` + resourceID + `","resource_type":"approval_audit_result","kind":"approval","state":"recorded","revision":2,"digest":"approval-r2","display_summary":"批复 r2"}]}`))
	}))
	defer invalid.Close()
	validator, _ = NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{"approval-audit-results": {BaseURL: invalid.URL, AccessToken: "token"}})
	if _, err := validator.ResolveCompletionResults(context.Background(), tenantID, resourceID); !errors.Is(err, projects.ErrReferenceInvalid) {
		t.Fatalf("malformed projection error=%v", err)
	}
}
