package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"omsbizfoundry/people-groups/internal/auth"
	"omsbizfoundry/people-groups/internal/peoplegroups"
)

type stubUseCases struct {
	UseCases
	person    peoplegroups.Person
	personErr error
}

func (stubUseCases) Ping(context.Context) error { return nil }
func (s stubUseCases) GetPerson(context.Context, string, string) (peoplegroups.Person, error) {
	return s.person, s.personErr
}

type stubAuthenticator struct {
	principal auth.Principal
	err       error
}

func (s stubAuthenticator) Authenticate(*http.Request) (auth.Principal, error) {
	return s.principal, s.err
}

func (stubAuthenticator) ProviderName() string { return "test" }

func TestAPIRequiresCredential(t *testing.T) {
	server, err := New(stubUseCases{}, stubAuthenticator{err: auth.ErrUnauthenticated}, Options{})
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/persons", nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "credential_required") {
		t.Fatalf("body does not identify the credential failure: %s", recorder.Body.String())
	}
}

func TestWriteRequiresOperationCapability(t *testing.T) {
	principal := auth.Principal{
		SubjectID:      "subject-1",
		TenantID:       "tenant-1",
		AllowedActions: map[string]bool{auth.ActionView: true},
	}
	server, err := New(stubUseCases{}, stubAuthenticator{principal: principal}, Options{})
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/persons", strings.NewReader(`{}`)))

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), auth.ActionPersonManage) {
		t.Fatalf("body does not identify the missing capability: %s", recorder.Body.String())
	}
}

func TestResourceCredentialCannotListOrReadAnotherPerson(t *testing.T) {
	principal := auth.Principal{
		SubjectID: "subject-1", TenantID: "tenant-1", ScopeKind: "resources",
		AllowedActions: map[string]bool{auth.ActionView: true},
		ResourceIDs:    map[string]bool{"person\x00person-a": true},
	}
	server, err := New(stubUseCases{}, stubAuthenticator{principal: principal}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/persons", "/api/v1/persons/person-b", "/api/v1/catalog"} {
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "scope_denied") {
			t.Fatalf("%s status=%d body=%s", path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestPersonReferenceRequiresExactCapabilityAndScope(t *testing.T) {
	const personID = "77777777-7777-4777-8777-777777777777"
	const otherID = "88888888-8888-4888-8888-888888888888"
	person := peoplegroups.Person{ID: personID, TenantID: "tenant-1", DisplayName: "张工程师", Status: "active", Revision: 3, IAMSubjectID: "must-not-leak"}

	for _, test := range []struct {
		name       string
		principal  auth.Principal
		wantStatus int
		wantBody   string
	}{
		{
			name: "exact reference credential", wantStatus: http.StatusOK, wantBody: `"display_name":"张工程师"`,
			principal: auth.Principal{TenantID: "tenant-1", ScopeKind: "resources", AllowedActions: map[string]bool{auth.ActionPersonReferenceRead: true}, ResourceIDs: map[string]bool{"person\x00" + personID: true}},
		},
		{
			name: "adjacent view action", wantStatus: http.StatusForbidden, wantBody: "capability_denied",
			principal: auth.Principal{TenantID: "tenant-1", ScopeKind: "resources", AllowedActions: map[string]bool{auth.ActionView: true}, ResourceIDs: map[string]bool{"person\x00" + personID: true}},
		},
		{
			name: "adjacent person scope", wantStatus: http.StatusForbidden, wantBody: "scope_denied",
			principal: auth.Principal{TenantID: "tenant-1", ScopeKind: "resources", AllowedActions: map[string]bool{auth.ActionPersonReferenceRead: true}, ResourceIDs: map[string]bool{"person\x00" + otherID: true}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			server, err := New(stubUseCases{person: person}, stubAuthenticator{principal: test.principal}, Options{})
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/references/persons/"+personID, nil))
			if response.Code != test.wantStatus || !strings.Contains(response.Body.String(), test.wantBody) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "must-not-leak") || strings.Contains(response.Body.String(), "iam_subject_id") {
				t.Fatalf("reference leaked IAM linkage: %s", response.Body.String())
			}
		})
	}
}

func TestPersonReferenceReturnsTypedNotFoundWithoutCurrentData(t *testing.T) {
	const personID = "77777777-7777-4777-8777-777777777777"
	principal := auth.Principal{TenantID: "tenant-1", ScopeKind: "resources", AllowedActions: map[string]bool{auth.ActionPersonReferenceRead: true}, ResourceIDs: map[string]bool{"person\x00" + personID: true}}
	server, err := New(stubUseCases{personErr: peoplegroups.ErrNotFound}, stubAuthenticator{principal: principal}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/references/persons/"+personID, nil))
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), "person_not_found") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestHealthReadinessDoesNotRequireBusinessCredential(t *testing.T) {
	server, err := New(stubUseCases{}, stubAuthenticator{err: errors.New("must not be called")}, Options{})
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func TestTemplateLoginParsesFormBeforeCSRFValidation(t *testing.T) {
	const accessToken = "people-groups-template-token-for-test"
	provider, err := auth.NewTemplateProvider(
		"55555555-5555-4555-8555-555555555555",
		"66666666-6666-4666-8666-666666666666",
		"Demo Admin",
		accessToken,
	)
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(stubUseCases{}, provider, Options{TemplateProvider: provider})
	if err != nil {
		t.Fatal(err)
	}

	page := httptest.NewRecorder()
	server.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/login", nil))
	if page.Code != http.StatusOK || len(page.Result().Cookies()) != 1 {
		t.Fatalf("login page status=%d cookies=%d", page.Code, len(page.Result().Cookies()))
	}
	csrfCookie := page.Result().Cookies()[0]
	csrfToken := csrfCookie.Value

	form := url.Values{"csrf_token": {csrfToken}, "access_token": {accessToken}}
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(csrfCookie)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/" {
		t.Fatalf("status=%d location=%q body=%s", recorder.Code, recorder.Header().Get("Location"), recorder.Body.String())
	}
	foundSession := false
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == auth.TemplateCookieName && cookie.Value == accessToken {
			foundSession = true
		}
	}
	if !foundSession {
		t.Fatal("successful template login did not set the session cookie")
	}
}
