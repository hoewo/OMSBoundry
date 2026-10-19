package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/projects"
)

const maxRequestBytes = 1 << 20

type UseCases interface {
	CounterpartyNameSuggestions(context.Context, string, projects.CounterpartyNamesQuery) (projects.CounterpartyNames, error)
	CounterpartyContext(context.Context, string, string) (projects.CounterpartyContextView, error)
	ChangeCounterpartyContext(context.Context, projects.Actor, string, projects.CounterpartyContextInput) (projects.CounterpartyContextView, error)
	Ping(context.Context) error
	List(context.Context, string, int) ([]projects.Project, error)
	Directory(context.Context, string, projects.DirectoryQuery) (projects.DirectoryPage, error)
	Create(context.Context, projects.Actor, projects.CreateInput) (projects.Project, error)
	DispatchInitiation(context.Context, projects.Actor, projects.InitiationDispatchInput) (projects.Project, error)
	CreatePackage(context.Context, projects.Actor, projects.PackageCreateInput) (projects.Project, error)
	ConvertPackage(context.Context, projects.Actor, string, projects.PackageConversionCommand) (projects.PackageConversionResult, error)
	RollbackPackageConversion(context.Context, projects.Actor, string, string, projects.PackageRollbackCommand) (projects.PackageConversionResult, error)
	Update(context.Context, projects.Actor, string, projects.UpdateInput) (projects.Project, error)
	CorrectProfile(context.Context, projects.Actor, string, projects.ProfileCorrectionInput) (projects.Project, error)
	ChangeBasicInfo(context.Context, projects.Actor, string, projects.BasicInfoChangeInput) (projects.BasicInfoChangeResult, error)
	ChangeInitiationInfo(context.Context, projects.Actor, string, projects.BasicInfoChangeInput) (projects.BasicInfoChangeResult, error)
	ApplyAssignment(context.Context, projects.Actor, string, projects.AssignmentCommand) (projects.AssignmentResult, error)
	ApplyResourceLink(context.Context, projects.Actor, string, projects.ResourceLinkCommand) (projects.ResourceLinkResult, error)
	CompleteInitiation(context.Context, projects.Actor, string, projects.RevisionCommand) (projects.Project, error)
	Cancel(context.Context, projects.Actor, string, projects.CancelCommand) (projects.Project, error)
	Get(context.Context, string, string, bool) (projects.Details, error)
	CommercialSummary(context.Context, string, string) (projects.CommercialSummary, error)
	Materials(context.Context, string, string) (projects.MaterialCollection, error)
	History(context.Context, string, string, string, int) (projects.HistoryPage, error)
	ReferenceProject(context.Context, string, string) (projects.ProjectReference, error)
	ReferenceAssignment(context.Context, string, string, string) (projects.AssignmentReference, error)
	EvaluateCompletion(context.Context, string, string, string) (projects.CompletionAssessment, error)
	ConfirmCompletion(context.Context, projects.Actor, string, projects.CompletionCommand) (projects.CompletionConfirmation, error)
}

type Server struct {
	useCases      UseCases
	authenticator auth.Authenticator
	handler       http.Handler
	oaProduction  OAProductionUseCases
}

type principalKey struct{}
type requestIDKey struct{}

type projectListResponse struct {
	Items          []projects.Project `json:"items"`
	AllowedActions []string           `json:"allowed_actions"`
}

type projectDetailsResponse struct {
	projects.Details
	AllowedActions []string `json:"allowed_actions"`
}

type completionAssessmentResponse struct {
	projects.CompletionAssessment
	AllowedActions []string `json:"allowed_actions"`
}

func New(useCases UseCases, authenticator auth.Authenticator) (*Server, error) {
	return NewWithOAProduction(useCases, authenticator, nil)
}

func NewWithOAProduction(useCases UseCases, authenticator auth.Authenticator, oa OAProductionUseCases) (*Server, error) {
	if useCases == nil || authenticator == nil {
		return nil, fmt.Errorf("use cases and authenticator are required")
	}
	s := &Server{useCases: useCases, authenticator: authenticator, oaProduction: oa}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", s.live)
	mux.HandleFunc("GET /health/ready", s.ready)
	mux.HandleFunc("GET /assets/demo.js", projectDemoScript)
	mux.HandleFunc("GET /assets/completion.js", projectCompletionScript)
	mux.HandleFunc("GET /completion/", s.completionPage)
	mux.HandleFunc("GET /", s.index)
	mux.Handle("GET /api/v1/projects", s.protected(auth.ActionRead, false, http.HandlerFunc(s.list)))
	mux.HandleFunc("GET /api/v1/project-directory", s.directory)
	mux.HandleFunc("GET /api/v1/counterparty-context/name-suggestions", s.counterpartyNameSuggestions)
	mux.Handle("POST /api/v1/projects", s.protected(auth.ActionCreate, false, http.HandlerFunc(s.create)))
	mux.Handle("POST /api/v1/initiation-dispatches", s.protectedAny([]string{auth.ActionCreate, auth.ActionHierarchyManage}, false, http.HandlerFunc(s.dispatchInitiation)))
	mux.Handle("POST /api/v1/project-packages", s.protected(auth.ActionHierarchyManage, false, http.HandlerFunc(s.createPackage)))
	mux.Handle("POST /api/v1/projects/{project_id}/package-conversions", s.protected(auth.ActionHierarchyManage, false, http.HandlerFunc(s.convertPackage)))
	mux.Handle("POST /api/v1/projects/{project_id}/package-conversions/{conversion_id}/rollback", s.protected(auth.ActionHierarchyManage, false, http.HandlerFunc(s.rollbackPackageConversion)))
	mux.Handle("PUT /api/v1/projects/{project_id}", s.protected(auth.ActionUpdate, true, http.HandlerFunc(s.update)))
	mux.Handle("POST /api/v1/projects/{project_id}/profile-corrections", s.protected(auth.ActionProfileCorrect, true, http.HandlerFunc(s.correctProfile)))
	mux.Handle("POST /api/v1/projects/{project_id}/basic-info-changes", s.protectedAny([]string{auth.ActionProfileCorrect, auth.ActionAssignmentManage, auth.ActionAssignmentBusinessOwnerManage, auth.ActionAssignmentProductionOwnerManage}, true, http.HandlerFunc(s.changeBasicInfo)))
	mux.Handle("POST /api/v1/projects/{project_id}/initiation-info-changes", s.protectedAny([]string{auth.ActionUpdate, auth.ActionAssignmentManage, auth.ActionAssignmentBusinessOwnerManage, auth.ActionAssignmentProductionOwnerManage}, true, http.HandlerFunc(s.changeInitiationInfo)))
	mux.Handle("GET /api/v1/projects/{project_id}/counterparty-context", s.protected(auth.ActionCounterpartyContextRead, true, http.HandlerFunc(s.counterpartyContext)))
	mux.Handle("POST /api/v1/projects/{project_id}/counterparty-context", s.protected(auth.ActionCounterpartyContextManage, true, http.HandlerFunc(s.changeCounterpartyContext)))
	mux.Handle("POST /api/v1/projects/{project_id}/assignments", s.protectedAny([]string{
		auth.ActionAssignmentManage,
		auth.ActionAssignmentBusinessOwnerManage,
		auth.ActionAssignmentProductionOwnerManage,
		auth.ActionAssignmentParticipantManage,
		auth.ActionAssignmentCommercialContributorManage,
	}, true, http.HandlerFunc(s.assignment)))
	mux.Handle("POST /api/v1/projects/{project_id}/resource-links", s.protected(auth.ActionResourceManage, true, http.HandlerFunc(s.resourceLink)))
	mux.Handle("POST /api/v1/projects/{project_id}/complete-initiation", s.protected(auth.ActionComplete, true, http.HandlerFunc(s.complete)))
	mux.Handle("POST /api/v1/projects/{project_id}/cancel", s.protected(auth.ActionCancel, true, http.HandlerFunc(s.cancel)))
	mux.Handle("GET /api/v1/projects/{project_id}", s.protected(auth.ActionRead, true, http.HandlerFunc(s.get)))
	mux.Handle("GET /api/v1/projects/{project_id}/commercial-summary", s.protected(auth.ActionCommercialSummaryRead, true, http.HandlerFunc(s.commercialSummary)))
	mux.Handle("GET /api/v1/projects/{project_id}/materials", s.protected(auth.ActionMaterialsRead, true, http.HandlerFunc(s.materials)))
	mux.Handle("GET /api/v1/projects/{project_id}/history", s.protected(auth.ActionHistoryRead, true, http.HandlerFunc(s.history)))
	mux.Handle("GET /api/v1/projects/{project_id}/completion-assessment", s.protected(auth.ActionCompletionRead, true, http.HandlerFunc(s.completionAssessment)))
	mux.Handle("POST /api/v1/projects/{project_id}/completion-confirmations", s.protected(auth.ActionCompletionConfirm, true, http.HandlerFunc(s.confirmCompletion)))
	mux.Handle("GET /api/v1/references/projects/{project_id}", s.protected(auth.ActionRead, true, http.HandlerFunc(s.referenceProject)))
	mux.Handle("GET /api/v1/references/projects/{project_id}/assignments/{assignment_id}", s.protected(auth.ActionRead, true, http.HandlerFunc(s.referenceAssignment)))
	mux.Handle("GET /api/v1/policy-facts/projects/{project_id}/assignments/{assignment_id}", s.protected(auth.ActionPolicyFactRead, true, http.HandlerFunc(s.referenceAssignment)))
	if oa != nil {
		s.registerOAProduction(mux)
	}
	s.handler = requestContext(securityHeaders(mux))
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"service": "project-workspace", "status": "live"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.useCases.Ping(r.Context()); err != nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "not_ready", "数据库暂不可用", true, nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"service": "project-workspace", "status": "ready"})
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	renderProjectOverview(w)
}

func (s *Server) completionPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/completion/" {
		http.NotFound(w, r)
		return
	}
	renderProjectCompletion(w)
}

func (s *Server) protected(action string, projectScoped bool, next http.Handler) http.Handler {
	return s.protectedAny([]string{action}, projectScoped, next)
}

func (s *Server) protectedAny(actions []string, projectScoped bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stripUntrustedIdentityHeaders(r.Header)
		principal, err := s.authenticator.Authenticate(r)
		if err != nil {
			writeProblem(w, r, http.StatusUnauthorized, "credential_required", "需要有效访问凭证", false, nil)
			return
		}
		allowed := false
		for _, action := range actions {
			if principal.Can(action) {
				allowed = true
				break
			}
		}
		if !allowed {
			writeProblem(w, r, http.StatusForbidden, "capability_denied", "缺少所需业务能力", false, nil)
			return
		}
		if projectScoped {
			projectID := r.PathValue("project_id")
			if !principal.CanProject(projectID) {
				writeProblem(w, r, http.StatusForbidden, "scope_denied", "凭证范围不包含该 Project", false, nil)
				return
			}
		} else if principal.ScopeKind != "tenant" {
			writeProblem(w, r, http.StatusForbidden, "scope_denied", "创建 Project 需要租户级凭证", false, nil)
			return
		}
		ctx := context.WithValue(r.Context(), principalKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var input projects.CreateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.Create(r.Context(), actorFrom(r), input)
	s.writeResult(w, r, http.StatusCreated, value, err)
}

func (s *Server) createPackage(w http.ResponseWriter, r *http.Request) {
	var input projects.PackageCreateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.CreatePackage(r.Context(), actorFrom(r), input)
	s.writeResult(w, r, http.StatusCreated, value, err)
}

func (s *Server) convertPackage(w http.ResponseWriter, r *http.Request) {
	var input projects.PackageConversionCommand
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.ConvertPackage(r.Context(), actorFrom(r), r.PathValue("project_id"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) rollbackPackageConversion(w http.ResponseWriter, r *http.Request) {
	var input projects.PackageRollbackCommand
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.RollbackPackageConversion(r.Context(), actorFrom(r), r.PathValue("project_id"), r.PathValue("conversion_id"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeProblem(w, r, http.StatusBadRequest, "invalid_request", "limit 必须是整数", false, nil)
			return
		}
		limit = value
	}
	principal := principalFrom(r)
	items, err := s.useCases.List(r.Context(), principal.TenantID, limit)
	s.writeResult(w, r, http.StatusOK, projectListResponse{Items: items, AllowedActions: credentialActions(principal)}, err)
}

func (s *Server) update(w http.ResponseWriter, r *http.Request) {
	var input projects.UpdateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.Update(r.Context(), actorFrom(r), r.PathValue("project_id"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) correctProfile(w http.ResponseWriter, r *http.Request) {
	var input projects.ProfileCorrectionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.CorrectProfile(r.Context(), actorFrom(r), r.PathValue("project_id"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) assignment(w http.ResponseWriter, r *http.Request) {
	var input projects.AssignmentCommand
	if !decodeJSON(w, r, &input) {
		return
	}
	principal := principalFrom(r)
	input.Operation = strings.ToLower(strings.TrimSpace(input.Operation))
	input.RoleKey = strings.ToLower(strings.TrimSpace(input.RoleKey))
	roleKey := input.RoleKey
	if input.Operation == "end" {
		value, err := s.useCases.ReferenceAssignment(r.Context(), principal.TenantID, r.PathValue("project_id"), input.AssignmentID)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		roleKey = value.RoleKey
	}
	// A newly introduced participation purpose requires an explicit grant.
	// Old broad assignment grants retain exactly their three-role scope.
	allowed := principal.Can(assignmentManageAction(roleKey))
	if roleKey != projects.RoleCommercialContributor {
		allowed = allowed || principal.Can(auth.ActionAssignmentManage)
	}
	if !allowed {
		writeProblem(w, r, http.StatusForbidden, "capability_denied", "缺少该分工类型的管理能力", false, nil)
		return
	}
	value, err := s.useCases.ApplyAssignment(r.Context(), actorFrom(r), r.PathValue("project_id"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func assignmentManageAction(roleKey string) string {
	switch roleKey {
	case projects.RoleBusinessOwner:
		return auth.ActionAssignmentBusinessOwnerManage
	case projects.RoleProductionOwner:
		return auth.ActionAssignmentProductionOwnerManage
	case projects.RoleParticipant:
		return auth.ActionAssignmentParticipantManage
	case projects.RoleCommercialContributor:
		return auth.ActionAssignmentCommercialContributorManage
	default:
		return ""
	}
}

func (s *Server) resourceLink(w http.ResponseWriter, r *http.Request) {
	var input projects.ResourceLinkCommand
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.ApplyResourceLink(r.Context(), actorFrom(r), r.PathValue("project_id"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) complete(w http.ResponseWriter, r *http.Request) {
	var input projects.RevisionCommand
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.CompleteInitiation(r.Context(), actorFrom(r), r.PathValue("project_id"), input)
	var incomplete *projects.InitiationError
	if errors.As(err, &incomplete) {
		writeProblem(w, r, http.StatusUnprocessableEntity, "initiation_incomplete", "立项资料尚未补全", false, map[string]any{"project": value, "missing": incomplete.Missing})
		return
	}
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
	var input projects.CancelCommand
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.Cancel(r.Context(), actorFrom(r), r.PathValue("project_id"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	includeInactive := false
	if raw := r.URL.Query().Get("include_inactive"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			writeProblem(w, r, http.StatusBadRequest, "invalid_request", "include_inactive 必须是布尔值", false, nil)
			return
		}
		includeInactive = value
	}
	principal := principalFrom(r)
	value, err := s.useCases.Get(r.Context(), principal.TenantID, r.PathValue("project_id"), includeInactive)
	if value.Parent != nil && !principal.CanProject(value.Parent.ID) {
		value.Parent = nil
	}
	children := make([]projects.ProjectSummary, 0, len(value.Children))
	for _, child := range value.Children {
		if principal.CanProject(child.ID) {
			children = append(children, child)
		}
	}
	value.Children = children
	s.writeResult(w, r, http.StatusOK, projectDetailsResponse{Details: value, AllowedActions: detailsAllowedActions(principal, value)}, err)
}

func (s *Server) commercialSummary(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	value, err := s.useCases.CommercialSummary(r.Context(), principal.TenantID, r.PathValue("project_id"))
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) materials(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	value, err := s.useCases.Materials(r.Context(), principal.TenantID, r.PathValue("project_id"))
	s.writeResult(w, r, http.StatusOK, value, err)
}

func credentialActions(principal auth.Principal) []string {
	actions := make([]string, 0, len(auth.AllActions))
	for _, action := range auth.AllActions {
		if principal.Can(action) {
			actions = append(actions, action)
		}
	}
	return actions
}

func detailsAllowedActions(principal auth.Principal, details projects.Details) []string {
	project := details.Project
	mutable := project.Status != projects.StatusCancelled
	initializing := project.Status == projects.StatusDraft || project.Status == projects.StatusPendingCompletion
	hierarchyMutable := mutable && principal.ScopeKind == "tenant" && ((project.NodeType == projects.NodeTypeProject && project.ParentProjectID == nil) || details.ActivePackageConversion != nil)
	applicable := map[string]bool{
		auth.ActionCounterpartyContextRead:               true,
		auth.ActionCounterpartyContextManage:             project.Status == projects.StatusActive,
		auth.ActionRead:                                  true,
		auth.ActionCommercialSummaryRead:                 true,
		auth.ActionMaterialsRead:                         true,
		auth.ActionUpdate:                                initializing,
		auth.ActionProfileCorrect:                        project.Status == projects.StatusActive,
		auth.ActionAssignmentManage:                      mutable,
		auth.ActionAssignmentBusinessOwnerManage:         mutable,
		auth.ActionAssignmentProductionOwnerManage:       mutable,
		auth.ActionAssignmentParticipantManage:           mutable,
		auth.ActionAssignmentCommercialContributorManage: mutable,
		auth.ActionResourceManage:                        mutable,
		auth.ActionComplete:                              initializing,
		auth.ActionCancel:                                mutable,
		auth.ActionHistoryRead:                           true,
		auth.ActionHierarchyManage:                       hierarchyMutable,
	}
	actions := make([]string, 0, len(applicable))
	for _, action := range auth.AllActions {
		if applicable[action] && principal.Can(action) {
			actions = append(actions, action)
		}
	}
	return actions
}

func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeProblem(w, r, http.StatusBadRequest, "invalid_request", "limit 必须是整数", false, nil)
			return
		}
		limit = value
	}
	principal := principalFrom(r)
	value, err := s.useCases.History(r.Context(), principal.TenantID, r.PathValue("project_id"), r.URL.Query().Get("cursor"), limit)
	value = redactCounterpartyHistory(value, principal)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) completionAssessment(w http.ResponseWriter, r *http.Request) {
	profileKey := strings.TrimSpace(r.URL.Query().Get("profile"))
	if profileKey == "" {
		profileKey = projects.CompletionProfileReferenceOAC5V1
	}
	principal := principalFrom(r)
	value, err := s.useCases.EvaluateCompletion(r.Context(), principal.TenantID, r.PathValue("project_id"), profileKey)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	actions := []string{auth.ActionCompletionRead}
	if principal.Can(auth.ActionCompletionConfirm) && value.OverallStatus == projects.CompletionSatisfied {
		actions = append(actions, auth.ActionCompletionConfirm)
	}
	writeJSON(w, http.StatusOK, completionAssessmentResponse{CompletionAssessment: value, AllowedActions: actions})
}

func (s *Server) confirmCompletion(w http.ResponseWriter, r *http.Request) {
	var input projects.CompletionCommand
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.useCases.ConfirmCompletion(r.Context(), actorFrom(r), r.PathValue("project_id"), input)
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) referenceProject(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	value, err := s.useCases.ReferenceProject(r.Context(), principal.TenantID, r.PathValue("project_id"))
	s.writeResult(w, r, http.StatusOK, value, err)
}

func (s *Server) referenceAssignment(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	value, err := s.useCases.ReferenceAssignment(r.Context(), principal.TenantID, r.PathValue("project_id"), r.PathValue("assignment_id"))
	s.writeResult(w, r, http.StatusOK, value, err)
}

func actorFrom(r *http.Request) projects.Actor {
	p := principalFrom(r)
	kind := "iam_subject"
	if p.Provider == "template" {
		kind = "template_subject"
	}
	return projects.Actor{Kind: kind, ID: p.SubjectID, DisplayName: p.DisplayName, TenantID: p.TenantID}
}

func principalFrom(r *http.Request) auth.Principal {
	return r.Context().Value(principalKey{}).(auth.Principal)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	if contentType := r.Header.Get("Content-Type"); contentType != "" && !strings.HasPrefix(strings.ToLower(contentType), "application/json") {
		writeProblem(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "请求体必须使用 application/json", false, nil)
		return false
	}
	reader := http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_json", "JSON 请求体无效", false, nil)
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeProblem(w, r, http.StatusBadRequest, "invalid_json", "JSON 请求体只能包含一个对象", false, nil)
		return false
	}
	return true
}

func (s *Server) writeResult(w http.ResponseWriter, r *http.Request, status int, value any, err error) {
	if err == nil {
		writeJSONStatus(w, status, value)
		return
	}
	s.fail(w, r, err)
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, projects.ErrNotFound):
		writeProblem(w, r, http.StatusNotFound, "project_not_found", "Project 不存在", false, nil)
	case errors.Is(err, projects.ErrRevisionConflict):
		writeProblem(w, r, http.StatusConflict, "revision_conflict", "Project revision 已变化", false, nil)
	case errors.Is(err, projects.ErrNumberConflict):
		writeProblem(w, r, http.StatusConflict, "project_number_conflict", "项目编号已存在", false, nil)
	case errors.Is(err, projects.ErrCommandConflict):
		writeProblem(w, r, http.StatusConflict, "command_conflict", "command_id 已用于不同请求", false, nil)
	case errors.Is(err, projects.ErrStateConflict):
		writeProblem(w, r, http.StatusConflict, "state_conflict", "当前 Project 状态不允许该操作", false, nil)
	case errors.Is(err, projects.ErrOwnerConflict):
		writeProblem(w, r, http.StatusConflict, "active_owner_conflict", "该负责人角色已有 active Assignment", false, nil)
	case errors.Is(err, projects.ErrActiveLinkConflict):
		writeProblem(w, r, http.StatusConflict, "active_link_conflict", "同一 Resource Link 已存在", false, nil)
	case errors.Is(err, projects.ErrHierarchyConflict):
		writeProblem(w, r, http.StatusConflict, "project_hierarchy_conflict", "Project 层级或转换状态不允许该操作", false, nil)
	case errors.Is(err, projects.ErrRollbackUnsafe):
		writeProblem(w, r, http.StatusConflict, "package_rollback_unsafe", "项目包已产生新增子项或业务事实，不能自动撤销", false, nil)
	case errors.Is(err, projects.ErrCompletionIncomplete):
		writeProblem(w, r, http.StatusUnprocessableEntity, "completion_incomplete", "完成条件尚未满足，Project 未发生变化", false, nil)
	case errors.Is(err, projects.ErrCompletionDrift):
		writeProblem(w, r, http.StatusConflict, "completion_sources_changed", "完成来源已经变化，请重新评估后再确认", false, nil)
	case errors.Is(err, projects.ErrReferenceInvalid):
		writeProblem(w, r, http.StatusUnprocessableEntity, "reference_invalid", "外部引用不存在、跨租户、已失效或 revision 不匹配", false, nil)
	case errors.Is(err, projects.ErrReferenceForbidden):
		writeProblem(w, r, http.StatusForbidden, "reference_forbidden", "当前凭证无权读取外部引用", false, nil)
	case errors.Is(err, projects.ErrReferenceUnavailable):
		writeProblem(w, r, http.StatusServiceUnavailable, "reference_unavailable", "外部引用服务暂不可用", true, nil)
	default:
		if strings.HasPrefix(err.Error(), "invalid ") || strings.Contains(err.Error(), " is required") || strings.Contains(err.Error(), " must be ") || strings.Contains(err.Error(), "precedes") {
			writeProblem(w, r, http.StatusBadRequest, "invalid_request", err.Error(), false, nil)
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "服务暂时无法完成请求", true, nil)
	}
}

func requestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Request-ID")))
		if !projects.ValidUUID(requestID) {
			requestID = newUUID()
		}
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func stripUntrustedIdentityHeaders(header http.Header) {
	for _, name := range []string{"X-Actor-ID", "X-Actor-Name", "X-Tenant-ID", "X-IAM-Subject", "X-Allowed-Actions", "X-Capability-Scope"} {
		header.Del(name)
	}
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, message string, retryable bool, details map[string]any) {
	problem := map[string]any{
		"code":       code,
		"message":    message,
		"retryable":  retryable,
		"request_id": requestIDFrom(r),
	}
	for key, value := range details {
		problem[key] = value
	}
	writeJSONStatus(w, status, map[string]any{"error": problem})
}

func writeJSON(w http.ResponseWriter, status int, value any) { writeJSONStatus(w, status, value) }

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func requestIDFrom(r *http.Request) string {
	value, _ := r.Context().Value(requestIDKey{}).(string)
	return value
}

func newUUID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(value[:])
	return raw[:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:]
}
