package httpserver

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/contract"
	"omsbizfoundry/contract-ledger/internal/evidence"
	"omsbizfoundry/contract-ledger/internal/identity"
	"omsbizfoundry/contract-ledger/internal/ledgerexport"
	"omsbizfoundry/contract-ledger/internal/ledgerrestore"
	"omsbizfoundry/contract-ledger/internal/party"
)

var defaultTestPrincipal = identity.Principal{
	UserID:            "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
	Username:          "test-admin",
	DisplayName:       "测试管理员",
	Role:              identity.RoleAdministrator,
	SessionGeneration: 1,
}

func unitFormHiddenValue(t *testing.T, body, action, name string) string {
	t.Helper()
	formPattern := regexp.MustCompile(`(?s)<form[^>]*action="` + regexp.QuoteMeta(action) + `"[^>]*>(.*?)</form>`)
	formMatch := formPattern.FindStringSubmatch(body)
	if len(formMatch) != 2 {
		t.Fatalf("page missing form action %q: %q", action, body)
	}
	inputPattern := regexp.MustCompile(`name="` + regexp.QuoteMeta(name) + `" value="([^"]*)"`)
	inputMatch := inputPattern.FindStringSubmatch(formMatch[1])
	if len(inputMatch) != 2 {
		t.Fatalf("form %q missing hidden input %q: %q", action, name, formMatch[1])
	}
	return inputMatch[1]
}

type identityUseCasesStub struct {
	principal    identity.Principal
	login        identity.LoginResult
	loginErr     error
	users        []identity.User
	lastCreate   identity.CreateUserInput
	lastRole     identity.ChangeRoleInput
	lastStatus   identity.ChangeStatusInput
	lastPassword identity.ChangePasswordInput
	lastReset    identity.ResetPasswordInput
}

func (s *identityUseCasesStub) Login(context.Context, string, string) (identity.LoginResult, error) {
	return s.login, s.loginErr
}

func (s *identityUseCasesStub) ResolveSession(_ context.Context, token string) (identity.Principal, bool, error) {
	if token != "test-session" {
		return identity.Principal{}, false, nil
	}
	principal := s.principal
	if principal.UserID == "" {
		principal = defaultTestPrincipal
	}
	return principal, true, nil
}

func (s *identityUseCasesStub) Logout(context.Context, string) error { return nil }
func (s *identityUseCasesStub) ListUsers(context.Context) ([]identity.User, error) {
	return append([]identity.User(nil), s.users...), nil
}
func (s *identityUseCasesStub) CreateUser(_ context.Context, input identity.CreateUserInput) (identity.User, error) {
	s.lastCreate = input
	return identity.User{ID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Username: input.Username}, nil
}
func (s *identityUseCasesStub) ChangeRole(_ context.Context, input identity.ChangeRoleInput) error {
	s.lastRole = input
	return nil
}
func (s *identityUseCasesStub) ChangeStatus(_ context.Context, input identity.ChangeStatusInput) error {
	s.lastStatus = input
	return nil
}
func (s *identityUseCasesStub) ChangePassword(_ context.Context, input identity.ChangePasswordInput) error {
	s.lastPassword = input
	return nil
}
func (s *identityUseCasesStub) ResetPassword(_ context.Context, input identity.ResetPasswordInput) error {
	s.lastReset = input
	return nil
}

type auditUseCasesStub struct{ entries []audit.Entry }

func (s *auditUseCasesStub) List(context.Context, int) ([]audit.Entry, error) {
	return append([]audit.Entry(nil), s.entries...), nil
}

type ledgerExportUseCasesStub struct {
	artifact ledgerexport.Artifact
	err      error
	last     ledgerexport.Input
}

func (s *ledgerExportUseCasesStub) Export(_ context.Context, input ledgerexport.Input) (ledgerexport.Artifact, error) {
	s.last = input
	return s.artifact, s.err
}

type ledgerRestoreUseCasesStub struct {
	result ledgerrestore.Result
	err    error
	last   ledgerrestore.Input
	calls  int
}

func (s *ledgerRestoreUseCasesStub) Restore(_ context.Context, input ledgerrestore.Input) (ledgerrestore.Result, error) {
	s.calls++
	s.last = input
	return s.result, s.err
}

type evidenceUseCasesStub struct {
	maxBytes      int64
	documents     []evidence.Document
	listErr       error
	createErr     error
	addVersionErr error
	openErr       error
	lastCreate    evidence.CreateInput
	lastAdd       evidence.AddVersionInput
	lastDownload  evidence.DownloadInput
	lastContent   []byte
	download      evidence.Download
}

func (s *evidenceUseCasesStub) MaxUploadBytes() int64 {
	if s.maxBytes == 0 {
		return evidence.DefaultMaxUploadBytes
	}
	return s.maxBytes
}

func (s *evidenceUseCasesStub) Create(_ context.Context, input evidence.CreateInput, content io.Reader) (evidence.UploadResult, error) {
	s.lastCreate = input
	s.lastContent, _ = io.ReadAll(content)
	return evidence.UploadResult{}, s.createErr
}

func (s *evidenceUseCasesStub) AddVersion(_ context.Context, input evidence.AddVersionInput, content io.Reader) (evidence.UploadResult, error) {
	s.lastAdd = input
	s.lastContent, _ = io.ReadAll(content)
	return evidence.UploadResult{}, s.addVersionErr
}

func (s *evidenceUseCasesStub) List(context.Context, string) ([]evidence.Document, error) {
	return append([]evidence.Document(nil), s.documents...), s.listErr
}

func (s *evidenceUseCasesStub) Open(_ context.Context, input evidence.DownloadInput) (evidence.Download, error) {
	s.lastDownload = input
	return s.download, s.openErr
}

type testReadSeekCloser struct{ *bytes.Reader }

func (testReadSeekCloser) Close() error { return nil }

func newTestHandler(readiness ReadinessChecker, parties PartyUseCases, contracts ContractUseCases) (http.Handler, error) {
	return newTestHandlerWithPrincipal(readiness, parties, contracts, defaultTestPrincipal)
}

func New(readiness ReadinessChecker, parties PartyUseCases, contracts ContractUseCases) (http.Handler, error) {
	return newTestHandler(readiness, parties, contracts)
}

func NewWithActor(
	readiness ReadinessChecker,
	parties PartyUseCases,
	contracts ContractUseCases,
	actor contract.ActorRef,
) (http.Handler, error) {
	return newTestHandlerWithPrincipal(readiness, parties, contracts, identity.Principal{
		UserID:            actor.ID,
		Username:          actor.ID,
		DisplayName:       actor.DisplayName,
		Role:              identity.RoleAdministrator,
		SessionGeneration: 1,
	})
}

func newTestHandlerWithPrincipal(
	readiness ReadinessChecker,
	parties PartyUseCases,
	contracts ContractUseCases,
	principal identity.Principal,
) (http.Handler, error) {
	handler, err := NewServer(readiness, parties, contracts, &evidenceUseCasesStub{}, &ledgerExportUseCasesStub{}, &ledgerRestoreUseCasesStub{}, &identityUseCasesStub{principal: principal}, &auditUseCasesStub{}, Options{})
	if err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie(sessionCookieName); err != nil {
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
		}
		handler.ServeHTTP(w, r)
	}), nil
}

func newRawTestHandler(
	t *testing.T,
	identities *identityUseCasesStub,
	parties PartyUseCases,
	contracts ContractUseCases,
	options Options,
) http.Handler {
	t.Helper()
	handler, err := NewServer(readinessStub{}, parties, contracts, &evidenceUseCasesStub{}, &ledgerExportUseCasesStub{}, &ledgerRestoreUseCasesStub{}, identities, &auditUseCasesStub{}, options)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return handler
}

func newEvidenceTestHandler(
	t *testing.T,
	principal identity.Principal,
	contracts ContractUseCases,
	evidenceUseCases EvidenceUseCases,
) http.Handler {
	t.Helper()
	handler, err := NewServer(
		readinessStub{}, &partyUseCasesStub{}, contracts, evidenceUseCases, &ledgerExportUseCasesStub{},
		&ledgerRestoreUseCasesStub{},
		&identityUseCasesStub{principal: principal}, &auditUseCasesStub{}, Options{},
	)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie(sessionCookieName); err != nil {
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
		}
		handler.ServeHTTP(w, r)
	})
}

func newLedgerExportTestHandler(
	t *testing.T,
	principal identity.Principal,
	contracts ContractUseCases,
	exports LedgerExportUseCases,
) http.Handler {
	t.Helper()
	handler, err := NewServer(
		readinessStub{}, &partyUseCasesStub{}, contracts, &evidenceUseCasesStub{}, exports,
		&ledgerRestoreUseCasesStub{},
		&identityUseCasesStub{principal: principal}, &auditUseCasesStub{}, Options{},
	)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return handler
}

func newLedgerRestoreTestHandler(
	t *testing.T,
	principal identity.Principal,
	restores LedgerRestoreUseCases,
) http.Handler {
	t.Helper()
	handler, err := NewServer(
		readinessStub{}, &partyUseCasesStub{}, &contractUseCasesStub{}, &evidenceUseCasesStub{},
		&ledgerExportUseCasesStub{}, restores,
		&identityUseCasesStub{principal: principal}, &auditUseCasesStub{}, Options{},
	)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return handler
}

func newUnitMultipartRequest(
	t *testing.T,
	target string,
	values url.Values,
	fileField string,
	filename string,
	content []byte,
) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, fieldValues := range values {
		for _, value := range fieldValues {
			if err := writer.WriteField(name, value); err != nil {
				t.Fatalf("write multipart field %q: %v", name, err)
			}
		}
	}
	part, err := writer.CreateFormFile(fileField, filename)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, target, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

type readinessStub struct {
	err error
}

func (s readinessStub) Ping(context.Context) error { return s.err }

type partyUseCasesStub struct {
	parties    []party.Party
	createErr  error
	listErr    error
	lastCreate party.CreateInput
}

func (s *partyUseCasesStub) Create(_ context.Context, input party.CreateInput) (party.Party, error) {
	s.lastCreate = input
	if s.createErr != nil {
		return party.Party{}, s.createErr
	}
	created := party.Party{ID: "created", Kind: party.Kind(input.Kind), LegalName: input.LegalName, Revision: 1}
	s.parties = append(s.parties, created)
	return created, nil
}

func (s *partyUseCasesStub) List(context.Context) ([]party.Party, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return append([]party.Party(nil), s.parties...), nil
}

type contractUseCasesStub struct {
	contracts              []contract.Contract
	searchPage             contract.ContractSearchPage
	createErr              error
	listErr                error
	searchErr              error
	details                contract.DraftDetails
	detailsByID            map[string]contract.DraftDetails
	getErr                 error
	addErr                 error
	removeErr              error
	updateErr              error
	cancelErr              error
	activateErr            error
	changeErr              error
	terminateErr           error
	relationErr            error
	settlementErr          error
	closureErr             error
	correctionErr          error
	lastCreate             contract.CreateDraftInput
	lastSearch             contract.SearchContractsInput
	lastAdd                contract.AddDraftPartyInput
	lastRemove             contract.RemoveDraftPartyInput
	lastUpdate             contract.UpdateDraftInput
	lastCancel             contract.CancelDraftInput
	lastActivate           contract.ActivateContractInput
	lastChange             contract.AppendContractChangeInput
	lastTerminate          contract.TerminateContractInput
	lastRelation           contract.LinkContractRelationInput
	lastSettlement         contract.RecordContractSettlementInput
	lastClosure            contract.CloseContractInput
	lastCorrection         contract.CorrectContractInput
	relationResult         contract.ContractRelationResult
	settlementReference    contract.ContractSettlement
	settlementReferenceErr error
}

func (s *contractUseCasesStub) GetSettlement(context.Context, string) (contract.ContractSettlement, error) {
	return s.settlementReference, s.settlementReferenceErr
}

func (s *contractUseCasesStub) CorrectContract(_ context.Context, input contract.CorrectContractInput) (contract.ContractCorrectionResult, error) {
	s.lastCorrection = input
	if s.correctionErr != nil {
		return contract.ContractCorrectionResult{}, s.correctionErr
	}
	return contract.ContractCorrectionResult{
		ContractID: input.ContractID, InstrumentID: "abababab-abab-4bab-8bab-abababababab",
		Revision: input.ExpectedRevision + 1, EffectiveAmount: input.CorrectedTotalAmount,
		Status: contract.Status(input.ExpectedStatus),
	}, nil
}

func (s *contractUseCasesStub) CloseContract(_ context.Context, input contract.CloseContractInput) (contract.ContractClosureResult, error) {
	s.lastClosure = input
	if s.closureErr != nil {
		return contract.ContractClosureResult{}, s.closureErr
	}
	return contract.ContractClosureResult{Closure: contract.ContractClosure{
		ID: "99999999-9999-4999-8999-999999999999", ContractID: input.ContractID,
		SourceRevision:           input.ExpectedContractRevision,
		SourceSettlementSequence: input.ExpectedSettlementSequence,
		SettlementDisposition:    contract.SettlementDisposition(input.SettlementDisposition),
		ClosedOn:                 input.ClosedOn, Reason: input.Reason, Actor: input.Actor,
	}}, nil
}

func (s *contractUseCasesStub) RecordSettlement(_ context.Context, input contract.RecordContractSettlementInput) (contract.ContractSettlementResult, error) {
	s.lastSettlement = input
	if s.settlementErr != nil {
		return contract.ContractSettlementResult{}, s.settlementErr
	}
	return contract.ContractSettlementResult{Settlement: contract.ContractSettlement{
		ID: "ffffffff-ffff-4fff-8fff-ffffffffffff", ContractID: input.ContractID,
		SourceRevision: input.ExpectedContractRevision,
		Sequence:       input.ExpectedSettlementSequence + 1,
		Kind:           contract.SettlementKind(input.Kind), StatementNumber: input.StatementNumber,
		Title: input.Title, Amount: input.Amount, ConfirmedOn: input.ConfirmedOn, Basis: input.Basis,
		Actor: input.Actor,
	}}, nil
}

func (s *contractUseCasesStub) CreateDraft(_ context.Context, input contract.CreateDraftInput) (contract.Contract, error) {
	s.lastCreate = input
	if s.createErr != nil {
		return contract.Contract{}, s.createErr
	}
	created := contract.Contract{
		ID:              "created-contract",
		CommandID:       input.CommandID,
		NumberNamespace: input.NumberNamespace,
		ContractNumber:  input.ContractNumber,
		Title:           input.Title,
		Currency:        input.Currency,
		TotalAmount:     input.TotalAmount,
		Status:          contract.StatusDraft,
		DraftVersion:    1,
	}
	s.contracts = append(s.contracts, created)
	return created, nil
}

func (s *contractUseCasesStub) List(context.Context) ([]contract.Contract, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return append([]contract.Contract(nil), s.contracts...), nil
}

func (s *contractUseCasesStub) Search(_ context.Context, input contract.SearchContractsInput) (contract.ContractSearchPage, error) {
	s.lastSearch = input
	if s.searchErr != nil {
		return contract.ContractSearchPage{}, s.searchErr
	}
	page := s.searchPage
	if page.Contracts == nil {
		page.Contracts = append([]contract.Contract(nil), s.contracts...)
	}
	if page.Keyword == "" {
		page.Keyword = strings.TrimSpace(input.Keyword)
	}
	if page.Status == "" {
		page.Status = contract.Status(strings.ToLower(strings.TrimSpace(input.Status)))
	}
	if page.Currency == "" {
		page.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	}
	return page, nil
}

func (s *contractUseCasesStub) GetDraft(_ context.Context, contractID string) (contract.DraftDetails, error) {
	if s.getErr != nil {
		return contract.DraftDetails{}, s.getErr
	}
	if details, found := s.detailsByID[contractID]; found {
		return details, nil
	}
	if s.details.Contract.ID != "" {
		return s.details, nil
	}
	for _, value := range s.contracts {
		if value.ID == contractID {
			return contract.DraftDetails{Contract: value}, nil
		}
	}
	return contract.DraftDetails{}, &contract.ContractNotFoundError{ContractID: contractID}
}

func (s *contractUseCasesStub) AddDraftParty(_ context.Context, input contract.AddDraftPartyInput) (contract.DraftPartyMutationResult, error) {
	s.lastAdd = input
	if s.addErr != nil {
		return contract.DraftPartyMutationResult{}, s.addErr
	}
	return contract.DraftPartyMutationResult{
		Party: contract.DraftParty{
			ID:         "added-link",
			ContractID: input.ContractID,
			PartyID:    input.PartyID,
			Role:       input.Role,
		},
		DraftVersion: input.ExpectedDraftVersion + 1,
	}, nil
}

func (s *contractUseCasesStub) RemoveDraftParty(_ context.Context, input contract.RemoveDraftPartyInput) (contract.DraftPartyMutationResult, error) {
	s.lastRemove = input
	if s.removeErr != nil {
		return contract.DraftPartyMutationResult{}, s.removeErr
	}
	return contract.DraftPartyMutationResult{
		Party:        contract.DraftParty{ID: input.PartyLinkID, ContractID: input.ContractID},
		DraftVersion: input.ExpectedDraftVersion + 1,
	}, nil
}

func (s *contractUseCasesStub) UpdateDraft(_ context.Context, input contract.UpdateDraftInput) (contract.DraftMutationResult, error) {
	s.lastUpdate = input
	if s.updateErr != nil {
		return contract.DraftMutationResult{}, s.updateErr
	}
	return contract.DraftMutationResult{
		ContractID:   input.ContractID,
		DraftVersion: input.ExpectedDraftVersion + 1,
		Status:       contract.StatusDraft,
	}, nil
}

func (s *contractUseCasesStub) CancelDraft(_ context.Context, input contract.CancelDraftInput) (contract.DraftMutationResult, error) {
	s.lastCancel = input
	if s.cancelErr != nil {
		return contract.DraftMutationResult{}, s.cancelErr
	}
	return contract.DraftMutationResult{
		ContractID:   input.ContractID,
		DraftVersion: input.ExpectedDraftVersion + 1,
		Status:       contract.StatusCancelled,
	}, nil
}

func (s *contractUseCasesStub) Activate(_ context.Context, input contract.ActivateContractInput) (contract.ActivationResult, error) {
	s.lastActivate = input
	if s.activateErr != nil {
		return contract.ActivationResult{}, s.activateErr
	}
	return contract.ActivationResult{ContractID: input.ContractID, Revision: 1}, nil
}

func (s *contractUseCasesStub) AppendChange(_ context.Context, input contract.AppendContractChangeInput) (contract.ContractChangeResult, error) {
	s.lastChange = input
	if s.changeErr != nil {
		return contract.ContractChangeResult{}, s.changeErr
	}
	return contract.ContractChangeResult{
		ContractID: input.ContractID, InstrumentID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
		Revision: input.ExpectedRevision + 1, EffectiveAmount: input.AmountValue,
	}, nil
}

func (s *contractUseCasesStub) Terminate(_ context.Context, input contract.TerminateContractInput) (contract.TerminationResult, error) {
	s.lastTerminate = input
	if s.terminateErr != nil {
		return contract.TerminationResult{}, s.terminateErr
	}
	return contract.TerminationResult{
		ContractID: input.ContractID, InstrumentID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
		Revision: input.ExpectedRevision + 1, Status: contract.StatusTerminated,
	}, nil
}

func (s *contractUseCasesStub) LinkRelation(_ context.Context, input contract.LinkContractRelationInput) (contract.ContractRelationResult, error) {
	s.lastRelation = input
	if s.relationErr != nil {
		return contract.ContractRelationResult{}, s.relationErr
	}
	if s.relationResult.Relation.ID != "" {
		return s.relationResult, nil
	}
	return contract.ContractRelationResult{Relation: contract.ContractRelation{
		ID:     "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee",
		Source: contract.ContractRelationSnapshot{ContractID: input.SourceContractID, Revision: input.ExpectedSourceRevision},
		Target: contract.ContractRelationSnapshot{ContractID: input.TargetContractID, Revision: input.ExpectedTargetRevision},
		Type:   contract.RelationType(input.RelationType), EffectiveOn: input.EffectiveOn, Note: input.Note,
		Actor: input.Actor,
	}}, nil
}

func TestHealthEndpoints(t *testing.T) {
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, &contractUseCasesStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for _, path := range []string{"/health/live", "/health/ready"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
			}
			if !strings.Contains(response.Body.String(), serviceName) {
				t.Fatalf("body = %q, want service name", response.Body.String())
			}
		})
	}
}

func TestReadinessFailure(t *testing.T) {
	handler, err := New(readinessStub{err: errors.New("database unavailable")}, &partyUseCasesStub{}, &contractUseCasesStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestHome(t *testing.T) {
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, &contractUseCasesStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "Contract Ledger") {
		t.Fatalf("body does not contain product name: %q", response.Body.String())
	}
}

func TestPartyListAndCreate(t *testing.T) {
	parties := &partyUseCasesStub{parties: []party.Party{{
		ID:        "existing",
		Kind:      party.KindOrganization,
		LegalName: "现有主体",
		Revision:  1,
	}}}
	handler, err := New(readinessStub{}, parties, &contractUseCasesStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/parties", nil)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), "现有主体") {
		t.Fatalf("GET /parties status = %d, body = %q", listResponse.Code, listResponse.Body.String())
	}

	tokenPattern := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)
	match := tokenPattern.FindStringSubmatch(listResponse.Body.String())
	if len(match) != 2 {
		t.Fatalf("CSRF token missing from body: %q", listResponse.Body.String())
	}
	cookies := listResponse.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("CSRF cookie missing")
	}

	form := url.Values{
		"csrf_token":              {match[1]},
		"kind":                    {"individual"},
		"legal_name":              {"张三"},
		"registration_identifier": {"PERSON-001"},
	}
	createRequest := httptest.NewRequest(http.MethodPost, "/parties", strings.NewReader(form.Encode()))
	createRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	createRequest.AddCookie(cookies[0])
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, createRequest)

	if createResponse.Code != http.StatusSeeOther {
		t.Fatalf("POST /parties status = %d, body = %q", createResponse.Code, createResponse.Body.String())
	}
	if location := createResponse.Header().Get("Location"); location != "/parties?created=1" {
		t.Fatalf("Location = %q", location)
	}
	if parties.lastCreate.Kind != "individual" || parties.lastCreate.LegalName != "张三" {
		t.Fatalf("Create input = %#v", parties.lastCreate)
	}
}

func TestPartyCreateRejectsMissingCSRFToken(t *testing.T) {
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, &contractUseCasesStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/parties", strings.NewReader("kind=organization&legal_name=example"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestPartyCreateShowsValidationError(t *testing.T) {
	parties := &partyUseCasesStub{createErr: &party.ValidationError{Field: "legal_name", Message: "法定名称或姓名不能为空"}}
	handler, err := New(readinessStub{}, parties, &contractUseCasesStub{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/parties", nil)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	match := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(listResponse.Body.String())
	cookie := listResponse.Result().Cookies()[0]

	form := url.Values{"csrf_token": {match[1]}, "kind": {"organization"}, "legal_name": {""}}
	request := httptest.NewRequest(http.MethodPost, "/parties", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "法定名称或姓名不能为空") {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
}

func TestContractListAndCreate(t *testing.T) {
	contracts := &contractUseCasesStub{contracts: []contract.Contract{{
		ID:              "existing",
		NumberNamespace: "default",
		ContractNumber:  "HT-001",
		Title:           "现有合同草稿",
		Currency:        "CNY",
		Status:          contract.StatusDraft,
		DraftVersion:    1,
	}}}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/contracts", nil)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), "现有合同草稿") {
		t.Fatalf("GET /contracts status = %d, body = %q", listResponse.Code, listResponse.Body.String())
	}

	csrfMatch := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(listResponse.Body.String())
	commandMatch := regexp.MustCompile(`name="command_id" value="([^"]+)"`).FindStringSubmatch(listResponse.Body.String())
	if len(csrfMatch) != 2 || len(commandMatch) != 2 {
		t.Fatalf("form tokens missing from body: %q", listResponse.Body.String())
	}

	form := url.Values{
		"csrf_token":       {csrfMatch[1]},
		"command_id":       {commandMatch[1]},
		"number_namespace": {"default"},
		"contract_number":  {"HT-002"},
		"title":            {"新合同草稿"},
		"category":         {"服务"},
		"currency":         {"CNY"},
		"total_amount":     {"1000.50"},
		"signed_on":        {"2026-09-19"},
		"effective_on":     {"2026-10-01"},
	}
	request := httptest.NewRequest(http.MethodPost, "/contracts", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(listResponse.Result().Cookies()[0])
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("POST /contracts status = %d, body = %q", response.Code, response.Body.String())
	}
	if !strings.HasPrefix(response.Header().Get("Location"), "/contracts?created=1&contract_id=") {
		t.Fatalf("Location = %q", response.Header().Get("Location"))
	}
	if contracts.lastCreate.ContractNumber != "HT-002" || contracts.lastCreate.TotalAmount != "1000.50" {
		t.Fatalf("CreateDraft input = %#v", contracts.lastCreate)
	}
}

func TestContractSearchFiltersAndNextPage(t *testing.T) {
	contracts := &contractUseCasesStub{searchPage: contract.ContractSearchPage{
		Contracts: []contract.Contract{{
			ID: "11111111-1111-4111-8111-111111111111", NumberNamespace: "sales",
			ContractNumber: "HT-2026-001", Title: "年度服务合同", Currency: "CNY",
			TotalAmount: "15000.125", Status: contract.StatusTerminated, CurrentRevision: 3,
		}},
		Keyword: "年度服务", Status: contract.StatusTerminated, Currency: "CNY", NextCursor: "cursor-token_1",
	}}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	query := url.Values{"q": {"年度服务"}, "status": {"terminated"}, "currency": {"cny"}}
	request := httptest.NewRequest(http.MethodGet, "/contracts?"+query.Encode(), nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, expected := range []string{
		"年度服务合同", "15000.125 CNY", "已终止 r3", "下一页", "cursor=cursor-token_1",
		`value="年度服务"`, `value="terminated" selected`, `value="CNY"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("body missing %q: %q", expected, body)
		}
	}
	if contracts.lastSearch.Keyword != "年度服务" || contracts.lastSearch.Status != "terminated" || contracts.lastSearch.Currency != "cny" {
		t.Fatalf("Search input = %#v", contracts.lastSearch)
	}
}

func TestContractSearchValidationErrorPreservesFilters(t *testing.T) {
	contracts := &contractUseCasesStub{searchErr: &contract.ValidationError{
		Field: "status", Message: "合同状态筛选无效",
	}}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/contracts?q=HT-001&status=archived&currency=cny", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "合同状态筛选无效") {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	for _, expected := range []string{`value="HT-001"`, `value="archived" selected`, `value="cny"`, "清除筛选"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("body missing %q: %q", expected, response.Body.String())
		}
	}
}

func TestContractRelationTargetSelectionAndLink(t *testing.T) {
	const (
		sourceID = "11111111-1111-4111-8111-111111111111"
		targetID = "22222222-2222-4222-8222-222222222222"
	)
	contracts := &contractUseCasesStub{
		detailsByID: map[string]contract.DraftDetails{
			sourceID: {Contract: contract.Contract{
				ID: sourceID, NumberNamespace: "sales", ContractNumber: "UP-001", Title: "上游合同",
				Currency: "CNY", TotalAmount: "1000", Status: contract.StatusActive, CurrentRevision: 3,
			}},
			targetID: {Contract: contract.Contract{
				ID: targetID, NumberNamespace: "delivery", ContractNumber: "DOWN-001", Title: "下游合同",
				Currency: "CNY", TotalAmount: "800", Status: contract.StatusTerminated, CurrentRevision: 2,
			}},
		},
		searchPage: contract.ContractSearchPage{
			Contracts: []contract.Contract{{
				ID: targetID, NumberNamespace: "delivery", ContractNumber: "DOWN-001", Title: "下游合同",
				Currency: "CNY", TotalAmount: "800", Status: contract.StatusTerminated, CurrentRevision: 2,
			}},
			Keyword: "下游", Status: contract.StatusTerminated, Currency: "CNY", NextCursor: "relation-cursor_1",
		},
	}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	searchQuery := url.Values{"q": {"下游"}, "status": {"terminated"}, "currency": {"cny"}}
	searchRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+sourceID+"/relations/new?"+searchQuery.Encode(), nil)
	searchResponse := httptest.NewRecorder()
	handler.ServeHTTP(searchResponse, searchRequest)
	if searchResponse.Code != http.StatusOK || !strings.Contains(searchResponse.Body.String(), "选择为目标") ||
		!strings.Contains(searchResponse.Body.String(), "relation-cursor_1") ||
		contracts.lastSearch.Status != "terminated" || contracts.lastSearch.Currency != "cny" {
		t.Fatalf("relation target search status=%d input=%#v body=%q", searchResponse.Code, contracts.lastSearch, searchResponse.Body.String())
	}

	selectRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+sourceID+"/relations/new?target_id="+targetID, nil)
	selectResponse := httptest.NewRecorder()
	handler.ServeHTTP(selectResponse, selectRequest)
	if selectResponse.Code != http.StatusOK {
		t.Fatalf("relation selection status=%d body=%q", selectResponse.Code, selectResponse.Body.String())
	}
	selectBody := selectResponse.Body.String()
	for _, expected := range []string{
		"上游合同", "下游合同", "来源合同是目标合同的分包合同",
		`name="expected_source_revision" value="3"`, `name="expected_target_revision" value="2"`,
	} {
		if !strings.Contains(selectBody, expected) {
			t.Fatalf("selected relation page missing %q: %q", expected, selectBody)
		}
	}
	csrf := unitFormHiddenValue(t, selectBody, "/contracts/"+sourceID+"/relations", "csrf_token")
	commandID := unitFormHiddenValue(t, selectBody, "/contracts/"+sourceID+"/relations", "command_id")
	form := url.Values{
		"csrf_token": {csrf}, "command_id": {commandID}, "target_contract_id": {targetID},
		"expected_source_revision": {"3"}, "expected_target_revision": {"2"},
		"relation_type": {"subcontract_of"}, "effective_on": {"2026-10-02"}, "note": {"下游承接分包工作"},
		"actor_id": {"forged"},
	}
	request := httptest.NewRequest(http.MethodPost, "/contracts/"+sourceID+"/relations", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range selectResponse.Result().Cookies() {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/contracts/"+sourceID+"?relation_linked=1" {
		t.Fatalf("link relation status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	if contracts.lastRelation.TargetContractID != targetID || contracts.lastRelation.RelationType != "subcontract_of" ||
		contracts.lastRelation.ExpectedSourceRevision != 3 || contracts.lastRelation.ExpectedTargetRevision != 2 ||
		contracts.lastRelation.Note != "下游承接分包工作" || contracts.lastRelation.Actor != defaultTestPrincipal.Actor() {
		t.Fatalf("LinkRelation input = %#v", contracts.lastRelation)
	}
}

func TestContractRelationConflictRendersCurrentRevisions(t *testing.T) {
	const (
		sourceID = "11111111-1111-4111-8111-111111111111"
		targetID = "22222222-2222-4222-8222-222222222222"
	)
	contracts := &contractUseCasesStub{
		detailsByID: map[string]contract.DraftDetails{
			sourceID: {Contract: contract.Contract{ID: sourceID, Title: "来源", Currency: "CNY", Status: contract.StatusActive, CurrentRevision: 4}},
			targetID: {Contract: contract.Contract{ID: targetID, Title: "目标", Currency: "CNY", Status: contract.StatusActive, CurrentRevision: 5}},
		},
		relationErr: &contract.ContractRelationCycleError{},
	}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	getRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+sourceID+"/relations/new?target_id="+targetID, nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	body := getResponse.Body.String()
	csrf := unitFormHiddenValue(t, body, "/contracts/"+sourceID+"/relations", "csrf_token")
	commandID := unitFormHiddenValue(t, body, "/contracts/"+sourceID+"/relations", "command_id")
	form := url.Values{
		"csrf_token": {csrf}, "command_id": {commandID}, "target_contract_id": {targetID},
		"expected_source_revision": {"3"}, "expected_target_revision": {"3"},
		"relation_type": {"subcontract_of"}, "note": {"形成环的说明"},
	}
	request := httptest.NewRequest(http.MethodPost, "/contracts/"+sourceID+"/relations", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range getResponse.Result().Cookies() {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "形成不允许的有向环") ||
		!strings.Contains(response.Body.String(), `name="expected_source_revision" value="4"`) ||
		!strings.Contains(response.Body.String(), `name="expected_target_revision" value="5"`) ||
		!strings.Contains(response.Body.String(), "形成环的说明") {
		t.Fatalf("relation conflict status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestContractDetailShowsIncomingAndOutgoingRelations(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	details := contract.DraftDetails{Contract: contract.Contract{
		ID: contractID, NumberNamespace: "main", ContractNumber: "MAIN-001", Title: "关系中心合同",
		Currency: "CNY", Status: contract.StatusActive, CurrentRevision: 2,
	}}
	details.OutgoingRelations = []contract.ContractRelation{{
		ID: "33333333-3333-4333-8333-333333333333", Type: contract.RelationTypeFundingSourceFor,
		Source:      contract.ContractRelationSnapshot{ContractID: contractID, Revision: 2},
		Target:      contract.ContractRelationSnapshot{ContractID: "22222222-2222-4222-8222-222222222222", Revision: 1, NumberNamespace: "down", ContractNumber: "DOWN-1", Title: "目标快照标题"},
		EffectiveOn: "2026-10-02", Note: "资金来源说明", Actor: defaultTestPrincipal.Actor(), CreatedAt: time.Now().UTC(),
	}}
	details.IncomingRelations = []contract.ContractRelation{{
		ID: "44444444-4444-4444-8444-444444444444", Type: contract.RelationTypeReferences,
		Source: contract.ContractRelationSnapshot{ContractID: "55555555-5555-4555-8555-555555555555", Revision: 3, NumberNamespace: "ref", ContractNumber: "REF-1", Title: "来源快照标题"},
		Target: contract.ContractRelationSnapshot{ContractID: contractID, Revision: 2},
		Actor:  defaultTestPrincipal.Actor(), CreatedAt: time.Now().UTC(),
	}}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, &contractUseCasesStub{details: details})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID+"?relation_linked=1", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	for _, expected := range []string{"合同关系已建立", "资金来源／上游依据", "目标快照标题", "来源快照标题", "资金来源说明", "建立合同关系"} {
		if response.Code != http.StatusOK || !strings.Contains(body, expected) {
			t.Fatalf("detail missing %q status=%d body=%q", expected, response.Code, body)
		}
	}
}

func TestViewerCannotOpenOrSubmitContractRelations(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	viewer := defaultTestPrincipal
	viewer.Role = identity.RoleViewer
	contracts := &contractUseCasesStub{}
	handler := newRawTestHandler(t, &identityUseCasesStub{principal: viewer}, &partyUseCasesStub{}, contracts, Options{})
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/contracts/"+contractID+"/relations/new", nil),
		httptest.NewRequest(http.MethodPost, "/contracts/"+contractID+"/relations", strings.NewReader("")),
	} {
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s relation route status=%d body=%q", request.Method, response.Code, response.Body.String())
		}
	}
	if contracts.lastRelation.CommandID != "" || contracts.lastSearch.Status != "" {
		t.Fatalf("viewer reached relation use cases: relation=%#v search=%#v", contracts.lastRelation, contracts.lastSearch)
	}
}

func TestContractCreateShowsDuplicateNumber(t *testing.T) {
	contracts := &contractUseCasesStub{createErr: &contract.DuplicateNumberError{
		NumberNamespace: "default",
		ContractNumber:  "HT-001",
	}}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/contracts", nil)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	csrfMatch := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(listResponse.Body.String())
	commandMatch := regexp.MustCompile(`name="command_id" value="([^"]+)"`).FindStringSubmatch(listResponse.Body.String())

	form := url.Values{
		"csrf_token":       {csrfMatch[1]},
		"command_id":       {commandMatch[1]},
		"number_namespace": {"default"},
		"contract_number":  {"HT-001"},
		"title":            {"重复编号"},
		"currency":         {"CNY"},
	}
	request := httptest.NewRequest(http.MethodPost, "/contracts", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(listResponse.Result().Cookies()[0])
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "已存在相同合同编号") {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
}

func TestContractDetailAddAndRemoveParty(t *testing.T) {
	const (
		contractID = "11111111-1111-4111-8111-111111111111"
		partyID    = "22222222-2222-4222-8222-222222222222"
		linkID     = "33333333-3333-4333-8333-333333333333"
	)
	parties := &partyUseCasesStub{parties: []party.Party{{
		ID:                     partyID,
		Kind:                   party.KindOrganization,
		LegalName:              "示例签约主体",
		RegistrationIdentifier: "PARTY-001",
		Revision:               1,
	}}}
	contracts := &contractUseCasesStub{details: contract.DraftDetails{
		Contract: contract.Contract{
			ID:              contractID,
			NumberNamespace: "default",
			ContractNumber:  "HT-001",
			Title:           "签约主体测试合同",
			Currency:        "CNY",
			Status:          contract.StatusDraft,
			DraftVersion:    1,
		},
		Parties: []contract.DraftParty{{
			ID:                     linkID,
			ContractID:             contractID,
			PartyID:                partyID,
			PartyLegalName:         "示例签约主体",
			RegistrationIdentifier: "PARTY-001",
			PartyRevision:          1,
			Role:                   "委托方",
		}},
	}}
	handler, err := New(readinessStub{}, parties, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), "示例签约主体") || !strings.Contains(listResponse.Body.String(), "委托方") {
		t.Fatalf("GET contract detail status = %d, body = %q", listResponse.Code, listResponse.Body.String())
	}
	csrfMatch := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(listResponse.Body.String())
	if len(csrfMatch) != 2 || len(listResponse.Result().Cookies()) == 0 {
		t.Fatalf("contract detail form security missing: %q", listResponse.Body.String())
	}
	commandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	addForm := url.Values{
		"csrf_token":             {csrfMatch[1]},
		"command_id":             {commandID},
		"party_id":               {partyID},
		"role":                   {"受托方"},
		"expected_draft_version": {"1"},
	}
	addRequest := httptest.NewRequest(http.MethodPost, "/contracts/"+contractID+"/parties", strings.NewReader(addForm.Encode()))
	addRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addRequest.AddCookie(listResponse.Result().Cookies()[0])
	addResponse := httptest.NewRecorder()
	handler.ServeHTTP(addResponse, addRequest)
	if addResponse.Code != http.StatusSeeOther || addResponse.Header().Get("Location") != "/contracts/"+contractID+"?party_added=1" {
		t.Fatalf("add party status = %d, location = %q, body = %q", addResponse.Code, addResponse.Header().Get("Location"), addResponse.Body.String())
	}
	if contracts.lastAdd.PartyID != partyID || contracts.lastAdd.Role != "受托方" || contracts.lastAdd.ExpectedDraftVersion != 1 {
		t.Fatalf("AddDraftParty input = %#v", contracts.lastAdd)
	}

	removeCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	removeForm := url.Values{
		"csrf_token":             {csrfMatch[1]},
		"command_id":             {removeCommandID},
		"expected_draft_version": {"1"},
	}
	removeRequest := httptest.NewRequest(http.MethodPost, "/contracts/"+contractID+"/parties/"+linkID+"/remove", strings.NewReader(removeForm.Encode()))
	removeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	removeRequest.AddCookie(listResponse.Result().Cookies()[0])
	removeResponse := httptest.NewRecorder()
	handler.ServeHTTP(removeResponse, removeRequest)
	if removeResponse.Code != http.StatusSeeOther || removeResponse.Header().Get("Location") != "/contracts/"+contractID+"?party_removed=1" {
		t.Fatalf("remove party status = %d, location = %q, body = %q", removeResponse.Code, removeResponse.Header().Get("Location"), removeResponse.Body.String())
	}
	if contracts.lastRemove.PartyLinkID != linkID || contracts.lastRemove.ExpectedDraftVersion != 1 {
		t.Fatalf("RemoveDraftParty input = %#v", contracts.lastRemove)
	}
}

func TestContractDetailShowsDraftVersionConflict(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	contracts := &contractUseCasesStub{
		details: contract.DraftDetails{Contract: contract.Contract{
			ID: contractID, Title: "并发测试合同", Currency: "CNY", Status: contract.StatusDraft, DraftVersion: 3,
		}},
		addErr: &contract.DraftVersionConflictError{Expected: 2, Actual: 3},
	}
	handler, err := New(readinessStub{}, &partyUseCasesStub{parties: []party.Party{{
		ID: "22222222-2222-4222-8222-222222222222", LegalName: "主体", Revision: 1,
	}}}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	csrfMatch := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(getResponse.Body.String())
	commandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	form := url.Values{
		"csrf_token":             {csrfMatch[1]},
		"command_id":             {commandID},
		"party_id":               {"22222222-2222-4222-8222-222222222222"},
		"role":                   {"委托方"},
		"expected_draft_version": {"2"},
	}
	request := httptest.NewRequest(http.MethodPost, "/contracts/"+contractID+"/parties", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(getResponse.Result().Cookies()[0])
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "已被其他操作更新") {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `name="expected_draft_version" value="3"`) {
		t.Fatalf("current draft version missing from response: %q", response.Body.String())
	}
}

func TestContractDraftMutationsRejectMissingCSRF(t *testing.T) {
	contracts := &contractUseCasesStub{}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	for _, path := range []string{
		"/contracts/11111111-1111-4111-8111-111111111111",
		"/contracts/11111111-1111-4111-8111-111111111111/cancel",
		"/contracts/11111111-1111-4111-8111-111111111111/activate",
		"/contracts/11111111-1111-4111-8111-111111111111/parties",
	} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, path, strings.NewReader("role=委托方"))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
			}
		})
	}
	if contracts.lastUpdate.CommandID != "" || contracts.lastCancel.CommandID != "" || contracts.lastActivate.CommandID != "" || contracts.lastAdd.CommandID != "" {
		t.Fatalf("CSRF-rejected request reached use case: update=%#v cancel=%#v activate=%#v add=%#v", contracts.lastUpdate, contracts.lastCancel, contracts.lastActivate, contracts.lastAdd)
	}
}

func TestContractDraftPartyMutationRejectsOversizedBody(t *testing.T) {
	contracts := &contractUseCasesStub{}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	body := "role=" + strings.Repeat("x", (32<<10)+1)
	request := httptest.NewRequest(http.MethodPost, "/contracts/11111111-1111-4111-8111-111111111111/parties", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if contracts.lastAdd.CommandID != "" {
		t.Fatalf("oversized request reached use case: %#v", contracts.lastAdd)
	}
}

func TestContractDetailUpdateAndCancel(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	contracts := &contractUseCasesStub{details: contract.DraftDetails{Contract: contract.Contract{
		ID:              contractID,
		NumberNamespace: "default",
		ContractNumber:  "HT-001",
		Title:           "编辑前标题",
		Category:        "服务",
		Currency:        "CNY",
		TotalAmount:     "1000.5",
		Status:          contract.StatusDraft,
		DraftVersion:    2,
	}}}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	getRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK || !strings.Contains(getResponse.Body.String(), "编辑合同草稿") {
		t.Fatalf("GET detail status = %d, body = %q", getResponse.Code, getResponse.Body.String())
	}
	csrfMatch := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(getResponse.Body.String())
	cookie := getResponse.Result().Cookies()[0]

	updateCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	updateForm := url.Values{
		"csrf_token":             {csrfMatch[1]},
		"command_id":             {updateCommandID},
		"expected_draft_version": {"2"},
		"number_namespace":       {"revised"},
		"contract_number":        {"HT-002"},
		"title":                  {"编辑后标题"},
		"category":               {"咨询"},
		"currency":               {"USD"},
		"total_amount":           {"2000.25"},
		"signed_on":              {"2026-09-20"},
		"effective_on":           {"2026-09-01"},
	}
	updateRequest := httptest.NewRequest(http.MethodPost, "/contracts/"+contractID, strings.NewReader(updateForm.Encode()))
	updateRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	updateRequest.AddCookie(cookie)
	updateResponse := httptest.NewRecorder()
	handler.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusSeeOther || updateResponse.Header().Get("Location") != "/contracts/"+contractID+"?updated=1" {
		t.Fatalf("update status = %d, location = %q, body = %q", updateResponse.Code, updateResponse.Header().Get("Location"), updateResponse.Body.String())
	}
	if contracts.lastUpdate.Title != "编辑后标题" || contracts.lastUpdate.TotalAmount != "2000.25" || contracts.lastUpdate.ExpectedDraftVersion != 2 {
		t.Fatalf("UpdateDraft input = %#v", contracts.lastUpdate)
	}

	cancelCommandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	cancelForm := url.Values{
		"csrf_token":             {csrfMatch[1]},
		"command_id":             {cancelCommandID},
		"expected_draft_version": {"2"},
		"cancellation_reason":    {"双方不再签署"},
	}
	cancelRequest := httptest.NewRequest(http.MethodPost, "/contracts/"+contractID+"/cancel", strings.NewReader(cancelForm.Encode()))
	cancelRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	cancelRequest.AddCookie(cookie)
	cancelResponse := httptest.NewRecorder()
	handler.ServeHTTP(cancelResponse, cancelRequest)
	if cancelResponse.Code != http.StatusSeeOther || cancelResponse.Header().Get("Location") != "/contracts/"+contractID+"?cancelled=1" {
		t.Fatalf("cancel status = %d, location = %q, body = %q", cancelResponse.Code, cancelResponse.Header().Get("Location"), cancelResponse.Body.String())
	}
	if contracts.lastCancel.Reason != "双方不再签署" || contracts.lastCancel.ExpectedDraftVersion != 2 {
		t.Fatalf("CancelDraft input = %#v", contracts.lastCancel)
	}
}

func TestContractDraftUpdateShowsNoChanges(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	contracts := &contractUseCasesStub{
		details: contract.DraftDetails{Contract: contract.Contract{
			ID: contractID, NumberNamespace: "default", ContractNumber: "HT-001", Title: "原标题", Currency: "CNY", Status: contract.StatusDraft, DraftVersion: 3,
		}},
		updateErr: &contract.NoDraftChangesError{},
	}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	getRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	csrfMatch := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(getResponse.Body.String())
	commandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	form := url.Values{
		"csrf_token":             {csrfMatch[1]},
		"command_id":             {commandID},
		"expected_draft_version": {"3"},
		"number_namespace":       {"default"},
		"contract_number":        {"HT-001"},
		"title":                  {"原标题"},
		"currency":               {"CNY"},
	}
	request := httptest.NewRequest(http.MethodPost, "/contracts/"+contractID, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(getResponse.Result().Cookies()[0])
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "草稿内容没有变化") {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
}

func TestCancelledContractDetailIsReadOnly(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	contracts := &contractUseCasesStub{details: contract.DraftDetails{
		Contract: contract.Contract{
			ID: contractID, NumberNamespace: "default", ContractNumber: "HT-001", Title: "已取消合同", Currency: "CNY", Status: contract.StatusCancelled, DraftVersion: 4,
		},
		Cancellation: &contract.DraftCancellation{
			CommandID: "22222222-2222-4222-8222-222222222222", Reason: "双方终止准备", CancelledAt: time.Date(2026, 9, 19, 8, 0, 0, 0, time.UTC), DraftVersion: 4,
		},
	}}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "已取消") || !strings.Contains(body, "双方终止准备") {
		t.Fatalf("status = %d, body = %q", response.Code, body)
	}
	for _, forbidden := range []string{
		`action="/contracts/` + contractID + `"`,
		`action="/contracts/` + contractID + `/parties"`,
		`action="/contracts/` + contractID + `/cancel"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("cancelled page contains mutation form %q: %q", forbidden, body)
		}
	}
}

func TestContractDraftUpdateRejectsOversizedBody(t *testing.T) {
	contracts := &contractUseCasesStub{}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	body := "title=" + strings.Repeat("x", (64<<10)+1)
	request := httptest.NewRequest(http.MethodPost, "/contracts/11111111-1111-4111-8111-111111111111", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if contracts.lastUpdate.CommandID != "" {
		t.Fatalf("oversized request reached use case: %#v", contracts.lastUpdate)
	}
}

func TestContractDraftCancelRejectsOversizedBody(t *testing.T) {
	contracts := &contractUseCasesStub{}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	body := "cancellation_reason=" + strings.Repeat("x", (16<<10)+1)
	request := httptest.NewRequest(http.MethodPost, "/contracts/11111111-1111-4111-8111-111111111111/cancel", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if contracts.lastCancel.CommandID != "" {
		t.Fatalf("oversized request reached use case: %#v", contracts.lastCancel)
	}
}

func TestContractActivationReadinessAndSubmit(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	contracts := &contractUseCasesStub{details: contract.DraftDetails{
		Contract: contract.Contract{
			ID: contractID, NumberNamespace: "default", ContractNumber: "HT-001", Title: "待激活合同", Currency: "CNY",
			SignedOn: "2026-09-19", EffectiveOn: "2026-10-01", Status: contract.StatusDraft, DraftVersion: 3,
		},
		Parties: []contract.DraftParty{
			{ID: "31111111-1111-4111-8111-111111111111", PartyID: "21111111-1111-4111-8111-111111111111", PartyLegalName: "主体甲", PartyRevision: 1, Role: "委托方"},
			{ID: "32222222-2222-4222-8222-222222222222", PartyID: "22222222-2222-4222-8222-222222222222", PartyLegalName: "主体乙", PartyRevision: 2, Role: "受托方"},
		},
	}}
	actor := contract.ActorRef{
		Kind:        "local_user",
		ID:          "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		DisplayName: "验收操作者",
	}
	handler, err := NewWithActor(readinessStub{}, &partyUseCasesStub{}, contracts, actor)
	if err != nil {
		t.Fatalf("NewWithActor() error = %v", err)
	}
	getRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	body := getResponse.Body.String()
	if getResponse.Code != http.StatusOK || !strings.Contains(body, "激活并形成 revision 1") || !strings.Contains(body, actor.String()) {
		t.Fatalf("GET activation-ready detail status = %d, body = %q", getResponse.Code, body)
	}
	csrf := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(body)[1]
	commandID, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("NewCommandID() error = %v", err)
	}
	form := url.Values{
		"csrf_token":             {csrf},
		"command_id":             {commandID},
		"expected_draft_version": {"3"},
	}
	request := httptest.NewRequest(http.MethodPost, "/contracts/"+contractID+"/activate", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(getResponse.Result().Cookies()[0])
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/contracts/"+contractID+"?activated=1" {
		t.Fatalf("activate status = %d, location = %q, body = %q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	if contracts.lastActivate.CommandID != commandID || contracts.lastActivate.ExpectedDraftVersion != 3 || contracts.lastActivate.Actor != actor {
		t.Fatalf("Activate input = %#v", contracts.lastActivate)
	}
}

func TestContractActivationShowsMissingPrerequisites(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	contracts := &contractUseCasesStub{details: contract.DraftDetails{
		Contract: contract.Contract{
			ID: contractID, NumberNamespace: "default", ContractNumber: "HT-001", Title: "不完整草稿", Currency: "CNY", Status: contract.StatusDraft, DraftVersion: 1,
		},
		Parties: []contract.DraftParty{{PartyID: "22222222-2222-4222-8222-222222222222", PartyLegalName: "唯一主体", Role: "双方角色"}},
	}}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	for _, message := range []string{"需要填写签署日期", "需要填写生效日期", "需要至少两个不同的签约主体"} {
		if !strings.Contains(body, message) {
			t.Fatalf("missing activation issue %q: %q", message, body)
		}
	}
	if strings.Contains(body, `action="/contracts/`+contractID+`/activate"`) {
		t.Fatalf("activation form rendered before prerequisites: %q", body)
	}
}

func TestActiveContractDetailUsesRevisionSnapshots(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	activatedAt := time.Date(2026, 9, 19, 9, 0, 0, 0, time.UTC)
	actor := contract.ActorRef{Kind: "local", ID: "operator-1"}
	contracts := &contractUseCasesStub{details: contract.DraftDetails{
		Contract: contract.Contract{
			ID: contractID, NumberNamespace: "default", ContractNumber: "HT-001", Title: "已激活合同", Currency: "CNY",
			Status: contract.StatusActive, DraftVersion: 3, CurrentRevision: 1,
		},
		Revision: &contract.ContractRevision{
			ContractID: contractID, Revision: 1, SourceDraftVersion: 3, NumberNamespace: "default", ContractNumber: "HT-001",
			Title: "已激活合同", Currency: "CNY", TotalAmount: "5000", SignedOn: "2026-09-19", EffectiveOn: "2026-10-01", Actor: actor, RecordedAt: activatedAt,
		},
		PartySnapshots: []contract.ContractPartySnapshot{
			{Position: 1, LegalName: "冻结主体甲", RegistrationIdentifier: "A-1", SourcePartyRevision: 2, Role: "委托方"},
			{Position: 2, LegalName: "冻结主体乙", RegistrationIdentifier: "B-1", SourcePartyRevision: 4, Role: "受托方"},
		},
		AuditEntries: []contract.ContractAuditEntry{{
			EventType: contract.AuditEventContractActivated, Actor: actor, OccurredAt: activatedAt, SourceDraftVersion: 3, ResultingRevision: 1,
		}},
	}}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	for _, expected := range []string{"不可变 revision 1", "冻结主体甲", "冻结主体乙", contract.AuditEventContractActivated, actor.String()} {
		if !strings.Contains(body, expected) {
			t.Fatalf("active detail missing %q: %q", expected, body)
		}
	}
	if !strings.Contains(body, `action="/contracts/`+contractID+`/changes"`) || !strings.Contains(body, "登记生效补充协议") ||
		!strings.Contains(body, `action="/contracts/`+contractID+`/terminate"`) || !strings.Contains(body, "登记生效终止文书") {
		t.Fatalf("active editor page missing supplement or termination form: %q", body)
	}
	for _, forbidden := range []string{
		`action="/contracts/` + contractID + `"`,
		`action="/contracts/` + contractID + `/parties"`,
		`action="/contracts/` + contractID + `/cancel"`,
		`action="/contracts/` + contractID + `/activate"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("active page contains mutation form %q: %q", forbidden, body)
		}
	}
}

func TestAppendContractChangeUsesAuthenticatedActorAndRedirects(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	contracts := &contractUseCasesStub{details: contract.DraftDetails{
		Contract: contract.Contract{
			ID: contractID, NumberNamespace: "default", ContractNumber: "HT-001", Title: "已激活合同",
			Currency: "CNY", TotalAmount: "1000", Status: contract.StatusActive, DraftVersion: 3, CurrentRevision: 1,
		},
		Revision: &contract.ContractRevision{
			ContractID: contractID, Revision: 1, SourceDraftVersion: 3,
			SourceInstrumentID: "22222222-2222-4222-8222-222222222222",
			NumberNamespace:    "default", ContractNumber: "HT-001", Title: "已激活合同",
			Currency: "CNY", TotalAmount: "1000", SignedOn: "2026-09-19", EffectiveOn: "2026-10-01",
			Actor: defaultTestPrincipal.Actor(), RecordedAt: time.Now(),
		},
	}}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	getRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%q", getResponse.Code, getResponse.Body.String())
	}
	body := getResponse.Body.String()
	csrf := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(body)[1]
	commandID := regexp.MustCompile(`name="command_id" value="([^"]+)"`).FindAllStringSubmatch(body, -1)
	if len(commandID) == 0 {
		t.Fatalf("detail missing command ID: %q", body)
	}
	changeCommandID := regexp.MustCompile(`action="/contracts/` + contractID + `/changes"[\s\S]*?name="command_id" value="([^"]+)"`).FindStringSubmatch(body)
	if len(changeCommandID) != 2 {
		t.Fatalf("detail missing change command ID: %q", body)
	}
	var csrfCookie *http.Cookie
	for _, cookie := range getResponse.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			csrfCookie = cookie
		}
	}
	form := url.Values{
		"csrf_token": {csrf}, "command_id": {changeCommandID[1]},
		"expected_revision": {"1"}, "effective_order": {"2"},
		"instrument_number": {"补-001"}, "instrument_title": {"第一份补充协议"},
		"change_summary": {"调整合同金额"}, "instrument_signed_on": {"2026-10-02"},
		"instrument_effective_on": {"2026-10-02"}, "amount_effect": {"adjustment"}, "amount_value": {"-100.50"},
		"actor_kind": {"forged"}, "actor_id": {"forged"},
	}
	request := httptest.NewRequest(http.MethodPost, "/contracts/"+contractID+"/changes", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(csrfCookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/contracts/"+contractID+"?changed=1" {
		t.Fatalf("change status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	if contracts.lastChange.Actor != defaultTestPrincipal.Actor() || contracts.lastChange.AmountValue != "-100.50" || contracts.lastChange.EffectiveOrder != 2 {
		t.Fatalf("change input = %#v", contracts.lastChange)
	}
}

func TestTerminateContractUsesAuthenticatedActorAndRedirects(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	contracts := &contractUseCasesStub{details: contract.DraftDetails{
		Contract: contract.Contract{
			ID: contractID, NumberNamespace: "default", ContractNumber: "HT-001", Title: "已激活合同",
			Currency: "CNY", TotalAmount: "1000", Status: contract.StatusActive, DraftVersion: 3, CurrentRevision: 2,
		},
		Revision: &contract.ContractRevision{
			ContractID: contractID, Revision: 2,
			SourceInstrumentID: "22222222-2222-4222-8222-222222222222",
			NumberNamespace:    "default", ContractNumber: "HT-001", Title: "已激活合同",
			Currency: "CNY", TotalAmount: "1000", SignedOn: "2026-09-19", EffectiveOn: "2026-10-01",
			Actor: defaultTestPrincipal.Actor(), RecordedAt: time.Now(),
		},
	}}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	getRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%q", getResponse.Code, getResponse.Body.String())
	}
	body := getResponse.Body.String()
	csrf := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(body)[1]
	terminationCommandID := regexp.MustCompile(`action="/contracts/` + contractID + `/terminate"[\s\S]*?name="command_id" value="([^"]+)"`).FindStringSubmatch(body)
	if len(terminationCommandID) != 2 {
		t.Fatalf("detail missing termination command ID: %q", body)
	}
	var csrfCookie *http.Cookie
	for _, cookie := range getResponse.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			csrfCookie = cookie
		}
	}
	form := url.Values{
		"csrf_token": {csrf}, "command_id": {terminationCommandID[1]},
		"expected_revision": {"2"}, "effective_order": {"3"},
		"termination_instrument_number": {"终-001"},
		"termination_instrument_title":  {"合同终止协议"},
		"termination_reason":            {"双方确认合同终止"},
		"termination_signed_on":         {"2026-10-10"},
		"termination_effective_on":      {"2026-10-15"},
		"actor_kind":                    {"forged"}, "actor_id": {"forged"},
	}
	request := httptest.NewRequest(http.MethodPost, "/contracts/"+contractID+"/terminate", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(csrfCookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/contracts/"+contractID+"?terminated=1" {
		t.Fatalf("termination status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	if contracts.lastTerminate.Actor != defaultTestPrincipal.Actor() || contracts.lastTerminate.ExpectedRevision != 2 ||
		contracts.lastTerminate.EffectiveOrder != 3 || contracts.lastTerminate.TerminationReason != "双方确认合同终止" {
		t.Fatalf("termination input = %#v", contracts.lastTerminate)
	}
}

func TestAdministratorCorrectsCurrentRevisionWithAuthenticatedActor(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	actor := defaultTestPrincipal.Actor()
	contracts := &contractUseCasesStub{details: contract.DraftDetails{
		Contract: contract.Contract{
			ID: contractID, NumberNamespace: "default", ContractNumber: "HT-001", Title: "更正前合同",
			Category: "旧分类", Currency: "CNY", TotalAmount: "1000", SignedOn: "2026-09-19",
			EffectiveOn: "2026-10-01", Status: contract.StatusClosed, DraftVersion: 3, CurrentRevision: 3,
		},
		Revision: &contract.ContractRevision{
			ContractID: contractID, Revision: 3, SourceInstrumentID: "22222222-2222-4222-8222-222222222222",
			NumberNamespace: "default", ContractNumber: "HT-001", Title: "更正前合同", Category: "旧分类",
			Currency: "CNY", TotalAmount: "1000", SignedOn: "2026-09-19", EffectiveOn: "2026-10-01",
			Actor: actor, RecordedAt: time.Now(),
		},
		PartySnapshots: []contract.ContractPartySnapshot{
			{Position: 1, LegalName: "固定主体甲", SourcePartyRevision: 2, Role: "委托方"},
			{Position: 2, LegalName: "固定主体乙", SourcePartyRevision: 1, Role: "受托方"},
		},
	}}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	getRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	body := getResponse.Body.String()
	if getResponse.Code != http.StatusOK || !strings.Contains(body, `action="/contracts/`+contractID+`/corrections"`) ||
		!strings.Contains(body, `name="expected_status" value="closed"`) ||
		!strings.Contains(body, `name="expected_revision" value="3"`) ||
		!strings.Contains(body, `name="effective_order" value="4"`) ||
		!strings.Contains(body, `name="corrected_title" value="更正前合同"`) ||
		!strings.Contains(body, "固定主体甲") {
		t.Fatalf("administrator correction form status=%d body=%q", getResponse.Code, body)
	}
	commandMatch := regexp.MustCompile(`action="/contracts/` + contractID + `/corrections"[\s\S]*?name="command_id" value="([^"]+)"`).FindStringSubmatch(body)
	csrfMatch := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(body)
	if len(commandMatch) != 2 || len(csrfMatch) != 2 {
		t.Fatalf("correction form missing secure fields: %q", body)
	}
	var csrfCookie *http.Cookie
	for _, cookie := range getResponse.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			csrfCookie = cookie
		}
	}
	form := url.Values{
		"csrf_token": {csrfMatch[1]}, "command_id": {commandMatch[1]},
		"expected_status": {"closed"}, "expected_revision": {"3"}, "effective_order": {"4"},
		"correction_instrument_number": {"更-001"}, "correction_instrument_title": {"合同数据更正确认单"},
		"correction_reason":               {"核对原件后修正录入错误"},
		"correction_instrument_signed_on": {"2026-10-20"}, "correction_instrument_effective_on": {"2026-10-20"},
		"corrected_title": {"更正后合同"}, "corrected_category": {"新分类"}, "corrected_total_amount": {"1000.125000"},
		"corrected_signed_on": {"2026-09-20"}, "corrected_effective_on": {"2026-10-02"},
		"actor_kind": {"forged"}, "actor_id": {"forged"},
	}
	request := httptest.NewRequest(http.MethodPost, "/contracts/"+contractID+"/corrections", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(csrfCookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/contracts/"+contractID+"?corrected=1" {
		t.Fatalf("correction status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	got := contracts.lastCorrection
	if got.Actor != actor || got.ExpectedStatus != "closed" || got.ExpectedRevision != 3 || got.EffectiveOrder != 4 ||
		got.InstrumentNumber != "更-001" || got.CorrectedTitle != "更正后合同" || got.CorrectedTotalAmount != "1000.125000" {
		t.Fatalf("CorrectContract input = %#v", got)
	}
}

func TestOnlyAdministratorCanSeeOrSubmitCorrection(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	details := contract.DraftDetails{
		Contract: contract.Contract{ID: contractID, NumberNamespace: "default", ContractNumber: "HT-001", Title: "权限测试合同", Currency: "CNY", Status: contract.StatusActive, CurrentRevision: 1},
		Revision: &contract.ContractRevision{ContractID: contractID, Revision: 1, NumberNamespace: "default", ContractNumber: "HT-001", Title: "权限测试合同", Currency: "CNY", SignedOn: "2026-09-19", EffectiveOn: "2026-10-01"},
	}
	for _, role := range []identity.Role{identity.RoleEditor, identity.RoleViewer} {
		t.Run(string(role), func(t *testing.T) {
			principal := defaultTestPrincipal
			principal.Role = role
			contracts := &contractUseCasesStub{details: details}
			handler := newRawTestHandler(t, &identityUseCasesStub{principal: principal}, &partyUseCasesStub{}, contracts, Options{})
			getRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
			getRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
			getResponse := httptest.NewRecorder()
			handler.ServeHTTP(getResponse, getRequest)
			if getResponse.Code != http.StatusOK || strings.Contains(getResponse.Body.String(), "/corrections") || strings.Contains(getResponse.Body.String(), "受控更正当前 revision") {
				t.Fatalf("%s correction visibility status=%d body=%q", role, getResponse.Code, getResponse.Body.String())
			}
			postRequest := httptest.NewRequest(http.MethodPost, "/contracts/"+contractID+"/corrections", strings.NewReader(""))
			postRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
			postResponse := httptest.NewRecorder()
			handler.ServeHTTP(postResponse, postRequest)
			if postResponse.Code != http.StatusForbidden || contracts.lastCorrection.CommandID != "" {
				t.Fatalf("%s correction status=%d input=%#v", role, postResponse.Code, contracts.lastCorrection)
			}
		})
	}
	contracts := &contractUseCasesStub{details: details}
	handler := newRawTestHandler(t, &identityUseCasesStub{}, &partyUseCasesStub{}, contracts, Options{})
	request := httptest.NewRequest(http.MethodPost, "/contracts/"+contractID+"/corrections", strings.NewReader(""))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || contracts.lastCorrection.CommandID != "" {
		t.Fatalf("anonymous correction status=%d input=%#v", response.Code, contracts.lastCorrection)
	}
}

func TestAdministratorCanDownloadContractLedgerArchive(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	content := []byte("PK\x03\x04test-ledger-archive")
	exports := &ledgerExportUseCasesStub{artifact: ledgerexport.Artifact{
		Filename: "contract-" + contractID + "-ledger.zip",
		Content:  content,
	}}
	handler := newLedgerExportTestHandler(t, defaultTestPrincipal, &contractUseCasesStub{}, exports)
	request := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID+"/export", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), content) {
		t.Fatalf("export status=%d body=%q", response.Code, response.Body.Bytes())
	}
	if response.Header().Get("Content-Type") != "application/zip" ||
		response.Header().Get("Cache-Control") != "no-store" ||
		response.Header().Get("X-Content-Type-Options") != "nosniff" ||
		response.Header().Get("Content-Length") != strconv.Itoa(len(content)) ||
		!strings.Contains(response.Header().Get("Content-Disposition"), `filename=contract-`+contractID+`-ledger.zip`) {
		t.Fatalf("export headers = %#v", response.Header())
	}
	if exports.last.ContractID != contractID || exports.last.Actor != defaultTestPrincipal.Actor() {
		t.Fatalf("export input = %#v", exports.last)
	}
}

func TestOnlyAdministratorCanSeeOrDownloadContractLedger(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	details := contract.DraftDetails{Contract: contract.Contract{
		ID: contractID, NumberNamespace: "default", ContractNumber: "HT-001", Title: "导出权限合同",
		Currency: "CNY", Status: contract.StatusDraft, DraftVersion: 1,
	}}

	adminHandler, err := New(readinessStub{}, &partyUseCasesStub{}, &contractUseCasesStub{details: details})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	adminPage := httptest.NewRecorder()
	adminHandler.ServeHTTP(adminPage, httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil))
	if adminPage.Code != http.StatusOK || !strings.Contains(adminPage.Body.String(), "/contracts/"+contractID+"/export") ||
		!strings.Contains(adminPage.Body.String(), "导出可验证账簿") {
		t.Fatalf("administrator export visibility status=%d body=%q", adminPage.Code, adminPage.Body.String())
	}

	for _, role := range []identity.Role{identity.RoleEditor, identity.RoleViewer} {
		t.Run(string(role), func(t *testing.T) {
			principal := defaultTestPrincipal
			principal.Role = role
			exports := &ledgerExportUseCasesStub{}
			handler := newLedgerExportTestHandler(t, principal, &contractUseCasesStub{details: details}, exports)

			pageRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
			pageRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
			pageResponse := httptest.NewRecorder()
			handler.ServeHTTP(pageResponse, pageRequest)
			if pageResponse.Code != http.StatusOK || strings.Contains(pageResponse.Body.String(), "/contracts/"+contractID+"/export") {
				t.Fatalf("%s export visibility status=%d body=%q", role, pageResponse.Code, pageResponse.Body.String())
			}

			downloadRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID+"/export", nil)
			downloadRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
			downloadResponse := httptest.NewRecorder()
			handler.ServeHTTP(downloadResponse, downloadRequest)
			if downloadResponse.Code != http.StatusForbidden || exports.last.ContractID != "" {
				t.Fatalf("%s export status=%d input=%#v", role, downloadResponse.Code, exports.last)
			}
		})
	}

	exports := &ledgerExportUseCasesStub{}
	anonymousHandler := newLedgerExportTestHandler(t, defaultTestPrincipal, &contractUseCasesStub{}, exports)
	anonymousRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID+"/export", nil)
	anonymousResponse := httptest.NewRecorder()
	anonymousHandler.ServeHTTP(anonymousResponse, anonymousRequest)
	if anonymousResponse.Code != http.StatusSeeOther || exports.last.ContractID != "" ||
		!strings.HasPrefix(anonymousResponse.Header().Get("Location"), "/login?next=") {
		t.Fatalf("anonymous export status=%d location=%q input=%#v", anonymousResponse.Code, anonymousResponse.Header().Get("Location"), exports.last)
	}
}

func TestContractLedgerExportErrorsDoNotSendArchive(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "not found", err: &ledgerexport.NotFoundError{ContractID: contractID}, status: http.StatusNotFound},
		{name: "invalid ID", err: &ledgerexport.ValidationError{Field: "contract_id", Message: "bad"}, status: http.StatusNotFound},
		{name: "audit failure", err: errors.New("audit insert failed"), status: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exports := &ledgerExportUseCasesStub{err: test.err}
			handler := newLedgerExportTestHandler(t, defaultTestPrincipal, &contractUseCasesStub{}, exports)
			request := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID+"/export", nil)
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || response.Header().Get("Content-Type") == "application/zip" ||
				bytes.HasPrefix(response.Body.Bytes(), []byte("PK")) {
				t.Fatalf("export error status=%d headers=%#v body=%q", response.Code, response.Header(), response.Body.Bytes())
			}
		})
	}
}

func TestAdministratorCanUploadSingleContractRestoreArchive(t *testing.T) {
	restores := &ledgerRestoreUseCasesStub{result: ledgerrestore.Result{
		ContractID: "11111111-1111-4111-8111-111111111111", Status: "closed",
		CurrentRevision: 4, MissingEvidenceVersions: 2,
	}}
	handler := newLedgerRestoreTestHandler(t, defaultTestPrincipal, restores)
	getRequest := httptest.NewRequest(http.MethodGet, "/admin/restore", nil)
	getRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	body := getResponse.Body.String()
	if getResponse.Code != http.StatusOK || !strings.Contains(body, "恢复单合同账簿") ||
		!strings.Contains(body, "目标库不得已有任何合同或主体") {
		t.Fatalf("GET restore status=%d body=%q", getResponse.Code, body)
	}
	csrf := unitFormHiddenValue(t, body, "/admin/restore", "csrf_token")
	commandID := unitFormHiddenValue(t, body, "/admin/restore", "command_id")
	var csrfCookie *http.Cookie
	for _, cookie := range getResponse.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			csrfCookie = cookie
		}
	}
	if csrfCookie == nil {
		t.Fatal("restore GET missing CSRF cookie")
	}

	request := newUnitMultipartRequest(t, "/admin/restore", url.Values{
		"csrf_token": {csrf}, "command_id": {commandID}, "confirm_empty": {"yes"},
	}, "archive", "contract-ledger.zip", []byte("archive-bytes"))
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
	request.AddCookie(csrfCookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/contracts/11111111-1111-4111-8111-111111111111?missing_evidence=2&restored=1" {
		t.Fatalf("POST restore status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	if restores.calls != 1 || restores.last.CommandID != commandID || string(restores.last.Archive) != "archive-bytes" || restores.last.Actor != defaultTestPrincipal.Actor() {
		t.Fatalf("restore calls=%d input=%#v", restores.calls, restores.last)
	}
}

func TestRestoreRequiresAdministratorAndCSRF(t *testing.T) {
	for _, role := range []identity.Role{identity.RoleEditor, identity.RoleViewer} {
		principal := defaultTestPrincipal
		principal.Role = role
		restores := &ledgerRestoreUseCasesStub{}
		handler := newLedgerRestoreTestHandler(t, principal, restores)
		request := httptest.NewRequest(http.MethodGet, "/admin/restore", nil)
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden || restores.calls != 0 {
			t.Fatalf("role=%s status=%d calls=%d", role, response.Code, restores.calls)
		}
	}

	restores := &ledgerRestoreUseCasesStub{}
	handler := newLedgerRestoreTestHandler(t, defaultTestPrincipal, restores)
	request := newUnitMultipartRequest(t, "/admin/restore", url.Values{
		"command_id": {"11111111-1111-4111-8111-111111111111"}, "confirm_empty": {"yes"},
	}, "archive", "ledger.zip", []byte("archive"))
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || restores.calls != 0 {
		t.Fatalf("missing CSRF status=%d calls=%d", response.Code, restores.calls)
	}
}

func TestRestoreHTTPErrorMapping(t *testing.T) {
	tests := []struct {
		err    error
		status int
	}{
		{err: &ledgerrestore.ValidationError{Message: "invalid archive"}, status: http.StatusUnprocessableEntity},
		{err: &ledgerrestore.UnsupportedRelationsError{}, status: http.StatusUnprocessableEntity},
		{err: &ledgerrestore.TooLargeError{}, status: http.StatusRequestEntityTooLarge},
		{err: &ledgerrestore.DestinationNotEmptyError{}, status: http.StatusConflict},
		{err: &ledgerrestore.DestinationConflictError{}, status: http.StatusConflict},
		{err: &ledgerrestore.IdempotencyConflictError{}, status: http.StatusConflict},
		{err: errors.New("database detail"), status: http.StatusInternalServerError},
	}
	for _, test := range tests {
		status, message := restoreHTTPError(test.err)
		if status != test.status || message == "" || (status == http.StatusInternalServerError && strings.Contains(message, "database detail")) {
			t.Fatalf("restoreHTTPError(%T) = %d %q", test.err, status, message)
		}
	}
}

func TestRecordSettlementUsesAuthenticatedActorAndSelectedParties(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	actor := defaultTestPrincipal.Actor()
	contracts := &contractUseCasesStub{details: contract.DraftDetails{
		Contract: contract.Contract{
			ID: contractID, NumberNamespace: "default", ContractNumber: "HT-001", Title: "待结算合同",
			Currency: "CNY", TotalAmount: "1500", Status: contract.StatusTerminated, DraftVersion: 3, CurrentRevision: 2,
		},
		Revision: &contract.ContractRevision{
			ContractID: contractID, Revision: 2, SourceInstrumentID: "22222222-2222-4222-8222-222222222222",
			NumberNamespace: "default", ContractNumber: "HT-001", Title: "待结算合同", Currency: "CNY", TotalAmount: "1500",
			SignedOn: "2026-09-19", EffectiveOn: "2026-10-01", Actor: actor, RecordedAt: time.Now(),
		},
		Revisions: []contract.ContractRevision{
			{ContractID: contractID, Revision: 1, Currency: "CNY", TotalAmount: "1000"},
			{ContractID: contractID, Revision: 2, Currency: "CNY", TotalAmount: "1500"},
		},
		PartySnapshots: []contract.ContractPartySnapshot{
			{Position: 1, LegalName: "确认主体甲", SourcePartyRevision: 2, Role: "委托方"},
			{Position: 2, LegalName: "确认主体乙", SourcePartyRevision: 4, Role: "受托方"},
		},
		Settlements: []contract.ContractSettlement{{
			ID: "33333333-3333-4333-8333-333333333333", ContractID: contractID,
			SourceRevision: 1, Sequence: 1, Kind: contract.SettlementKindProgress,
			StatementNumber: "结-001", Title: "第一阶段结算", Amount: "800", Currency: "CNY",
			ConfirmedOn: "2026-10-10", Basis: "第一阶段验收", Actor: actor, RecordedAt: time.Now(),
		}},
	}}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	getRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%q", getResponse.Code, getResponse.Body.String())
	}
	body := getResponse.Body.String()
	for _, expected := range []string{"原始合同金额", "1000 CNY", "当前有效金额", "1500 CNY", "最新累计结算金额", "800 CNY", "第一阶段结算", "确认主体甲"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("settlement detail missing %q: %q", expected, body)
		}
	}
	csrf := unitFormHiddenValue(t, body, "/contracts/"+contractID+"/settlements", "csrf_token")
	commandID := unitFormHiddenValue(t, body, "/contracts/"+contractID+"/settlements", "command_id")
	var csrfCookie *http.Cookie
	for _, cookie := range getResponse.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			csrfCookie = cookie
		}
	}
	form := url.Values{
		"csrf_token": {csrf}, "command_id": {commandID},
		"expected_revision": {"2"}, "expected_settlement_sequence": {"1"},
		"settlement_kind": {"final"}, "settlement_number": {"结-002"},
		"settlement_title": {"最终结算"}, "settlement_amount": {"1200.500000"},
		"settlement_confirmed_on": {"2026-11-05"}, "settlement_basis": {"双方最终确认"},
		"confirming_party_position": {"2", "1"},
		"actor_kind":                {"forged"}, "actor_id": {"forged"},
	}
	request := httptest.NewRequest(http.MethodPost, "/contracts/"+contractID+"/settlements", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(csrfCookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/contracts/"+contractID+"?settlement_recorded=1" {
		t.Fatalf("settlement status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	input := contracts.lastSettlement
	if input.Actor != actor || input.ExpectedContractRevision != 2 || input.ExpectedSettlementSequence != 1 ||
		input.Kind != "final" || input.Amount != "1200.500000" || len(input.ConfirmingPartyPositions) != 2 ||
		input.ConfirmingPartyPositions[0] != 2 || input.ConfirmingPartyPositions[1] != 1 {
		t.Fatalf("settlement input = %#v", input)
	}
}

func TestFinalSettlementHidesSettlementForm(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	contracts := &contractUseCasesStub{details: contract.DraftDetails{
		Contract:    contract.Contract{ID: contractID, Title: "已最终结算合同", Currency: "CNY", Status: contract.StatusTerminated, CurrentRevision: 1},
		Revision:    &contract.ContractRevision{ContractID: contractID, Revision: 1, Currency: "CNY", Actor: defaultTestPrincipal.Actor()},
		Settlements: []contract.ContractSettlement{{Sequence: 1, Kind: contract.SettlementKindFinal, StatementNumber: "终结-001", Title: "最终结算", Amount: "0", Currency: "CNY", Actor: defaultTestPrincipal.Actor()}},
	}}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "最终结算") ||
		strings.Contains(response.Body.String(), `action="/contracts/`+contractID+`/settlements"`) ||
		!strings.Contains(response.Body.String(), `action="/contracts/`+contractID+`/close"`) {
		t.Fatalf("final settlement detail status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestCloseContractUsesFinalSettlementAndAuthenticatedActor(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	actor := defaultTestPrincipal.Actor()
	contracts := &contractUseCasesStub{details: contract.DraftDetails{
		Contract: contract.Contract{
			ID: contractID, NumberNamespace: "default", ContractNumber: "HT-001", Title: "待关闭合同",
			Currency: "CNY", TotalAmount: "1000", Status: contract.StatusTerminated, CurrentRevision: 3,
		},
		Revision: &contract.ContractRevision{
			ContractID: contractID, Revision: 3, SourceInstrumentID: "22222222-2222-4222-8222-222222222222",
			Currency: "CNY", TotalAmount: "1000", Actor: actor, RecordedAt: time.Now(),
		},
		Settlements: []contract.ContractSettlement{{
			ID: "33333333-3333-4333-8333-333333333333", ContractID: contractID,
			SourceRevision: 3, Sequence: 2, Kind: contract.SettlementKindFinal,
			StatementNumber: "FINAL-001", Title: "最终结算", Amount: "950", Currency: "CNY",
			ConfirmedOn: "2026-11-05", Actor: actor, RecordedAt: time.Now(),
		}},
	}}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	getRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%q", getResponse.Code, getResponse.Body.String())
	}
	body := getResponse.Body.String()
	closePath := "/contracts/" + contractID + "/close"
	csrf := unitFormHiddenValue(t, body, closePath, "csrf_token")
	commandID := unitFormHiddenValue(t, body, closePath, "command_id")
	if disposition := unitFormHiddenValue(t, body, closePath, "settlement_disposition"); disposition != "final_recorded" {
		t.Fatalf("closure disposition = %q", disposition)
	}
	var csrfCookie *http.Cookie
	for _, cookie := range getResponse.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			csrfCookie = cookie
		}
	}
	form := url.Values{
		"csrf_token": {csrf}, "command_id": {commandID},
		"expected_revision": {"3"}, "expected_settlement_sequence": {"2"},
		"settlement_disposition": {"final_recorded"}, "closed_on": {"2026-11-10"},
		"closure_reason": {"最终结算已确认，常规维护结束"},
		"actor_id":       {"forged"},
	}
	request := httptest.NewRequest(http.MethodPost, closePath, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(csrfCookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/contracts/"+contractID+"?closed=1" {
		t.Fatalf("close status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	input := contracts.lastClosure
	if input.Actor != actor || input.ExpectedContractRevision != 3 || input.ExpectedSettlementSequence != 2 ||
		input.SettlementDisposition != "final_recorded" || input.ClosedOn != "2026-11-10" {
		t.Fatalf("closure input = %#v", input)
	}
}

func TestActiveContractCanExplicitlyCloseWithoutFinalSettlement(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	contracts := &contractUseCasesStub{details: contract.DraftDetails{
		Contract: contract.Contract{ID: contractID, Title: "无需结算合同", Currency: "CNY", Status: contract.StatusActive, CurrentRevision: 1},
		Revision: &contract.ContractRevision{ContractID: contractID, Revision: 1, Currency: "CNY", Actor: defaultTestPrincipal.Actor()},
	}}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "我确认该合同无需再登记独立的最终结算事实") ||
		!strings.Contains(body, `name="settlement_disposition" value="not_required"`) {
		t.Fatalf("active closure form status=%d body=%q", response.Code, body)
	}
}

func TestClosedContractShowsClosureAndHidesRoutineWriteForms(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	actor := defaultTestPrincipal.Actor()
	contracts := &contractUseCasesStub{details: contract.DraftDetails{
		Contract: contract.Contract{ID: contractID, Title: "已关闭合同", Currency: "CNY", Status: contract.StatusClosed, CurrentRevision: 2},
		Revision: &contract.ContractRevision{ContractID: contractID, Revision: 2, Currency: "CNY", Actor: actor},
		Closure: &contract.ContractClosure{
			ID: "99999999-9999-4999-8999-999999999999", ContractID: contractID,
			SourceRevision: 2, SourceSettlementSequence: 1,
			SettlementDisposition: contract.SettlementDispositionNotRequired,
			ClosedOn:              "2026-11-10", Reason: "常规合同维护结束", Actor: actor, RecordedAt: time.Now(),
		},
	}}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID+"?closed=1", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "合同已完成操作性关闭") ||
		!strings.Contains(body, "常规合同维护结束") || !strings.Contains(body, "无需单独最终结算") {
		t.Fatalf("closed detail status=%d body=%q", response.Code, body)
	}
	for _, action := range []string{"/changes", "/terminate", "/settlements", "/close"} {
		if strings.Contains(body, `action="/contracts/`+contractID+action+`"`) {
			t.Fatalf("closed detail contains routine write form %q: %q", action, body)
		}
	}
}

func TestContractEvidenceUploadVersionAndDownloadUseAuthenticatedActor(t *testing.T) {
	const (
		contractID = "11111111-1111-4111-8111-111111111111"
		closureID  = "22222222-2222-4222-8222-222222222222"
		documentID = "33333333-3333-4333-8333-333333333333"
		versionID  = "44444444-4444-4444-8444-444444444444"
	)
	principal := defaultTestPrincipal
	principal.DisplayName = "可信证据操作者"
	actor := principal.Actor()
	contracts := &contractUseCasesStub{details: contract.DraftDetails{
		Contract: contract.Contract{
			ID: contractID, ContractNumber: "EVIDENCE-001", Title: "证据测试合同",
			Currency: "CNY", Status: contract.StatusClosed, CurrentRevision: 1,
		},
		Revision: &contract.ContractRevision{ContractID: contractID, Revision: 1, Currency: "CNY", Actor: actor},
		Closure: &contract.ContractClosure{
			ID: closureID, ContractID: contractID, SourceRevision: 1,
			SettlementDisposition: contract.SettlementDispositionNotRequired,
			ClosedOn:              "2026-09-19", Reason: "证据测试关闭", Actor: actor,
		},
	}}
	downloadBody := []byte("<script>download only</script>")
	evidenceCases := &evidenceUseCasesStub{
		maxBytes: 1024,
		documents: []evidence.Document{{
			ID: documentID, ContractID: contractID, TargetType: evidence.TargetClosure, TargetID: closureID,
			Title: "关闭证明", Purpose: "证明关闭依据", Actor: actor, Latest: 1, TargetLabel: "操作性关闭 / 2026-09-19",
			Versions: []evidence.Version{{
				ID: versionID, DocumentID: documentID, Version: 1, OriginalFilename: "closure-proof.html",
				DetectedMediaType: "text/html; charset=utf-8", SizeBytes: int64(len(downloadBody)),
				SHA256: strings.Repeat("a", 64), Actor: actor,
			}},
		}},
		download: evidence.Download{
			Version: evidence.Version{
				ID: versionID, DocumentID: documentID, Version: 1,
				OriginalFilename: "closure-proof.html", SizeBytes: int64(len(downloadBody)),
			},
			Content: testReadSeekCloser{Reader: bytes.NewReader(downloadBody)},
		},
	}
	handler := newEvidenceTestHandler(t, principal, contracts, evidenceCases)

	detailRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	detailResponse := httptest.NewRecorder()
	handler.ServeHTTP(detailResponse, detailRequest)
	page := detailResponse.Body.String()
	createPath := "/contracts/" + contractID + "/evidence"
	versionPath := createPath + "/" + documentID + "/versions"
	if detailResponse.Code != http.StatusOK || !strings.Contains(page, "关闭证明") ||
		!strings.Contains(page, "closure-proof.html") || !strings.Contains(page, "操作性关闭 / 2026-09-19") ||
		!strings.Contains(page, `action="`+createPath+`"`) || !strings.Contains(page, `action="`+versionPath+`"`) {
		t.Fatalf("evidence detail status=%d body=%q", detailResponse.Code, page)
	}
	var csrfCookie *http.Cookie
	for _, cookie := range detailResponse.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			csrfCookie = cookie
		}
	}
	if csrfCookie == nil {
		t.Fatal("evidence detail did not set CSRF cookie")
	}

	createContent := []byte("first evidence bytes")
	createRequest := newUnitMultipartRequest(t, createPath, url.Values{
		"csrf_token":       {unitFormHiddenValue(t, page, createPath, "csrf_token")},
		"command_id":       {unitFormHiddenValue(t, page, createPath, "command_id")},
		"evidence_target":  {"closure:" + closureID},
		"evidence_title":   {"新关闭证明"},
		"evidence_purpose": {"补充证明材料"},
		"actor_kind":       {"forged"},
		"actor_id":         {"forged"},
	}, "evidence_file", "new-proof.pdf", createContent)
	createRequest.AddCookie(csrfCookie)
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusSeeOther || createResponse.Header().Get("Location") != createPath[:len(createPath)-len("/evidence")]+"?evidence_uploaded=1" {
		t.Fatalf("create evidence status=%d location=%q body=%q", createResponse.Code, createResponse.Header().Get("Location"), createResponse.Body.String())
	}
	if evidenceCases.lastCreate.ContractID != contractID || evidenceCases.lastCreate.TargetType != "closure" ||
		evidenceCases.lastCreate.TargetID != closureID || evidenceCases.lastCreate.Title != "新关闭证明" ||
		evidenceCases.lastCreate.Purpose != "补充证明材料" || evidenceCases.lastCreate.OriginalFilename != "new-proof.pdf" ||
		evidenceCases.lastCreate.Actor != actor || !bytes.Equal(evidenceCases.lastContent, createContent) {
		t.Fatalf("create evidence input=%#v content=%q", evidenceCases.lastCreate, evidenceCases.lastContent)
	}

	versionContent := []byte("second evidence bytes")
	versionRequest := newUnitMultipartRequest(t, versionPath, url.Values{
		"csrf_token":              {unitFormHiddenValue(t, page, versionPath, "csrf_token")},
		"command_id":              {unitFormHiddenValue(t, page, versionPath, "command_id")},
		"expected_latest_version": {unitFormHiddenValue(t, page, versionPath, "expected_latest_version")},
	}, "evidence_file", "new-proof-v2.pdf", versionContent)
	versionRequest.AddCookie(csrfCookie)
	versionResponse := httptest.NewRecorder()
	handler.ServeHTTP(versionResponse, versionRequest)
	if versionResponse.Code != http.StatusSeeOther {
		t.Fatalf("add evidence version status=%d body=%q", versionResponse.Code, versionResponse.Body.String())
	}
	if evidenceCases.lastAdd.ContractID != contractID || evidenceCases.lastAdd.DocumentID != documentID ||
		evidenceCases.lastAdd.ExpectedLatestVersion != 1 || evidenceCases.lastAdd.OriginalFilename != "new-proof-v2.pdf" ||
		evidenceCases.lastAdd.Actor != actor || !bytes.Equal(evidenceCases.lastContent, versionContent) {
		t.Fatalf("add evidence version input=%#v content=%q", evidenceCases.lastAdd, evidenceCases.lastContent)
	}

	downloadPath := versionPath + "/" + versionID + "/download"
	downloadRequest := httptest.NewRequest(http.MethodGet, downloadPath, nil)
	downloadResponse := httptest.NewRecorder()
	handler.ServeHTTP(downloadResponse, downloadRequest)
	if downloadResponse.Code != http.StatusOK || downloadResponse.Header().Get("Content-Type") != "application/octet-stream" ||
		!strings.Contains(downloadResponse.Header().Get("Content-Disposition"), "attachment") ||
		!strings.Contains(downloadResponse.Header().Get("Content-Disposition"), "closure-proof.html") ||
		downloadResponse.Header().Get("Cache-Control") != "private, no-store" ||
		downloadResponse.Header().Get("X-Content-Type-Options") != "nosniff" ||
		!bytes.Equal(downloadResponse.Body.Bytes(), downloadBody) {
		t.Fatalf("download status=%d headers=%v body=%q", downloadResponse.Code, downloadResponse.Header(), downloadResponse.Body.Bytes())
	}
	if evidenceCases.lastDownload.ContractID != contractID || evidenceCases.lastDownload.DocumentID != documentID ||
		evidenceCases.lastDownload.VersionID != versionID || evidenceCases.lastDownload.Actor != actor {
		t.Fatalf("download input=%#v", evidenceCases.lastDownload)
	}
}

func TestViewerCanReadAndDownloadEvidenceButCannotUpload(t *testing.T) {
	const (
		contractID = "11111111-1111-4111-8111-111111111111"
		documentID = "33333333-3333-4333-8333-333333333333"
		versionID  = "44444444-4444-4444-8444-444444444444"
	)
	viewer := defaultTestPrincipal
	viewer.Role = identity.RoleViewer
	contracts := &contractUseCasesStub{details: contract.DraftDetails{Contract: contract.Contract{
		ID: contractID, ContractNumber: "VIEW-001", Title: "只读证据合同", Currency: "CNY", Status: contract.StatusActive,
	}}}
	content := []byte("viewer download")
	evidenceCases := &evidenceUseCasesStub{
		documents: []evidence.Document{{
			ID: documentID, ContractID: contractID, Title: "只读证据", Purpose: "供查看者下载", Latest: 1,
			Versions: []evidence.Version{{ID: versionID, DocumentID: documentID, Version: 1, OriginalFilename: "viewer.txt", SizeBytes: int64(len(content))}},
		}},
		download: evidence.Download{
			Version: evidence.Version{ID: versionID, DocumentID: documentID, Version: 1, OriginalFilename: "viewer.txt", SizeBytes: int64(len(content))},
			Content: testReadSeekCloser{Reader: bytes.NewReader(content)},
		},
	}
	handler := newEvidenceTestHandler(t, viewer, contracts, evidenceCases)
	createPath := "/contracts/" + contractID + "/evidence"
	versionPath := createPath + "/" + documentID + "/versions"

	pageRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	pageResponse := httptest.NewRecorder()
	handler.ServeHTTP(pageResponse, pageRequest)
	page := pageResponse.Body.String()
	if pageResponse.Code != http.StatusOK || !strings.Contains(page, "只读证据") || !strings.Contains(page, "viewer.txt") ||
		strings.Contains(page, `action="`+createPath+`"`) || strings.Contains(page, `action="`+versionPath+`"`) {
		t.Fatalf("viewer evidence page status=%d body=%q", pageResponse.Code, page)
	}

	uploadRequest := httptest.NewRequest(http.MethodPost, createPath, strings.NewReader("evidence_title=forged"))
	uploadRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	uploadResponse := httptest.NewRecorder()
	handler.ServeHTTP(uploadResponse, uploadRequest)
	if uploadResponse.Code != http.StatusForbidden || evidenceCases.lastCreate.ContractID != "" {
		t.Fatalf("viewer upload status=%d input=%#v", uploadResponse.Code, evidenceCases.lastCreate)
	}

	downloadRequest := httptest.NewRequest(http.MethodGet, versionPath+"/"+versionID+"/download", nil)
	downloadResponse := httptest.NewRecorder()
	handler.ServeHTTP(downloadResponse, downloadRequest)
	if downloadResponse.Code != http.StatusOK || !bytes.Equal(downloadResponse.Body.Bytes(), content) ||
		evidenceCases.lastDownload.Actor != viewer.Actor() {
		t.Fatalf("viewer download status=%d input=%#v body=%q", downloadResponse.Code, evidenceCases.lastDownload, downloadResponse.Body.Bytes())
	}
}

func TestEvidenceHTTPErrorMapping(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "validation", err: &evidence.ValidationError{}, status: http.StatusUnprocessableEntity},
		{name: "target", err: &evidence.TargetConflictError{}, status: http.StatusUnprocessableEntity},
		{name: "contract not found", err: &evidence.ContractNotFoundError{}, status: http.StatusNotFound},
		{name: "document not found", err: &evidence.DocumentNotFoundError{}, status: http.StatusNotFound},
		{name: "version not found", err: &evidence.VersionNotFoundError{}, status: http.StatusNotFound},
		{name: "version conflict", err: &evidence.VersionConflictError{}, status: http.StatusConflict},
		{name: "idempotency conflict", err: &evidence.IdempotencyConflictError{}, status: http.StatusConflict},
		{name: "empty file", err: &evidence.EmptyFileError{}, status: http.StatusUnprocessableEntity},
		{name: "too large", err: &evidence.FileTooLargeError{}, status: http.StatusRequestEntityTooLarge},
		{name: "integrity", err: &evidence.IntegrityError{}, status: http.StatusServiceUnavailable},
		{name: "unavailable", err: &evidence.UnavailableError{}, status: http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, _, known := evidenceHTTPError(test.err)
			if !known || status != test.status {
				t.Fatalf("evidenceHTTPError(%T) status=%d known=%v", test.err, status, known)
			}
		})
	}
}

func TestSettlementHTTPErrorMapping(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "validation", err: &contract.ValidationError{Field: "settlement_amount", Message: "invalid"}, status: http.StatusUnprocessableEntity},
		{name: "party", err: &contract.ContractSettlementPartyError{Message: "invalid party"}, status: http.StatusUnprocessableEntity},
		{name: "not found", err: &contract.ContractNotFoundError{}, status: http.StatusNotFound},
		{name: "state", err: &contract.ContractSettlementStateConflictError{Status: contract.StatusDraft}, status: http.StatusConflict},
		{name: "revision", err: &contract.ContractRevisionConflictError{Expected: 1, Actual: 2}, status: http.StatusConflict},
		{name: "sequence", err: &contract.ContractSettlementSequenceConflictError{Expected: 1, Actual: 2}, status: http.StatusConflict},
		{name: "final", err: &contract.ContractSettlementFinalizedError{}, status: http.StatusConflict},
		{name: "number", err: &contract.DuplicateSettlementNumberError{}, status: http.StatusConflict},
		{name: "idempotency", err: &contract.IdempotencyConflictError{}, status: http.StatusConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, message, known := settlementHTTPError(test.err)
			if !known || status != test.status || message == "" {
				t.Fatalf("settlementHTTPError(%T) = %d %q %v", test.err, status, message, known)
			}
		})
	}
}

func TestClosureHTTPErrorMapping(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "validation", err: &contract.ValidationError{Field: "closed_on", Message: "invalid"}, status: http.StatusUnprocessableEntity},
		{name: "date", err: &contract.ContractClosurePreconditionError{Field: "closed_on", Message: "too early"}, status: http.StatusUnprocessableEntity},
		{name: "not found", err: &contract.ContractNotFoundError{}, status: http.StatusNotFound},
		{name: "state", err: &contract.ContractClosureStateConflictError{Status: contract.StatusClosed}, status: http.StatusConflict},
		{name: "revision", err: &contract.ContractRevisionConflictError{Expected: 1, Actual: 2}, status: http.StatusConflict},
		{name: "sequence", err: &contract.ContractSettlementSequenceConflictError{Expected: 1, Actual: 2}, status: http.StatusConflict},
		{name: "settlement", err: &contract.ContractClosureSettlementConflictError{Message: "mismatch"}, status: http.StatusConflict},
		{name: "idempotency", err: &contract.IdempotencyConflictError{}, status: http.StatusConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, message, known := closureHTTPError(test.err)
			if !known || status != test.status || message == "" {
				t.Fatalf("closureHTTPError(%T) = %d %q %v", test.err, status, message, known)
			}
		})
	}
}

func TestCorrectionHTTPErrorMapping(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "validation", err: &contract.ValidationError{Field: "corrected_title", Message: "invalid"}, status: http.StatusUnprocessableEntity},
		{name: "precondition", err: &contract.ContractCorrectionPreconditionError{Field: "effective_order", Message: "invalid order"}, status: http.StatusUnprocessableEntity},
		{name: "no changes", err: &contract.ContractCorrectionNoChangesError{}, status: http.StatusUnprocessableEntity},
		{name: "not found", err: &contract.ContractNotFoundError{}, status: http.StatusNotFound},
		{name: "state", err: &contract.ContractCorrectionStateConflictError{Expected: contract.StatusActive, Actual: contract.StatusClosed}, status: http.StatusConflict},
		{name: "revision", err: &contract.ContractRevisionConflictError{Expected: 1, Actual: 2}, status: http.StatusConflict},
		{name: "number", err: &contract.DuplicateInstrumentNumberError{}, status: http.StatusConflict},
		{name: "idempotency", err: &contract.IdempotencyConflictError{}, status: http.StatusConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, message, known := correctionHTTPError(test.err)
			if !known || status != test.status || message == "" {
				t.Fatalf("correctionHTTPError(%T) = %d %q %v", test.err, status, message, known)
			}
		})
	}
}

func TestTerminatedContractDetailIsReadOnly(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	terminatedAt := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	actor := defaultTestPrincipal.Actor()
	contracts := &contractUseCasesStub{details: contract.DraftDetails{
		Contract: contract.Contract{
			ID: contractID, NumberNamespace: "default", ContractNumber: "HT-001", Title: "已终止合同",
			Currency: "CNY", TotalAmount: "1000", Status: contract.StatusTerminated, DraftVersion: 3, CurrentRevision: 2,
		},
		Revision: &contract.ContractRevision{
			ContractID: contractID, Revision: 2, SourceInstrumentID: "22222222-2222-4222-8222-222222222222",
			NumberNamespace: "default", ContractNumber: "HT-001", Title: "已终止合同", Currency: "CNY", TotalAmount: "1000",
			SignedOn: "2026-09-19", EffectiveOn: "2026-10-01", Actor: actor, RecordedAt: terminatedAt,
		},
		Instruments: []contract.ContractInstrument{{
			ID: "22222222-2222-4222-8222-222222222222", ContractID: contractID,
			InstrumentType: contract.InstrumentTypeTermination, InstrumentNumber: "终-001", Title: "合同终止协议",
			ChangeSummary: "双方确认合同终止", SignedOn: "2026-10-10", EffectiveOn: "2026-10-15",
			EffectiveOrder: 2, AmountEffect: contract.AmountEffectUnchanged, ResultingRevision: 2, Actor: actor,
		}},
	}}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "已终止") || !strings.Contains(body, "终止文书 / 终-001") {
		t.Fatalf("terminated detail status=%d body=%q", response.Code, body)
	}
	for _, forbidden := range []string{
		`action="/contracts/` + contractID + `/changes"`,
		`action="/contracts/` + contractID + `/terminate"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("terminated detail contains write form %q: %q", forbidden, body)
		}
	}
}

func TestViewerCannotSeeOrSubmitContractChange(t *testing.T) {
	const contractID = "11111111-1111-4111-8111-111111111111"
	viewer := defaultTestPrincipal
	viewer.Role = identity.RoleViewer
	contracts := &contractUseCasesStub{details: contract.DraftDetails{
		Contract: contract.Contract{
			ID: contractID, NumberNamespace: "default", ContractNumber: "HT-001", Title: "已激活合同",
			Currency: "CNY", Status: contract.StatusActive, CurrentRevision: 1,
		},
		Revision: &contract.ContractRevision{
			ContractID: contractID, Revision: 1, SourceInstrumentID: "22222222-2222-4222-8222-222222222222",
			NumberNamespace: "default", ContractNumber: "HT-001", Title: "已激活合同", Currency: "CNY",
			SignedOn: "2026-09-19", EffectiveOn: "2026-10-01", Actor: viewer.Actor(), RecordedAt: time.Now(),
		},
	}}
	handler := newRawTestHandler(t, &identityUseCasesStub{principal: viewer}, &partyUseCasesStub{}, contracts, Options{})
	getRequest := httptest.NewRequest(http.MethodGet, "/contracts/"+contractID, nil)
	getRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK || strings.Contains(getResponse.Body.String(), "登记生效补充协议") || strings.Contains(getResponse.Body.String(), "登记生效终止文书") {
		t.Fatalf("viewer detail status=%d body=%q", getResponse.Code, getResponse.Body.String())
	}
	postRequest := httptest.NewRequest(http.MethodPost, "/contracts/"+contractID+"/changes", strings.NewReader(""))
	postRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
	postResponse := httptest.NewRecorder()
	handler.ServeHTTP(postResponse, postRequest)
	if postResponse.Code != http.StatusForbidden || contracts.lastChange.CommandID != "" {
		t.Fatalf("viewer change status=%d input=%#v", postResponse.Code, contracts.lastChange)
	}
	terminationRequest := httptest.NewRequest(http.MethodPost, "/contracts/"+contractID+"/terminate", strings.NewReader(""))
	terminationRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
	terminationResponse := httptest.NewRecorder()
	handler.ServeHTTP(terminationResponse, terminationRequest)
	if terminationResponse.Code != http.StatusForbidden || contracts.lastTerminate.CommandID != "" {
		t.Fatalf("viewer termination status=%d input=%#v", terminationResponse.Code, contracts.lastTerminate)
	}
	settlementRequest := httptest.NewRequest(http.MethodPost, "/contracts/"+contractID+"/settlements", strings.NewReader(""))
	settlementRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
	settlementResponse := httptest.NewRecorder()
	handler.ServeHTTP(settlementResponse, settlementRequest)
	if settlementResponse.Code != http.StatusForbidden || contracts.lastSettlement.CommandID != "" {
		t.Fatalf("viewer settlement status=%d input=%#v", settlementResponse.Code, contracts.lastSettlement)
	}
	closureRequest := httptest.NewRequest(http.MethodPost, "/contracts/"+contractID+"/close", strings.NewReader(""))
	closureRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
	closureResponse := httptest.NewRecorder()
	handler.ServeHTTP(closureResponse, closureRequest)
	if closureResponse.Code != http.StatusForbidden || contracts.lastClosure.CommandID != "" {
		t.Fatalf("viewer closure status=%d input=%#v", closureResponse.Code, contracts.lastClosure)
	}
}

func TestContractTerminationRejectsOversizedBody(t *testing.T) {
	contracts := &contractUseCasesStub{}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	body := "command_id=" + strings.Repeat("x", (64<<10)+1)
	request := httptest.NewRequest(http.MethodPost, "/contracts/11111111-1111-4111-8111-111111111111/terminate", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if contracts.lastTerminate.CommandID != "" {
		t.Fatalf("oversized termination request reached use case: %#v", contracts.lastTerminate)
	}
}

func TestContractActivationRejectsOversizedBody(t *testing.T) {
	contracts := &contractUseCasesStub{}
	handler, err := New(readinessStub{}, &partyUseCasesStub{}, contracts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	body := "command_id=" + strings.Repeat("x", (16<<10)+1)
	request := httptest.NewRequest(http.MethodPost, "/contracts/11111111-1111-4111-8111-111111111111/activate", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if contracts.lastActivate.CommandID != "" {
		t.Fatalf("oversized request reached use case: %#v", contracts.lastActivate)
	}
}

func TestAuthenticationBoundaryKeepsHealthPublicAndProtectsBusinessRoutes(t *testing.T) {
	handler := newRawTestHandler(t, &identityUseCasesStub{}, &partyUseCasesStub{}, &contractUseCasesStub{}, Options{})

	healthRequest := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	healthResponse := httptest.NewRecorder()
	handler.ServeHTTP(healthResponse, healthRequest)
	if healthResponse.Code != http.StatusOK {
		t.Fatalf("health status = %d", healthResponse.Code)
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/contracts?state=draft", nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusSeeOther || getResponse.Header().Get("Location") != "/login?next=%2Fcontracts%3Fstate%3Ddraft" {
		t.Fatalf("protected GET status=%d location=%q", getResponse.Code, getResponse.Header().Get("Location"))
	}

	postRequest := httptest.NewRequest(http.MethodPost, "/parties", strings.NewReader(""))
	postResponse := httptest.NewRecorder()
	handler.ServeHTTP(postResponse, postRequest)
	if postResponse.Code != http.StatusUnauthorized {
		t.Fatalf("protected POST status = %d, want 401", postResponse.Code)
	}
}

func TestIAMModeUsesCapabilityCookieHidesLocalIdentityAndPrefixesLinks(t *testing.T) {
	principal := identity.Principal{UserID: "11111111-1111-4111-8111-111111111111", DisplayName: "中心用户", Provider: "iam", ScopeKind: "tenant", TenantID: "22222222-2222-4222-8222-222222222222", AllowedActions: map[string]bool{"contract-ledger.view": true}}
	identities := &identityUseCasesStub{principal: principal}
	handler := newRawTestHandler(t, identities, &partyUseCasesStub{}, &contractUseCasesStub{}, Options{ExternalIdentity: true, SessionCookieName: "oms_cap_contract_ledger", PublicBasePath: "/contract"})

	withoutCredential := httptest.NewRequest(http.MethodGet, "/", nil)
	withoutResponse := httptest.NewRecorder()
	handler.ServeHTTP(withoutResponse, withoutCredential)
	if withoutResponse.Code != http.StatusUnauthorized {
		t.Fatalf("missing IAM credential status=%d", withoutResponse.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "oms_cap_contract_ledger", Value: "test-session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("IAM home status=%d body=%q", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, expected := range []string{`href="/contract/contracts"`, `action="/gateway/logout"`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("IAM page missing %s: %s", expected, body)
		}
	}
	if strings.Contains(body, "/account/password") || strings.Contains(body, "/admin/users") {
		t.Fatalf("IAM page exposed local identity actions: %s", body)
	}
	partiesRequest := httptest.NewRequest(http.MethodGet, "/parties", nil)
	partiesRequest.AddCookie(&http.Cookie{Name: "oms_cap_contract_ledger", Value: "test-session"})
	partiesResponse := httptest.NewRecorder()
	handler.ServeHTTP(partiesResponse, partiesRequest)
	if strings.Contains(partiesResponse.Body.String(), `action="/contract/parties"`) {
		t.Fatalf("view-only IAM principal saw party registration form: %s", partiesResponse.Body.String())
	}
	identities.principal.AllowedActions["contract-ledger.party.register"] = true
	partiesResponse = httptest.NewRecorder()
	handler.ServeHTTP(partiesResponse, partiesRequest)
	if !strings.Contains(partiesResponse.Body.String(), `action="/contract/parties"`) {
		t.Fatalf("party.register IAM principal did not see registration form: %s", partiesResponse.Body.String())
	}

	adminRequest := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	adminRequest.AddCookie(&http.Cookie{Name: "oms_cap_contract_ledger", Value: "test-session"})
	adminResponse := httptest.NewRecorder()
	handler.ServeHTTP(adminResponse, adminRequest)
	if adminResponse.Code != http.StatusNotFound {
		t.Fatalf("local user route status=%d", adminResponse.Code)
	}

	writeRequest := httptest.NewRequest(http.MethodPost, "/parties", nil)
	writeRequest.AddCookie(&http.Cookie{Name: "oms_cap_contract_ledger", Value: "test-session"})
	writeResponse := httptest.NewRecorder()
	handler.ServeHTTP(writeResponse, writeRequest)
	if writeResponse.Code != http.StatusForbidden {
		t.Fatalf("view-only IAM principal wrote party: status=%d", writeResponse.Code)
	}
}

func TestLoginSetsHashedServerSessionCookieContract(t *testing.T) {
	identities := &identityUseCasesStub{login: identity.LoginResult{
		Principal: defaultTestPrincipal,
		Token:     "session-token-from-identity-service",
		ExpiresAt: time.Now().Add(2 * time.Hour),
	}}
	handler := newRawTestHandler(t, identities, &partyUseCasesStub{}, &contractUseCasesStub{}, Options{SessionCookieSecure: true})

	getRequest := httptest.NewRequest(http.MethodGet, "/login?next=%2Fcontracts", nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("login GET status = %d, body=%q", getResponse.Code, getResponse.Body.String())
	}
	csrfMatch := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(getResponse.Body.String())
	if len(csrfMatch) != 2 {
		t.Fatalf("login page missing CSRF: %q", getResponse.Body.String())
	}
	var csrfCookie *http.Cookie
	for _, cookie := range getResponse.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			csrfCookie = cookie
		}
	}
	if csrfCookie == nil || !csrfCookie.HttpOnly || !csrfCookie.Secure || csrfCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("CSRF cookie = %#v", csrfCookie)
	}

	form := url.Values{
		"csrf_token": {csrfMatch[1]}, "username": {"admin"}, "password": {"correct-password"}, "next": {"/contracts"},
	}
	postRequest := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	postRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postRequest.AddCookie(csrfCookie)
	postResponse := httptest.NewRecorder()
	handler.ServeHTTP(postResponse, postRequest)
	if postResponse.Code != http.StatusSeeOther || postResponse.Header().Get("Location") != "/contracts" {
		t.Fatalf("login POST status=%d location=%q body=%q", postResponse.Code, postResponse.Header().Get("Location"), postResponse.Body.String())
	}
	var sessionCookie *http.Cookie
	for _, cookie := range postResponse.Result().Cookies() {
		if cookie.Name == sessionCookieName && cookie.Value != "" {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil || sessionCookie.Value != identities.login.Token || !sessionCookie.HttpOnly || !sessionCookie.Secure || sessionCookie.SameSite != http.SameSiteLaxMode || sessionCookie.Path != "/" {
		t.Fatalf("session cookie = %#v", sessionCookie)
	}
}

func TestRolesEnforceServerAuthorizationAndHideMutationForms(t *testing.T) {
	t.Run("viewer", func(t *testing.T) {
		parties := &partyUseCasesStub{}
		contracts := &contractUseCasesStub{contracts: []contract.Contract{{
			ID: "11111111-1111-4111-8111-111111111111", Title: "只读检索合同",
			Currency: "CNY", Status: contract.StatusActive, CurrentRevision: 2,
		}}}
		viewer := defaultTestPrincipal
		viewer.Role = identity.RoleViewer
		handler := newRawTestHandler(t, &identityUseCasesStub{principal: viewer}, parties, contracts, Options{})

		getRequest := httptest.NewRequest(http.MethodGet, "/parties", nil)
		getRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
		getResponse := httptest.NewRecorder()
		handler.ServeHTTP(getResponse, getRequest)
		if getResponse.Code != http.StatusOK || strings.Contains(getResponse.Body.String(), "新建主体") {
			t.Fatalf("viewer party page status=%d body=%q", getResponse.Code, getResponse.Body.String())
		}

		postRequest := httptest.NewRequest(http.MethodPost, "/parties", strings.NewReader("kind=organization&legal_name=forged"))
		postRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		postRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
		postResponse := httptest.NewRecorder()
		handler.ServeHTTP(postResponse, postRequest)
		if postResponse.Code != http.StatusForbidden || parties.lastCreate.LegalName != "" {
			t.Fatalf("viewer POST status=%d input=%#v", postResponse.Code, parties.lastCreate)
		}

		contractRequest := httptest.NewRequest(http.MethodGet, "/contracts?q=只读&status=active&currency=cny", nil)
		contractRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
		contractResponse := httptest.NewRecorder()
		handler.ServeHTTP(contractResponse, contractRequest)
		if contractResponse.Code != http.StatusOK || !strings.Contains(contractResponse.Body.String(), "只读检索合同") ||
			strings.Contains(contractResponse.Body.String(), "新建合同草稿") || contracts.lastSearch.Status != "active" {
			t.Fatalf("viewer contract search status=%d input=%#v body=%q", contractResponse.Code, contracts.lastSearch, contractResponse.Body.String())
		}
	})

	t.Run("editor", func(t *testing.T) {
		editor := defaultTestPrincipal
		editor.Role = identity.RoleEditor
		handler := newRawTestHandler(t, &identityUseCasesStub{principal: editor}, &partyUseCasesStub{}, &contractUseCasesStub{}, Options{})
		request := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("editor admin status = %d", response.Code)
		}
	})
}

func TestBusinessWriteActorComesOnlyFromAuthenticatedSession(t *testing.T) {
	parties := &partyUseCasesStub{}
	principal := defaultTestPrincipal
	principal.DisplayName = "可信操作者"
	handler := newRawTestHandler(t, &identityUseCasesStub{principal: principal}, parties, &contractUseCasesStub{}, Options{})

	getRequest := httptest.NewRequest(http.MethodGet, "/parties", nil)
	getRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	csrf := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(getResponse.Body.String())[1]
	var csrfCookie *http.Cookie
	for _, cookie := range getResponse.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			csrfCookie = cookie
		}
	}
	form := url.Values{
		"csrf_token": {csrf}, "kind": {"organization"}, "legal_name": {"可信主体"},
		"actor_kind": {"forged"}, "actor_id": {"forged"},
	}
	request := httptest.NewRequest(http.MethodPost, "/parties", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
	request.AddCookie(csrfCookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("party POST status=%d body=%q", response.Code, response.Body.String())
	}
	if parties.lastCreate.Actor != principal.Actor() {
		t.Fatalf("actor = %#v, want %#v", parties.lastCreate.Actor, principal.Actor())
	}
}

func TestIdentityMutationsRequireCSRFAndUseSessionActor(t *testing.T) {
	identities := &identityUseCasesStub{principal: defaultTestPrincipal}
	handler := newRawTestHandler(t, identities, &partyUseCasesStub{}, &contractUseCasesStub{}, Options{})
	for _, path := range []string{
		"/logout",
		"/account/password",
		"/admin/users",
		"/admin/users/bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb/role",
		"/admin/users/bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb/status",
		"/admin/users/bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb/reset-password",
	} {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(""))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s status = %d, want 403", path, response.Code)
		}
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	getRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	csrf := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(getResponse.Body.String())[1]
	var csrfCookie *http.Cookie
	for _, cookie := range getResponse.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			csrfCookie = cookie
		}
	}
	form := url.Values{
		"csrf_token": {csrf}, "username": {"viewer-one"}, "display_name": {"只读一"},
		"password": {"viewer-one-password"}, "role": {"viewer"}, "actor_id": {"forged"},
	}
	request := httptest.NewRequest(http.MethodPost, "/admin/users", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
	request.AddCookie(csrfCookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || identities.lastCreate.Actor != defaultTestPrincipal.Actor() {
		t.Fatalf("create user status=%d input=%#v", response.Code, identities.lastCreate)
	}
}

func TestChangingOwnPasswordClearsBrowserSession(t *testing.T) {
	identities := &identityUseCasesStub{principal: defaultTestPrincipal}
	handler := newRawTestHandler(t, identities, &partyUseCasesStub{}, &contractUseCasesStub{}, Options{})
	getRequest := httptest.NewRequest(http.MethodGet, "/account/password", nil)
	getRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	csrf := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(getResponse.Body.String())[1]
	var csrfCookie *http.Cookie
	for _, cookie := range getResponse.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			csrfCookie = cookie
		}
	}
	form := url.Values{
		"csrf_token": {csrf}, "current_password": {"current-password"}, "new_password": {"changed-password"},
	}
	request := httptest.NewRequest(http.MethodPost, "/account/password", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
	request.AddCookie(csrfCookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login?password_changed=1" {
		t.Fatalf("password status=%d location=%q", response.Code, response.Header().Get("Location"))
	}
	if identities.lastPassword.UserID != defaultTestPrincipal.UserID || identities.lastPassword.Actor != defaultTestPrincipal.Actor() {
		t.Fatalf("password input = %#v", identities.lastPassword)
	}
	cleared := false
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == sessionCookieName && cookie.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatalf("password change did not clear session cookie")
	}
}
