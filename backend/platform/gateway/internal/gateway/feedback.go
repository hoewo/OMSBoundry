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
	"strconv"
	"strings"
	"time"
)

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

type feedbackListRecord struct {
	ID int64 `json:"id"`
}

type feedbackListEnvelope struct {
	Code string               `json:"code"`
	Data []feedbackListRecord `json:"data"`
	Meta struct {
		Total int64 `json:"total"`
	} `json:"meta"`
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

// feedbackSubmissionSnapshot is a narrow same-origin observation port for the
// Workbench host. The embedded SDK does not expose a submission-completed
// event, so the host compares this current-account ID/count snapshot before and
// after the modal opens and can treat creation as the terminal UI result. The Gateway
// neither stores nor mutates Feedback facts and never returns the server-held
// API Key to this endpoint.
func (s *Server) feedbackSubmissionSnapshot(w http.ResponseWriter, r *http.Request) {
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
	customUserID := deriveFeedbackUserID(s.feedback.identityHMACKey, s.feedback.identityVersion, principal.Principal.TenantID, principal.Principal.SubjectID)
	ids, total, err := s.fetchFeedbackIDs(r.Context(), customUserID)
	if err != nil {
		writeProblem(w, http.StatusBadGateway, "feedback_unavailable", "产品反馈暂时不可用，请稍后重试")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"feedback_ids": ids, "feedback_count": strconv.FormatInt(total, 10)})
}

func (s *Server) fetchFeedbackIDs(ctx context.Context, customUserID string) ([]string, int64, error) {
	endpoint, err := url.Parse(s.feedback.gatewayURL + "/workshop/v2/apikey/feedbacks")
	if err != nil {
		return nil, 0, err
	}
	query := endpoint.Query()
	query.Set("page", "1")
	query.Set("page_size", "20")
	query.Set("project_id", strconv.Itoa(s.feedback.projectID))
	query.Set("custom_user_id", customUserID)
	endpoint.RawQuery = query.Encode()

	for index, key := range s.feedback.apiKeys {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return nil, 0, err
		}
		request.Header.Set("Authorization", "Bearer "+key)
		request.Header.Set("Accept", "application/json")
		response, err := s.client.Do(request)
		if err != nil {
			return nil, 0, err
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 128<<10))
		_ = response.Body.Close()
		if readErr != nil {
			return nil, 0, readErr
		}
		if (response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden) && index+1 < len(s.feedback.apiKeys) {
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, 0, fmt.Errorf("feedback list returned HTTP %d", response.StatusCode)
		}
		var envelope feedbackListEnvelope
		if err := json.Unmarshal(body, &envelope); err != nil || (envelope.Code != "" && envelope.Code != "OK") {
			return nil, 0, fmt.Errorf("feedback list response is invalid")
		}
		ids := make([]string, 0, len(envelope.Data))
		for _, item := range envelope.Data {
			if item.ID < 1 {
				return nil, 0, fmt.Errorf("feedback list contains an invalid ID")
			}
			ids = append(ids, strconv.FormatInt(item.ID, 10))
		}
		total := envelope.Meta.Total
		if total < int64(len(ids)) {
			total = int64(len(ids))
		}
		return ids, total, nil
	}
	return nil, 0, fmt.Errorf("feedback list failed")
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
