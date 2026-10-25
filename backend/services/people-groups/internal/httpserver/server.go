package httpserver

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"omsbizfoundry/people-groups/internal/auth"
	"omsbizfoundry/people-groups/internal/peoplegroups"
)

const csrfCookieName = "oms_people_groups_csrf"

type UseCases interface {
	Ping(context.Context) error
	CreatePerson(context.Context, peoplegroups.Actor, peoplegroups.CreatePersonInput) (peoplegroups.Person, error)
	UpdatePersonProfile(context.Context, peoplegroups.Actor, string, peoplegroups.UpdatePersonProfileInput) (peoplegroups.Person, error)
	SetPersonStatus(context.Context, peoplegroups.Actor, string, peoplegroups.SetPersonStatusInput) (peoplegroups.Person, error)
	SetPersonIAMSubject(context.Context, peoplegroups.Actor, string, peoplegroups.SetIAMSubjectInput) (peoplegroups.Person, error)
	ListPersons(context.Context, string) ([]peoplegroups.Person, error)
	GetPerson(context.Context, string, string) (peoplegroups.Person, error)
	ListEmploymentEvents(context.Context, string, string) ([]peoplegroups.EmploymentEvent, error)
	CreateDefinition(context.Context, peoplegroups.Actor, peoplegroups.DefinitionKind, peoplegroups.CreateDefinitionInput) (peoplegroups.Definition, error)
	UpdateDefinition(context.Context, peoplegroups.Actor, peoplegroups.DefinitionKind, string, peoplegroups.UpdateDefinitionInput) (peoplegroups.Definition, error)
	ListDefinitions(context.Context, string) ([]peoplegroups.Definition, error)
	CreateGroup(context.Context, peoplegroups.Actor, peoplegroups.CreateGroupInput) (peoplegroups.Group, error)
	UpdateGroup(context.Context, peoplegroups.Actor, string, peoplegroups.UpdateGroupInput) (peoplegroups.Group, error)
	ArchiveGroup(context.Context, peoplegroups.Actor, string, peoplegroups.GroupCommandInput) (peoplegroups.Group, error)
	ListGroups(context.Context, string) ([]peoplegroups.Group, error)
	GetGroup(context.Context, string, string) (peoplegroups.Group, error)
	GetGroupDetails(context.Context, string, string) (peoplegroups.GroupDetails, error)
	JoinMembership(context.Context, peoplegroups.Actor, string, peoplegroups.JoinMembershipInput) (peoplegroups.Membership, error)
	ChangeMembershipRole(context.Context, peoplegroups.Actor, string, string, peoplegroups.ChangeMembershipRoleInput) (peoplegroups.Membership, error)
	EndMembership(context.Context, peoplegroups.Actor, string, string, peoplegroups.EndMembershipInput) (peoplegroups.Membership, error)
	AssignCapability(context.Context, peoplegroups.Actor, string, peoplegroups.AssignCapabilityInput) (peoplegroups.GroupCapability, error)
	EndCapability(context.Context, peoplegroups.Actor, string, string, peoplegroups.EndCapabilityInput) (peoplegroups.GroupCapability, error)
	PublishSnapshot(context.Context, peoplegroups.Actor, string, peoplegroups.GroupCommandInput) (peoplegroups.RosterSnapshot, error)
	GetSnapshot(context.Context, string, string, int64) (peoplegroups.RosterSnapshot, error)
}

type Options struct {
	PublicBasePath      string
	SessionCookieSecure bool
	TemplateProvider    *auth.TemplateProvider
}

type Server struct {
	useCases          UseCases
	authenticator     auth.Authenticator
	templateProvider  *auth.TemplateProvider
	basePath          string
	cookieSecure      bool
	dashboardTemplate *template.Template
	groupTemplate     *template.Template
	handler           http.Handler
}

type dashboardData struct {
	BasePath     string
	CSRFToken    string
	CommandID    string
	Principal    auth.Principal
	Persons      []peoplegroups.Person
	Groups       []peoplegroups.Group
	Definitions  []peoplegroups.Definition
	GroupTypes   []peoplegroups.Definition
	Roles        []peoplegroups.Definition
	Capabilities []peoplegroups.Definition
	Today        string
}
type groupData struct {
	BasePath     string
	CSRFToken    string
	CommandID    string
	Principal    auth.Principal
	Details      peoplegroups.GroupDetails
	Persons      []peoplegroups.Person
	Roles        []peoplegroups.Definition
	Capabilities []peoplegroups.Definition
	Today        string
}

func New(useCases UseCases, authenticator auth.Authenticator, options Options) (*Server, error) {
	if useCases == nil || authenticator == nil {
		return nil, fmt.Errorf("use cases and authenticator are required")
	}
	if options.PublicBasePath != "" && !strings.HasPrefix(options.PublicBasePath, "/") {
		return nil, fmt.Errorf("public base path is invalid")
	}
	dashboard, err := template.New("dashboard").Parse(demoDashboardHTML)
	if err != nil {
		return nil, err
	}
	group, err := template.New("group").Parse(demoGroupHTML)
	if err != nil {
		return nil, err
	}
	s := &Server{useCases: useCases, authenticator: authenticator, templateProvider: options.TemplateProvider, basePath: options.PublicBasePath, cookieSecure: options.SessionCookieSecure, dashboardTemplate: dashboard, groupTemplate: group}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", s.live)
	mux.HandleFunc("GET /health/ready", s.ready)
	mux.HandleFunc("GET /login", s.loginPage)
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("POST /logout", s.logout)
	mux.Handle("GET /api/v1/policy-facts/persons/{person_id}", s.protectedPolicyFact(http.HandlerFunc(s.personPolicyFact)))
	mux.Handle("GET /api/v1/references/persons/{person_id}", s.protectedPersonReference(http.HandlerFunc(s.personReference)))
	mux.Handle("/", s.protected(http.HandlerFunc(s.route)))
	s.handler = securityHeaders(limitBody(mux))
	return s, nil
}

func (s *Server) protectedPersonReference(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := s.authenticator.Authenticate(r)
		if err != nil {
			writeProblem(w, http.StatusUnauthorized, "credential_required", "需要有效访问凭证")
			return
		}
		if !principal.Can(auth.ActionPersonReferenceRead) {
			writeProblem(w, http.StatusForbidden, "capability_denied", "缺少 "+auth.ActionPersonReferenceRead)
			return
		}
		personID := r.PathValue("person_id")
		if !principal.CanResource("person", personID) {
			writeProblem(w, http.StatusForbidden, "scope_denied", "凭证范围不包含该 Person")
			return
		}
		ctx := context.WithValue(r.Context(), principalKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) personReference(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	person, err := s.useCases.GetPerson(r.Context(), p.TenantID, r.PathValue("person_id"))
	if err != nil {
		if errors.Is(err, peoplegroups.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "person_not_found", "人员不存在")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "reference_unavailable", "人员引用暂不可用")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"schema_version":     "1",
		"source_application": "people-groups",
		"id":                 person.ID,
		"tenant_id":          person.TenantID,
		"resource_type":      "person",
		"display_name":       person.DisplayName,
		"status":             person.Status,
		"revision":           person.Revision,
	})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]string{"service": "people-groups", "status": "live"})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.useCases.Ping(r.Context()); err != nil {
		writeProblem(w, 503, "not_ready", "数据库暂不可用")
		return
	}
	writeJSON(w, 200, map[string]string{"service": "people-groups", "status": "ready"})
}

func (s *Server) protected(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := s.authenticator.Authenticate(r)
		if err != nil {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeProblem(w, 401, "credential_required", "需要有效访问凭证")
			} else if s.templateProvider != nil {
				http.Redirect(w, r, s.basePath+"/login", http.StatusSeeOther)
			} else {
				http.Redirect(w, r, "/workbench/?return_to="+s.basePath+r.URL.RequestURI(), http.StatusSeeOther)
			}
			return
		}
		if !principal.Can(auth.ActionView) {
			writeProblem(w, 403, "capability_denied", "缺少 people-groups.view")
			return
		}
		ctx := context.WithValue(r.Context(), principalKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) protectedPolicyFact(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := s.authenticator.Authenticate(r)
		if err != nil {
			writeProblem(w, 401, "credential_required", "需要有效访问凭证")
			return
		}
		if !principal.Can(auth.ActionPolicyFactRead) {
			writeProblem(w, 403, "capability_denied", "缺少 "+auth.ActionPolicyFactRead)
			return
		}
		personID := r.PathValue("person_id")
		if !principal.CanResource("person", personID) {
			writeProblem(w, 403, "scope_denied", "凭证范围不包含该 Person")
			return
		}
		ctx := context.WithValue(r.Context(), principalKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) personPolicyFact(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	person, err := s.useCases.GetPerson(r.Context(), p.TenantID, r.PathValue("person_id"))
	if err != nil {
		s.writeResult(w, nil, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"schema_version":     "1",
		"source_application": "people-groups",
		"fact_kind":          "person-iam-subject-link",
		"id":                 person.ID,
		"tenant_id":          person.TenantID,
		"status":             person.Status,
		"revision":           person.Revision,
		"iam_subject_id":     person.IAMSubjectID,
	})
}

type principalKey struct{}

func principalFrom(r *http.Request) auth.Principal {
	return r.Context().Value(principalKey{}).(auth.Principal)
}
func actorFrom(p auth.Principal) peoplegroups.Actor {
	kind := "iam_subject"
	if p.Provider == "template" {
		kind = "template_subject"
	}
	return peoplegroups.Actor{Kind: kind, ID: p.SubjectID, DisplayName: p.DisplayName, TenantID: p.TenantID}
}

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	if s.templateProvider == nil {
		http.NotFound(w, r)
		return
	}
	token := s.ensureCSRF(w, r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>人员与编组 · 本地模板登录</title><style>`+baseCSS+`</style><main class="shell"><p class="eyebrow">People &amp; Groups</p><h1>本地模板身份</h1><section class="panel narrow"><p class="muted">这是显式模板 Provider，不会关闭鉴权，也不保存业务账号。</p><form method="post" action="`+template.HTMLEscapeString(s.basePath)+`/login"><input type="hidden" name="csrf_token" value="`+template.HTMLEscapeString(token)+`"><label>模板访问令牌<input name="access_token" type="password" required autofocus></label><button>进入应用</button></form></section></main></html>`)
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.templateProvider == nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		writeProblem(w, 400, "invalid_form", "表单无效")
		return
	}
	if !s.validCSRF(r) {
		writeProblem(w, 403, "csrf_failed", "安全校验失败")
		return
	}
	if !s.templateProvider.ValidToken(r.PostForm.Get("access_token")) {
		writeProblem(w, 401, "login_failed", "模板访问令牌无效")
		return
	}
	path := s.basePath
	if path == "" {
		path = "/"
	}
	http.SetCookie(w, &http.Cookie{Name: auth.TemplateCookieName, Value: r.PostForm.Get("access_token"), Path: path, Secure: s.cookieSecure, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 12 * 3600})
	s.redirect(w, r, "/")
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if s.templateProvider == nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		writeProblem(w, 400, "invalid_form", "表单无效")
		return
	}
	if !s.validCSRF(r) {
		writeProblem(w, 403, "csrf_failed", "安全校验失败")
		return
	}
	path := s.basePath
	if path == "" {
		path = "/"
	}
	http.SetCookie(w, &http.Cookie{Name: auth.TemplateCookieName, Value: "", Path: path, Secure: s.cookieSecure, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0)})
	http.Redirect(w, r, s.basePath+"/login", http.StatusSeeOther)
}

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/v1/") {
		s.api(w, r)
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/":
		s.dashboard(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/groups/"):
		s.groupPage(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/people":
		s.createPersonForm(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/definitions":
		s.createDefinitionForm(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/groups":
		s.createGroupForm(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memberships"):
		s.joinMembershipForm(w, r)
	case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/memberships/") && strings.HasSuffix(r.URL.Path, "/role"):
		s.changeRoleForm(w, r)
	case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/memberships/") && strings.HasSuffix(r.URL.Path, "/end"):
		s.endMembershipForm(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/capabilities"):
		s.assignCapabilityForm(w, r)
	case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/capabilities/") && strings.HasSuffix(r.URL.Path, "/end"):
		s.endCapabilityForm(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/snapshots"):
		s.publishSnapshotForm(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/archive"):
		s.archiveGroupForm(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	persons, err := s.useCases.ListPersons(r.Context(), p.TenantID)
	if err != nil {
		s.fail(w, err)
		return
	}
	groups, err := s.useCases.ListGroups(r.Context(), p.TenantID)
	if err != nil {
		s.fail(w, err)
		return
	}
	defs, err := s.useCases.ListDefinitions(r.Context(), p.TenantID)
	if err != nil {
		s.fail(w, err)
		return
	}
	data := dashboardData{BasePath: s.basePath, CSRFToken: s.ensureCSRF(w, r), CommandID: newUUID(), Principal: p, Persons: persons, Groups: groups, Definitions: defs, Today: time.Now().Format("2006-01-02")}
	for _, d := range defs {
		if d.Status != "active" {
			continue
		}
		switch d.Kind {
		case peoplegroups.DefinitionGroupType:
			data.GroupTypes = append(data.GroupTypes, d)
		case peoplegroups.DefinitionMembershipRole:
			data.Roles = append(data.Roles, d)
		case peoplegroups.DefinitionCapability:
			data.Capabilities = append(data.Capabilities, d)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.dashboardTemplate.Execute(w, data); err != nil {
		http.Error(w, "无法显示页面", 500)
	}
}
func (s *Server) groupPage(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	id := strings.TrimPrefix(r.URL.Path, "/groups/")
	if strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	details, err := s.useCases.GetGroupDetails(r.Context(), p.TenantID, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	persons, err := s.useCases.ListPersons(r.Context(), p.TenantID)
	if err != nil {
		s.fail(w, err)
		return
	}
	defs, err := s.useCases.ListDefinitions(r.Context(), p.TenantID)
	if err != nil {
		s.fail(w, err)
		return
	}
	data := groupData{BasePath: s.basePath, CSRFToken: s.ensureCSRF(w, r), CommandID: newUUID(), Principal: p, Details: details, Persons: persons, Today: time.Now().Format("2006-01-02")}
	for _, d := range defs {
		if d.Status == "active" && d.Kind == peoplegroups.DefinitionMembershipRole {
			data.Roles = append(data.Roles, d)
		}
		if d.Status == "active" && d.Kind == peoplegroups.DefinitionCapability {
			data.Capabilities = append(data.Capabilities, d)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.groupTemplate.Execute(w, data); err != nil {
		http.Error(w, "无法显示页面", 500)
	}
}

func (s *Server) createPersonForm(w http.ResponseWriter, r *http.Request) {
	p, ok := s.form(w, r, auth.ActionPersonManage)
	if !ok {
		return
	}
	_, err := s.useCases.CreatePerson(r.Context(), actorFrom(p), peoplegroups.CreatePersonInput{CommandID: r.PostForm.Get("command_id"), ReferenceCode: r.PostForm.Get("reference_code"), DisplayName: r.PostForm.Get("display_name"), IAMSubjectID: r.PostForm.Get("iam_subject_id")})
	if err != nil {
		s.fail(w, err)
		return
	}
	s.redirect(w, r, "/")
}
func (s *Server) createDefinitionForm(w http.ResponseWriter, r *http.Request) {
	p, ok := s.form(w, r, auth.ActionConfigurationManage)
	if !ok {
		return
	}
	kind, err := peoplegroups.NormalizeDefinitionKind(r.PostForm.Get("kind"))
	if err != nil {
		s.fail(w, err)
		return
	}
	_, err = s.useCases.CreateDefinition(r.Context(), actorFrom(p), kind, peoplegroups.CreateDefinitionInput{CommandID: r.PostForm.Get("command_id"), Key: r.PostForm.Get("key"), DisplayName: r.PostForm.Get("display_name"), Description: r.PostForm.Get("description")})
	if err != nil {
		s.fail(w, err)
		return
	}
	s.redirect(w, r, "/")
}
func (s *Server) createGroupForm(w http.ResponseWriter, r *http.Request) {
	p, ok := s.form(w, r, auth.ActionGroupManage)
	if !ok {
		return
	}
	group, err := s.useCases.CreateGroup(r.Context(), actorFrom(p), peoplegroups.CreateGroupInput{CommandID: r.PostForm.Get("command_id"), GroupTypeKey: r.PostForm.Get("group_type_key"), Name: r.PostForm.Get("name"), Purpose: r.PostForm.Get("purpose")})
	if err != nil {
		s.fail(w, err)
		return
	}
	s.redirect(w, r, "/groups/"+group.ID)
}
func (s *Server) joinMembershipForm(w http.ResponseWriter, r *http.Request) {
	p, ok := s.form(w, r, auth.ActionMembershipManage)
	if !ok {
		return
	}
	groupID := pathSegment(r.URL.Path, "groups", 1)
	_, err := s.useCases.JoinMembership(r.Context(), actorFrom(p), groupID, peoplegroups.JoinMembershipInput{CommandID: r.PostForm.Get("command_id"), ExpectedRevision: formInt(r, "expected_revision"), PersonID: r.PostForm.Get("person_id"), RoleKey: r.PostForm.Get("role_key"), StartDate: r.PostForm.Get("start_date")})
	if err != nil {
		s.fail(w, err)
		return
	}
	s.redirect(w, r, "/groups/"+groupID)
}
func (s *Server) changeRoleForm(w http.ResponseWriter, r *http.Request) {
	p, ok := s.form(w, r, auth.ActionMembershipManage)
	if !ok {
		return
	}
	groupID := pathSegment(r.URL.Path, "groups", 1)
	membershipID := pathSegment(r.URL.Path, "memberships", 1)
	_, err := s.useCases.ChangeMembershipRole(r.Context(), actorFrom(p), groupID, membershipID, peoplegroups.ChangeMembershipRoleInput{CommandID: r.PostForm.Get("command_id"), ExpectedGroupRevision: formInt(r, "expected_group_revision"), ExpectedRevision: formInt(r, "expected_revision"), RoleKey: r.PostForm.Get("role_key")})
	if err != nil {
		s.fail(w, err)
		return
	}
	s.redirect(w, r, "/groups/"+groupID)
}
func (s *Server) endMembershipForm(w http.ResponseWriter, r *http.Request) {
	p, ok := s.form(w, r, auth.ActionMembershipManage)
	if !ok {
		return
	}
	groupID := pathSegment(r.URL.Path, "groups", 1)
	membershipID := pathSegment(r.URL.Path, "memberships", 1)
	_, err := s.useCases.EndMembership(r.Context(), actorFrom(p), groupID, membershipID, peoplegroups.EndMembershipInput{CommandID: r.PostForm.Get("command_id"), ExpectedGroupRevision: formInt(r, "expected_group_revision"), ExpectedRevision: formInt(r, "expected_revision"), EndDate: r.PostForm.Get("end_date")})
	if err != nil {
		s.fail(w, err)
		return
	}
	s.redirect(w, r, "/groups/"+groupID)
}
func (s *Server) assignCapabilityForm(w http.ResponseWriter, r *http.Request) {
	p, ok := s.form(w, r, auth.ActionCapabilityManage)
	if !ok {
		return
	}
	groupID := pathSegment(r.URL.Path, "groups", 1)
	_, err := s.useCases.AssignCapability(r.Context(), actorFrom(p), groupID, peoplegroups.AssignCapabilityInput{CommandID: r.PostForm.Get("command_id"), ExpectedRevision: formInt(r, "expected_revision"), CapabilityKey: r.PostForm.Get("capability_key"), Note: r.PostForm.Get("note"), StartDate: r.PostForm.Get("start_date")})
	if err != nil {
		s.fail(w, err)
		return
	}
	s.redirect(w, r, "/groups/"+groupID)
}
func (s *Server) endCapabilityForm(w http.ResponseWriter, r *http.Request) {
	p, ok := s.form(w, r, auth.ActionCapabilityManage)
	if !ok {
		return
	}
	groupID := pathSegment(r.URL.Path, "groups", 1)
	assignmentID := pathSegment(r.URL.Path, "capabilities", 1)
	_, err := s.useCases.EndCapability(r.Context(), actorFrom(p), groupID, assignmentID, peoplegroups.EndCapabilityInput{CommandID: r.PostForm.Get("command_id"), ExpectedGroupRevision: formInt(r, "expected_group_revision"), ExpectedRevision: formInt(r, "expected_revision"), EndDate: r.PostForm.Get("end_date")})
	if err != nil {
		s.fail(w, err)
		return
	}
	s.redirect(w, r, "/groups/"+groupID)
}
func (s *Server) publishSnapshotForm(w http.ResponseWriter, r *http.Request) {
	p, ok := s.form(w, r, auth.ActionSnapshotPublish)
	if !ok {
		return
	}
	groupID := pathSegment(r.URL.Path, "groups", 1)
	_, err := s.useCases.PublishSnapshot(r.Context(), actorFrom(p), groupID, peoplegroups.GroupCommandInput{CommandID: r.PostForm.Get("command_id"), ExpectedRevision: formInt(r, "expected_revision")})
	if err != nil {
		s.fail(w, err)
		return
	}
	s.redirect(w, r, "/groups/"+groupID)
}
func (s *Server) archiveGroupForm(w http.ResponseWriter, r *http.Request) {
	p, ok := s.form(w, r, auth.ActionGroupManage)
	if !ok {
		return
	}
	groupID := pathSegment(r.URL.Path, "groups", 1)
	_, err := s.useCases.ArchiveGroup(r.Context(), actorFrom(p), groupID, peoplegroups.GroupCommandInput{CommandID: r.PostForm.Get("command_id"), ExpectedRevision: formInt(r, "expected_revision")})
	if err != nil {
		s.fail(w, err)
		return
	}
	s.redirect(w, r, "/groups/"+groupID)
}

func (s *Server) api(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	actor := actorFrom(p)
	path := strings.TrimPrefix(r.URL.Path, "/api/v1")
	segments := splitPath(path)
	if p.ScopeKind == "resources" {
		allowed := r.Method == http.MethodGet && ((len(segments) == 2 && ((segments[0] == "persons" && p.CanResource("person", segments[1])) || (segments[0] == "groups" && p.CanResource("group", segments[1])))) || (len(segments) == 3 && segments[0] == "persons" && segments[2] == "employment-history" && p.CanResource("person", segments[1])))
		if !allowed {
			writeProblem(w, http.StatusForbidden, "scope_denied", "凭证范围不包含该人员与编组资源")
			return
		}
	}
	if r.Method == http.MethodGet && path == "/catalog" {
		values, err := s.useCases.ListDefinitions(r.Context(), p.TenantID)
		s.writeResult(w, values, err)
		return
	}
	if r.Method == http.MethodGet && path == "/persons" {
		values, err := s.useCases.ListPersons(r.Context(), p.TenantID)
		s.writeResult(w, values, err)
		return
	}
	if r.Method == http.MethodPost && path == "/persons" {
		if !s.requireAction(w, p, auth.ActionPersonManage) {
			return
		}
		var in peoplegroups.CreatePersonInput
		if !decodeJSON(w, r, &in) {
			return
		}
		in.CommandID = commandID(r, in.CommandID)
		value, err := s.useCases.CreatePerson(r.Context(), actor, in)
		s.writeCreated(w, value, err)
		return
	}
	if len(segments) >= 2 && segments[0] == "persons" {
		id := segments[1]
		if r.Method == http.MethodGet && len(segments) == 2 {
			value, err := s.useCases.GetPerson(r.Context(), p.TenantID, id)
			s.writeResult(w, value, err)
			return
		}
		if r.Method == http.MethodGet && len(segments) == 3 && segments[2] == "employment-history" {
			values, err := s.useCases.ListEmploymentEvents(r.Context(), p.TenantID, id)
			s.writeResult(w, values, err)
			return
		}
		if r.Method == http.MethodPost && len(segments) == 3 {
			if !s.requireAction(w, p, auth.ActionPersonManage) {
				return
			}
			switch segments[2] {
			case "profile":
				var in peoplegroups.UpdatePersonProfileInput
				if !decodeJSON(w, r, &in) {
					return
				}
				in.CommandID = commandID(r, in.CommandID)
				v, e := s.useCases.UpdatePersonProfile(r.Context(), actor, id, in)
				s.writeResult(w, v, e)
				return
			case "status":
				var in peoplegroups.SetPersonStatusInput
				if !decodeJSON(w, r, &in) {
					return
				}
				in.CommandID = commandID(r, in.CommandID)
				v, e := s.useCases.SetPersonStatus(r.Context(), actor, id, in)
				s.writeResult(w, v, e)
				return
			case "iam-subject":
				var in peoplegroups.SetIAMSubjectInput
				if !decodeJSON(w, r, &in) {
					return
				}
				in.CommandID = commandID(r, in.CommandID)
				v, e := s.useCases.SetPersonIAMSubject(r.Context(), actor, id, in)
				s.writeResult(w, v, e)
				return
			}
		}
	}
	if len(segments) >= 2 && segments[0] == "definitions" {
		kind, err := peoplegroups.NormalizeDefinitionKind(segments[1])
		if err != nil {
			s.fail(w, err)
			return
		}
		if r.Method == http.MethodPost && len(segments) == 2 {
			if !s.requireAction(w, p, auth.ActionConfigurationManage) {
				return
			}
			var in peoplegroups.CreateDefinitionInput
			if !decodeJSON(w, r, &in) {
				return
			}
			in.CommandID = commandID(r, in.CommandID)
			v, e := s.useCases.CreateDefinition(r.Context(), actor, kind, in)
			s.writeCreated(w, v, e)
			return
		}
		if r.Method == http.MethodPost && len(segments) == 3 {
			if !s.requireAction(w, p, auth.ActionConfigurationManage) {
				return
			}
			var in peoplegroups.UpdateDefinitionInput
			if !decodeJSON(w, r, &in) {
				return
			}
			in.CommandID = commandID(r, in.CommandID)
			v, e := s.useCases.UpdateDefinition(r.Context(), actor, kind, segments[2], in)
			s.writeResult(w, v, e)
			return
		}
	}
	if r.Method == http.MethodGet && path == "/groups" {
		values, err := s.useCases.ListGroups(r.Context(), p.TenantID)
		s.writeResult(w, values, err)
		return
	}
	if r.Method == http.MethodPost && path == "/groups" {
		if !s.requireAction(w, p, auth.ActionGroupManage) {
			return
		}
		var in peoplegroups.CreateGroupInput
		if !decodeJSON(w, r, &in) {
			return
		}
		in.CommandID = commandID(r, in.CommandID)
		v, e := s.useCases.CreateGroup(r.Context(), actor, in)
		s.writeCreated(w, v, e)
		return
	}
	if len(segments) >= 2 && segments[0] == "groups" {
		groupID := segments[1]
		if r.Method == http.MethodGet && len(segments) == 2 {
			v, e := s.useCases.GetGroupDetails(r.Context(), p.TenantID, groupID)
			s.writeResult(w, v, e)
			return
		}
		if r.Method == http.MethodPost && len(segments) == 3 {
			switch segments[2] {
			case "profile":
				if !s.requireAction(w, p, auth.ActionGroupManage) {
					return
				}
				var in peoplegroups.UpdateGroupInput
				if !decodeJSON(w, r, &in) {
					return
				}
				in.CommandID = commandID(r, in.CommandID)
				v, e := s.useCases.UpdateGroup(r.Context(), actor, groupID, in)
				s.writeResult(w, v, e)
				return
			case "archive":
				if !s.requireAction(w, p, auth.ActionGroupManage) {
					return
				}
				var in peoplegroups.GroupCommandInput
				if !decodeJSON(w, r, &in) {
					return
				}
				in.CommandID = commandID(r, in.CommandID)
				v, e := s.useCases.ArchiveGroup(r.Context(), actor, groupID, in)
				s.writeResult(w, v, e)
				return
			case "memberships":
				if !s.requireAction(w, p, auth.ActionMembershipManage) {
					return
				}
				var in peoplegroups.JoinMembershipInput
				if !decodeJSON(w, r, &in) {
					return
				}
				in.CommandID = commandID(r, in.CommandID)
				v, e := s.useCases.JoinMembership(r.Context(), actor, groupID, in)
				s.writeCreated(w, v, e)
				return
			case "capabilities":
				if !s.requireAction(w, p, auth.ActionCapabilityManage) {
					return
				}
				var in peoplegroups.AssignCapabilityInput
				if !decodeJSON(w, r, &in) {
					return
				}
				in.CommandID = commandID(r, in.CommandID)
				v, e := s.useCases.AssignCapability(r.Context(), actor, groupID, in)
				s.writeCreated(w, v, e)
				return
			case "snapshots":
				if !s.requireAction(w, p, auth.ActionSnapshotPublish) {
					return
				}
				var in peoplegroups.GroupCommandInput
				if !decodeJSON(w, r, &in) {
					return
				}
				in.CommandID = commandID(r, in.CommandID)
				v, e := s.useCases.PublishSnapshot(r.Context(), actor, groupID, in)
				s.writeCreated(w, map[string]any{"id": v.ID, "group_revision": v.GroupRevision, "sha256": v.SHA256, "created_at": v.CreatedAt}, e)
				return
			}
		}
		if len(segments) == 5 && segments[2] == "memberships" && r.Method == http.MethodPost {
			if !s.requireAction(w, p, auth.ActionMembershipManage) {
				return
			}
			if segments[4] == "role" {
				var in peoplegroups.ChangeMembershipRoleInput
				if !decodeJSON(w, r, &in) {
					return
				}
				in.CommandID = commandID(r, in.CommandID)
				v, e := s.useCases.ChangeMembershipRole(r.Context(), actor, groupID, segments[3], in)
				s.writeResult(w, v, e)
				return
			}
			if segments[4] == "end" {
				var in peoplegroups.EndMembershipInput
				if !decodeJSON(w, r, &in) {
					return
				}
				in.CommandID = commandID(r, in.CommandID)
				v, e := s.useCases.EndMembership(r.Context(), actor, groupID, segments[3], in)
				s.writeResult(w, v, e)
				return
			}
		}
		if len(segments) == 5 && segments[2] == "capabilities" && segments[4] == "end" && r.Method == http.MethodPost {
			if !s.requireAction(w, p, auth.ActionCapabilityManage) {
				return
			}
			var in peoplegroups.EndCapabilityInput
			if !decodeJSON(w, r, &in) {
				return
			}
			in.CommandID = commandID(r, in.CommandID)
			v, e := s.useCases.EndCapability(r.Context(), actor, groupID, segments[3], in)
			s.writeResult(w, v, e)
			return
		}
		if len(segments) == 4 && segments[2] == "snapshots" && r.Method == http.MethodGet {
			revision, err := strconv.ParseInt(segments[3], 10, 64)
			if err != nil {
				s.fail(w, &peoplegroups.ValidationError{Field: "group_revision", Message: "group_revision must be an integer"})
				return
			}
			snapshot, err := s.useCases.GetSnapshot(r.Context(), p.TenantID, groupID, revision)
			if err != nil {
				s.fail(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Content-SHA256", snapshot.SHA256)
			w.WriteHeader(200)
			_, _ = w.Write(snapshot.Payload)
			return
		}
	}
	writeProblem(w, 404, "not_found", "接口不存在")
}

func (s *Server) form(w http.ResponseWriter, r *http.Request, action string) (auth.Principal, bool) {
	p := principalFrom(r)
	if !s.requireAction(w, p, action) {
		return auth.Principal{}, false
	}
	if err := r.ParseForm(); err != nil {
		writeProblem(w, 400, "invalid_form", "表单无效")
		return auth.Principal{}, false
	}
	if !s.validCSRF(r) {
		writeProblem(w, 403, "csrf_failed", "安全校验失败")
		return auth.Principal{}, false
	}
	return p, true
}
func (s *Server) requireAction(w http.ResponseWriter, p auth.Principal, action string) bool {
	if !p.Can(action) {
		writeProblem(w, 403, "capability_denied", "缺少 "+action)
		return false
	}
	return true
}
func (s *Server) ensureCSRF(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(csrfCookieName); err == nil && len(c.Value) >= 32 {
		return c.Value
	}
	raw := make([]byte, 24)
	_, _ = rand.Read(raw)
	token := hex.EncodeToString(raw)
	path := s.basePath
	if path == "" {
		path = "/"
	}
	http.SetCookie(w, &http.Cookie{Name: csrfCookieName, Value: token, Path: path, Secure: s.cookieSecure, HttpOnly: false, SameSite: http.SameSiteStrictMode, MaxAge: 3600})
	return token
}
func (s *Server) validCSRF(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	token := r.PostForm.Get("csrf_token")
	return len(token) == len(cookie.Value) && subtle.ConstantTimeCompare([]byte(token), []byte(cookie.Value)) == 1
}
func (s *Server) redirect(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, s.basePath+path, http.StatusSeeOther)
}
func (s *Server) writeResult(w http.ResponseWriter, value any, err error) {
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, value)
}
func (s *Server) writeCreated(w http.ResponseWriter, value any, err error) {
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 201, value)
}
func (s *Server) fail(w http.ResponseWriter, err error) {
	var validation *peoplegroups.ValidationError
	switch {
	case errors.As(err, &validation):
		writeProblem(w, 400, "validation_failed", validation.Error())
	case errors.Is(err, peoplegroups.ErrNotFound):
		writeProblem(w, 404, "not_found", "资源不存在")
	case errors.Is(err, peoplegroups.ErrConflict):
		writeProblem(w, 409, "conflict", "资源状态、业务键、Command ID 或期望 revision 冲突")
	case errors.Is(err, peoplegroups.ErrSubjectReferenceInvalid):
		writeProblem(w, 422, "iam_subject_invalid", "IAM 账号不存在或不属于当前租户")
	case errors.Is(err, peoplegroups.ErrSubjectReferenceUnavailable):
		writeProblem(w, 503, "iam_subject_unavailable", "IAM 账号校验暂时不可用")
	default:
		writeProblem(w, 500, "internal_error", "服务暂时无法完成请求")
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeProblem(w, 415, "content_type_required", "需要 application/json")
		return false
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeProblem(w, 400, "invalid_json", "JSON 请求无效")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeProblem(w, 400, "invalid_json", "JSON 只能包含一个对象")
		return false
	}
	return true
}
func commandID(r *http.Request, body string) string {
	if value := strings.TrimSpace(r.Header.Get("Idempotency-Key")); value != "" {
		return value
	}
	return body
}
func formInt(r *http.Request, name string) int64 {
	value, _ := strconv.ParseInt(r.PostForm.Get(name), 10, 64)
	return value
}
func splitPath(path string) []string {
	raw := strings.Split(strings.Trim(path, "/"), "/")
	if len(raw) == 1 && raw[0] == "" {
		return nil
	}
	return raw
}
func pathSegment(path, marker string, offset int) string {
	parts := splitPath(path)
	for index, value := range parts {
		if value == marker && index+offset < len(parts) {
			return parts[index+offset]
		}
	}
	return ""
}
func newUUID() string {
	raw := make([]byte, 16)
	_, _ = rand.Read(raw)
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeProblem(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		}
		next.ServeHTTP(w, r)
	})
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

const baseCSS = `:root{font-family:Inter,ui-sans-serif,system-ui,sans-serif;color:#17211f;background:#f4f3ee}*{box-sizing:border-box}body{margin:0}.shell{max-width:1180px;margin:auto;padding:42px 24px 72px}.eyebrow{text-transform:uppercase;letter-spacing:.13em;color:#58706a;font-size:12px}h1{font-size:clamp(34px,6vw,64px);line-height:1;margin:10px 0 30px}h2{margin:0 0 16px}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(300px,1fr));gap:18px}.panel{background:#fff;border:1px solid #d9ddd8;border-radius:16px;padding:22px}.narrow{max-width:560px}.muted{color:#667772}.row{display:flex;gap:10px;align-items:center;flex-wrap:wrap}.stack{display:grid;gap:10px}.item{padding:13px 0;border-top:1px solid #edf0ed}.item:first-child{border:0}label{display:grid;gap:6px;font-size:13px;color:#465650}input,select,textarea{width:100%;padding:10px;border:1px solid #b8c2bd;border-radius:8px;background:#fff;font:inherit}textarea{min-height:76px}button,.button{display:inline-block;padding:10px 14px;border:0;border-radius:8px;background:#175749;color:#fff;font:inherit;text-decoration:none;cursor:pointer}.button.secondary,button.secondary{background:#e5ece8;color:#17493e}.danger{background:#8b362f}code,.tag{font:12px ui-monospace,SFMono-Regular,monospace;background:#eef2ef;border-radius:5px;padding:3px 6px}.status{font-size:12px;color:#667772}.top{display:flex;justify-content:space-between;align-items:center;gap:16px}.top h1{margin-bottom:20px}a{color:#175749}.history{opacity:.72}.inline{display:flex;gap:8px;align-items:end}.inline>*{flex:1}.inline button{flex:0 0 auto}`

const dashboardHTML = `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>人员与编组</title><style>` + baseCSS + `</style></head><body><main class="shell"><div class="top"><div><p class="eyebrow">People &amp; Groups · {{.Principal.DisplayName}}</p><h1>人员与编组</h1></div>{{if eq .Principal.Provider "template"}}<form method="post" action="{{.BasePath}}/logout"><input type="hidden" name="csrf_token" value="{{.CSRFToken}}"><button class="secondary">退出</button></form>{{end}}</div><div class="grid"><section class="panel"><h2>Groups</h2><div class="stack">{{range .Groups}}<a class="item" href="{{$.BasePath}}/groups/{{.ID}}"><strong>{{.Name}}</strong><br><span class="status">{{.GroupTypeKey}} · {{.Status}} · revision {{.Revision}}</span></a>{{else}}<p class="muted">还没有 Group。</p>{{end}}</div>{{if .GroupTypes}}<form method="post" action="{{.BasePath}}/groups" class="stack"><input type="hidden" name="csrf_token" value="{{.CSRFToken}}"><input type="hidden" name="command_id" value="{{.CommandID}}"><label>名称<input name="name" required maxlength="120"></label><label>Group Type<select name="group_type_key">{{range .GroupTypes}}<option value="{{.Key}}">{{.DisplayName}}</option>{{end}}</select></label><label>Purpose<textarea name="purpose" maxlength="1000"></textarea></label><button>建立 Group</button></form>{{else}}<p class="muted">先配置一个 Group Type。</p>{{end}}</section><section class="panel"><h2>Persons</h2><div>{{range .Persons}}<div class="item"><strong>{{.DisplayName}}</strong> {{if .ReferenceCode}}<code>{{.ReferenceCode}}</code>{{end}}<br><span class="status">{{.Status}} · revision {{.Revision}}{{if .IAMSubjectID}} · IAM linked{{end}}</span></div>{{else}}<p class="muted">还没有 Person。</p>{{end}}</div><form method="post" action="{{.BasePath}}/people" class="stack"><input type="hidden" name="csrf_token" value="{{.CSRFToken}}"><input type="hidden" name="command_id" value="{{.CommandID}}"><label>显示名<input name="display_name" required maxlength="100"></label><label>业务引用码（可选）<input name="reference_code" maxlength="64"></label><label>IAM Subject UUID（可选）<input name="iam_subject_id"></label><button>建立 Person</button></form></section><section class="panel"><h2>可配置字典</h2><div>{{range .Definitions}}<div class="item"><span class="tag">{{.Kind}}/{{.Key}}</span> <strong>{{.DisplayName}}</strong><br><span class="status">{{.Status}} · revision {{.Revision}}</span></div>{{else}}<p class="muted">先建立组类型、组内角色和能力。</p>{{end}}</div><form method="post" action="{{.BasePath}}/definitions" class="stack"><input type="hidden" name="csrf_token" value="{{.CSRFToken}}"><input type="hidden" name="command_id" value="{{.CommandID}}"><label>种类<select name="kind"><option value="group_type">Group Type</option><option value="membership_role">Membership Role</option><option value="capability">Capability</option></select></label><label>稳定 key<input name="key" pattern="[a-z][a-z0-9_-]{1,62}[a-z0-9]" required></label><label>显示名<input name="display_name" required maxlength="100"></label><label>说明<textarea name="description" maxlength="500"></textarea></label><button>建立配置</button></form></section></div></main></body></html>`

const groupHTML = `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Details.Group.Name}} · 人员与编组</title><style>` + baseCSS + `</style></head><body><main class="shell"><p><a href="{{.BasePath}}/">← 返回</a></p><p class="eyebrow">{{.Details.Group.GroupTypeKey}} · {{.Details.Group.Status}} · revision {{.Details.Group.Revision}}</p><h1>{{.Details.Group.Name}}</h1><p class="muted">{{.Details.Group.Purpose}}</p><div class="grid"><section class="panel"><h2>Memberships</h2>{{range .Details.Memberships}}<div class="item {{if eq .Status "ended"}}history{{end}}"><strong>{{.Person.DisplayName}}</strong> <span class="tag">{{.RoleKey}}</span><br><span class="status">{{.StartDate}}{{if .EndDate}} → {{.EndDate}}{{end}} · {{.Status}} · membership revision {{.Revision}}</span>{{if and (eq .Status "active") (eq $.Details.Group.Status "active")}}<form method="post" action="{{$.BasePath}}/groups/{{$.Details.Group.ID}}/memberships/{{.ID}}/role" class="inline"><input type="hidden" name="csrf_token" value="{{$.CSRFToken}}"><input type="hidden" name="command_id" value="{{$.CommandID}}"><input type="hidden" name="expected_group_revision" value="{{$.Details.Group.Revision}}"><input type="hidden" name="expected_revision" value="{{.Revision}}"><label>换角色<select name="role_key">{{range $.Roles}}<option value="{{.Key}}">{{.DisplayName}}</option>{{end}}</select></label><button class="secondary">调整</button></form><form method="post" action="{{$.BasePath}}/groups/{{$.Details.Group.ID}}/memberships/{{.ID}}/end" class="inline"><input type="hidden" name="csrf_token" value="{{$.CSRFToken}}"><input type="hidden" name="command_id" value="{{$.CommandID}}"><input type="hidden" name="expected_group_revision" value="{{$.Details.Group.Revision}}"><input type="hidden" name="expected_revision" value="{{.Revision}}"><label>结束日期<input type="date" name="end_date" value="{{$.Today}}" required></label><button class="danger">结束</button></form>{{end}}</div>{{else}}<p class="muted">还没有 Membership。</p>{{end}}{{if eq .Details.Group.Status "active"}}<form method="post" action="{{.BasePath}}/groups/{{.Details.Group.ID}}/memberships" class="stack"><input type="hidden" name="csrf_token" value="{{.CSRFToken}}"><input type="hidden" name="command_id" value="{{.CommandID}}"><input type="hidden" name="expected_revision" value="{{.Details.Group.Revision}}"><label>Person<select name="person_id">{{range .Persons}}{{if eq .Status "active"}}<option value="{{.ID}}">{{.DisplayName}}</option>{{end}}{{end}}</select></label><label>组内角色<select name="role_key">{{range .Roles}}<option value="{{.Key}}">{{.DisplayName}}</option>{{end}}</select></label><label>开始日期<input type="date" name="start_date" value="{{.Today}}" required></label><button>加入成员</button></form>{{end}}</section><section class="panel"><h2>Group Capabilities</h2>{{range .Details.Capabilities}}<div class="item {{if eq .Status "ended"}}history{{end}}"><span class="tag">{{.CapabilityKey}}</span> {{.Note}}<br><span class="status">{{.StartDate}}{{if .EndDate}} → {{.EndDate}}{{end}} · {{.Status}}</span>{{if and (eq .Status "active") (eq $.Details.Group.Status "active")}}<form method="post" action="{{$.BasePath}}/groups/{{$.Details.Group.ID}}/capabilities/{{.ID}}/end" class="inline"><input type="hidden" name="csrf_token" value="{{$.CSRFToken}}"><input type="hidden" name="command_id" value="{{$.CommandID}}"><input type="hidden" name="expected_group_revision" value="{{$.Details.Group.Revision}}"><input type="hidden" name="expected_revision" value="{{.Revision}}"><label>结束日期<input type="date" name="end_date" value="{{$.Today}}" required></label><button class="danger">结束</button></form>{{end}}</div>{{else}}<p class="muted">还没有能力画像。</p>{{end}}{{if eq .Details.Group.Status "active"}}<form method="post" action="{{.BasePath}}/groups/{{.Details.Group.ID}}/capabilities" class="stack"><input type="hidden" name="csrf_token" value="{{.CSRFToken}}"><input type="hidden" name="command_id" value="{{.CommandID}}"><input type="hidden" name="expected_revision" value="{{.Details.Group.Revision}}"><label>能力<select name="capability_key">{{range .Capabilities}}<option value="{{.Key}}">{{.DisplayName}}</option>{{end}}</select></label><label>说明<input name="note" maxlength="500"></label><label>开始日期<input type="date" name="start_date" value="{{.Today}}" required></label><button>声明能力</button></form>{{end}}</section><section class="panel"><h2>Roster Snapshots</h2>{{range .Details.Snapshots}}<div class="item"><strong>revision {{.GroupRevision}}</strong><br><code>{{.SHA256}}</code><br><a href="{{$.BasePath}}/api/v1/groups/{{$.Details.Group.ID}}/snapshots/{{.GroupRevision}}">查看 JSON</a></div>{{else}}<p class="muted">还没有发布快照。</p>{{end}}{{if eq .Details.Group.Status "active"}}<form method="post" action="{{.BasePath}}/groups/{{.Details.Group.ID}}/snapshots"><input type="hidden" name="csrf_token" value="{{.CSRFToken}}"><input type="hidden" name="command_id" value="{{.CommandID}}"><input type="hidden" name="expected_revision" value="{{.Details.Group.Revision}}"><button>发布 revision {{.Details.Group.Revision}} 快照</button></form><form method="post" action="{{.BasePath}}/groups/{{.Details.Group.ID}}/archive"><input type="hidden" name="csrf_token" value="{{.CSRFToken}}"><input type="hidden" name="command_id" value="{{.CommandID}}"><input type="hidden" name="expected_revision" value="{{.Details.Group.Revision}}"><button class="danger">归档 Group</button></form>{{end}}</section></div></main></body></html>`
