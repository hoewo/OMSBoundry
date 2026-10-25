package personreference

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"omsbizfoundry/payroll/internal/payroll"
)

func TestValidatorPinsTenantPersonAndRevision(t *testing.T) {
	const tenant = "11111111-1111-4111-8111-111111111111"
	const person = "22222222-2222-4222-8222-222222222222"
	const token = "people-reference-token-123456"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "", 401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"schema_version":"1","source_application":"people-groups","id":"` + person + `","tenant_id":"` + tenant + `","resource_type":"person","display_name":"成员甲","status":"active","revision":7}`))
	}))
	defer server.Close()
	validator, err := NewHTTPValidator(server.URL, token, nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	value, err := validator.ValidatePerson(context.Background(), tenant, person)
	if err != nil || value.Revision != 7 || value.DisplayName != "成员甲" {
		t.Fatalf("value=%#v err=%v", value, err)
	}
	if _, err = validator.ValidatePerson(context.Background(), "33333333-3333-4333-8333-333333333333", person); !errors.Is(err, payroll.ErrReferenceInvalid) {
		t.Fatalf("wrong tenant error=%v", err)
	}
}
