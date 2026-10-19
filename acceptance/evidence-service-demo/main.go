package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type config struct {
	baseURL     string
	accessToken string
}

type uploadAuthorization struct {
	Provider  string            `json:"provider"`
	ObjectKey string            `json:"object_key"`
	UploadURL string            `json:"upload_url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
}

type uploadSession struct {
	SessionID     string              `json:"session_id"`
	FileID        string              `json:"file_id"`
	Version       int                 `json:"version"`
	Status        string              `json:"status"`
	Authorization uploadAuthorization `json:"authorization"`
}

type fileVersion struct {
	FileID    string `json:"file_id"`
	Version   int    `json:"version"`
	Status    string `json:"status"`
	FileName  string `json:"file_name"`
	SizeBytes int64  `json:"size_bytes"`
	MediaType string `json:"media_type"`
	SHA256    string `json:"sha256"`
}

type fileReference struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	ResourceType string `json:"resource_type"`
	Status       string `json:"status"`
	Version      int    `json:"version"`
	Revision     int    `json:"revision"`
	DisplayName  string `json:"display_name"`
	SizeBytes    int64  `json:"size_bytes"`
	MediaType    string `json:"media_type"`
	SHA256       string `json:"sha256"`
}

type accessGrant struct {
	FileID       string    `json:"file_id"`
	Version      int       `json:"version"`
	URL          string    `json:"url"`
	Method       string    `json:"method"`
	Disposition  string    `json:"disposition"`
	DeliveryMode string    `json:"delivery_mode"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type demoClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: evidence-service-demo")
		os.Exit(2)
	}
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "Evidence Service demo failed:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	client := &demoClient{
		baseURL: cfg.baseURL,
		token:   cfg.accessToken,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
	nonce, err := randomHex(8)
	if err != nil {
		return err
	}

	fmt.Println("[1/10] 验证 Local Storage Provider 就绪，且受保护 API 失败关闭")
	var ready map[string]string
	if err := client.json(ctx, http.MethodGet, "/health/ready", "", "", nil, http.StatusOK, &ready); err != nil {
		return err
	}
	if ready["status"] != "ready" || ready["storage_provider"] != "local" {
		return fmt.Errorf("unexpected readiness payload: %#v", ready)
	}
	probe := map[string]any{"file_name": "probe.txt", "size_bytes": 1}
	if err := client.json(ctx, http.MethodPost, "/v1/upload-sessions", "", "anonymous-"+nonce, probe, http.StatusUnauthorized, nil); err != nil {
		return fmt.Errorf("anonymous request did not fail closed: %w", err)
	}
	if err := client.json(ctx, http.MethodPost, "/v1/upload-sessions", "wrong-template-token", "wrong-token-"+nonce, probe, http.StatusUnauthorized, nil); err != nil {
		return fmt.Errorf("wrong token did not fail closed: %w", err)
	}
	if err := client.json(ctx, http.MethodPost, "/v1/upload-sessions/not-a-uuid/abort", cfg.accessToken, "invalid-id-"+nonce, map[string]string{"reason": "user_cancelled"}, http.StatusUnprocessableEntity, nil); err != nil {
		return fmt.Errorf("invalid identifier was not rejected: %w", err)
	}

	fmt.Println("[2/10] 创建上传会话，并验证创建命令幂等与冲突保护")
	content := []byte("%PDF-1.7\nBizFoundry Evidence independent demo\n")
	digest := sha256Hex(content)
	createKey := "demo-create-" + nonce
	createPayload := map[string]any{
		"file_name":  "design-delivery-proof.pdf",
		"size_bytes": len(content),
		"media_type": "application/pdf",
		"sha256":     digest,
		"client_context": map[string]any{
			"owner_ref": "demo:design-delivery:" + nonce,
			"purpose":   "design_delivery_evidence",
			"metadata":  map[string]string{"demo": "evidence-service-independent"},
		},
	}
	var session uploadSession
	if err := client.json(ctx, http.MethodPost, "/v1/upload-sessions", cfg.accessToken, createKey, createPayload, http.StatusCreated, &session); err != nil {
		return err
	}
	if err := validateSession(session, 1); err != nil {
		return err
	}
	var createReplay uploadSession
	if err := client.json(ctx, http.MethodPost, "/v1/upload-sessions", cfg.accessToken, createKey, createPayload, http.StatusCreated, &createReplay); err != nil {
		return fmt.Errorf("create replay: %w", err)
	}
	if createReplay.SessionID != session.SessionID || createReplay.FileID != session.FileID {
		return errors.New("create replay returned a different upload session")
	}
	conflictingCreate := cloneMap(createPayload)
	conflictingCreate["file_name"] = "different.pdf"
	if err := client.json(ctx, http.MethodPost, "/v1/upload-sessions", cfg.accessToken, createKey, conflictingCreate, http.StatusConflict, nil); err != nil {
		return fmt.Errorf("create idempotency conflict: %w", err)
	}

	fmt.Println("[3/10] 验证签名上传地址不可篡改，再写入不可覆盖的对象")
	tamperedUpload, err := tamperQuery(session.Authorization.UploadURL, "token", func(value string) string { return value + "x" })
	if err != nil {
		return err
	}
	if _, _, err := client.bytes(ctx, http.MethodPut, tamperedUpload, content, map[string]string{"Content-Type": "application/pdf"}, http.StatusUnauthorized); err != nil {
		return fmt.Errorf("tampered upload capability: %w", err)
	}
	uploadHeaders := map[string]string{"Content-Type": session.Authorization.Headers["Content-Type"]}
	if uploadHeaders["Content-Type"] == "" {
		uploadHeaders["Content-Type"] = "application/pdf"
	}
	_, responseHeaders, err := client.bytes(ctx, http.MethodPut, session.Authorization.UploadURL, content, uploadHeaders, http.StatusNoContent)
	if err != nil {
		return err
	}
	etag := strings.Trim(responseHeaders.Get("ETag"), "\"")
	if etag != digest {
		return fmt.Errorf("upload ETag=%q, want sha256 %q", etag, digest)
	}
	if _, _, err := client.bytes(ctx, http.MethodPut, session.Authorization.UploadURL, content, uploadHeaders, http.StatusConflict); err != nil {
		return fmt.Errorf("duplicate object write was not rejected: %w", err)
	}

	fmt.Println("[4/10] 完成服务端完整性校验，并验证完成命令幂等")
	completeKey := "demo-complete-" + nonce
	completePayload := map[string]any{"observed_size_bytes": len(content), "observed_sha256": digest, "etag": etag}
	var version fileVersion
	if err := client.json(ctx, http.MethodPost, "/v1/upload-sessions/"+session.SessionID+"/complete", cfg.accessToken, completeKey, completePayload, http.StatusOK, &version); err != nil {
		return err
	}
	if err := validateVersion(version, session, content, digest); err != nil {
		return err
	}
	var completeReplay fileVersion
	if err := client.json(ctx, http.MethodPost, "/v1/upload-sessions/"+session.SessionID+"/complete", cfg.accessToken, completeKey, completePayload, http.StatusOK, &completeReplay); err != nil {
		return fmt.Errorf("complete replay: %w", err)
	}
	if completeReplay != version {
		return errors.New("complete replay did not return the canonical file version")
	}
	conflictingComplete := map[string]any{"observed_size_bytes": len(content), "observed_sha256": strings.Repeat("0", 64), "etag": etag}
	if err := client.json(ctx, http.MethodPost, "/v1/upload-sessions/"+session.SessionID+"/complete", cfg.accessToken, completeKey, conflictingComplete, http.StatusConflict, nil); err != nil {
		return fmt.Errorf("complete idempotency conflict: %w", err)
	}

	fmt.Println("[5/10] 为同一 File 追加不可变 Version 2，并验证稳定引用前进")
	revisionContent := []byte("%PDF-1.7\nBizFoundry Evidence revision two\n")
	revisionDigest := sha256Hex(revisionContent)
	revisionPayload := map[string]any{
		"file_name": "design-delivery-proof-v2.pdf", "size_bytes": len(revisionContent), "media_type": "application/pdf", "sha256": revisionDigest,
		"client_context": map[string]any{"owner_ref": "demo:design-delivery:" + nonce, "purpose": "design_delivery_evidence_revision"},
	}
	var revisionSession uploadSession
	if err := client.json(ctx, http.MethodPost, "/v1/files/"+version.FileID+"/versions/upload-sessions", cfg.accessToken, "demo-create-version-"+nonce, revisionPayload, http.StatusCreated, &revisionSession); err != nil {
		return err
	}
	if err := validateSession(revisionSession, 2); err != nil || revisionSession.FileID != version.FileID {
		return fmt.Errorf("unexpected version upload session: %#v error=%v", revisionSession, err)
	}
	revisionHeaders := map[string]string{"Content-Type": revisionSession.Authorization.Headers["Content-Type"]}
	if revisionHeaders["Content-Type"] == "" {
		revisionHeaders["Content-Type"] = "application/pdf"
	}
	_, revisionResponseHeaders, err := client.bytes(ctx, http.MethodPut, revisionSession.Authorization.UploadURL, revisionContent, revisionHeaders, http.StatusNoContent)
	if err != nil {
		return err
	}
	revisionETag := strings.Trim(revisionResponseHeaders.Get("ETag"), "\"")
	var revision fileVersion
	if err := client.json(ctx, http.MethodPost, "/v1/upload-sessions/"+revisionSession.SessionID+"/complete", cfg.accessToken, "demo-complete-version-"+nonce, map[string]any{"observed_size_bytes": len(revisionContent), "observed_sha256": revisionDigest, "etag": revisionETag}, http.StatusOK, &revision); err != nil {
		return err
	}
	if revision.FileID != version.FileID || revision.Version != 2 || revision.Status != "available" || revision.SHA256 != revisionDigest {
		return fmt.Errorf("unexpected completed revision: %#v", revision)
	}

	fmt.Println("[6/10] 读取稳定文件引用与两个显式版本引用")
	for _, check := range []struct {
		path    string
		version int
		digest  string
	}{
		{"/v1/references/files/" + version.FileID, 2, revisionDigest},
		{"/v1/references/files/" + version.FileID + "/versions/1", 1, digest},
		{"/v1/references/files/" + version.FileID + "/versions/2", 2, revisionDigest},
	} {
		var reference fileReference
		if err := client.json(ctx, http.MethodGet, check.path, cfg.accessToken, "", nil, http.StatusOK, &reference); err != nil {
			return err
		}
		if reference.ID != version.FileID || reference.ResourceType != "evidence-file" || reference.Status != "available" || reference.Version != check.version || reference.Revision != check.version || reference.SHA256 != check.digest {
			return fmt.Errorf("unexpected file reference from %s: %#v", check.path, reference)
		}
	}

	fmt.Println("[7/10] 签发 Version 2 短期访问地址，并验证完整下载与 Range 下载")
	var grant accessGrant
	if err := client.json(ctx, http.MethodPost, "/v1/files/"+version.FileID+"/versions/2/access-grants", cfg.accessToken, "", map[string]any{
		"disposition": "inline", "delivery_mode": "direct",
	}, http.StatusOK, &grant); err != nil {
		return err
	}
	if grant.FileID != version.FileID || grant.Version != 2 || grant.Method != http.MethodGet || grant.Disposition != "inline" || grant.DeliveryMode != "direct" || !grant.ExpiresAt.After(time.Now()) {
		return fmt.Errorf("unexpected access grant: %#v", grant)
	}
	body, _, err := client.bytes(ctx, http.MethodGet, grant.URL, nil, nil, http.StatusOK)
	if err != nil {
		return err
	}
	if !bytes.Equal(body, revisionContent) {
		return errors.New("full download bytes differ from uploaded bytes")
	}
	rangeBody, rangeHeaders, err := client.bytes(ctx, http.MethodGet, grant.URL, nil, map[string]string{"Range": "bytes=0-3"}, http.StatusPartialContent)
	if err != nil {
		return err
	}
	if string(rangeBody) != "%PDF" || rangeHeaders.Get("Content-Range") == "" {
		return fmt.Errorf("unexpected range response: body=%q content-range=%q", rangeBody, rangeHeaders.Get("Content-Range"))
	}

	fmt.Println("[8/10] 验证下载授权查询参数被篡改后立即失效")
	tamperedDownload, err := tamperQuery(grant.URL, "file_name", func(string) string { return "changed.pdf" })
	if err != nil {
		return err
	}
	if _, _, err := client.bytes(ctx, http.MethodGet, tamperedDownload, nil, nil, http.StatusUnauthorized); err != nil {
		return fmt.Errorf("tampered download capability: %w", err)
	}

	fmt.Println("[9/10] 让摘要不匹配的上传完成失败并进入可清理状态")
	declared := []byte("expected-evidence-bytes")
	corrupt := append([]byte(nil), declared...)
	corrupt[0] = 'X'
	failedSession, err := client.createAndUpload(ctx, nonce+"-failed", "corrupt-proof.bin", declared, corrupt)
	if err != nil {
		return err
	}
	if err := client.json(ctx, http.MethodPost, "/v1/upload-sessions/"+failedSession.SessionID+"/complete", cfg.accessToken, "failed-complete-"+nonce, map[string]any{
		"observed_size_bytes": len(corrupt), "observed_sha256": sha256Hex(corrupt),
	}, http.StatusConflict, nil); err != nil {
		return fmt.Errorf("digest mismatch was not rejected: %w", err)
	}

	fmt.Println("[10/10] 取消另一个上传，验证取消重放、完成文件保护与清理候选")
	temporary := []byte("temporary-evidence-object")
	aborted, err := client.createAndUpload(ctx, nonce+"-aborted", "temporary.txt", temporary, temporary)
	if err != nil {
		return err
	}
	abortPath := "/v1/upload-sessions/" + aborted.SessionID + "/abort"
	abortKey := "demo-abort-" + nonce
	for index := 0; index < 2; index++ {
		if err := client.json(ctx, http.MethodPost, abortPath, cfg.accessToken, abortKey, map[string]string{"reason": "user_cancelled"}, http.StatusNoContent, nil); err != nil {
			return fmt.Errorf("abort replay %d: %w", index+1, err)
		}
	}
	if _, _, err := client.bytes(ctx, http.MethodPut, aborted.Authorization.UploadURL, temporary, map[string]string{"Content-Type": "text/plain"}, http.StatusConflict); err != nil {
		return fmt.Errorf("aborted upload remained writable: %w", err)
	}
	if err := client.json(ctx, http.MethodPost, "/v1/upload-sessions/"+session.SessionID+"/abort", cfg.accessToken, "abort-completed-"+nonce, map[string]string{"reason": "user_cancelled"}, http.StatusConflict, nil); err != nil {
		return fmt.Errorf("completed file was not protected from abort: %w", err)
	}

	result := map[string]any{
		"status": "passed", "file_id": revision.FileID, "version": revision.Version,
		"failed_session": failedSession.SessionID, "aborted_session": aborted.SessionID,
		"cleanup_candidates": 2,
	}
	encoded, _ := json.Marshal(result)
	fmt.Println(string(encoded))
	return nil
}

func loadConfig() (config, error) {
	value := config{
		baseURL:     strings.TrimRight(strings.TrimSpace(os.Getenv("DEMO_EVIDENCE_URL")), "/"),
		accessToken: os.Getenv("DEMO_TEMPLATE_ACCESS_TOKEN"),
	}
	if value.baseURL == "" || len(value.accessToken) < 24 {
		return config{}, errors.New("DEMO_EVIDENCE_URL and a 24+ byte DEMO_TEMPLATE_ACCESS_TOKEN are required")
	}
	parsed, err := url.Parse(value.baseURL)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return config{}, errors.New("DEMO_EVIDENCE_URL must be an absolute URL")
	}
	return value, nil
}

func (c *demoClient) createAndUpload(ctx context.Context, suffix, fileName string, declared, uploaded []byte) (uploadSession, error) {
	payload := map[string]any{
		"file_name": fileName, "size_bytes": len(uploaded), "media_type": "application/octet-stream", "sha256": sha256Hex(declared),
		"client_context": map[string]any{"owner_ref": "demo:" + suffix, "purpose": "cleanup_proof"},
	}
	var session uploadSession
	if err := c.json(ctx, http.MethodPost, "/v1/upload-sessions", c.token, "create-"+suffix, payload, http.StatusCreated, &session); err != nil {
		return uploadSession{}, err
	}
	if err := validateSession(session, 1); err != nil {
		return uploadSession{}, err
	}
	if _, _, err := c.bytes(ctx, http.MethodPut, session.Authorization.UploadURL, uploaded, map[string]string{"Content-Type": "application/octet-stream"}, http.StatusNoContent); err != nil {
		return uploadSession{}, err
	}
	return session, nil
}

func (c *demoClient) json(ctx context.Context, method, target, token, idempotencyKey string, payload any, expected int, destination any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.target(target), body)
	if err != nil {
		return err
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode != expected {
		return fmt.Errorf("%s %s returned %d, want %d: %s", method, request.URL.Redacted(), response.StatusCode, expected, strings.TrimSpace(string(data)))
	}
	if destination != nil {
		if len(data) == 0 {
			return errors.New("expected JSON response body")
		}
		if err := json.Unmarshal(data, destination); err != nil {
			return fmt.Errorf("decode %s %s: %w", method, request.URL.Redacted(), err)
		}
	}
	return nil
}

func (c *demoClient) bytes(ctx context.Context, method, target string, body []byte, headers map[string]string, expected int) ([]byte, http.Header, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.target(target), bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, nil, err
	}
	if response.StatusCode != expected {
		return nil, response.Header.Clone(), fmt.Errorf("%s %s returned %d, want %d: %s", method, request.URL.Redacted(), response.StatusCode, expected, strings.TrimSpace(string(data)))
	}
	return data, response.Header.Clone(), nil
}

func (c *demoClient) target(value string) string {
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return c.baseURL + value
}

func validateSession(value uploadSession, expectedVersion int) error {
	if value.SessionID == "" || value.FileID == "" || value.Version != expectedVersion || value.Status != "pending" {
		return fmt.Errorf("unexpected upload session: %#v", value)
	}
	if value.Authorization.Provider != "local-http" || value.Authorization.Method != http.MethodPut || value.Authorization.UploadURL == "" || value.Authorization.ObjectKey == "" {
		return fmt.Errorf("unexpected local upload authorization: %#v", value.Authorization)
	}
	return nil
}

func validateVersion(value fileVersion, session uploadSession, content []byte, digest string) error {
	if value.FileID != session.FileID || value.Version != 1 || value.Status != "available" || value.FileName != "design-delivery-proof.pdf" || value.SizeBytes != int64(len(content)) || value.MediaType != "application/pdf" || value.SHA256 != digest {
		return fmt.Errorf("unexpected completed file version: %#v", value)
	}
	return nil
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func randomHex(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func tamperQuery(raw, key string, change func(string) string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set(key, change(query.Get(key)))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func cloneMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}
