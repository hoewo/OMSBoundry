package gateway

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const feedbackUseAction = "feedback.use"

var feedbackIdentityVersionPattern = regexp.MustCompile(`^v[1-9][0-9]{0,3}$`)

// FeedbackOptions configures the external Feedback SDK adapter. APIKeys are
// server-held deployment secrets. The first key is active; later keys are
// accepted only as a bounded fallback while rotating session-mode deployments.
type FeedbackOptions struct {
	AuthMode             string
	GatewayURL           string
	ProjectID            int
	APIKeys              []string
	IdentityHMACKey      []byte
	IdentityVersion      string
	NotificationsEnabled bool
}

type feedbackRuntime struct {
	enabled              bool
	authMode             string
	gatewayURL           string
	projectID            int
	apiKeys              []string
	identityHMACKey      []byte
	identityVersion      string
	notificationsEnabled bool
}

type feedbackSession struct {
	Token        string    `json:"token"`
	TokenType    string    `json:"token_type"`
	ProjectID    int       `json:"project_id"`
	CustomUserID string    `json:"custom_user_id"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type feedbackSessionEnvelope struct {
	Code string          `json:"code"`
	Data feedbackSession `json:"data"`
}

func newFeedbackRuntime(options FeedbackOptions) (feedbackRuntime, error) {
	if strings.TrimSpace(options.AuthMode) == "" {
		return feedbackRuntime{}, nil
	}
	if options.AuthMode != "apiKey" && options.AuthMode != "session" {
		return feedbackRuntime{}, fmt.Errorf("feedback auth mode must be apiKey or session")
	}
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(options.GatewayURL), "/"))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" {
		return feedbackRuntime{}, fmt.Errorf("feedback gateway URL is invalid")
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && loopbackHost(parsed.Hostname())) {
		return feedbackRuntime{}, fmt.Errorf("feedback gateway URL must use HTTPS")
	}
	if options.ProjectID < 1 || len(options.IdentityHMACKey) < 32 || !feedbackIdentityVersionPattern.MatchString(options.IdentityVersion) {
		return feedbackRuntime{}, fmt.Errorf("feedback project or identity configuration is invalid")
	}
	if len(options.APIKeys) > 2 {
		return feedbackRuntime{}, fmt.Errorf("feedback API Key rotation accepts at most two keys")
	}
	keys := make([]string, 0, len(options.APIKeys))
	seen := map[string]bool{}
	for _, key := range options.APIKeys {
		key = strings.TrimSpace(key)
		if len(key) < 16 || seen[key] {
			return feedbackRuntime{}, fmt.Errorf("feedback API Key configuration is invalid")
		}
		seen[key] = true
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return feedbackRuntime{}, fmt.Errorf("feedback API Key is required")
	}
	return feedbackRuntime{
		enabled:              true,
		authMode:             options.AuthMode,
		gatewayURL:           parsed.String(),
		projectID:            options.ProjectID,
		apiKeys:              keys,
		identityHMACKey:      append([]byte(nil), options.IdentityHMACKey...),
		identityVersion:      options.IdentityVersion,
		notificationsEnabled: options.NotificationsEnabled,
	}, nil
}

func loopbackHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func deriveFeedbackUserID(secret []byte, version, tenantID, subjectID string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte("oms1-feedback\x00" + version + "\x00" + tenantID + "\x00" + subjectID))
	return "oms1_" + version + "_" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Server) feedbackConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !s.feedback.enabled {
		writeProblem(w, http.StatusServiceUnavailable, "feedback_not_configured", "产品反馈暂未配置")
		return
	}
	if !s.validCSRF(r) {
		writeProblem(w, http.StatusForbidden, "csrf_failed", "安全校验失败")
		return
	}
	principal, ok := s.resolveSession(r.Context(), r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "session_required", "请重新登录后使用产品反馈")
		return
	}
	if err := s.requireFeedbackCapability(r.Context(), r, principal.Principal.TenantID, principal.Principal.SubjectID); err != nil {
		writeProblem(w, http.StatusForbidden, "feedback_capability_denied", "当前账号没有产品反馈权限")
		return
	}
	customUserID := deriveFeedbackUserID(s.feedback.identityHMACKey, s.feedback.identityVersion, principal.Principal.TenantID, principal.Principal.SubjectID)
	config := map[string]any{
		"feedbackV2Enabled":              true,
		"feedbackV2NotificationsEnabled": s.feedback.notificationsEnabled,
		"feedbackV2AuthMode":             s.feedback.authMode,
		"projectId":                      s.feedback.projectID,
		"customUserId":                   customUserID,
		"gatewayUrl":                     s.feedback.gatewayURL,
		"theme":                          "system",
		"locale":                         "zh-CN",
	}
	if s.feedback.authMode == "apiKey" {
		// Direct mode is an explicit first-release risk acceptance. The key is
		// browser-visible, but never enters source, logs, URLs or persistence.
		config["apiKey"] = s.feedback.apiKeys[0]
	} else {
		session, err := s.exchangeFeedbackSession(r.Context(), customUserID)
		if err != nil {
			writeProblem(w, http.StatusBadGateway, "feedback_unavailable", "产品反馈暂时不可用，请稍后重试")
			return
		}
		config["feedbackSessionToken"] = session.Token
		config["expiresAt"] = session.ExpiresAt.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, map[string]any{"config": config})
}

func (s *Server) requireFeedbackCapability(ctx context.Context, original *http.Request, tenantID, subjectID string) error {
	payload, _ := json.Marshal(map[string]string{"audience": "feedback"})
	request, err := s.iamRequest(ctx, http.MethodPost, "/api/v1/credentials", bytes.NewReader(payload), original)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("IAM rejected feedback capability")
	}
	var issued credentialResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&issued); err != nil || issued.Token == "" || issued.TokenType != "Bearer" {
		return fmt.Errorf("IAM returned invalid feedback credential")
	}
	claims, err := s.verifier.Verify(issued.Token, "feedback", feedbackUseAction)
	if err != nil || claims.Tenant != tenantID || claims.Subject != subjectID {
		return fmt.Errorf("feedback credential identity mismatch")
	}
	return nil
}

func (s *Server) exchangeFeedbackSession(ctx context.Context, customUserID string) (feedbackSession, error) {
	payload, _ := json.Marshal(map[string]any{"project_id": s.feedback.projectID, "custom_user_id": customUserID})
	endpoint := s.feedback.gatewayURL + "/workshop/v2/apikey/feedback-sessions"
	for index, key := range s.feedback.apiKeys {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return feedbackSession{}, err
		}
		request.Header.Set("Authorization", "Bearer "+key)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json")
		response, err := s.client.Do(request)
		if err != nil {
			return feedbackSession{}, err
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		_ = response.Body.Close()
		if readErr != nil {
			return feedbackSession{}, readErr
		}
		if (response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden) && index+1 < len(s.feedback.apiKeys) {
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return feedbackSession{}, fmt.Errorf("feedback session exchange returned HTTP %d", response.StatusCode)
		}
		var envelope feedbackSessionEnvelope
		if err := json.Unmarshal(body, &envelope); err != nil || (envelope.Code != "" && envelope.Code != "OK") {
			return feedbackSession{}, fmt.Errorf("feedback session response is invalid")
		}
		value := envelope.Data
		if value.Token == "" || value.TokenType != "Bearer" || value.ProjectID != s.feedback.projectID || value.CustomUserID != customUserID || !value.ExpiresAt.After(time.Now().UTC()) {
			return feedbackSession{}, fmt.Errorf("feedback session scope is invalid")
		}
		return value, nil
	}
	return feedbackSession{}, fmt.Errorf("feedback session exchange failed")
}
