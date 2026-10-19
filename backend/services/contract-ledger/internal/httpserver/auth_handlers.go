package httpserver

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/identity"
)

const sessionCookieName = "contract_ledger_session"

type principalContextKey struct{}

type commonPageData struct {
	Principal          identity.Principal
	CSRFToken          string
	CanRegisterParty   bool
	CanManageDraft     bool
	CanManageLifecycle bool
	CanManageEvidence  bool
	CanCorrect         bool
	CanExport          bool
	CanRestore         bool
	CanManage          bool
	BasePath           string
	LogoutPath         string
	UsesLocalIdentity  bool
}

func commonData(principal identity.Principal, csrfToken, basePath string, externalIdentity bool) commonPageData {
	logoutPath := basePath + "/logout"
	if externalIdentity {
		logoutPath = "/gateway/logout"
	}
	return commonPageData{
		Principal:          principal,
		CSRFToken:          csrfToken,
		CanRegisterParty:   principal.Can(identity.PermissionPartyRegister),
		CanManageDraft:     principal.Can(identity.PermissionDraftManage),
		CanManageLifecycle: principal.Can(identity.PermissionLifecycleManage),
		CanManageEvidence:  principal.Can(identity.PermissionEvidenceManage),
		CanCorrect:         principal.Can(identity.PermissionBusinessCorrect),
		CanExport:          principal.Can(identity.PermissionBusinessExport),
		CanRestore:         principal.Can(identity.PermissionBusinessRestore),
		CanManage:          principal.Can(identity.PermissionIdentityManage),
		BasePath:           basePath, LogoutPath: logoutPath, UsesLocalIdentity: !externalIdentity,
	}
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(s.sessionCookieName)
		if err != nil {
			s.requireLogin(w, r)
			return
		}
		principal, found, err := s.identities.ResolveSession(r.Context(), cookie.Value)
		if err != nil {
			http.Error(w, "无法验证登录会话", http.StatusInternalServerError)
			return
		}
		if !found {
			if !s.externalIdentity {
				s.clearSessionCookie(w)
			}
			s.requireLogin(w, r)
			return
		}
		// Legacy HTML includes tenant-wide lists and lookups. Exact resource
		// credentials are served only by versioned reference/workspace handlers.
		if principal.Provider == "iam" && principal.ScopeKind != "tenant" {
			http.Error(w, "当前凭证不允许租户级工作面", http.StatusForbidden)
			return
		}
		ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) requirePermission(permission identity.Permission, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := currentPrincipal(r.Context())
		if !ok {
			http.Error(w, "未登录", http.StatusUnauthorized)
			return
		}
		if !principal.Can(permission) {
			http.Error(w, "当前用户没有执行此操作的权限", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func currentPrincipal(ctx context.Context) (identity.Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(identity.Principal)
	return principal, ok
}

func mustPrincipal(r *http.Request) identity.Principal {
	principal, _ := currentPrincipal(r.Context())
	return principal
}

func (s *Server) requireLogin(w http.ResponseWriter, r *http.Request) {
	if s.externalIdentity {
		http.Error(w, "需要有效的 IAM Capability Credential", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "请先登录", http.StatusUnauthorized)
		return
	}
	next := safeReturnPath(r.URL.RequestURI())
	http.Redirect(w, r, "/login?next="+url.QueryEscape(next), http.StatusSeeOther)
}

func safeReturnPath(raw string) string {
	if raw == "" {
		return "/"
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") {
		return "/"
	}
	if parsed.Path == "/login" {
		return "/"
	}
	return parsed.RequestURI()
}

type loginPageData struct {
	BasePath  string
	CSRFToken string
	Username  string
	Next      string
	Error     string
}

func (s *Server) showLogin(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(s.sessionCookieName); err == nil {
		if _, found, resolveErr := s.identities.ResolveSession(r.Context(), cookie.Value); resolveErr == nil && found {
			http.Redirect(w, r, safeReturnPath(r.URL.Query().Get("next")), http.StatusSeeOther)
			return
		}
	}
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, http.StatusOK, s.loginPage, "login.html", loginPageData{
		BasePath:  s.publicBasePath,
		CSRFToken: token,
		Next:      safeReturnPath(r.URL.Query().Get("next")),
	})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	username := r.PostForm.Get("username")
	next := safeReturnPath(r.PostForm.Get("next"))
	result, err := s.identities.Login(r.Context(), username, r.PostForm.Get("password"))
	if err != nil {
		var authenticationError *identity.AuthenticationError
		if !errors.As(err, &authenticationError) {
			http.Error(w, "登录失败", http.StatusInternalServerError)
			return
		}
		token, tokenErr := s.ensureCSRFToken(w, r)
		if tokenErr != nil {
			http.Error(w, "登录失败", http.StatusInternalServerError)
			return
		}
		s.renderTemplate(w, http.StatusUnauthorized, s.loginPage, "login.html", loginPageData{
			BasePath:  s.publicBasePath,
			CSRFToken: token,
			Username:  username,
			Next:      next,
			Error:     authenticationError.Error(),
		})
		return
	}
	maxAge := int(time.Until(result.ExpiresAt).Seconds())
	if maxAge < 1 {
		http.Error(w, "登录会话已过期", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     s.sessionCookieName,
		Value:    result.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.sessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
		Expires:  result.ExpiresAt,
	})
	s.clearCSRFCookie(w)
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return
	}
	if cookie, err := r.Cookie(s.sessionCookieName); err == nil {
		if err := s.identities.Logout(r.Context(), cookie.Value); err != nil {
			http.Error(w, "注销失败", http.StatusInternalServerError)
			return
		}
	}
	s.clearSessionCookie(w)
	s.clearCSRFCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.sessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	})
}

func (s *Server) clearCSRFCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.sessionCookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	})
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, http.StatusOK, s.homeTemplate, "home.html", struct {
		commonPageData
		Title  string
		Status string
	}{
		commonPageData: commonData(mustPrincipal(r), token, s.publicBasePath, s.externalIdentity),
		Title:          "Contract Ledger",
		Status:         "M2 进行中：补充协议与终止文书闭环已完成",
	})
}

type userForm struct {
	Username    string
	DisplayName string
	Role        string
}

type usersPageData struct {
	commonPageData
	Users   []identity.User
	Form    userForm
	Error   string
	Message string
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	s.renderUsersPage(w, r, http.StatusOK, "", userForm{})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	if !s.parseProtectedForm(w, r, 32<<10) {
		return
	}
	form := userForm{
		Username:    r.PostForm.Get("username"),
		DisplayName: r.PostForm.Get("display_name"),
		Role:        r.PostForm.Get("role"),
	}
	_, err := s.identities.CreateUser(r.Context(), identity.CreateUserInput{
		Username: form.Username, DisplayName: form.DisplayName,
		Password: r.PostForm.Get("password"), Role: form.Role,
		Actor: mustPrincipal(r).Actor(),
	})
	if err == nil {
		http.Redirect(w, r, "/admin/users?created=1", http.StatusSeeOther)
		return
	}
	s.renderIdentityMutationError(w, r, err, form)
}

func (s *Server) changeUserRole(w http.ResponseWriter, r *http.Request) {
	if !s.parseProtectedForm(w, r, 16<<10) {
		return
	}
	err := s.identities.ChangeRole(r.Context(), identity.ChangeRoleInput{
		UserID: r.PathValue("userID"), Role: r.PostForm.Get("role"), Actor: mustPrincipal(r).Actor(),
	})
	s.finishIdentityMutation(w, r, err, "role_changed")
}

func (s *Server) changeUserStatus(w http.ResponseWriter, r *http.Request) {
	if !s.parseProtectedForm(w, r, 16<<10) {
		return
	}
	err := s.identities.ChangeStatus(r.Context(), identity.ChangeStatusInput{
		UserID: r.PathValue("userID"), Status: r.PostForm.Get("status"), Actor: mustPrincipal(r).Actor(),
	})
	s.finishIdentityMutation(w, r, err, "status_changed")
}

func (s *Server) resetUserPassword(w http.ResponseWriter, r *http.Request) {
	if !s.parseProtectedForm(w, r, 16<<10) {
		return
	}
	err := s.identities.ResetPassword(r.Context(), identity.ResetPasswordInput{
		UserID: r.PathValue("userID"), NewPassword: r.PostForm.Get("new_password"), Actor: mustPrincipal(r).Actor(),
	})
	s.finishIdentityMutation(w, r, err, "password_reset")
}

func (s *Server) finishIdentityMutation(w http.ResponseWriter, r *http.Request, err error, message string) {
	if err == nil {
		http.Redirect(w, r, "/admin/users?"+message+"=1", http.StatusSeeOther)
		return
	}
	s.renderIdentityMutationError(w, r, err, userForm{})
}

func (s *Server) renderIdentityMutationError(w http.ResponseWriter, r *http.Request, err error, form userForm) {
	status := http.StatusUnprocessableEntity
	var duplicate *identity.DuplicateUsernameError
	var lastAdmin *identity.LastAdministratorError
	var selfDisable *identity.SelfDisableError
	var noChange *identity.NoChangeError
	var notFound *identity.UserNotFoundError
	switch {
	case errors.As(err, &duplicate), errors.As(err, &lastAdmin), errors.As(err, &selfDisable), errors.As(err, &noChange):
		status = http.StatusConflict
	case errors.As(err, &notFound):
		status = http.StatusNotFound
	default:
		var validation *identity.ValidationError
		if !errors.As(err, &validation) {
			http.Error(w, "本地用户操作失败", http.StatusInternalServerError)
			return
		}
	}
	s.renderUsersPage(w, r, status, err.Error(), form)
}

func (s *Server) renderUsersPage(w http.ResponseWriter, r *http.Request, status int, errorMessage string, form userForm) {
	users, err := s.identities.ListUsers(r.Context())
	if err != nil {
		http.Error(w, "无法读取本地用户", http.StatusInternalServerError)
		return
	}
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	message := ""
	switch {
	case r.URL.Query().Get("created") == "1":
		message = "本地用户已创建。"
	case r.URL.Query().Get("role_changed") == "1":
		message = "用户角色已更新。"
	case r.URL.Query().Get("status_changed") == "1":
		message = "用户状态已更新。"
	case r.URL.Query().Get("password_reset") == "1":
		message = "用户密码已重置，旧会话已经失效。"
	}
	s.renderTemplate(w, status, s.usersPage, "users.html", usersPageData{
		commonPageData: commonData(mustPrincipal(r), token, s.publicBasePath, s.externalIdentity),
		Users:          users,
		Form:           form,
		Error:          errorMessage,
		Message:        message,
	})
}

type passwordPageData struct {
	commonPageData
	Error string
}

func (s *Server) showPassword(w http.ResponseWriter, r *http.Request) {
	s.renderPasswordPage(w, r, http.StatusOK, "")
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	if !s.parseProtectedForm(w, r, 16<<10) {
		return
	}
	principal := mustPrincipal(r)
	err := s.identities.ChangePassword(r.Context(), identity.ChangePasswordInput{
		UserID: principal.UserID, CurrentPassword: r.PostForm.Get("current_password"),
		NewPassword: r.PostForm.Get("new_password"), SessionGeneration: principal.SessionGeneration,
		Actor: principal.Actor(),
	})
	if err == nil {
		s.clearSessionCookie(w)
		s.clearCSRFCookie(w)
		http.Redirect(w, r, "/login?password_changed=1", http.StatusSeeOther)
		return
	}
	var validation *identity.ValidationError
	var authentication *identity.AuthenticationError
	if !errors.As(err, &validation) && !errors.As(err, &authentication) {
		http.Error(w, "修改密码失败", http.StatusInternalServerError)
		return
	}
	s.renderPasswordPage(w, r, http.StatusUnprocessableEntity, err.Error())
}

func (s *Server) renderPasswordPage(w http.ResponseWriter, r *http.Request, status int, errorMessage string) {
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, status, s.passwordPage, "password.html", passwordPageData{
		commonPageData: commonData(mustPrincipal(r), token, s.publicBasePath, s.externalIdentity),
		Error:          errorMessage,
	})
}

type auditPageData struct {
	commonPageData
	Entries []audit.Entry
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	entries, err := s.audits.List(r.Context(), 200)
	if err != nil {
		http.Error(w, "无法读取审计记录", http.StatusInternalServerError)
		return
	}
	token, err := s.ensureCSRFToken(w, r)
	if err != nil {
		http.Error(w, "无法建立安全表单", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, http.StatusOK, s.auditPage, "audit.html", auditPageData{
		commonPageData: commonData(mustPrincipal(r), token, s.publicBasePath, s.externalIdentity),
		Entries:        entries,
	})
}

func (s *Server) parseProtectedForm(w http.ResponseWriter, r *http.Request, maximumBytes int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maximumBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "提交内容无效", http.StatusBadRequest)
		return false
	}
	if !validCSRFToken(r) {
		http.Error(w, "安全校验失败，请刷新页面后重试", http.StatusForbidden)
		return false
	}
	return true
}

func (s *Server) renderTemplate(w http.ResponseWriter, status int, page *template.Template, name string, data any) {
	var rendered bytes.Buffer
	if err := page.ExecuteTemplate(&rendered, name, data); err != nil {
		http.Error(w, "页面渲染失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = rendered.WriteTo(w)
}
