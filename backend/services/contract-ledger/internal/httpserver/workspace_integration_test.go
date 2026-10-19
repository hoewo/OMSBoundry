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
	"strings"
	"testing"
	"time"

	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/contract"
	"omsbizfoundry/contract-ledger/internal/httpserver"
	"omsbizfoundry/contract-ledger/internal/identity"
	"omsbizfoundry/contract-ledger/internal/ledgerexport"
	"omsbizfoundry/contract-ledger/internal/ledgerrestore"
	"omsbizfoundry/contract-ledger/internal/party"
	"omsbizfoundry/contract-ledger/internal/referenceoa"
)

type materialValidatorFixture struct {
	calls       int
	unavailable bool
	tenant      string
}

func (v *materialValidatorFixture) ValidateFile(_ context.Context, tenant, id string, version int) (referenceoa.FileReference, error) {
	v.calls++
	if v.unavailable {
		return referenceoa.FileReference{}, referenceoa.ErrEvidenceUnavailable
	}
	return referenceoa.FileReference{Schema: "1", Source: "evidence-service", ResourceType: "evidence-file", ID: id, FileID: id, TenantID: v.tenant, Version: version, Revision: version, Status: "available", Name: "合同验证附件.pdf", Size: 5, MediaType: "application/pdf", SHA256: strings.Repeat("a", 64)}, nil
}

// This uses a real IAM verifier and a private empty PostgreSQL DB; it does not
// claim a production IAM exchange, original component, or public UI journey.
func TestWorkspaceNativePostgresExactMoneyHistoryAndReplay(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()
	pool := openIsolatedSystemDatabase(t, ctx, url)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	kid := "33333333-3333-4333-8333-333333333333"
	tenant := "22222222-2222-4222-8222-222222222222"
	subject := "11111111-1111-4111-8111-111111111111"
	keys, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": kid, "x": base64.RawURLEncoding.EncodeToString(public), "status": "active"}}})
	keyPath := filepath.Join(t.TempDir(), "public-keys.json")
	if err = os.WriteFile(keyPath, keys, 0600); err != nil {
		t.Fatal(err)
	}
	provider, err := identity.NewIAMProvider("https://test.example/auth", tenant, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	credential := func(resource string, actions []string) string {
		now := time.Now().UTC()
		scope := map[string]any{"kind": "tenant", "resources": []any{}}
		if resource != "" {
			scope = map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "contract", "id": resource}}}
		}
		claims := map[string]any{"ver": "1", "iss": "https://test.example/auth", "sub": subject, "tid": tenant, "aud": "contract-ledger", "name": "可信测试操作者", "actions": actions, "scope": scope, "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(4 * time.Minute).Unix(), "auth_time": now.Add(-time.Minute).Unix(), "jti": "44444444-4444-4444-8444-444444444444"}
		h, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": kid})
		v, _ := json.Marshal(claims)
		unsigned := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(v)
		return unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, []byte(unsigned)))
	}
	parties := party.NewService(party.NewPostgresRepository(pool))
	actor := audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: subject, DisplayName: "可信测试操作者"}
	client, err := parties.Create(ctx, party.CreateInput{Kind: "organization", LegalName: "原生合同甲方", Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	providerParty, err := parties.Create(ctx, party.CreateInput{Kind: "organization", LegalName: "原生合同乙方", Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	exports, err := ledgerexport.NewService(ledgerexport.NewPostgresRepository(pool))
	if err != nil {
		t.Fatal(err)
	}
	restores, err := ledgerrestore.NewService(ledgerrestore.NewPostgresRepository(pool))
	if err != nil {
		t.Fatal(err)
	}
	refOA := referenceoa.NewStore(pool)
	fileValidator := &materialValidatorFixture{tenant: tenant}
	refOA.SetEvidenceValidator(fileValidator)
	handler, err := httpserver.NewServer(pool, parties, contract.NewService(contract.NewPostgresRepository(pool)), newSystemEvidenceService(t, pool), exports, restores, provider, audit.NewRepository(pool), httpserver.Options{ExternalIdentity: true, ReferenceOADrafts: refOA})
	if err != nil {
		t.Fatal(err)
	}
	command := func() string {
		v, e := contract.NewCommandID()
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	all := []string{"contract-ledger.view", "contract-ledger.draft.manage", "contract-ledger.lifecycle.manage", "contract-ledger.evidence.manage"}
	token := credential("", all)
	call := func(method, path string, body any, want int) map[string]any {
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s status=%d body=%s", method, path, w.Code, w.Body.String())
		}
		var value map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	input := map[string]any{"command_id": command(), "number_namespace": "native-test", "contract_number": "NATIVE-001", "title": "合同写端口真库测试", "currency": "CNY", "total_amount": "9007199254740993.01", "signed_on": "2026-10-03", "effective_on": "2026-10-03"}
	candidates := call("GET", "/api/v1/contracts/party-candidates", nil, 200)
	if candidates["directory_kind"] != "contract_local" || len(candidates["parties"].([]any)) != 2 {
		t.Fatal("local Party candidate list lost")
	}
	created := call("POST", "/api/v1/contracts", input, 200)
	id := created["receipt"].(map[string]any)["contract_id"].(string)
	base := "/api/v1/contracts/" + id
	replayed := call("POST", "/api/v1/contracts", input, 200)
	if replayed["receipt"].(map[string]any)["contract_id"] != id {
		t.Fatal("create replay duplicated fact")
	}
	token = credential(id, all)
	call("GET", "/api/v1/contracts/party-candidates", nil, 403)
	metadata := call("GET", "/api/v1/contracts/workspace-context", nil, 200)
	if len(metadata["allowed_operations"].([]any)) != 0 {
		t.Fatal("exact contract metadata expanded tenant create permission")
	}
	call("POST", "/api/v1/contracts", input, 403)
	overlay := map[string]any{"command_id": command(), "expected_revision": 0, "expected_draft_version": 1, "project_id": "66666666-6666-4666-8666-666666666666", "chain_type": "client_to_qualification", "fees": map[string]string{"design_fee": "9007199254740993.01", "survey_fee": "0", "consultation_fee": "0"}, "rate_bps": 0, "settlement_follows_contract_amount": true, "settlement_amount": nil, "amendments": []any{}}
	call("POST", base+"/reference-oa-draft", overlay, 200)
	call("POST", base+"/reference-oa-draft", overlay, 200)
	beforeOverlay := call("GET", base+"/workspace", nil, 200)["reference_oa_draft"].(map[string]any)
	if beforeOverlay["rate_bps"] != float64(0) || beforeOverlay["revision"] != float64(1) {
		t.Fatal("overlay nullable zero or replay changed")
	}
	overlay["chain_type"] = "qualification_to_our_company"
	call("POST", base+"/reference-oa-draft", overlay, 409)
	overlay["command_id"] = command()
	overlay["expected_revision"] = 1
	call("POST", base+"/reference-oa-draft", overlay, 403)
	overlay["chain_type"] = "client_to_qualification"
	overlay["command_id"] = command()
	overlay["rate_bps"] = nil
	call("POST", base+"/reference-oa-draft", overlay, 200)
	if _, err := exports.Export(ctx, ledgerexport.Input{ContractID: id, Actor: actor}); err == nil {
		t.Fatal("lossy overlay export allowed")
	}
	var historyRows int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM reference_oa_contract_draft_commands WHERE contract_id=$1`, id).Scan(&historyRows); err != nil || historyRows != 2 {
		t.Fatal("overlay replay duplicated history", err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM reference_oa_contract_draft_commands WHERE contract_id=$1`, id); err == nil {
		t.Fatal("overlay history deletion allowed")
	}
	input["command_id"] = command()
	input["expected_draft_version"] = 1
	input["title"] = "草稿精确更正"
	call("POST", base, input, 200)
	call("POST", base, input, 200)
	for i, p := range []party.Party{client, providerParty} {
		body := map[string]any{"command_id": command(), "expected_draft_version": 2 + i, "party_id": p.ID, "role": []string{"client", "provider"}[i]}
		call("POST", base+"/parties", body, 200)
		call("POST", base+"/parties", body, 200)
	}
	activation := map[string]any{"command_id": command(), "expected_draft_version": 4}
	call("POST", base+"/activate", activation, 200)
	call("POST", base+"/activate", activation, 200)
	overlay["command_id"] = command()
	overlay["expected_revision"] = 2
	overlay["expected_draft_version"] = 4
	overlay["expected_contract_revision"] = 1
	overlay["fees"] = map[string]string{"design_fee": "1.00", "survey_fee": "0", "consultation_fee": "0"}
	call("POST", base+"/reference-oa-draft", overlay, 409)
	overlay["fees"] = map[string]string{"design_fee": "9007199254740993.01", "survey_fee": "0", "consultation_fee": "0"}
	amendmentID := command()
	amendment := map[string]any{"id": amendmentID, "amendment_number": "OA-SUP-001", "signed_on": "2026-10-03", "change_type": "adjustment", "fees": map[string]string{"design_fee": "-0.01", "survey_fee": "0", "consultation_fee": "0"}, "rate_bps": nil, "description": "减少一分钱", "cancelled": false}
	overlay["amendments"] = []any{amendment}
	overlay["command_id"] = command()
	call("POST", base+"/reference-oa-draft", overlay, 200)
	confirmation := map[string]any{"command_id": command(), "expected_overlay_revision": 3, "expected_contract_revision": 1, "effective_on": "2026-10-03", "amendment": amendment}
	confirmPath := base + "/reference-oa-amendments/" + amendmentID + "/confirm"
	amendment["description"] = "伪造快照"
	call("POST", confirmPath, confirmation, 409)
	amendment["description"] = "减少一分钱"
	confirmed := call("POST", confirmPath, confirmation, 200)["confirmation"].(map[string]any)
	if confirmed["contract_revision"] != float64(2) || confirmed["effective_amount"] != "9007199254740993" {
		t.Fatal("formal customer amendment lost exact money", confirmed)
	}
	call("POST", confirmPath, confirmation, 200)
	confirmation["effective_on"] = "2026-10-04"
	call("POST", confirmPath, confirmation, 409)
	confirmation["effective_on"] = "2026-10-03"
	overlay["command_id"] = command()
	overlay["expected_revision"] = 3
	overlay["expected_contract_revision"] = 2
	amendment["cancelled"] = true
	call("POST", base+"/reference-oa-draft", overlay, 409)
	amendment["cancelled"] = false
	var confirmationCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM reference_oa_amendment_confirmations WHERE contract_id=$1`, id).Scan(&confirmationCount); err != nil || confirmationCount != 1 {
		t.Fatal("confirmation replay duplicated mapping", err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM reference_oa_amendment_confirmations WHERE contract_id=$1`, id); err == nil {
		t.Fatal("confirmed customer mapping deletion allowed")
	}
	amendmentB := map[string]any{"id": command(), "amendment_number": "OA-SUP-002", "signed_on": "2026-10-03", "change_type": "revision", "fees": map[string]string{"design_fee": "9007199254740993.01", "survey_fee": "0.00", "consultation_fee": "0.00"}, "rate_bps": 0, "description": "修订为明确目标额", "cancelled": false}
	amendmentC := map[string]any{"id": command(), "amendment_number": "OA-SUP-003", "signed_on": "2026-10-03", "change_type": "no_amount_change", "fees": map[string]string{"design_fee": "0.00", "survey_fee": "0.00", "consultation_fee": "0.00"}, "rate_bps": nil, "description": "仅修订条款", "cancelled": false}
	overlay["command_id"] = command()
	overlay["amendments"] = []any{amendment, amendmentB, amendmentC}
	call("POST", base+"/reference-oa-draft", overlay, 200)
	confirmB := map[string]any{"command_id": command(), "expected_overlay_revision": 4, "expected_contract_revision": 2, "effective_on": "2026-10-03", "amendment": amendmentB}
	confirmC := map[string]any{"command_id": command(), "expected_overlay_revision": 4, "expected_contract_revision": 2, "effective_on": "2026-10-03", "amendment": amendmentC}
	call("POST", base+"/reference-oa-amendments/"+amendmentC["id"].(string)+"/confirm", confirmC, 409)
	call("POST", base+"/reference-oa-amendments/"+amendmentB["id"].(string)+"/confirm", confirmB, 200)
	confirmC["expected_contract_revision"] = 3
	call("POST", base+"/reference-oa-amendments/"+amendmentC["id"].(string)+"/confirm", confirmC, 200)
	call("POST", base+"/reference-oa-amendments/"+amendmentC["id"].(string)+"/confirm", confirmC, 200)
	var oldHash string
	if err = pool.QueryRow(ctx, "SELECT md5(to_jsonb(t)::text) FROM contract_revisions t WHERE contract_id=$1 AND revision=1", id).Scan(&oldHash); err != nil {
		t.Fatal(err)
	}
	change := map[string]any{"command_id": command(), "expected_revision": 4, "instrument_number": "SUP-001", "title": "补充协议", "change_summary": "减少一分钱", "signed_on": "2026-10-03", "effective_on": "2026-10-03", "effective_order": 5, "amount_effect": "adjustment", "amount_value": "-0.01"}
	call("POST", base+"/changes", change, 200)
	call("POST", base+"/changes", change, 200)
	settlement := map[string]any{"command_id": command(), "expected_contract_revision": 5, "expected_settlement_sequence": 0, "kind": "final", "statement_number": "FINAL-001", "title": "最终结算", "amount": "9007199254740993", "confirmed_on": "2026-10-03", "basis": "双方确认", "confirming_party_positions": []int64{1, 2}}
	call("POST", base+"/settlements", settlement, 200)
	call("POST", base+"/settlements", settlement, 200)
	read := call("GET", base+"/workspace", nil, 200)
	value := read["contract"].(map[string]any)
	if value["total_amount"] != "9007199254740993" || value["current_revision"] != float64(5) || len(read["instruments"].([]any)) != 5 || len(read["settlements"].([]any)) != 1 || len(read["reference_oa_confirmations"].([]any)) != 3 {
		t.Fatalf("native read=%v", read)
	}
	settlementView := read["settlements"].([]any)[0].(map[string]any)
	confirmingParties := settlementView["confirming_parties"].([]any)
	if settlementView["source_revision"] != float64(5) || len(confirmingParties) != 2 {
		t.Fatal("settlement source or frozen confirming parties omitted")
	}
	for _, item := range confirmingParties {
		party := item.(map[string]any)
		if party["source_party_id"] == "" || party["source_party_revision"] != float64(1) || party["legal_name"] == "" || party["role"] == "" {
			t.Fatal("settlement frozen Party snapshot incomplete")
		}
	}
	var newHash string
	if err = pool.QueryRow(ctx, "SELECT md5(to_jsonb(t)::text) FROM contract_revisions t WHERE contract_id=$1 AND revision=1", id).Scan(&newHash); err != nil || newHash != oldHash {
		t.Fatal("old confirmed revision overwritten")
	}
	var foreignActors int
	materialRevision := int64(0)
	var firstMaterial map[string]any
	for _, kind := range []string{"contract", "amendment", "finalization"} {
		body := map[string]any{"command_id": command(), "expected_revision": materialRevision, "operation": "append", "kind": kind, "file_id": "77777777-7777-4777-8777-777777777777", "version": 2}
		if kind == "amendment" {
			body["amendment_id"] = amendmentID
		}
		result := call("POST", base+"/reference-oa-materials", body, 200)
		materialRevision++
		if result["result"].(map[string]any)["materials"].(map[string]any)["revision"] != float64(materialRevision) {
			t.Fatal("material revision lost")
		}
		if firstMaterial == nil {
			firstMaterial = body
		}
	}
	fileValidator.unavailable = true
	beforeFileCalls := fileValidator.calls
	call("POST", base+"/reference-oa-materials", firstMaterial, 200)
	if fileValidator.calls != beforeFileCalls {
		t.Fatal("material replay depended on external source")
	}
	failedMaterial := map[string]any{"command_id": command(), "expected_revision": 3, "operation": "append", "kind": "contract", "file_id": command(), "version": 1}
	call("POST", base+"/reference-oa-materials", failedMaterial, 503)
	fileValidator.unavailable = false
	fileValidator.tenant = subject
	call("POST", base+"/reference-oa-materials", failedMaterial, 422)
	fileValidator.tenant = tenant
	failedMaterial["expected_revision"] = 0
	call("POST", base+"/reference-oa-materials", failedMaterial, 409)
	failedMaterial["expected_revision"] = 3
	failedMaterial["kind"] = "amendment"
	failedMaterial["amendment_id"] = command()
	call("POST", base+"/reference-oa-materials", failedMaterial, 403)
	remove := map[string]any{"command_id": command(), "expected_revision": 3, "operation": "remove", "kind": "contract", "material_id": firstMaterial["command_id"]}
	call("POST", base+"/reference-oa-materials", remove, 200)
	call("POST", base+"/reference-oa-materials", remove, 200)
	materialState := call("GET", base+"/workspace", nil, 200)["reference_oa_materials"].(map[string]any)
	if materialState["revision"] != float64(4) || len(materialState["items"].([]any)) != 3 || materialState["items"].([]any)[0].(map[string]any)["removed_at"] == nil {
		t.Fatal("material history or current reference lost")
	}
	var materialCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM reference_oa_material_commands WHERE contract_id=$1`, id).Scan(&materialCount); err != nil || materialCount != 4 {
		t.Fatal("failed material wrote history", err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM reference_oa_material_commands WHERE contract_id=$1`, id); err == nil {
		t.Fatal("material history deletion permitted")
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM audit_entries WHERE object_id=$1 AND (actor_kind<>'iam_subject' OR actor_id<>$2)", id, subject).Scan(&foreignActors); err != nil || foreignActors != 0 {
		t.Fatal("trusted actor lost")
	}
	token = credential("55555555-5555-4555-8555-555555555555", all)
	call("GET", base+"/workspace", nil, 403)
	call("POST", base+"/changes", change, 403)
	call("POST", confirmPath, confirmation, 403)
	call("POST", base+"/reference-oa-materials", firstMaterial, 403)
	token = credential(id, []string{"contract-ledger.view"})
	call("GET", "/api/v1/contracts/workspace-context", nil, 200)
	call("GET", base+"/workspace", nil, 200)
	call("POST", base+"/settlements", settlement, 403)
	call("POST", base+"/reference-oa-materials", firstMaterial, 403)
	call("POST", confirmPath, confirmation, 403)
	token = credential(id, all)
	input["command_id"] = command()
	call("POST", base, input, 409)
}
