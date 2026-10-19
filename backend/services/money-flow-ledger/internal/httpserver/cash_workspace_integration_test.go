//go:build integration

package httpserver_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/allocation"
	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/httpserver"
	"omsbizfoundry/money-flow-ledger/internal/identity"
	"omsbizfoundry/money-flow-ledger/internal/internaltransfer"
	"omsbizfoundry/money-flow-ledger/internal/ledgerexport"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
	"omsbizfoundry/money-flow-ledger/internal/operatingsummary"
	"omsbizfoundry/money-flow-ledger/internal/party"
	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

// Real PG18 and native IAM verifier, not a real IAM issuer or public OA journey.
func TestCashWorkspacePostgresExactMoneyScopeHistoryAndReplay(t *testing.T) {
	env := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if e := database.Migrate(ctx, env.Pool); e != nil {
		t.Fatal(e)
	}
	pub, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	kid := "33333333-3333-4333-8333-333333333333"
	tenant := "22222222-2222-4222-8222-222222222222"
	subject := "11111111-1111-4111-8111-111111111111"
	data, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": kid, "x": base64.RawURLEncoding.EncodeToString(pub), "status": "active"}}})
	path := filepath.Join(t.TempDir(), "keys.json")
	if e = os.WriteFile(path, data, 0600); e != nil {
		t.Fatal(e)
	}
	provider, e := identity.NewIAMProvider("https://cash.example.test", tenant, path)
	if e != nil {
		t.Fatal(e)
	}
	credential := func(id string, actions []string) string {
		now := time.Now()
		scope := map[string]any{"kind": "tenant", "resources": []any{}}
		if id != "" {
			scope = map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "cash_transaction", "id": id}}}
		}
		claims := map[string]any{"ver": "1", "iss": "https://cash.example.test", "sub": subject, "tid": tenant, "aud": "money-flow-ledger", "name": "trusted cash verifier fixture", "actions": actions, "scope": scope, "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "auth_time": now.Unix(), "jti": "44444444-4444-4444-8444-444444444444"}
		h, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": kid})
		b, _ := json.Marshal(claims)
		unsigned := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(b)
		return unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(unsigned)))
	}
	partyService := party.NewService(party.NewPostgresRepository(env.Pool))
	actor := audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: subject, DisplayName: "trusted cash verifier fixture"}
	command := func() string {
		v, e := cashtransaction.NewCommandID()
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	createParty := func(name string) string {
		r, e := partyService.Create(ctx, party.CreateInput{CommandID: command(), Kind: "organization", LegalName: name, Actor: actor})
		if e != nil {
			t.Fatal(e)
		}
		return r.Party.ID
	}
	owner, payer := createParty("Money current receiver"), createParty("Money current payer")
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(env.Pool))
	export, e := ledgerexport.NewService(ledgerexport.NewPostgresRepository(env.Pool))
	if e != nil {
		t.Fatal(e)
	}
	handler, e := httpserver.New(env.Pool, provider, partyService, obligation.NewService(obligation.NewPostgresRepository(env.Pool)), cashService, allocation.NewService(allocation.NewPostgresRepository(env.Pool)), operatingsummary.NewService(operatingsummary.NewPostgresRepository(env.Pool)), export, httpserver.Options{ExternalIdentity: true}, internaltransfer.NewService(internaltransfer.NewPostgresRepository(env.Pool)))
	if e != nil {
		t.Fatal(e)
	}
	all := []string{"money-flow-ledger.view", "money-flow-ledger.amount.read", "money-flow-ledger.cash.record", "money-flow-ledger.cash.reverse"}
	token := credential("", all)
	call := func(method, path string, body any, want int) map[string]any {
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("X-Actor-ID", "forged")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s status=%d body=%s", method, path, w.Code, w.Body.String())
		}
		var v map[string]any
		if e := json.Unmarshal(w.Body.Bytes(), &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	contextView := call("GET", "/api/v1/cash-workspace", nil, 200)
	if contextView["directory_kind"] != "money_local" || len(contextView["parties"].([]any)) != 2 {
		t.Fatal(contextView)
	}
	input := map[string]any{"command_id": command(), "type": "receipt", "ledger_owner_party_id": owner, "counterparty_party_id": payer, "description": "actual receipt fixture", "amount": "9007199254740993.01", "currency": "CNY", "occurred_on": "2026-10-04", "account_reference": "bank-01", "manual_source_reference": "source-01"}
	created := call("POST", "/api/v1/cash-transactions", input, 200)
	transaction := created["transaction"].(map[string]any)
	id := transaction["id"].(string)
	if transaction["amount"] != "9007199254740993.01" || transaction["payee"].(map[string]any)["id"] != owner || transaction["payer"].(map[string]any)["id"] != payer {
		t.Fatal(transaction)
	}
	replay := call("POST", "/api/v1/cash-transactions", input, 200)
	if replay["replayed"] != true || replay["transaction"].(map[string]any)["id"] != id {
		t.Fatal(replay)
	}
	input["amount"] = "0.01"
	call("POST", "/api/v1/cash-transactions", input, 409)
	input["command_id"] = command()
	input["occurred_on"] = "2026-02-30"
	call("POST", "/api/v1/cash-transactions", input, 422)
	input["occurred_on"] = "2026-10-04"
	input["actor_id"] = "forged"
	call("POST", "/api/v1/cash-transactions", input, 400)
	delete(input, "actor_id")
	input["amount"] = 0.01
	call("POST", "/api/v1/cash-transactions", input, 400)
	exact := []string{"money-flow-ledger.view", "money-flow-ledger.amount.read", "money-flow-ledger.cash.reverse"}
	token = credential(id, exact)
	call("GET", "/api/v1/cash-transactions/55555555-5555-4555-8555-555555555555", nil, 403)
	narrow := call("GET", "/api/v1/cash-workspace", nil, 200)
	if len(narrow["parties"].([]any)) != 0 || len(narrow["allowed_operations"].([]any)) != 0 {
		t.Fatal("exact scope leaked directory", narrow)
	}
	call("POST", "/api/v1/cash-transactions", input, 403)
	token = credential(id, []string{"money-flow-ledger.view", "money-flow-ledger.cash.reverse"})
	hidden := call("GET", "/api/v1/cash-transactions/"+id, nil, 200)["transaction"].(map[string]any)
	if hidden["amount"] != nil || hidden["currency"] != nil || hidden["amount_visible"] != false || len(hidden["allowed_operations"].([]any)) != 0 {
		t.Fatal("blind cash leak", hidden)
	}
	reversal := map[string]any{"command_id": command(), "reason": "wrong actual receipt"}
	call("POST", "/api/v1/cash-transactions/"+id+"/reversal", reversal, 403)
	token = credential(id, exact)
	first := call("POST", "/api/v1/cash-transactions/"+id+"/reversal", reversal, 200)
	repeat := call("POST", "/api/v1/cash-transactions/"+id+"/reversal", reversal, 200)
	if first["receipt"].(map[string]any)["reversal_id"] != repeat["receipt"].(map[string]any)["reversal_id"] || repeat["receipt"].(map[string]any)["replayed"] != true {
		t.Fatal("reversal replay duplicated history")
	}
	frozen := call("GET", "/api/v1/cash-transactions/"+id, nil, 200)["transaction"].(map[string]any)
	if frozen["status"] != "reversed" || frozen["amount"] != "9007199254740993.01" || frozen["effective_amount"] != "0" {
		t.Fatal(frozen)
	}
	reversal["command_id"] = command()
	call("POST", "/api/v1/cash-transactions/"+id+"/reversal", reversal, 409)
	var rows, reversals, commands int
	var actorKind, actorID string
	if e = env.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM cash_transactions),(SELECT count(*) FROM cash_transaction_reversals),(SELECT count(*) FROM cash_transaction_create_commands)`).Scan(&rows, &reversals, &commands); e != nil || rows != 1 || reversals != 1 || commands != 1 {
		t.Fatal("append facts/replay mismatch", rows, reversals, commands, e)
	}
	if e = env.Pool.QueryRow(ctx, `SELECT recorded_by_actor_kind,recorded_by_actor_id FROM cash_transactions WHERE id=$1`, id).Scan(&actorKind, &actorID); e != nil || actorKind != audit.ActorKindIAMSubject || actorID != subject {
		t.Fatal("untrusted actor", actorKind, actorID, e)
	}
}
