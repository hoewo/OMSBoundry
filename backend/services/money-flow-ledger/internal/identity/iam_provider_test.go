package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

func TestIAMProviderVerifiesCredentialMapsCapabilitiesAndActor(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	keyID := "33333333-3333-4333-8333-333333333333"
	path := filepath.Join(t.TempDir(), "keys.json")
	keyData, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": keyID, "x": base64.RawURLEncoding.EncodeToString(publicKey), "status": "active"}}})
	if err := os.WriteFile(path, keyData, 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := NewIAMProvider("https://oms.example.test/auth", "22222222-2222-4222-8222-222222222222", path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	provider.now = func() time.Time { return now }
	claims := map[string]any{"ver": "1", "iss": "https://oms.example.test/auth", "sub": "11111111-1111-4111-8111-111111111111", "tid": "22222222-2222-4222-8222-222222222222", "aud": "money-flow-ledger", "name": "中心资金管理员", "actions": []string{"money-flow-ledger.view", "money-flow-ledger.party.register", "money-flow-ledger.statistics.read", "money-flow-ledger.amount.read", "money-flow-ledger.payable-suggestion.submit", "money-flow-ledger.payable-suggestion.read", "money-flow-ledger.payable-suggestion.withdraw", "money-flow-ledger.payable-suggestion.decide"}, "scope": map[string]any{"kind": "tenant", "resources": []any{}}, "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "auth_time": now.Add(-time.Minute).Unix(), "jti": "44444444-4444-4444-8444-444444444444"}
	principal, found, err := provider.ResolveSession(context.Background(), signIAMToken(t, keyID, privateKey, claims))
	if err != nil || !found {
		t.Fatalf("resolve=%v,%v", found, err)
	}
	if !principal.Can(PermissionView) || !principal.Can(PermissionPartyRegister) || !principal.Can(PermissionStatisticsRead) || !principal.Can(PermissionAmountRead) || !principal.Can(PermissionPayableSuggestionSubmit) || !principal.Can(PermissionPayableSuggestionRead) || !principal.Can(PermissionPayableSuggestionWithdraw) || !principal.Can(PermissionPayableSuggestionDecide) || principal.Can(PermissionCashRecord) {
		t.Fatalf("unexpected IAM permissions: %#v", principal.AllowedActions)
	}
	actor := principal.Actor()
	if actor.Kind != audit.ActorKindIAMSubject || actor.ID != principal.UserID || actor.DisplayName != "中心资金管理员" {
		t.Fatalf("actor=%#v", actor)
	}
	claims["aud"] = "contract-ledger"
	if _, found, err := provider.ResolveSession(context.Background(), signIAMToken(t, keyID, privateKey, claims)); err != nil || found {
		t.Fatal("wrong-audience credential accepted")
	}
}

func TestIAMProviderAcceptsOnlyExactMoneyReferenceCredential(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	keyID := "33333333-3333-4333-8333-333333333333"
	path := filepath.Join(t.TempDir(), "keys.json")
	keyData, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": keyID, "x": base64.RawURLEncoding.EncodeToString(publicKey), "status": "active"}}})
	if err := os.WriteFile(path, keyData, 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := NewIAMProvider("https://oms.example.test/auth", "22222222-2222-4222-8222-222222222222", path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	provider.now = func() time.Time { return now }
	claims := map[string]any{"ver": "1", "iss": "https://oms.example.test/auth", "sub": "11111111-1111-4111-8111-111111111111", "tid": "22222222-2222-4222-8222-222222222222", "aud": "money-flow-ledger", "name": "Billing workload", "actions": []string{"money-flow-ledger.reference.read"}, "scope": map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "obligation", "id": "55555555-5555-4555-8555-555555555555"}}}, "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "auth_time": now.Add(-time.Minute).Unix(), "jti": "44444444-4444-4444-8444-444444444444"}
	principal, found, err := provider.ResolveSession(context.Background(), signIAMToken(t, keyID, privateKey, claims))
	if err != nil || !found || !principal.Can(PermissionReferenceRead) || !principal.CanReference("obligation", "55555555-5555-4555-8555-555555555555") || principal.CanReference("obligation", "66666666-6666-4666-8666-666666666666") {
		t.Fatalf("principal=%#v found=%v err=%v", principal, found, err)
	}
	otherTenantClaims := copyClaims(claims)
	otherTenantClaims["tid"] = "66666666-6666-4666-8666-666666666666"
	otherTenantPrincipal, found, err := provider.ResolveSession(context.Background(), signIAMToken(t, keyID, privateKey, otherTenantClaims))
	if err != nil || !found || otherTenantPrincipal.TenantID != otherTenantClaims["tid"] {
		t.Fatalf("other tenant principal=%#v found=%v err=%v", otherTenantPrincipal, found, err)
	}
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"malformed tenant", func(value map[string]any) { value["tid"] = "not-a-tenant" }},
		{"wrong action", func(value map[string]any) { value["actions"] = []string{"money-flow-ledger.view"} }},
		{"wrong type", func(value map[string]any) {
			value["scope"] = map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "contract", "id": "55555555-5555-4555-8555-555555555555"}}}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			copyClaims := make(map[string]any, len(claims))
			for key, value := range claims {
				copyClaims[key] = value
			}
			test.mutate(copyClaims)
			if _, found, err := provider.ResolveSession(context.Background(), signIAMToken(t, keyID, privateKey, copyClaims)); err != nil || found {
				t.Fatalf("found=%v err=%v", found, err)
			}
		})
	}
}

func TestIAMProviderSeparatesReceiptSetConfirmAndExactReference(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	keyID := "33333333-3333-4333-8333-333333333333"
	path := filepath.Join(t.TempDir(), "keys.json")
	keyData, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": keyID, "x": base64.RawURLEncoding.EncodeToString(publicKey), "status": "active"}}})
	if err := os.WriteFile(path, keyData, 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := NewIAMProvider("https://oms.example.test/auth", "22222222-2222-4222-8222-222222222222", path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	provider.now = func() time.Time { return now }
	base := map[string]any{"ver": "1", "iss": "https://oms.example.test/auth", "sub": "11111111-1111-4111-8111-111111111111", "tid": "22222222-2222-4222-8222-222222222222", "aud": "money-flow-ledger", "name": "资金责任人", "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "auth_time": now.Unix(), "jti": "44444444-4444-4444-8444-444444444444"}
	confirm := copyClaims(base)
	confirm["actions"] = []string{"money-flow-ledger.receipt-set.confirm"}
	confirm["scope"] = map[string]any{"kind": "tenant", "resources": []any{}}
	principal, found, err := provider.ResolveSession(context.Background(), signIAMToken(t, keyID, privateKey, confirm))
	if err != nil || !found || !principal.Can(PermissionReceiptSetConfirm) || principal.Can(PermissionReceiptSetReferenceRead) {
		t.Fatalf("confirm principal=%#v found=%v err=%v", principal, found, err)
	}
	reference := copyClaims(base)
	reference["actions"] = []string{"money-flow-ledger.receipt-set.reference.read"}
	reference["scope"] = map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "receipt_set", "id": "55555555-5555-4555-8555-555555555555"}}}
	principal, found, err = provider.ResolveSession(context.Background(), signIAMToken(t, keyID, privateKey, reference))
	if err != nil || !found || !principal.Can(PermissionReceiptSetReferenceRead) || !principal.CanReference("receipt_set", "55555555-5555-4555-8555-555555555555") || principal.Can(PermissionReferenceRead) {
		t.Fatalf("reference principal=%#v found=%v err=%v", principal, found, err)
	}
	wrong := copyClaims(reference)
	wrong["actions"] = []string{"money-flow-ledger.reference.read"}
	if _, found, err := provider.ResolveSession(context.Background(), signIAMToken(t, keyID, privateKey, wrong)); err != nil || found {
		t.Fatalf("generic action accepted for receipt set found=%v err=%v", found, err)
	}
}

func TestIAMProviderAcceptsOnlyExactPayableSuggestionWorkloadCapabilities(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	keyID := "33333333-3333-4333-8333-333333333333"
	path := filepath.Join(t.TempDir(), "keys.json")
	keyData, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": keyID, "x": base64.RawURLEncoding.EncodeToString(publicKey), "status": "active"}}})
	if err := os.WriteFile(path, keyData, 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := NewIAMProvider("https://oms.example.test/auth", "22222222-2222-4222-8222-222222222222", path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	provider.now = func() time.Time { return now }
	base := map[string]any{
		"ver": "1", "iss": "https://oms.example.test/auth", "sub": "11111111-1111-4111-8111-111111111111",
		"tid": "22222222-2222-4222-8222-222222222222", "aud": "money-flow-ledger", "name": "Cost workload",
		"scope": map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "payable_suggestion", "id": "55555555-5555-4555-8555-555555555555"}}},
		"iat":   now.Unix(), "nbf": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "auth_time": now.Unix(), "jti": "44444444-4444-4444-8444-444444444444",
	}
	for _, test := range []struct {
		action     string
		permission Permission
	}{
		{"money-flow-ledger.payable-suggestion.submit", PermissionPayableSuggestionSubmit},
		{"money-flow-ledger.payable-suggestion.read", PermissionPayableSuggestionRead},
		{"money-flow-ledger.payable-suggestion.withdraw", PermissionPayableSuggestionWithdraw},
	} {
		t.Run(test.action, func(t *testing.T) {
			claims := copyClaims(base)
			claims["actions"] = []string{test.action}
			principal, found, err := provider.ResolveSession(context.Background(), signIAMToken(t, keyID, privateKey, claims))
			if err != nil || !found || !principal.Can(test.permission) || !principal.CanReference("payable_suggestion", "55555555-5555-4555-8555-555555555555") || principal.CanReference("payable_suggestion", "66666666-6666-4666-8666-666666666666") {
				t.Fatalf("principal=%#v found=%v err=%v", principal, found, err)
			}
		})
	}

	wrongType := copyClaims(base)
	wrongType["actions"] = []string{"money-flow-ledger.payable-suggestion.submit"}
	wrongType["scope"] = map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "obligation", "id": "55555555-5555-4555-8555-555555555555"}}}
	if _, found, err := provider.ResolveSession(context.Background(), signIAMToken(t, keyID, privateKey, wrongType)); err != nil || found {
		t.Fatalf("payable suggestion action accepted for obligation scope found=%v err=%v", found, err)
	}
}

func TestIAMProviderAcceptsOnlyExactInternalFinancePaymentCredential(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	keyID := "33333333-3333-4333-8333-333333333333"
	path := filepath.Join(t.TempDir(), "keys.json")
	keyData, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": keyID, "x": base64.RawURLEncoding.EncodeToString(publicKey), "status": "active"}}})
	if err := os.WriteFile(path, keyData, 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := NewIAMProvider("https://oms.example.test/auth", "22222222-2222-4222-8222-222222222222", path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	provider.now = func() time.Time { return now }
	commandID := "55555555-5555-4555-8555-555555555555"
	claims := map[string]any{
		"ver": "1", "iss": "https://oms.example.test/auth", "sub": "11111111-1111-4111-8111-111111111111",
		"tid": "22222222-2222-4222-8222-222222222222", "aud": "money-flow-ledger", "name": "Internal Finance workload",
		"actions": []string{"money-flow-ledger.internal-finance-payment.record"},
		"scope":   map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "internal_finance_payment", "id": commandID}}},
		"iat":     now.Unix(), "nbf": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "auth_time": now.Unix(), "jti": "44444444-4444-4444-8444-444444444444",
	}
	principal, found, err := provider.ResolveSession(context.Background(), signIAMToken(t, keyID, privateKey, claims))
	if err != nil || !found || !principal.Can(PermissionInternalFinancePaymentRecord) || !principal.CanReference("internal_finance_payment", commandID) || principal.CanReference("internal_finance_payment", "66666666-6666-4666-8666-666666666666") {
		t.Fatalf("principal=%#v found=%v err=%v", principal, found, err)
	}
	wrongType := copyClaims(claims)
	wrongType["scope"] = map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "payroll_payment", "id": commandID}}}
	if _, found, err := provider.ResolveSession(context.Background(), signIAMToken(t, keyID, privateKey, wrongType)); err != nil || found {
		t.Fatalf("internal finance action accepted for payroll scope found=%v err=%v", found, err)
	}
}

func copyClaims(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func signIAMToken(t *testing.T, keyID string, key ed25519.PrivateKey, claims any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": keyID})
	body, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(unsigned)))
}
