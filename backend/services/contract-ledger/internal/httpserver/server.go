package httpserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/contract"
	"omsbizfoundry/contract-ledger/internal/contractevidence"
	"omsbizfoundry/contract-ledger/internal/evidence"
	"omsbizfoundry/contract-ledger/internal/identity"
	"omsbizfoundry/contract-ledger/internal/ledgerexport"
	"omsbizfoundry/contract-ledger/internal/ledgerrestore"
	"omsbizfoundry/contract-ledger/internal/party"
	"omsbizfoundry/contract-ledger/internal/partymigration"
	"omsbizfoundry/contract-ledger/internal/referenceoa"
)

const serviceName = "contract-ledger"

//go:embed templates/*.html
var templateFiles embed.FS

type ReadinessChecker interface {
	Ping(context.Context) error
}

type PartyUseCases interface {
	Create(context.Context, party.CreateInput) (party.Party, error)
	List(context.Context) ([]party.Party, error)
}

type PartyRuntime interface {
	State(context.Context, string) (partymigration.RuntimeState, error)
	LocalPartyWritesAllowed(context.Context, string) (bool, error)
	ResolveReferences(context.Context, string, []string) ([]partymigration.RuntimePartyReference, error)
}

type DirectoryPartyProjector interface {
	ResolveOrProject(context.Context, string, string, int64, string, string) (partymigration.RuntimePartyReference, error)
}

type IdentityUseCases interface {
	Login(context.Context, string, string) (identity.LoginResult, error)
	ResolveSession(context.Context, string) (identity.Principal, bool, error)
	Logout(context.Context, string) error
	ListUsers(context.Context) ([]identity.User, error)
	CreateUser(context.Context, identity.CreateUserInput) (identity.User, error)
	ChangeRole(context.Context, identity.ChangeRoleInput) error
	ChangeStatus(context.Context, identity.ChangeStatusInput) error
	ChangePassword(context.Context, identity.ChangePasswordInput) error
	ResetPassword(context.Context, identity.ResetPasswordInput) error
}

type AuditUseCases interface {
	List(context.Context, int) ([]audit.Entry, error)
}

type EvidenceUseCases interface {
	MaxUploadBytes() int64
	Create(context.Context, evidence.CreateInput, io.Reader) (evidence.UploadResult, error)
	AddVersion(context.Context, evidence.AddVersionInput, io.Reader) (evidence.UploadResult, error)
	List(context.Context, string) ([]evidence.Document, error)
	Open(context.Context, evidence.DownloadInput) (evidence.Download, error)
}

type ContractEvidenceUseCases interface {
	Current(context.Context, string, string) (contractevidence.Snapshot, bool, error)
	Replace(context.Context, contractevidence.ReplaceInput) (contractevidence.Result, error)
}

type LedgerExportUseCases interface {
	Export(context.Context, ledgerexport.Input) (ledgerexport.Artifact, error)
}

type LedgerRestoreUseCases interface {
	Restore(context.Context, ledgerrestore.Input) (ledgerrestore.Result, error)
}

type Options struct {
	SessionCookieSecure  bool
	SessionCookieName    string
	ExternalIdentity     bool
	PublicBasePath       string
	ReferenceTenantID    string
	ReferenceAccessToken string
	PartyRuntime         PartyRuntime
	DirectoryParties     DirectoryPartyProjector
	ReferenceOADrafts    ReferenceOADrafts
	ContractEvidence     ContractEvidenceUseCases
}

type ReferenceOADrafts interface {
	Read(context.Context, string, string) (*referenceoa.Draft, error)
	Save(context.Context, referenceoa.SaveInput) (referenceoa.Result, error)
	Confirm(context.Context, referenceoa.ConfirmInput) (referenceoa.Confirmation, error)
	Confirmations(context.Context, string, string) ([]referenceoa.Confirmation, error)
	MaterialsAvailable() bool
	ReadMaterials(context.Context, string, string) (referenceoa.Materials, error)
	MutateMaterial(context.Context, referenceoa.MaterialInput) (referenceoa.MaterialResult, error)
}

type ContractUseCases interface {
	CreateDraft(context.Context, contract.CreateDraftInput) (contract.Contract, error)
	List(context.Context) ([]contract.Contract, error)
	Search(context.Context, contract.SearchContractsInput) (contract.ContractSearchPage, error)
	GetDraft(context.Context, string) (contract.DraftDetails, error)
	AddDraftParty(context.Context, contract.AddDraftPartyInput) (contract.DraftPartyMutationResult, error)
	RemoveDraftParty(context.Context, contract.RemoveDraftPartyInput) (contract.DraftPartyMutationResult, error)
	UpdateDraft(context.Context, contract.UpdateDraftInput) (contract.DraftMutationResult, error)
	CancelDraft(context.Context, contract.CancelDraftInput) (contract.DraftMutationResult, error)
	Activate(context.Context, contract.ActivateContractInput) (contract.ActivationResult, error)
	AppendChange(context.Context, contract.AppendContractChangeInput) (contract.ContractChangeResult, error)
	Terminate(context.Context, contract.TerminateContractInput) (contract.TerminationResult, error)
	LinkRelation(context.Context, contract.LinkContractRelationInput) (contract.ContractRelationResult, error)
	RecordSettlement(context.Context, contract.RecordContractSettlementInput) (contract.ContractSettlementResult, error)
	GetSettlement(context.Context, string) (contract.ContractSettlement, error)
	CloseContract(context.Context, contract.CloseContractInput) (contract.ContractClosureResult, error)
	CorrectContract(context.Context, contract.CorrectContractInput) (contract.ContractCorrectionResult, error)
}

type Server struct {
	readiness            ReadinessChecker
	parties              PartyUseCases
	partyRuntime         PartyRuntime
	directoryParties     DirectoryPartyProjector
	referenceOADrafts    ReferenceOADrafts
	contractEvidence     ContractEvidenceUseCases
	contracts            ContractUseCases
	evidence             EvidenceUseCases
	ledgerExports        LedgerExportUseCases
	ledgerRestores       LedgerRestoreUseCases
	identities           IdentityUseCases
	audits               AuditUseCases
	homeTemplate         *template.Template
	partyPage            *template.Template
	contractPage         *template.Template
	contractDetailPage   *template.Template
	contractRelationPage *template.Template
	loginPage            *template.Template
	usersPage            *template.Template
	passwordPage         *template.Template
	auditPage            *template.Template
	restorePage          *template.Template
	sessionCookieSecure  bool
	sessionCookieName    string
	externalIdentity     bool
	publicBasePath       string
	referenceTenantID    string
	referenceAccessToken string
}

func NewServer(
	readiness ReadinessChecker,
	parties PartyUseCases,
	contracts ContractUseCases,
	evidenceUseCases EvidenceUseCases,
	ledgerExportUseCases LedgerExportUseCases,
	ledgerRestoreUseCases LedgerRestoreUseCases,
	identities IdentityUseCases,
	audits AuditUseCases,
	options Options,
) (http.Handler, error) {
	home, err := template.ParseFS(templateFiles, "templates/home.html")
	if err != nil {
		return nil, err
	}
	partyPage, err := template.ParseFS(templateFiles, "templates/parties.html")
	if err != nil {
		return nil, err
	}
	contractPage, err := template.ParseFS(templateFiles, "templates/contracts.html")
	if err != nil {
		return nil, err
	}
	contractDetailPage, err := template.ParseFS(templateFiles, "templates/contract_detail.html")
	if err != nil {
		return nil, err
	}
	contractRelationPage, err := template.ParseFS(templateFiles, "templates/contract_relation.html")
	if err != nil {
		return nil, err
	}
	loginPage, err := template.ParseFS(templateFiles, "templates/login.html")
	if err != nil {
		return nil, err
	}
	usersPage, err := template.ParseFS(templateFiles, "templates/users.html")
	if err != nil {
		return nil, err
	}
	passwordPage, err := template.ParseFS(templateFiles, "templates/password.html")
	if err != nil {
		return nil, err
	}
	auditPage, err := template.ParseFS(templateFiles, "templates/audit.html")
	if err != nil {
		return nil, err
	}
	restorePage, err := template.ParseFS(templateFiles, "templates/restore.html")
	if err != nil {
		return nil, err
	}

	if evidenceUseCases == nil {
		return nil, fmt.Errorf("evidence use cases are required")
	}
	if ledgerExportUseCases == nil {
		return nil, fmt.Errorf("ledger export use cases are required")
	}
	if ledgerRestoreUseCases == nil {
		return nil, fmt.Errorf("ledger restore use cases are required")
	}
	if err := validateReferenceOptions(options.ReferenceTenantID, options.ReferenceAccessToken); err != nil {
		return nil, err
	}
	if options.SessionCookieName == "" {
		options.SessionCookieName = sessionCookieName
	}
	server := &Server{
		readiness:            readiness,
		parties:              parties,
		partyRuntime:         options.PartyRuntime,
		directoryParties:     options.DirectoryParties,
		referenceOADrafts:    options.ReferenceOADrafts,
		contractEvidence:     options.ContractEvidence,
		contracts:            contracts,
		evidence:             evidenceUseCases,
		ledgerExports:        ledgerExportUseCases,
		ledgerRestores:       ledgerRestoreUseCases,
		identities:           identities,
		audits:               audits,
		homeTemplate:         home,
		partyPage:            partyPage,
		contractPage:         contractPage,
		contractDetailPage:   contractDetailPage,
		contractRelationPage: contractRelationPage,
		loginPage:            loginPage,
		usersPage:            usersPage,
		passwordPage:         passwordPage,
		auditPage:            auditPage,
		restorePage:          restorePage,
		sessionCookieSecure:  options.SessionCookieSecure,
		sessionCookieName:    options.SessionCookieName,
		externalIdentity:     options.ExternalIdentity,
		publicBasePath:       options.PublicBasePath,
		referenceTenantID:    options.ReferenceTenantID,
		referenceAccessToken: options.ReferenceAccessToken,
	}

	publicMux := http.NewServeMux()
	publicMux.HandleFunc("GET /assets/demo-workbench.css", demoWorkbenchStyle)
	publicMux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"service": serviceName, "status": "ok"})
	})
	publicMux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := server.readiness.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"service": serviceName, "status": "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"service": serviceName, "status": "ready"})
	})
	if !server.externalIdentity {
		publicMux.HandleFunc("GET /login", server.showLogin)
		publicMux.HandleFunc("POST /login", server.login)
	}
	publicMux.Handle("GET /api/v1/references/contracts/{contractID}", server.authenticateReference("contract", "contractID", http.HandlerFunc(server.contractReference)))
	publicMux.Handle("GET /api/v1/references/settlements/{settlementID}", server.authenticateReference("settlement", "settlementID", http.HandlerFunc(server.settlementReference)))
	server.registerWorkspace(publicMux)

	protectedMux := http.NewServeMux()
	protectedMux.HandleFunc("GET /", server.home)
	if !server.externalIdentity {
		protectedMux.HandleFunc("POST /logout", server.logout)
		protectedMux.HandleFunc("GET /account/password", server.showPassword)
		protectedMux.HandleFunc("POST /account/password", server.changePassword)
	}
	protectedMux.HandleFunc("GET /audit", server.listAudit)
	protectedMux.HandleFunc("GET /parties", server.listParties)
	protectedMux.Handle("POST /parties", server.requirePermission(identity.PermissionPartyRegister, http.HandlerFunc(server.createParty)))
	protectedMux.HandleFunc("GET /contracts", server.listContracts)
	protectedMux.Handle("POST /contracts", server.requirePermission(identity.PermissionDraftManage, http.HandlerFunc(server.createContractDraft)))
	protectedMux.HandleFunc("GET /contracts/{contractID}", server.showContractDraft)
	protectedMux.Handle("POST /contracts/{contractID}", server.requirePermission(identity.PermissionDraftManage, http.HandlerFunc(server.updateContractDraft)))
	protectedMux.Handle("POST /contracts/{contractID}/cancel", server.requirePermission(identity.PermissionDraftManage, http.HandlerFunc(server.cancelContractDraft)))
	protectedMux.Handle("POST /contracts/{contractID}/activate", server.requirePermission(identity.PermissionLifecycleManage, http.HandlerFunc(server.activateContract)))
	protectedMux.Handle("POST /contracts/{contractID}/changes", server.requirePermission(identity.PermissionLifecycleManage, http.HandlerFunc(server.appendContractChange)))
	protectedMux.Handle("POST /contracts/{contractID}/terminate", server.requirePermission(identity.PermissionLifecycleManage, http.HandlerFunc(server.terminateContract)))
	protectedMux.Handle("POST /contracts/{contractID}/settlements", server.requirePermission(identity.PermissionLifecycleManage, http.HandlerFunc(server.recordContractSettlement)))
	protectedMux.Handle("POST /contracts/{contractID}/close", server.requirePermission(identity.PermissionLifecycleManage, http.HandlerFunc(server.closeContract)))
	protectedMux.Handle("POST /contracts/{contractID}/corrections", server.requirePermission(identity.PermissionBusinessCorrect, http.HandlerFunc(server.correctContract)))
	protectedMux.Handle("GET /contracts/{contractID}/export", server.requirePermission(identity.PermissionBusinessExport, http.HandlerFunc(server.exportContractLedger)))
	protectedMux.Handle("GET /admin/restore", server.requirePermission(identity.PermissionBusinessRestore, http.HandlerFunc(server.showRestore)))
	protectedMux.Handle("POST /admin/restore", server.requirePermission(identity.PermissionBusinessRestore, http.HandlerFunc(server.restoreContractLedger)))
	protectedMux.Handle("POST /contracts/{contractID}/evidence", server.requirePermission(identity.PermissionEvidenceManage, http.HandlerFunc(server.createContractEvidence)))
	protectedMux.Handle("POST /contracts/{contractID}/evidence/{documentID}/versions", server.requirePermission(identity.PermissionEvidenceManage, http.HandlerFunc(server.addContractEvidenceVersion)))
	protectedMux.HandleFunc("GET /contracts/{contractID}/evidence/{documentID}/versions/{versionID}/download", server.downloadContractEvidence)
	protectedMux.Handle("GET /contracts/{contractID}/relations/new", server.requirePermission(identity.PermissionLifecycleManage, http.HandlerFunc(server.showContractRelationNew)))
	protectedMux.Handle("POST /contracts/{contractID}/relations", server.requirePermission(identity.PermissionLifecycleManage, http.HandlerFunc(server.linkContractRelation)))
	protectedMux.Handle("POST /contracts/{contractID}/parties", server.requirePermission(identity.PermissionDraftManage, http.HandlerFunc(server.addContractDraftParty)))
	protectedMux.Handle("POST /contracts/{contractID}/parties/{partyLinkID}/remove", server.requirePermission(identity.PermissionDraftManage, http.HandlerFunc(server.removeContractDraftParty)))
	if !server.externalIdentity {
		protectedMux.Handle("GET /admin/users", server.requirePermission(identity.PermissionIdentityManage, http.HandlerFunc(server.listUsers)))
		protectedMux.Handle("POST /admin/users", server.requirePermission(identity.PermissionIdentityManage, http.HandlerFunc(server.createUser)))
		protectedMux.Handle("POST /admin/users/{userID}/role", server.requirePermission(identity.PermissionIdentityManage, http.HandlerFunc(server.changeUserRole)))
		protectedMux.Handle("POST /admin/users/{userID}/status", server.requirePermission(identity.PermissionIdentityManage, http.HandlerFunc(server.changeUserStatus)))
		protectedMux.Handle("POST /admin/users/{userID}/reset-password", server.requirePermission(identity.PermissionIdentityManage, http.HandlerFunc(server.resetUserPassword)))
	} else {
		for _, pattern := range []string{"POST /logout", "GET /account/password", "POST /account/password", "GET /admin/users", "POST /admin/users", "POST /admin/users/{userID}/role", "POST /admin/users/{userID}/status", "POST /admin/users/{userID}/reset-password"} {
			protectedMux.HandleFunc(pattern, http.NotFound)
		}
	}

	publicMux.Handle("/", server.authenticate(server.requirePermission(identity.PermissionBusinessView, protectedMux)))
	return securityHeaders(publicMux), nil
}

type draftPartyForm struct {
	CommandID            string
	PartyID              string
	Role                 string
	ExpectedDraftVersion int64
}

type draftPartyRow struct {
	Party           contract.DraftParty
	RemoveCommandID string
}

type evidenceTargetOption struct {
	Value string
	Label string
}

type evidenceDocumentRow struct {
	Document         evidence.Document
	VersionCommandID string
}

type evidenceForm struct {
	CommandID string
	Target    string
	Title     string
	Purpose   string
}

type contractDetailPageData struct {
	commonPageData
	CSRFToken           string
	PartyAdded          bool
	PartyRemoved        bool
	Updated             bool
	Cancelled           bool
	Activated           bool
	Changed             bool
	Terminated          bool
	RelationLinked      bool
	SettlementRecorded  bool
	ContractClosed      bool
	Corrected           bool
	EvidenceUploaded    bool
	Restored            bool
	MissingEvidence     int
	Error               string
	Contract            contract.Contract
	Cancellation        *contract.DraftCancellation
	Revision            *contract.ContractRevision
	Revisions           []contract.ContractRevision
	Instruments         []contract.ContractInstrument
	PartySnapshots      []contract.ContractPartySnapshot
	AuditEntries        []contract.ContractAuditEntry
	OutgoingRelations   []contract.ContractRelation
	IncomingRelations   []contract.ContractRelation
	Settlements         []contract.ContractSettlement
	OriginalRevision    *contract.ContractRevision
	LatestSettlement    *contract.ContractSettlement
	Closure             *contract.ContractClosure
	EvidenceDocuments   []evidenceDocumentRow
	EvidenceTargets     []evidenceTargetOption
	EvidenceForm        evidenceForm
	EvidenceMaxBytes    int64
	DraftParties        []draftPartyRow
	AvailableParties    []party.Party
	AddForm             draftPartyForm
	EditForm            contractForm
	CancelForm          cancelDraftForm
	ActivationForm      activationForm
	ChangeForm          contractChangeForm
	TerminationForm     terminationForm
	SettlementForm      settlementForm
	CanRecordSettlement bool
	ClosureForm         closureForm
	CanClose            bool
	CorrectionForm      correctionForm
	ActivationActor     contract.ActorRef
	ActivationIssues    []string
	ActivationReady     bool
}

type contractRelationForm struct {
	CommandID              string
	TargetContractID       string
	RelationType           string
	ExpectedSourceRevision int64
	ExpectedTargetRevision int64
	EffectiveOn            string
	Note                   string
}

type contractRelationPageData struct {
	commonPageData
	CSRFToken     string
	Error         string
	Source        contract.Contract
	Target        *contract.Contract
	SearchForm    contractSearchForm
	Targets       []contract.Contract
	NextURL       string
	Form          contractRelationForm
	RelationReady bool
}

type cancelDraftForm struct {
	CommandID            string
	Reason               string
	ExpectedDraftVersion int64
}

type activationForm struct {
	CommandID            string
	ExpectedDraftVersion int64
}

type contractChangeForm struct {
	CommandID        string
	ExpectedRevision int64
	InstrumentNumber string
	Title            string
	ChangeSummary    string
	SignedOn         string
	EffectiveOn      string
	EffectiveOrder   int64
	AmountEffect     string
	AmountValue      string
}

type terminationForm struct {
	CommandID         string
	ExpectedRevision  int64
	InstrumentNumber  string
	Title             string
	TerminationReason string
	SignedOn          string
	EffectiveOn       string
	EffectiveOrder    int64
}

type settlementForm struct {
	CommandID                  string
	ExpectedContractRevision   int64
	ExpectedSettlementSequence int64
	Kind                       string
	StatementNumber            string
	Title                      string
	Amount                     string
	ConfirmedOn                string
	Basis                      string
	SelectedPartyPositions     map[int64]bool
}

type closureForm struct {
	CommandID                  string
	ExpectedContractRevision   int64
	ExpectedSettlementSequence int64
	SettlementDisposition      string
	ClosedOn                   string
	Reason                     string
}

type correctionForm struct {
	CommandID             string
	ExpectedStatus        string
	ExpectedRevision      int64
	InstrumentNumber      string
	InstrumentTitle       string
	CorrectionReason      string
	InstrumentSignedOn    string
	InstrumentEffectiveOn string
	EffectiveOrder        int64
	CorrectedTitle        string
	CorrectedCategory     string
	CorrectedTotalAmount  string
	CorrectedSignedOn     string
	CorrectedEffectiveOn  string
}

func (s *Server) showContractDraft(w http.ResponseWriter, r *http.Request) {
	data, err := s.loadContractDetailPage(w, r, r.PathValue("contractID"))
	if err != nil {
		s.writeContractDetailLoadError(w, err)
		return
	}
	data.PartyAdded = r.URL.Query().Get("party_added") == "1"
	data.PartyRemoved = r.URL.Query().Get("party_removed") == "1"
	data.Updated = r.URL.Query().Get("updated") == "1"
	data.Cancelled = r.URL.Query().Get("cancelled") == "1"
	data.Activated = r.URL.Query().Get("activated") == "1"
	data.Changed = r.URL.Query().Get("changed") == "1"
	data.Terminated = r.URL.Query().Get("terminated") == "1"
	data.RelationLinked = r.URL.Query().Get("relation_linked") == "1"
	data.SettlementRecorded = r.URL.Query().Get("settlement_recorded") == "1"
	data.ContractClosed = r.URL.Query().Get("closed") == "1"
	data.Corrected = r.URL.Query().Get("corrected") == "1"
	data.EvidenceUploaded = r.URL.Query().Get("evidence_uploaded") == "1"
	data.Restored = r.URL.Query().Get("restored") == "1"
	if missing, err := strconv.Atoi(r.URL.Query().Get("missing_evidence")); err == nil && missing > 0 {
		data.MissingEvidence = missing
	}
	s.renderContractDetailPage(w, http.StatusOK, data)
}

func (s *Server) exportContractLedger(w http.ResponseWriter, r *http.Request) {
	artifact, err := s.ledgerExports.Export(r.Context(), ledgerexport.Input{
		ContractID: r.PathValue("contractID"),
		Actor:      mustPrincipal(r).Actor(),
	})
	if err != nil {
		var validation *ledgerexport.ValidationError
		var notFound *ledgerexport.NotFoundError
		switch {
		case errors.As(err, &validation), errors.As(err, &notFound):
			http.Error(w, "合同不存在", http.StatusNotFound)
		default:
			http.Error(w, "合同账簿导出失败", http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": artifact.Filename}))
	w.Header().Set("Content-Length", strconv.Itoa(len(artifact.Content)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(artifact.Content)
}

func (s *Server) createContractEvidence(w http.ResponseWriter, r *http.Request) {
	file, filename, cleanup, ok := s.parseEvidenceUpload(w, r)
	if !ok {
		return
	}
	defer cleanup()
	defer file.Close()
	targetType, targetID := splitEvidenceTarget(r.PostForm.Get("evidence_target"))
	form := evidenceForm{
		CommandID: r.PostForm.Get("command_id"), Target: r.PostForm.Get("evidence_target"),
		Title: r.PostForm.Get("evidence_title"), Purpose: r.PostForm.Get("evidence_purpose"),
	}
	contractID := r.PathValue("contractID")
	_, err := s.evidence.Create(r.Context(), evidence.CreateInput{
		CommandID: form.CommandID, ContractID: contractID,
		TargetType: targetType, TargetID: targetID,
		Title: form.Title, Purpose: form.Purpose, OriginalFilename: filename,
		Actor: mustPrincipal(r).Actor(),
	}, file)
	if err == nil {
		http.Redirect(w, r, "/contracts/"+contractID+"?evidence_uploaded=1", http.StatusSeeOther)
		return
	}
	s.renderContractEvidenceError(w, r, contractID, form, err)
}

func (s *Server) addContractEvidenceVersion(w http.ResponseWriter, r *http.Request) {
	file, filename, cleanup, ok := s.parseEvidenceUpload(w, r)
	if !ok {
		return
	}
	defer cleanup()
	defer file.Close()
	contractID := r.PathValue("contractID")
	_, err := s.evidence.AddVersion(r.Context(), evidence.AddVersionInput{
		CommandID: r.PostForm.Get("command_id"), ContractID: contractID,
		DocumentID:            r.PathValue("documentID"),
		ExpectedLatestVersion: parseDraftVersion(r.PostForm.Get("expected_latest_version")),
		OriginalFilename:      filename, Actor: mustPrincipal(r).Actor(),
	}, file)
	if err == nil {
		http.Redirect(w, r, "/contracts/"+contractID+"?evidence_uploaded=1", http.StatusSeeOther)
		return
	}
	s.renderContractEvidenceError(w, r, contractID, evidenceForm{}, err)
}

func (s *Server) parseEvidenceUpload(w http.ResponseWriter, r *http.Request) (io.ReadCloser, string, func(), bool) {
	maxBytes := s.evidence.MaxUploadBytes()
	bodyLimit := maxBytes
	if maxBytes <= 1<<62 {
		bodyLimit += 1 << 20
	}
	r.Body = http.MaxBytesReader(w, r.Body, bodyLimit)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var maxError *http.MaxBytesError
		if errors.As(err, &maxError) {
			http.Error(w, "上传请求超过允许大小", http.StatusRequestEntityTooLarge)
			return nil, "", func() {}, false
		}
		http.Error(w, "上传内容无效", http.StatusBadRequest)
		return nil, "", func() {}, false
	}
	cleanup := func() {}
	if r.MultipartForm != nil {
		cleanup = func() { _ = r.MultipartForm.RemoveAll() }
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		cleanup()
		return nil, "", func() {}, false
	}
	file, header, err := r.FormFile("evidence_file")
	if err != nil {
		http.Error(w, "必须选择证据文件", http.StatusUnprocessableEntity)
		cleanup()
		return nil, "", func() {}, false
	}
	return file, header.Filename, cleanup, true
}

func splitEvidenceTarget(value string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func (s *Server) renderContractEvidenceError(
	w http.ResponseWriter,
	r *http.Request,
	contractID string,
	form evidenceForm,
	err error,
) {
	status, message, known := evidenceHTTPError(err)
	if !known {
		http.Error(w, "合同证据上传失败", http.StatusInternalServerError)
		return
	}
	var notFound *evidence.ContractNotFoundError
	if errors.As(err, &notFound) {
		http.Error(w, message, status)
		return
	}
	data, loadErr := s.loadContractDetailPage(w, r, contractID)
	if loadErr != nil {
		s.writeContractDetailLoadError(w, loadErr)
		return
	}
	data.Error = message
	if form.CommandID != "" {
		form.CommandID = data.EvidenceForm.CommandID
		data.EvidenceForm = form
	}
	s.renderContractDetailPage(w, status, data)
}

func (s *Server) downloadContractEvidence(w http.ResponseWriter, r *http.Request) {
	download, err := s.evidence.Open(r.Context(), evidence.DownloadInput{
		ContractID: r.PathValue("contractID"), DocumentID: r.PathValue("documentID"),
		VersionID: r.PathValue("versionID"), Actor: mustPrincipal(r).Actor(),
	})
	if err != nil {
		status, message, known := evidenceHTTPError(err)
		if !known {
			http.Error(w, "无法下载合同证据", http.StatusInternalServerError)
			return
		}
		http.Error(w, message, status)
		return
	}
	defer download.Content.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": download.Version.OriginalFilename}))
	w.Header().Set("Content-Length", strconv.FormatInt(download.Version.SizeBytes, 10))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, download.Content)
}

func evidenceHTTPError(err error) (int, string, bool) {
	var validation *evidence.ValidationError
	var contractNotFound *evidence.ContractNotFoundError
	var targetConflict *evidence.TargetConflictError
	var documentNotFound *evidence.DocumentNotFoundError
	var versionNotFound *evidence.VersionNotFoundError
	var versionConflict *evidence.VersionConflictError
	var idempotency *evidence.IdempotencyConflictError
	var emptyFile *evidence.EmptyFileError
	var tooLarge *evidence.FileTooLargeError
	var integrity *evidence.IntegrityError
	var unavailable *evidence.UnavailableError
	switch {
	case errors.As(err, &validation):
		return http.StatusUnprocessableEntity, validation.Error(), true
	case errors.As(err, &contractNotFound):
		return http.StatusNotFound, contractNotFound.Error(), true
	case errors.As(err, &targetConflict):
		return http.StatusUnprocessableEntity, targetConflict.Error(), true
	case errors.As(err, &documentNotFound):
		return http.StatusNotFound, documentNotFound.Error(), true
	case errors.As(err, &versionNotFound):
		return http.StatusNotFound, versionNotFound.Error(), true
	case errors.As(err, &versionConflict):
		return http.StatusConflict, versionConflict.Error(), true
	case errors.As(err, &idempotency):
		return http.StatusConflict, idempotency.Error(), true
	case errors.As(err, &emptyFile):
		return http.StatusUnprocessableEntity, emptyFile.Error(), true
	case errors.As(err, &tooLarge):
		return http.StatusRequestEntityTooLarge, tooLarge.Error(), true
	case errors.As(err, &integrity):
		return http.StatusServiceUnavailable, integrity.Error(), true
	case errors.As(err, &unavailable):
		return http.StatusServiceUnavailable, unavailable.Error(), true
	default:
		return 0, "", false
	}
}

func (s *Server) closeContract(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	contractID := r.PathValue("contractID")
	form := closureForm{
		CommandID:                  r.PostForm.Get("command_id"),
		ExpectedContractRevision:   parseDraftVersion(r.PostForm.Get("expected_revision")),
		ExpectedSettlementSequence: parseSettlementSequence(r.PostForm.Get("expected_settlement_sequence")),
		SettlementDisposition:      r.PostForm.Get("settlement_disposition"),
		ClosedOn:                   r.PostForm.Get("closed_on"),
		Reason:                     r.PostForm.Get("closure_reason"),
	}
	_, err := s.contracts.CloseContract(r.Context(), contract.CloseContractInput{
		CommandID: form.CommandID, ContractID: contractID,
		ExpectedContractRevision:   form.ExpectedContractRevision,
		ExpectedSettlementSequence: form.ExpectedSettlementSequence,
		SettlementDisposition:      form.SettlementDisposition,
		ClosedOn:                   form.ClosedOn, Reason: form.Reason, Actor: mustPrincipal(r).Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/contracts/"+contractID+"?closed=1", http.StatusSeeOther)
		return
	}
	status, message, known := closureHTTPError(err)
	if !known {
		http.Error(w, "合同关闭失败", http.StatusInternalServerError)
		return
	}
	var notFound *contract.ContractNotFoundError
	if errors.As(err, &notFound) {
		http.Error(w, message, status)
		return
	}
	data, loadErr := s.loadContractDetailPage(w, r, contractID)
	if loadErr != nil {
		s.writeContractDetailLoadError(w, loadErr)
		return
	}
	data.Error = message
	form.ExpectedContractRevision = data.Contract.CurrentRevision
	form.ExpectedSettlementSequence = data.ClosureForm.ExpectedSettlementSequence
	var validation *contract.ValidationError
	var idempotency *contract.IdempotencyConflictError
	if (errors.As(err, &validation) && validation.Field == "command_id") || errors.As(err, &idempotency) {
		form.CommandID = data.ClosureForm.CommandID
	}
	data.ClosureForm = form
	s.renderContractDetailPage(w, status, data)
}

func (s *Server) correctContract(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	contractID := r.PathValue("contractID")
	form := correctionForm{
		CommandID:             r.PostForm.Get("command_id"),
		ExpectedStatus:        r.PostForm.Get("expected_status"),
		ExpectedRevision:      parseDraftVersion(r.PostForm.Get("expected_revision")),
		InstrumentNumber:      r.PostForm.Get("correction_instrument_number"),
		InstrumentTitle:       r.PostForm.Get("correction_instrument_title"),
		CorrectionReason:      r.PostForm.Get("correction_reason"),
		InstrumentSignedOn:    r.PostForm.Get("correction_instrument_signed_on"),
		InstrumentEffectiveOn: r.PostForm.Get("correction_instrument_effective_on"),
		EffectiveOrder:        parseDraftVersion(r.PostForm.Get("effective_order")),
		CorrectedTitle:        r.PostForm.Get("corrected_title"),
		CorrectedCategory:     r.PostForm.Get("corrected_category"),
		CorrectedTotalAmount:  r.PostForm.Get("corrected_total_amount"),
		CorrectedSignedOn:     r.PostForm.Get("corrected_signed_on"),
		CorrectedEffectiveOn:  r.PostForm.Get("corrected_effective_on"),
	}
	_, err := s.contracts.CorrectContract(r.Context(), contract.CorrectContractInput{
		CommandID: form.CommandID, ContractID: contractID,
		ExpectedStatus: form.ExpectedStatus, ExpectedRevision: form.ExpectedRevision,
		InstrumentNumber: form.InstrumentNumber, InstrumentTitle: form.InstrumentTitle,
		CorrectionReason:   form.CorrectionReason,
		InstrumentSignedOn: form.InstrumentSignedOn, InstrumentEffectiveOn: form.InstrumentEffectiveOn,
		EffectiveOrder: form.EffectiveOrder,
		CorrectedTitle: form.CorrectedTitle, CorrectedCategory: form.CorrectedCategory,
		CorrectedTotalAmount: form.CorrectedTotalAmount,
		CorrectedSignedOn:    form.CorrectedSignedOn, CorrectedEffectiveOn: form.CorrectedEffectiveOn,
		Actor: mustPrincipal(r).Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/contracts/"+contractID+"?corrected=1", http.StatusSeeOther)
		return
	}
	status, message, known := correctionHTTPError(err)
	if !known {
		http.Error(w, "合同受控更正失败", http.StatusInternalServerError)
		return
	}
	var notFound *contract.ContractNotFoundError
	if errors.As(err, &notFound) {
		http.Error(w, message, status)
		return
	}
	data, loadErr := s.loadContractDetailPage(w, r, contractID)
	if loadErr != nil {
		s.writeContractDetailLoadError(w, loadErr)
		return
	}
	data.Error = message
	form.ExpectedStatus = string(data.Contract.Status)
	form.ExpectedRevision = data.Contract.CurrentRevision
	form.EffectiveOrder = data.Contract.CurrentRevision + 1
	var validation *contract.ValidationError
	var idempotency *contract.IdempotencyConflictError
	if (errors.As(err, &validation) && validation.Field == "command_id") || errors.As(err, &idempotency) {
		form.CommandID = data.CorrectionForm.CommandID
	}
	data.CorrectionForm = form
	s.renderContractDetailPage(w, status, data)
}

func (s *Server) recordContractSettlement(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	contractID := r.PathValue("contractID")
	positions := make([]int64, 0, len(r.PostForm["confirming_party_position"]))
	selected := make(map[int64]bool)
	for _, raw := range r.PostForm["confirming_party_position"] {
		position := parseDraftVersion(raw)
		positions = append(positions, position)
		selected[position] = true
	}
	form := settlementForm{
		CommandID:                  r.PostForm.Get("command_id"),
		ExpectedContractRevision:   parseDraftVersion(r.PostForm.Get("expected_revision")),
		ExpectedSettlementSequence: parseSettlementSequence(r.PostForm.Get("expected_settlement_sequence")),
		Kind:                       r.PostForm.Get("settlement_kind"),
		StatementNumber:            r.PostForm.Get("settlement_number"),
		Title:                      r.PostForm.Get("settlement_title"),
		Amount:                     r.PostForm.Get("settlement_amount"),
		ConfirmedOn:                r.PostForm.Get("settlement_confirmed_on"),
		Basis:                      r.PostForm.Get("settlement_basis"),
		SelectedPartyPositions:     selected,
	}
	_, err := s.contracts.RecordSettlement(r.Context(), contract.RecordContractSettlementInput{
		CommandID: form.CommandID, ContractID: contractID,
		ExpectedContractRevision:   form.ExpectedContractRevision,
		ExpectedSettlementSequence: form.ExpectedSettlementSequence,
		Kind:                       form.Kind, StatementNumber: form.StatementNumber, Title: form.Title,
		Amount: form.Amount, ConfirmedOn: form.ConfirmedOn, Basis: form.Basis,
		ConfirmingPartyPositions: positions, Actor: mustPrincipal(r).Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/contracts/"+contractID+"?settlement_recorded=1", http.StatusSeeOther)
		return
	}
	status, message, known := settlementHTTPError(err)
	if !known {
		http.Error(w, "结算登记失败", http.StatusInternalServerError)
		return
	}
	var notFound *contract.ContractNotFoundError
	if errors.As(err, &notFound) {
		http.Error(w, message, status)
		return
	}
	data, loadErr := s.loadContractDetailPage(w, r, contractID)
	if loadErr != nil {
		s.writeContractDetailLoadError(w, loadErr)
		return
	}
	data.Error = message
	form.ExpectedContractRevision = data.Contract.CurrentRevision
	form.ExpectedSettlementSequence = data.SettlementForm.ExpectedSettlementSequence
	var validation *contract.ValidationError
	var idempotency *contract.IdempotencyConflictError
	if (errors.As(err, &validation) && validation.Field == "command_id") || errors.As(err, &idempotency) {
		form.CommandID = data.SettlementForm.CommandID
	}
	data.SettlementForm = form
	s.renderContractDetailPage(w, status, data)
}

func (s *Server) appendContractChange(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	contractID := r.PathValue("contractID")
	form := contractChangeForm{
		CommandID:        r.PostForm.Get("command_id"),
		ExpectedRevision: parseDraftVersion(r.PostForm.Get("expected_revision")),
		InstrumentNumber: r.PostForm.Get("instrument_number"),
		Title:            r.PostForm.Get("instrument_title"),
		ChangeSummary:    r.PostForm.Get("change_summary"),
		SignedOn:         r.PostForm.Get("instrument_signed_on"),
		EffectiveOn:      r.PostForm.Get("instrument_effective_on"),
		EffectiveOrder:   parseDraftVersion(r.PostForm.Get("effective_order")),
		AmountEffect:     r.PostForm.Get("amount_effect"),
		AmountValue:      r.PostForm.Get("amount_value"),
	}
	_, err := s.contracts.AppendChange(r.Context(), contract.AppendContractChangeInput{
		CommandID: form.CommandID, ContractID: contractID, ExpectedRevision: form.ExpectedRevision,
		InstrumentNumber: form.InstrumentNumber, Title: form.Title, ChangeSummary: form.ChangeSummary,
		SignedOn: form.SignedOn, EffectiveOn: form.EffectiveOn, EffectiveOrder: form.EffectiveOrder,
		AmountEffect: form.AmountEffect, AmountValue: form.AmountValue, Actor: mustPrincipal(r).Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/contracts/"+contractID+"?changed=1", http.StatusSeeOther)
		return
	}
	status, message, known := contractChangeHTTPError(err)
	if !known {
		http.Error(w, "补充协议登记失败", http.StatusInternalServerError)
		return
	}
	var notFound *contract.ContractNotFoundError
	if errors.As(err, &notFound) {
		http.Error(w, message, status)
		return
	}
	data, loadErr := s.loadContractDetailPage(w, r, contractID)
	if loadErr != nil {
		s.writeContractDetailLoadError(w, loadErr)
		return
	}
	data.Error = message
	form.ExpectedRevision = data.Contract.CurrentRevision
	form.EffectiveOrder = data.Contract.CurrentRevision + 1
	var validation *contract.ValidationError
	var idempotency *contract.IdempotencyConflictError
	if (errors.As(err, &validation) && validation.Field == "command_id") || errors.As(err, &idempotency) {
		form.CommandID = data.ChangeForm.CommandID
	}
	data.ChangeForm = form
	s.renderContractDetailPage(w, status, data)
}

func (s *Server) terminateContract(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	contractID := r.PathValue("contractID")
	form := terminationForm{
		CommandID:         r.PostForm.Get("command_id"),
		ExpectedRevision:  parseDraftVersion(r.PostForm.Get("expected_revision")),
		InstrumentNumber:  r.PostForm.Get("termination_instrument_number"),
		Title:             r.PostForm.Get("termination_instrument_title"),
		TerminationReason: r.PostForm.Get("termination_reason"),
		SignedOn:          r.PostForm.Get("termination_signed_on"),
		EffectiveOn:       r.PostForm.Get("termination_effective_on"),
		EffectiveOrder:    parseDraftVersion(r.PostForm.Get("effective_order")),
	}
	_, err := s.contracts.Terminate(r.Context(), contract.TerminateContractInput{
		CommandID: form.CommandID, ContractID: contractID, ExpectedRevision: form.ExpectedRevision,
		InstrumentNumber: form.InstrumentNumber, Title: form.Title,
		TerminationReason: form.TerminationReason,
		SignedOn:          form.SignedOn, EffectiveOn: form.EffectiveOn,
		EffectiveOrder: form.EffectiveOrder, Actor: mustPrincipal(r).Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/contracts/"+contractID+"?terminated=1", http.StatusSeeOther)
		return
	}
	status, message, known := terminationHTTPError(err)
	if !known {
		http.Error(w, "终止文书登记失败", http.StatusInternalServerError)
		return
	}
	var notFound *contract.ContractNotFoundError
	if errors.As(err, &notFound) {
		http.Error(w, message, status)
		return
	}
	data, loadErr := s.loadContractDetailPage(w, r, contractID)
	if loadErr != nil {
		s.writeContractDetailLoadError(w, loadErr)
		return
	}
	data.Error = message
	form.ExpectedRevision = data.Contract.CurrentRevision
	form.EffectiveOrder = data.Contract.CurrentRevision + 1
	var validation *contract.ValidationError
	var idempotency *contract.IdempotencyConflictError
	if (errors.As(err, &validation) && validation.Field == "command_id") || errors.As(err, &idempotency) {
		form.CommandID = data.TerminationForm.CommandID
	}
	data.TerminationForm = form
	s.renderContractDetailPage(w, status, data)
}

func (s *Server) showContractRelationNew(w http.ResponseWriter, r *http.Request) {
	data, err := s.loadContractRelationPage(w, r, r.PathValue("contractID"), r.URL.Query().Get("target_id"), contractRelationForm{})
	if err != nil {
		s.writeContractRelationLoadError(w, err)
		return
	}
	s.renderContractRelationPage(w, http.StatusOK, data)
}

func (s *Server) linkContractRelation(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	sourceContractID := r.PathValue("contractID")
	form := contractRelationForm{
		CommandID:              r.PostForm.Get("command_id"),
		TargetContractID:       r.PostForm.Get("target_contract_id"),
		RelationType:           r.PostForm.Get("relation_type"),
		ExpectedSourceRevision: parseDraftVersion(r.PostForm.Get("expected_source_revision")),
		ExpectedTargetRevision: parseDraftVersion(r.PostForm.Get("expected_target_revision")),
		EffectiveOn:            r.PostForm.Get("effective_on"),
		Note:                   r.PostForm.Get("note"),
	}
	_, err := s.contracts.LinkRelation(r.Context(), contract.LinkContractRelationInput{
		CommandID: form.CommandID, SourceContractID: sourceContractID, TargetContractID: form.TargetContractID,
		RelationType: form.RelationType, ExpectedSourceRevision: form.ExpectedSourceRevision,
		ExpectedTargetRevision: form.ExpectedTargetRevision, EffectiveOn: form.EffectiveOn,
		Note: form.Note, Actor: mustPrincipal(r).Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/contracts/"+sourceContractID+"?relation_linked=1", http.StatusSeeOther)
		return
	}
	status, message, known := contractRelationHTTPError(err)
	if !known {
		http.Error(w, "合同关系建立失败", http.StatusInternalServerError)
		return
	}
	var notFound *contract.ContractNotFoundError
	var stateConflict *contract.ContractRelationStateConflictError
	if errors.As(err, &notFound) || errors.As(err, &stateConflict) {
		http.Error(w, message, status)
		return
	}
	var validation *contract.ValidationError
	var idempotency *contract.IdempotencyConflictError
	if (errors.As(err, &validation) && validation.Field == "command_id") || errors.As(err, &idempotency) {
		form.CommandID = ""
	}
	data, loadErr := s.loadContractRelationPage(w, r, sourceContractID, form.TargetContractID, form)
	if loadErr != nil {
		s.writeContractRelationLoadError(w, loadErr)
		return
	}
	data.Error = message
	s.renderContractRelationPage(w, status, data)
}

func (s *Server) loadContractRelationPage(
	w http.ResponseWriter,
	r *http.Request,
	sourceContractID string,
	targetContractID string,
	form contractRelationForm,
) (contractRelationPageData, error) {
	sourceDetails, err := s.contracts.GetDraft(r.Context(), sourceContractID)
	if err != nil {
		return contractRelationPageData{}, err
	}
	if !formalContractStatus(sourceDetails.Contract.Status) {
		return contractRelationPageData{}, &contract.ContractRelationStateConflictError{Side: "source", Status: sourceDetails.Contract.Status}
	}
	statusFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if statusFilter == "" {
		statusFilter = string(contract.StatusActive)
	}
	if statusFilter != string(contract.StatusActive) && statusFilter != string(contract.StatusTerminated) && statusFilter != string(contract.StatusClosed) {
		return contractRelationPageData{}, &contract.ValidationError{Field: "status", Message: "关系目标只能筛选正式合同状态"}
	}
	searchPage, err := s.contracts.Search(r.Context(), contract.SearchContractsInput{
		Keyword: r.URL.Query().Get("q"), Status: statusFilter,
		Currency: r.URL.Query().Get("currency"), Cursor: r.URL.Query().Get("cursor"),
	})
	if err != nil {
		return contractRelationPageData{}, err
	}
	targets := make([]contract.Contract, 0, len(searchPage.Contracts))
	for _, candidate := range searchPage.Contracts {
		if candidate.ID != sourceDetails.Contract.ID {
			targets = append(targets, candidate)
		}
	}
	var target *contract.Contract
	if strings.TrimSpace(targetContractID) != "" {
		targetDetails, targetErr := s.contracts.GetDraft(r.Context(), targetContractID)
		if targetErr != nil {
			return contractRelationPageData{}, targetErr
		}
		targetValue := targetDetails.Contract
		target = &targetValue
	}
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		return contractRelationPageData{}, fmt.Errorf("create contract relation CSRF token: %w", err)
	}
	if form.CommandID == "" {
		form.CommandID, err = contract.NewCommandID()
		if err != nil {
			return contractRelationPageData{}, fmt.Errorf("create contract relation command ID: %w", err)
		}
	}
	form.TargetContractID = strings.TrimSpace(targetContractID)
	form.ExpectedSourceRevision = sourceDetails.Contract.CurrentRevision
	if form.RelationType == "" {
		form.RelationType = string(contract.RelationTypeReferences)
	}
	relationReady := target != nil && target.ID != sourceDetails.Contract.ID && formalContractStatus(target.Status)
	if target != nil {
		form.ExpectedTargetRevision = target.CurrentRevision
	}
	return contractRelationPageData{
		commonPageData: commonData(mustPrincipal(r), token, s.publicBasePath, s.externalIdentity), CSRFToken: token,
		Source: sourceDetails.Contract, Target: target,
		SearchForm: contractSearchForm{Keyword: searchPage.Keyword, Status: string(searchPage.Status), Currency: searchPage.Currency},
		Targets:    targets, NextURL: contractRelationTargetNextURL(sourceDetails.Contract.ID, searchPage),
		Form: form, RelationReady: relationReady,
	}, nil
}

func contractRelationTargetNextURL(sourceContractID string, page contract.ContractSearchPage) string {
	if page.NextCursor == "" {
		return ""
	}
	query := url.Values{"cursor": {page.NextCursor}, "status": {string(page.Status)}}
	if page.Keyword != "" {
		query.Set("q", page.Keyword)
	}
	if page.Currency != "" {
		query.Set("currency", page.Currency)
	}
	return "/contracts/" + sourceContractID + "/relations/new?" + query.Encode()
}

func formalContractStatus(status contract.Status) bool {
	return status == contract.StatusActive || status == contract.StatusTerminated || status == contract.StatusClosed
}

func contractRelationHTTPError(err error) (int, string, bool) {
	var validation *contract.ValidationError
	var notFound *contract.ContractNotFoundError
	var stateConflict *contract.ContractRelationStateConflictError
	var revisionConflict *contract.ContractRelationRevisionConflictError
	var duplicate *contract.DuplicateContractRelationError
	var cycle *contract.ContractRelationCycleError
	var idempotency *contract.IdempotencyConflictError
	switch {
	case errors.As(err, &validation):
		return http.StatusUnprocessableEntity, validation.Error(), true
	case errors.As(err, &notFound):
		return http.StatusNotFound, notFound.Error(), true
	case errors.As(err, &stateConflict):
		return http.StatusConflict, stateConflict.Error(), true
	case errors.As(err, &revisionConflict):
		return http.StatusConflict, revisionConflict.Error(), true
	case errors.As(err, &duplicate):
		return http.StatusConflict, duplicate.Error(), true
	case errors.As(err, &cycle):
		return http.StatusConflict, cycle.Error(), true
	case errors.As(err, &idempotency):
		return http.StatusConflict, idempotency.Error(), true
	default:
		return 0, "", false
	}
}

func (s *Server) writeContractRelationLoadError(w http.ResponseWriter, err error) {
	status, message, known := contractRelationHTTPError(err)
	if known {
		http.Error(w, message, status)
		return
	}
	http.Error(w, "无法读取合同关系页面", http.StatusInternalServerError)
}

func (s *Server) renderContractRelationPage(w http.ResponseWriter, status int, data contractRelationPageData) {
	var rendered bytes.Buffer
	if err := s.contractRelationPage.ExecuteTemplate(&rendered, "contract_relation.html", data); err != nil {
		http.Error(w, "页面渲染失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(rendered.Bytes())
}

func (s *Server) activateContract(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}

	contractID := r.PathValue("contractID")
	form := activationForm{
		CommandID:            r.PostForm.Get("command_id"),
		ExpectedDraftVersion: parseDraftVersion(r.PostForm.Get("expected_draft_version")),
	}
	_, err := s.contracts.Activate(r.Context(), contract.ActivateContractInput{
		CommandID:            form.CommandID,
		ContractID:           contractID,
		ExpectedDraftVersion: form.ExpectedDraftVersion,
		Actor:                mustPrincipal(r).Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/contracts/"+contractID+"?activated=1", http.StatusSeeOther)
		return
	}
	status, message, known := activationHTTPError(err)
	if !known {
		http.Error(w, "合同激活失败", http.StatusInternalServerError)
		return
	}
	var notFound *contract.ContractNotFoundError
	if errors.As(err, &notFound) {
		http.Error(w, message, status)
		return
	}
	data, loadErr := s.loadContractDetailPage(w, r, contractID)
	if loadErr != nil {
		s.writeContractDetailLoadError(w, loadErr)
		return
	}
	data.Error = message
	form.ExpectedDraftVersion = data.Contract.DraftVersion
	var validationError *contract.ValidationError
	var idempotencyConflict *contract.IdempotencyConflictError
	if (errors.As(err, &validationError) && validationError.Field == "command_id") || errors.As(err, &idempotencyConflict) {
		form.CommandID = data.ActivationForm.CommandID
	}
	data.ActivationForm = form
	s.renderContractDetailPage(w, status, data)
}

func (s *Server) updateContractDraft(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}

	contractID := r.PathValue("contractID")
	form := contractForm{
		CommandID:            r.PostForm.Get("command_id"),
		ExpectedDraftVersion: parseDraftVersion(r.PostForm.Get("expected_draft_version")),
		NumberNamespace:      r.PostForm.Get("number_namespace"),
		ContractNumber:       r.PostForm.Get("contract_number"),
		Title:                r.PostForm.Get("title"),
		Category:             r.PostForm.Get("category"),
		Currency:             r.PostForm.Get("currency"),
		TotalAmount:          r.PostForm.Get("total_amount"),
		SignedOn:             r.PostForm.Get("signed_on"),
		EffectiveOn:          r.PostForm.Get("effective_on"),
	}
	_, err := s.contracts.UpdateDraft(r.Context(), contract.UpdateDraftInput{
		CommandID:            form.CommandID,
		ContractID:           contractID,
		ExpectedDraftVersion: form.ExpectedDraftVersion,
		NumberNamespace:      form.NumberNamespace,
		ContractNumber:       form.ContractNumber,
		Title:                form.Title,
		Category:             form.Category,
		Currency:             form.Currency,
		TotalAmount:          form.TotalAmount,
		SignedOn:             form.SignedOn,
		EffectiveOn:          form.EffectiveOn,
		Actor:                mustPrincipal(r).Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/contracts/"+contractID+"?updated=1", http.StatusSeeOther)
		return
	}
	status, message, known := draftChangeHTTPError(err)
	if !known {
		http.Error(w, "合同草稿更新失败", http.StatusInternalServerError)
		return
	}
	var notFound *contract.ContractNotFoundError
	if errors.As(err, &notFound) {
		http.Error(w, message, status)
		return
	}
	data, loadErr := s.loadContractDetailPage(w, r, contractID)
	if loadErr != nil {
		s.writeContractDetailLoadError(w, loadErr)
		return
	}
	data.Error = message
	form.ExpectedDraftVersion = data.Contract.DraftVersion
	var validationError *contract.ValidationError
	var idempotencyConflict *contract.IdempotencyConflictError
	if (errors.As(err, &validationError) && validationError.Field == "command_id") || errors.As(err, &idempotencyConflict) {
		form.CommandID = data.EditForm.CommandID
	}
	data.EditForm = form
	s.renderContractDetailPage(w, status, data)
}

func (s *Server) cancelContractDraft(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}

	contractID := r.PathValue("contractID")
	form := cancelDraftForm{
		CommandID:            r.PostForm.Get("command_id"),
		Reason:               r.PostForm.Get("cancellation_reason"),
		ExpectedDraftVersion: parseDraftVersion(r.PostForm.Get("expected_draft_version")),
	}
	_, err := s.contracts.CancelDraft(r.Context(), contract.CancelDraftInput{
		CommandID:            form.CommandID,
		ContractID:           contractID,
		ExpectedDraftVersion: form.ExpectedDraftVersion,
		Reason:               form.Reason,
		Actor:                mustPrincipal(r).Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/contracts/"+contractID+"?cancelled=1", http.StatusSeeOther)
		return
	}
	status, message, known := draftChangeHTTPError(err)
	if !known {
		http.Error(w, "合同草稿取消失败", http.StatusInternalServerError)
		return
	}
	var notFound *contract.ContractNotFoundError
	if errors.As(err, &notFound) {
		http.Error(w, message, status)
		return
	}
	data, loadErr := s.loadContractDetailPage(w, r, contractID)
	if loadErr != nil {
		s.writeContractDetailLoadError(w, loadErr)
		return
	}
	data.Error = message
	form.ExpectedDraftVersion = data.Contract.DraftVersion
	var validationError *contract.ValidationError
	var idempotencyConflict *contract.IdempotencyConflictError
	if (errors.As(err, &validationError) && validationError.Field == "command_id") || errors.As(err, &idempotencyConflict) {
		form.CommandID = data.CancelForm.CommandID
	}
	data.CancelForm = form
	s.renderContractDetailPage(w, status, data)
}

func (s *Server) addContractDraftParty(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}

	contractID := r.PathValue("contractID")
	form := draftPartyForm{
		CommandID:            r.PostForm.Get("command_id"),
		PartyID:              r.PostForm.Get("party_id"),
		Role:                 r.PostForm.Get("role"),
		ExpectedDraftVersion: parseDraftVersion(r.PostForm.Get("expected_draft_version")),
	}
	_, err := s.contracts.AddDraftParty(r.Context(), contract.AddDraftPartyInput{
		CommandID:            form.CommandID,
		ContractID:           contractID,
		PartyID:              form.PartyID,
		Role:                 form.Role,
		ExpectedDraftVersion: form.ExpectedDraftVersion,
		Actor:                mustPrincipal(r).Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/contracts/"+contractID+"?party_added=1", http.StatusSeeOther)
		return
	}

	status, message, known := draftPartyHTTPError(err)
	if !known {
		http.Error(w, "签约主体添加失败", http.StatusInternalServerError)
		return
	}
	var notFound *contract.ContractNotFoundError
	if errors.As(err, &notFound) {
		http.Error(w, message, status)
		return
	}
	data, loadErr := s.loadContractDetailPage(w, r, contractID)
	if loadErr != nil {
		s.writeContractDetailLoadError(w, loadErr)
		return
	}
	data.Error = message
	form.ExpectedDraftVersion = data.Contract.DraftVersion
	var validationError *contract.ValidationError
	var idempotencyConflict *contract.IdempotencyConflictError
	if (errors.As(err, &validationError) && validationError.Field == "command_id") || errors.As(err, &idempotencyConflict) {
		form.CommandID = data.AddForm.CommandID
	}
	data.AddForm = form
	s.renderContractDetailPage(w, status, data)
}

func (s *Server) removeContractDraftParty(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}

	contractID := r.PathValue("contractID")
	_, err := s.contracts.RemoveDraftParty(r.Context(), contract.RemoveDraftPartyInput{
		CommandID:            r.PostForm.Get("command_id"),
		ContractID:           contractID,
		PartyLinkID:          r.PathValue("partyLinkID"),
		ExpectedDraftVersion: parseDraftVersion(r.PostForm.Get("expected_draft_version")),
		Actor:                mustPrincipal(r).Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/contracts/"+contractID+"?party_removed=1", http.StatusSeeOther)
		return
	}

	status, message, known := draftPartyHTTPError(err)
	if !known {
		http.Error(w, "签约主体移除失败", http.StatusInternalServerError)
		return
	}
	var notFound *contract.ContractNotFoundError
	if errors.As(err, &notFound) {
		http.Error(w, message, status)
		return
	}
	data, loadErr := s.loadContractDetailPage(w, r, contractID)
	if loadErr != nil {
		s.writeContractDetailLoadError(w, loadErr)
		return
	}
	data.Error = message
	s.renderContractDetailPage(w, status, data)
}

func (s *Server) loadContractDetailPage(w http.ResponseWriter, r *http.Request, contractID string) (contractDetailPageData, error) {
	principal := mustPrincipal(r)
	details, err := s.contracts.GetDraft(r.Context(), contractID)
	if err != nil {
		return contractDetailPageData{}, err
	}
	evidenceDocuments, err := s.evidence.List(r.Context(), details.Contract.ID)
	if err != nil {
		return contractDetailPageData{}, fmt.Errorf("list evidence for contract detail: %w", err)
	}
	availableParties, err := s.parties.List(r.Context())
	if err != nil {
		return contractDetailPageData{}, fmt.Errorf("list parties for contract draft: %w", err)
	}
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		return contractDetailPageData{}, fmt.Errorf("create contract detail CSRF token: %w", err)
	}
	addCommandID, err := contract.NewCommandID()
	if err != nil {
		return contractDetailPageData{}, fmt.Errorf("create add party command ID: %w", err)
	}
	editCommandID, err := contract.NewCommandID()
	if err != nil {
		return contractDetailPageData{}, fmt.Errorf("create update draft command ID: %w", err)
	}
	cancelCommandID, err := contract.NewCommandID()
	if err != nil {
		return contractDetailPageData{}, fmt.Errorf("create cancel draft command ID: %w", err)
	}
	activationCommandID, err := contract.NewCommandID()
	if err != nil {
		return contractDetailPageData{}, fmt.Errorf("create activation command ID: %w", err)
	}
	changeCommandID, err := contract.NewCommandID()
	if err != nil {
		return contractDetailPageData{}, fmt.Errorf("create contract change command ID: %w", err)
	}
	terminationCommandID, err := contract.NewCommandID()
	if err != nil {
		return contractDetailPageData{}, fmt.Errorf("create contract termination command ID: %w", err)
	}
	settlementCommandID, err := contract.NewCommandID()
	if err != nil {
		return contractDetailPageData{}, fmt.Errorf("create contract settlement command ID: %w", err)
	}
	closureCommandID, err := contract.NewCommandID()
	if err != nil {
		return contractDetailPageData{}, fmt.Errorf("create contract closure command ID: %w", err)
	}
	correctionCommandID, err := contract.NewCommandID()
	if err != nil {
		return contractDetailPageData{}, fmt.Errorf("create contract correction command ID: %w", err)
	}
	evidenceCommandID, err := contract.NewCommandID()
	if err != nil {
		return contractDetailPageData{}, fmt.Errorf("create contract evidence command ID: %w", err)
	}
	evidenceRows := make([]evidenceDocumentRow, 0, len(evidenceDocuments))
	for _, document := range evidenceDocuments {
		versionCommandID, err := contract.NewCommandID()
		if err != nil {
			return contractDetailPageData{}, fmt.Errorf("create evidence version command ID: %w", err)
		}
		evidenceRows = append(evidenceRows, evidenceDocumentRow{Document: document, VersionCommandID: versionCommandID})
	}
	rows := make([]draftPartyRow, 0, len(details.Parties))
	for _, linked := range details.Parties {
		removeCommandID, err := contract.NewCommandID()
		if err != nil {
			return contractDetailPageData{}, fmt.Errorf("create remove party command ID: %w", err)
		}
		rows = append(rows, draftPartyRow{Party: linked, RemoveCommandID: removeCommandID})
	}
	issues := activationIssues(details)
	displayRevision := details.Revision
	if displayRevision == nil && details.Contract.CurrentRevision > 0 {
		displayRevision = &contract.ContractRevision{
			ContractID: details.Contract.ID, Revision: details.Contract.CurrentRevision,
			NumberNamespace: details.Contract.NumberNamespace, ContractNumber: details.Contract.ContractNumber,
			Title: details.Contract.Title, Category: details.Contract.Category,
			Currency: details.Contract.Currency, TotalAmount: details.Contract.TotalAmount,
			SignedOn: details.Contract.SignedOn, EffectiveOn: details.Contract.EffectiveOn,
		}
	}
	var originalRevision *contract.ContractRevision
	if len(details.Revisions) > 0 {
		value := details.Revisions[0]
		originalRevision = &value
	} else if displayRevision != nil {
		value := *displayRevision
		originalRevision = &value
	}
	var latestSettlement *contract.ContractSettlement
	if len(details.Settlements) > 0 {
		value := details.Settlements[len(details.Settlements)-1]
		latestSettlement = &value
	}
	evidenceTargets := []evidenceTargetOption{{
		Value: string(evidence.TargetContract) + ":" + details.Contract.ID,
		Label: "合同整体 / " + details.Contract.ContractNumber,
	}}
	for _, instrument := range details.Instruments {
		kind := instrument.InstrumentType
		switch instrument.InstrumentType {
		case "original_contract":
			kind = "原合同"
		case "supplemental_agreement":
			kind = "补充协议"
		case "termination_instrument":
			kind = "终止文书"
		case "correction_instrument":
			kind = "更正文书"
		}
		evidenceTargets = append(evidenceTargets, evidenceTargetOption{
			Value: string(evidence.TargetInstrument) + ":" + instrument.ID,
			Label: kind + " / " + instrument.InstrumentNumber,
		})
	}
	for _, settlement := range details.Settlements {
		evidenceTargets = append(evidenceTargets, evidenceTargetOption{
			Value: string(evidence.TargetSettlement) + ":" + settlement.ID,
			Label: settlement.Kind.Label() + " / " + settlement.StatementNumber,
		})
	}
	if details.Closure != nil {
		evidenceTargets = append(evidenceTargets, evidenceTargetOption{
			Value: string(evidence.TargetClosure) + ":" + details.Closure.ID,
			Label: "操作性关闭 / " + details.Closure.ClosedOn,
		})
	}
	canRecordSettlement := principal.Can(identity.PermissionLifecycleManage) &&
		(details.Contract.Status == contract.StatusActive || details.Contract.Status == contract.StatusTerminated) &&
		(latestSettlement == nil || latestSettlement.Kind != contract.SettlementKindFinal)
	expectedSettlementSequence := int64(0)
	if latestSettlement != nil {
		expectedSettlementSequence = latestSettlement.Sequence
	}
	canClose := principal.Can(identity.PermissionLifecycleManage) &&
		(details.Contract.Status == contract.StatusActive || details.Contract.Status == contract.StatusTerminated)
	correction := correctionForm{
		CommandID: correctionCommandID, ExpectedStatus: string(details.Contract.Status),
		ExpectedRevision: details.Contract.CurrentRevision, EffectiveOrder: details.Contract.CurrentRevision + 1,
	}
	if displayRevision != nil {
		correction.CorrectedTitle = displayRevision.Title
		correction.CorrectedCategory = displayRevision.Category
		correction.CorrectedTotalAmount = displayRevision.TotalAmount
		correction.CorrectedSignedOn = displayRevision.SignedOn
		correction.CorrectedEffectiveOn = displayRevision.EffectiveOn
	}
	closureDisposition := ""
	if latestSettlement != nil && latestSettlement.Kind == contract.SettlementKindFinal {
		closureDisposition = string(contract.SettlementDispositionFinalRecorded)
	}
	return contractDetailPageData{
		commonPageData:    commonData(principal, token, s.publicBasePath, s.externalIdentity),
		CSRFToken:         token,
		Contract:          details.Contract,
		Cancellation:      details.Cancellation,
		Revision:          displayRevision,
		Revisions:         details.Revisions,
		Instruments:       details.Instruments,
		PartySnapshots:    details.PartySnapshots,
		AuditEntries:      details.AuditEntries,
		OutgoingRelations: details.OutgoingRelations,
		IncomingRelations: details.IncomingRelations,
		Settlements:       details.Settlements,
		OriginalRevision:  originalRevision,
		LatestSettlement:  latestSettlement,
		Closure:           details.Closure,
		EvidenceDocuments: evidenceRows,
		EvidenceTargets:   evidenceTargets,
		EvidenceForm: evidenceForm{
			CommandID: evidenceCommandID,
			Target:    string(evidence.TargetContract) + ":" + details.Contract.ID,
		},
		EvidenceMaxBytes: s.evidence.MaxUploadBytes(),
		DraftParties:     rows,
		AvailableParties: availableParties,
		AddForm: draftPartyForm{
			CommandID:            addCommandID,
			ExpectedDraftVersion: details.Contract.DraftVersion,
		},
		EditForm: contractForm{
			CommandID:            editCommandID,
			ExpectedDraftVersion: details.Contract.DraftVersion,
			NumberNamespace:      details.Contract.NumberNamespace,
			ContractNumber:       details.Contract.ContractNumber,
			Title:                details.Contract.Title,
			Category:             details.Contract.Category,
			Currency:             details.Contract.Currency,
			TotalAmount:          details.Contract.TotalAmount,
			SignedOn:             details.Contract.SignedOn,
			EffectiveOn:          details.Contract.EffectiveOn,
		},
		CancelForm: cancelDraftForm{
			CommandID:            cancelCommandID,
			ExpectedDraftVersion: details.Contract.DraftVersion,
		},
		ActivationForm: activationForm{
			CommandID:            activationCommandID,
			ExpectedDraftVersion: details.Contract.DraftVersion,
		},
		ChangeForm: contractChangeForm{
			CommandID:        changeCommandID,
			ExpectedRevision: details.Contract.CurrentRevision,
			EffectiveOrder:   details.Contract.CurrentRevision + 1,
			AmountEffect:     contract.AmountEffectUnchanged,
		},
		TerminationForm: terminationForm{
			CommandID:        terminationCommandID,
			ExpectedRevision: details.Contract.CurrentRevision,
			EffectiveOrder:   details.Contract.CurrentRevision + 1,
		},
		SettlementForm: settlementForm{
			CommandID:                  settlementCommandID,
			ExpectedContractRevision:   details.Contract.CurrentRevision,
			ExpectedSettlementSequence: expectedSettlementSequence,
			Kind:                       string(contract.SettlementKindProgress),
			SelectedPartyPositions:     make(map[int64]bool),
		},
		CanRecordSettlement: canRecordSettlement,
		ClosureForm: closureForm{
			CommandID:                  closureCommandID,
			ExpectedContractRevision:   details.Contract.CurrentRevision,
			ExpectedSettlementSequence: expectedSettlementSequence,
			SettlementDisposition:      closureDisposition,
		},
		CanClose:         canClose,
		CorrectionForm:   correction,
		ActivationActor:  principal.Actor(),
		ActivationIssues: issues,
		ActivationReady:  len(issues) == 0,
	}, nil
}

func activationIssues(details contract.DraftDetails) []string {
	if details.Contract.Status != contract.StatusDraft {
		return nil
	}
	issues := make([]string, 0, 3)
	if details.Contract.SignedOn == "" {
		issues = append(issues, "需要填写签署日期")
	}
	if details.Contract.EffectiveOn == "" {
		issues = append(issues, "需要填写生效日期")
	}
	distinctParties := make(map[string]struct{}, len(details.Parties))
	for _, linked := range details.Parties {
		distinctParties[linked.PartyID] = struct{}{}
	}
	if len(distinctParties) < 2 {
		issues = append(issues, "需要至少两个不同的签约主体")
	}
	return issues
}

func (s *Server) renderContractDetailPage(w http.ResponseWriter, status int, data contractDetailPageData) {
	var rendered bytes.Buffer
	if err := s.contractDetailPage.ExecuteTemplate(&rendered, "contract_detail.html", data); err != nil {
		http.Error(w, "页面渲染失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = rendered.WriteTo(w)
}

func (s *Server) writeContractDetailLoadError(w http.ResponseWriter, err error) {
	var notFound *contract.ContractNotFoundError
	var validationError *contract.ValidationError
	if errors.As(err, &notFound) || errors.As(err, &validationError) {
		http.Error(w, "合同不存在", http.StatusNotFound)
		return
	}
	http.Error(w, "无法读取合同草稿", http.StatusInternalServerError)
}

func draftPartyHTTPError(err error) (int, string, bool) {
	var validationError *contract.ValidationError
	var contractNotFound *contract.ContractNotFoundError
	var partyNotFound *contract.PartyNotFoundError
	var linkNotFound *contract.DraftPartyNotFoundError
	var stateConflict *contract.ContractStateConflictError
	var versionConflict *contract.DraftVersionConflictError
	var duplicate *contract.DuplicateDraftPartyError
	var idempotencyConflict *contract.IdempotencyConflictError
	switch {
	case errors.As(err, &validationError):
		return http.StatusUnprocessableEntity, validationError.Error(), true
	case errors.As(err, &contractNotFound):
		return http.StatusNotFound, contractNotFound.Error(), true
	case errors.As(err, &partyNotFound):
		return http.StatusUnprocessableEntity, partyNotFound.Error(), true
	case errors.As(err, &linkNotFound):
		return http.StatusNotFound, linkNotFound.Error(), true
	case errors.As(err, &stateConflict):
		return http.StatusConflict, stateConflict.Error(), true
	case errors.As(err, &versionConflict):
		return http.StatusConflict, versionConflict.Error(), true
	case errors.As(err, &duplicate):
		return http.StatusConflict, duplicate.Error(), true
	case errors.As(err, &idempotencyConflict):
		return http.StatusConflict, idempotencyConflict.Error(), true
	default:
		return 0, "", false
	}
}

func draftChangeHTTPError(err error) (int, string, bool) {
	var duplicateNumber *contract.DuplicateNumberError
	var noChanges *contract.NoDraftChangesError
	switch {
	case errors.As(err, &duplicateNumber):
		return http.StatusConflict, duplicateNumber.Error(), true
	case errors.As(err, &noChanges):
		return http.StatusUnprocessableEntity, noChanges.Error(), true
	default:
		return draftPartyHTTPError(err)
	}
}

func activationHTTPError(err error) (int, string, bool) {
	var precondition *contract.ActivationPreconditionError
	if errors.As(err, &precondition) {
		return http.StatusUnprocessableEntity, precondition.Error(), true
	}
	return draftPartyHTTPError(err)
}

func contractChangeHTTPError(err error) (int, string, bool) {
	var validation *contract.ValidationError
	var notFound *contract.ContractNotFoundError
	var stateConflict *contract.ContractChangeStateConflictError
	var revisionConflict *contract.ContractRevisionConflictError
	var duplicateNumber *contract.DuplicateInstrumentNumberError
	var precondition *contract.ContractChangePreconditionError
	var idempotency *contract.IdempotencyConflictError
	switch {
	case errors.As(err, &validation):
		return http.StatusUnprocessableEntity, validation.Error(), true
	case errors.As(err, &notFound):
		return http.StatusNotFound, notFound.Error(), true
	case errors.As(err, &stateConflict):
		return http.StatusConflict, stateConflict.Error(), true
	case errors.As(err, &revisionConflict):
		return http.StatusConflict, revisionConflict.Error(), true
	case errors.As(err, &duplicateNumber):
		return http.StatusConflict, duplicateNumber.Error(), true
	case errors.As(err, &precondition):
		return http.StatusUnprocessableEntity, precondition.Error(), true
	case errors.As(err, &idempotency):
		return http.StatusConflict, idempotency.Error(), true
	default:
		return 0, "", false
	}
}

func correctionHTTPError(err error) (int, string, bool) {
	var validation *contract.ValidationError
	var notFound *contract.ContractNotFoundError
	var stateConflict *contract.ContractCorrectionStateConflictError
	var revisionConflict *contract.ContractRevisionConflictError
	var duplicateNumber *contract.DuplicateInstrumentNumberError
	var precondition *contract.ContractCorrectionPreconditionError
	var noChanges *contract.ContractCorrectionNoChangesError
	var idempotency *contract.IdempotencyConflictError
	switch {
	case errors.As(err, &validation):
		return http.StatusUnprocessableEntity, validation.Error(), true
	case errors.As(err, &notFound):
		return http.StatusNotFound, notFound.Error(), true
	case errors.As(err, &stateConflict):
		return http.StatusConflict, stateConflict.Error(), true
	case errors.As(err, &revisionConflict):
		return http.StatusConflict, revisionConflict.Error(), true
	case errors.As(err, &duplicateNumber):
		return http.StatusConflict, duplicateNumber.Error(), true
	case errors.As(err, &precondition):
		return http.StatusUnprocessableEntity, precondition.Error(), true
	case errors.As(err, &noChanges):
		return http.StatusUnprocessableEntity, noChanges.Error(), true
	case errors.As(err, &idempotency):
		return http.StatusConflict, idempotency.Error(), true
	default:
		return 0, "", false
	}
}

func terminationHTTPError(err error) (int, string, bool) {
	var validation *contract.ValidationError
	var notFound *contract.ContractNotFoundError
	var stateConflict *contract.ContractTerminationStateConflictError
	var revisionConflict *contract.ContractRevisionConflictError
	var duplicateNumber *contract.DuplicateInstrumentNumberError
	var precondition *contract.ContractTerminationPreconditionError
	var idempotency *contract.IdempotencyConflictError
	switch {
	case errors.As(err, &validation):
		return http.StatusUnprocessableEntity, validation.Error(), true
	case errors.As(err, &notFound):
		return http.StatusNotFound, notFound.Error(), true
	case errors.As(err, &stateConflict):
		return http.StatusConflict, stateConflict.Error(), true
	case errors.As(err, &revisionConflict):
		return http.StatusConflict, revisionConflict.Error(), true
	case errors.As(err, &duplicateNumber):
		return http.StatusConflict, duplicateNumber.Error(), true
	case errors.As(err, &precondition):
		return http.StatusUnprocessableEntity, precondition.Error(), true
	case errors.As(err, &idempotency):
		return http.StatusConflict, idempotency.Error(), true
	default:
		return 0, "", false
	}
}

func settlementHTTPError(err error) (int, string, bool) {
	var validation *contract.ValidationError
	var notFound *contract.ContractNotFoundError
	var stateConflict *contract.ContractSettlementStateConflictError
	var revisionConflict *contract.ContractRevisionConflictError
	var sequenceConflict *contract.ContractSettlementSequenceConflictError
	var finalized *contract.ContractSettlementFinalizedError
	var duplicateNumber *contract.DuplicateSettlementNumberError
	var partyError *contract.ContractSettlementPartyError
	var idempotency *contract.IdempotencyConflictError
	switch {
	case errors.As(err, &validation):
		return http.StatusUnprocessableEntity, validation.Error(), true
	case errors.As(err, &notFound):
		return http.StatusNotFound, notFound.Error(), true
	case errors.As(err, &partyError):
		return http.StatusUnprocessableEntity, partyError.Error(), true
	case errors.As(err, &stateConflict):
		return http.StatusConflict, stateConflict.Error(), true
	case errors.As(err, &revisionConflict):
		return http.StatusConflict, revisionConflict.Error(), true
	case errors.As(err, &sequenceConflict):
		return http.StatusConflict, sequenceConflict.Error(), true
	case errors.As(err, &finalized):
		return http.StatusConflict, finalized.Error(), true
	case errors.As(err, &duplicateNumber):
		return http.StatusConflict, duplicateNumber.Error(), true
	case errors.As(err, &idempotency):
		return http.StatusConflict, idempotency.Error(), true
	default:
		return 0, "", false
	}
}

func closureHTTPError(err error) (int, string, bool) {
	var validation *contract.ValidationError
	var notFound *contract.ContractNotFoundError
	var stateConflict *contract.ContractClosureStateConflictError
	var revisionConflict *contract.ContractRevisionConflictError
	var sequenceConflict *contract.ContractSettlementSequenceConflictError
	var settlementConflict *contract.ContractClosureSettlementConflictError
	var precondition *contract.ContractClosurePreconditionError
	var idempotency *contract.IdempotencyConflictError
	switch {
	case errors.As(err, &validation):
		return http.StatusUnprocessableEntity, validation.Error(), true
	case errors.As(err, &notFound):
		return http.StatusNotFound, notFound.Error(), true
	case errors.As(err, &precondition):
		return http.StatusUnprocessableEntity, precondition.Error(), true
	case errors.As(err, &stateConflict):
		return http.StatusConflict, stateConflict.Error(), true
	case errors.As(err, &revisionConflict):
		return http.StatusConflict, revisionConflict.Error(), true
	case errors.As(err, &sequenceConflict):
		return http.StatusConflict, sequenceConflict.Error(), true
	case errors.As(err, &settlementConflict):
		return http.StatusConflict, settlementConflict.Error(), true
	case errors.As(err, &idempotency):
		return http.StatusConflict, idempotency.Error(), true
	default:
		return 0, "", false
	}
}

func parseDraftVersion(value string) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func parseSettlementSequence(value string) int64 {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return -1
	}
	parsed, err := strconv.ParseInt(normalized, 10, 64)
	if err != nil {
		return -1
	}
	return parsed
}

type contractForm struct {
	CommandID            string
	ExpectedDraftVersion int64
	NumberNamespace      string
	ContractNumber       string
	Title                string
	Category             string
	Currency             string
	TotalAmount          string
	SignedOn             string
	EffectiveOn          string
}

type contractPageData struct {
	commonPageData
	CSRFToken  string
	Created    bool
	Error      string
	Form       contractForm
	SearchForm contractSearchForm
	Contracts  []contract.Contract
	NextURL    string
	HasFilters bool
}

type contractSearchForm struct {
	Keyword  string
	Status   string
	Currency string
}

func (s *Server) listContracts(w http.ResponseWriter, r *http.Request) {
	input := contract.SearchContractsInput{
		Keyword: r.URL.Query().Get("q"), Currency: r.URL.Query().Get("currency"),
		Status: r.URL.Query().Get("status"), Cursor: r.URL.Query().Get("cursor"),
	}
	page, searchErr := s.contracts.Search(r.Context(), input)
	status := http.StatusOK
	errorMessage := ""
	searchForm := contractSearchForm{Keyword: page.Keyword, Status: string(page.Status), Currency: page.Currency}
	if searchErr != nil {
		var validation *contract.ValidationError
		if !errors.As(searchErr, &validation) {
			http.Error(w, "无法读取合同列表", http.StatusInternalServerError)
			return
		}
		status = http.StatusUnprocessableEntity
		errorMessage = validation.Error()
		searchForm = contractSearchForm{Keyword: input.Keyword, Status: input.Status, Currency: input.Currency}
	}
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	commandID, err := contract.NewCommandID()
	if err != nil {
		http.Error(w, "无法建立创建请求", http.StatusInternalServerError)
		return
	}
	s.renderContractPage(w, status, contractPageData{
		commonPageData: commonData(mustPrincipal(r), token, s.publicBasePath, s.externalIdentity),
		CSRFToken:      token,
		Created:        r.URL.Query().Get("created") == "1",
		Error:          errorMessage,
		Form: contractForm{
			CommandID:       commandID,
			NumberNamespace: "default",
			Currency:        "CNY",
		},
		SearchForm: searchForm,
		Contracts:  page.Contracts,
		NextURL:    contractSearchNextURL(page),
		HasFilters: strings.TrimSpace(input.Keyword) != "" || strings.TrimSpace(input.Status) != "" ||
			strings.TrimSpace(input.Currency) != "" || strings.TrimSpace(input.Cursor) != "",
	})
}

func contractSearchNextURL(page contract.ContractSearchPage) string {
	if page.NextCursor == "" {
		return ""
	}
	query := url.Values{"cursor": {page.NextCursor}}
	if page.Keyword != "" {
		query.Set("q", page.Keyword)
	}
	if page.Status != "" {
		query.Set("status", string(page.Status))
	}
	if page.Currency != "" {
		query.Set("currency", page.Currency)
	}
	return "/contracts?" + query.Encode()
}

func (s *Server) createContractDraft(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}

	form := contractForm{
		CommandID:       r.PostForm.Get("command_id"),
		NumberNamespace: r.PostForm.Get("number_namespace"),
		ContractNumber:  r.PostForm.Get("contract_number"),
		Title:           r.PostForm.Get("title"),
		Category:        r.PostForm.Get("category"),
		Currency:        r.PostForm.Get("currency"),
		TotalAmount:     r.PostForm.Get("total_amount"),
		SignedOn:        r.PostForm.Get("signed_on"),
		EffectiveOn:     r.PostForm.Get("effective_on"),
	}
	created, err := s.contracts.CreateDraft(r.Context(), contract.CreateDraftInput{
		CommandID:       form.CommandID,
		NumberNamespace: form.NumberNamespace,
		ContractNumber:  form.ContractNumber,
		Title:           form.Title,
		Category:        form.Category,
		Currency:        form.Currency,
		TotalAmount:     form.TotalAmount,
		SignedOn:        form.SignedOn,
		EffectiveOn:     form.EffectiveOn,
		Actor:           mustPrincipal(r).Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/contracts?created=1&contract_id="+created.ID, http.StatusSeeOther)
		return
	}

	status := http.StatusUnprocessableEntity
	message := ""
	var validationError *contract.ValidationError
	var duplicateNumberError *contract.DuplicateNumberError
	var idempotencyConflictError *contract.IdempotencyConflictError
	switch {
	case errors.As(err, &validationError):
		message = validationError.Message
		if validationError.Field == "command_id" {
			newCommandID, generateErr := contract.NewCommandID()
			if generateErr != nil {
				http.Error(w, "无法建立创建请求", http.StatusInternalServerError)
				return
			}
			form.CommandID = newCommandID
		}
	case errors.As(err, &duplicateNumberError):
		status = http.StatusConflict
		message = duplicateNumberError.Error()
	case errors.As(err, &idempotencyConflictError):
		status = http.StatusConflict
		message = idempotencyConflictError.Error()
	default:
		http.Error(w, "合同草稿创建失败", http.StatusInternalServerError)
		return
	}

	page, listErr := s.contracts.Search(r.Context(), contract.SearchContractsInput{})
	if listErr != nil {
		http.Error(w, "合同草稿创建失败", http.StatusInternalServerError)
		return
	}
	token, tokenErr := s.ensureCSRFToken(w, r)
	if tokenErr != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	s.renderContractPage(w, status, contractPageData{
		commonPageData: commonData(mustPrincipal(r), token, s.publicBasePath, s.externalIdentity),
		CSRFToken:      token,
		Error:          message,
		Form:           form,
		Contracts:      page.Contracts,
		NextURL:        contractSearchNextURL(page),
	})
}

func (s *Server) renderContractPage(w http.ResponseWriter, status int, data contractPageData) {
	var rendered bytes.Buffer
	if err := s.contractPage.ExecuteTemplate(&rendered, "contracts.html", data); err != nil {
		http.Error(w, "页面渲染失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = rendered.WriteTo(w)
}

type partyForm struct {
	Kind                   string
	LegalName              string
	RegistrationIdentifier string
}

type partyPageData struct {
	commonPageData
	CSRFToken string
	Created   bool
	Error     string
	Form      partyForm
	Parties   []party.Party
}

func (s *Server) listParties(w http.ResponseWriter, r *http.Request) {
	parties, err := s.parties.List(r.Context())
	if err != nil {
		http.Error(w, "无法读取主体列表", http.StatusInternalServerError)
		return
	}
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	s.renderPartyPage(w, http.StatusOK, partyPageData{
		commonPageData: commonData(mustPrincipal(r), token, s.publicBasePath, s.externalIdentity),
		CSRFToken:      token,
		Created:        r.URL.Query().Get("created") == "1",
		Form:           partyForm{Kind: string(party.KindOrganization)},
		Parties:        parties,
	})
}

func (s *Server) createParty(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	if s.partyRuntime != nil {
		allowed, err := s.partyRuntime.LocalPartyWritesAllowed(r.Context(), mustPrincipal(r).TenantID)
		if err != nil {
			http.Error(w, "主体目录状态暂时不可用", http.StatusServiceUnavailable)
			return
		}
		if !allowed {
			http.Error(w, "主体目录已切换到 Party Directory，本地主体登记保持冻结", http.StatusConflict)
			return
		}
	}

	form := partyForm{
		Kind:                   r.PostForm.Get("kind"),
		LegalName:              r.PostForm.Get("legal_name"),
		RegistrationIdentifier: r.PostForm.Get("registration_identifier"),
	}
	_, err := s.parties.Create(r.Context(), party.CreateInput{
		Kind:                   form.Kind,
		LegalName:              form.LegalName,
		RegistrationIdentifier: form.RegistrationIdentifier,
		Actor:                  mustPrincipal(r).Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/parties?created=1", http.StatusSeeOther)
		return
	}

	var validationError *party.ValidationError
	if !errors.As(err, &validationError) {
		http.Error(w, "主体创建失败", http.StatusInternalServerError)
		return
	}
	parties, listErr := s.parties.List(r.Context())
	if listErr != nil {
		http.Error(w, "主体创建失败", http.StatusInternalServerError)
		return
	}
	token, tokenErr := s.ensureCSRFToken(w, r)
	if tokenErr != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	s.renderPartyPage(w, http.StatusUnprocessableEntity, partyPageData{
		commonPageData: commonData(mustPrincipal(r), token, s.publicBasePath, s.externalIdentity),
		CSRFToken:      token,
		Error:          validationError.Message,
		Form:           form,
		Parties:        parties,
	})
}

func (s *Server) renderPartyPage(w http.ResponseWriter, status int, data partyPageData) {
	var rendered bytes.Buffer
	if err := s.partyPage.ExecuteTemplate(&rendered, "parties.html", data); err != nil {
		http.Error(w, "页面渲染失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = rendered.WriteTo(w)
}

const csrfCookieName = "contract_ledger_csrf"

func (s *Server) ensureCSRFToken(w http.ResponseWriter, r *http.Request) (string, error) {
	if cookie, err := r.Cookie(csrfCookieName); err == nil && validCSRFTokenShape(cookie.Value) {
		return cookie.Value, nil
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.sessionCookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int((12 * time.Hour).Seconds()),
	})
	return token, nil
}

func validCSRFToken(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || !validCSRFTokenShape(cookie.Value) {
		return false
	}
	formToken := strings.TrimSpace(r.PostForm.Get("csrf_token"))
	if !validCSRFTokenShape(formToken) || len(cookie.Value) != len(formToken) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(formToken)) == 1
}

func validCSRFTokenShape(token string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(decoded) == 32
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
