package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LocalProvider struct {
	root      string
	baseURL   string
	key       []byte
	accessTTL time.Duration
	now       func() time.Time
}

type localCapability struct {
	Version     string `json:"v"`
	Action      string `json:"a"`
	TenantID    string `json:"t"`
	TargetID    string `json:"i"`
	ObjectKey   string `json:"o"`
	Disposition string `json:"d,omitempty"`
	FileName    string `json:"n,omitempty"`
	ExpiresAt   int64  `json:"e"`
}

func NewLocalProvider(root, baseURL string, key []byte, accessTTL time.Duration) (*LocalProvider, error) {
	if !filepath.IsAbs(root) || strings.TrimSpace(baseURL) == "" || len(key) < 32 || accessTTL <= 0 {
		return nil, fmt.Errorf("local storage configuration is invalid")
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("create local storage root: %w", err)
	}
	return &LocalProvider{root: filepath.Clean(root), baseURL: strings.TrimRight(baseURL, "/"), key: append([]byte(nil), key...), accessTTL: accessTTL, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (p *LocalProvider) Name() string { return "local" }
func (p *LocalProvider) Ready(_ context.Context) error {
	file, err := os.CreateTemp(p.root, ".ready-*")
	if err != nil {
		return err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
}
func (p *LocalProvider) AuthorizeUpload(_ context.Context, intent UploadIntent) (UploadAuthorization, error) {
	if intent.Object.Provider != p.Name() || intent.SessionID == "" || intent.Object.ObjectKey == "" || !intent.ExpiresAt.After(p.now()) {
		return UploadAuthorization{}, ErrInvalidAuthorization
	}
	capability := localCapability{Version: "1", Action: "put", TenantID: intent.Object.TenantID, TargetID: intent.SessionID, ObjectKey: intent.Object.ObjectKey, ExpiresAt: intent.ExpiresAt.Unix()}
	token, err := p.sign(capability)
	if err != nil {
		return UploadAuthorization{}, err
	}
	u := p.baseURL + "/v1/local-uploads/" + url.PathEscape(intent.SessionID) + "?token=" + url.QueryEscape(token)
	return UploadAuthorization{Provider: "local-http", UploadURL: u, Method: "PUT", ObjectKey: intent.Object.ObjectKey, ExpiresAt: intent.ExpiresAt, Scope: AuthorizationScope{ObjectKey: intent.Object.ObjectKey, AllowedActions: []string{"evidence:PutObject"}}, Headers: map[string]string{"Content-Type": normalizedMediaType(intent.MediaType)}}, nil
}
func (p *LocalProvider) VerifyUploadToken(token string, intent UploadIntent) error {
	return p.verify(token, localCapability{Version: "1", Action: "put", TenantID: intent.Object.TenantID, TargetID: intent.SessionID, ObjectKey: intent.Object.ObjectKey, ExpiresAt: intent.ExpiresAt.Unix()})
}
func (p *LocalProvider) Put(ctx context.Context, intent UploadIntent, reader io.Reader, contentLength int64) (string, error) {
	if contentLength < 0 || contentLength != intent.ExpectedSize {
		return "", fmt.Errorf("content length does not match expected size")
	}
	path, err := p.path(intent.Object.ObjectKey)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return "", ErrObjectExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return "", err
	}
	tempName := temp.Name()
	keep := false
	defer func() {
		_ = temp.Close()
		if !keep {
			_ = os.Remove(tempName)
		}
	}()
	hash := sha256.New()
	limited := io.LimitReader(reader, intent.ExpectedSize+1)
	written, err := io.Copy(io.MultiWriter(temp, hash), &contextReader{ctx: ctx, reader: limited})
	if err != nil {
		return "", err
	}
	if written != intent.ExpectedSize {
		return "", fmt.Errorf("uploaded size %d does not match expected %d", written, intent.ExpectedSize)
	}
	if err := temp.Sync(); err != nil {
		return "", err
	}
	if err := temp.Close(); err != nil {
		return "", err
	}
	// Link publishes the fully-written temporary object without replacing an
	// existing path. os.Rename would silently overwrite on Unix, which would
	// break the one-upload-session/one-object invariant under concurrent PUTs.
	if err := os.Link(tempName, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", ErrObjectExists
		}
		return "", err
	}
	if err := os.Remove(tempName); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	keep = true
	return hex.EncodeToString(hash.Sum(nil)), nil
}
func (p *LocalProvider) Open(_ context.Context, ref ObjectRef) (ObjectReader, error) {
	path, err := p.path(ref.ObjectKey)
	if err != nil {
		return ObjectReader{}, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return ObjectReader{}, ErrNotFound
	}
	if err != nil {
		return ObjectReader{}, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return ObjectReader{}, err
	}
	return ObjectReader{Reader: file, Metadata: ObjectMetadata{SizeBytes: info.Size(), LastModified: info.ModTime().UTC()}}, nil
}
func (p *LocalProvider) AuthorizeAccess(_ context.Context, intent AccessIntent) (AccessGrant, error) {
	expires := p.now().Add(p.accessTTL)
	target := fmt.Sprintf("%s:%d", intent.FileID, intent.Version)
	capability := localCapability{Version: "1", Action: "get", TenantID: intent.Object.TenantID, TargetID: target, ObjectKey: intent.Object.ObjectKey, Disposition: intent.Disposition, FileName: intent.FileName, ExpiresAt: expires.Unix()}
	token, err := p.sign(capability)
	if err != nil {
		return AccessGrant{}, err
	}
	u := p.baseURL + "/v1/local-files/" + url.PathEscape(intent.FileID) + "/versions/" + fmt.Sprint(intent.Version) + "?disposition=" + url.QueryEscape(intent.Disposition) + "&file_name=" + url.QueryEscape(intent.FileName) + "&token=" + url.QueryEscape(token)
	return AccessGrant{URL: u, ExpiresAt: expires, RefreshAfter: expires.Add(-minDuration(time.Minute, p.accessTTL/2))}, nil
}
func (p *LocalProvider) VerifyAccessToken(token string, intent AccessIntent) error {
	target := fmt.Sprintf("%s:%d", intent.FileID, intent.Version)
	expected := localCapability{Version: "1", Action: "get", TenantID: intent.Object.TenantID, TargetID: target, ObjectKey: intent.Object.ObjectKey, Disposition: intent.Disposition, FileName: intent.FileName}
	return p.verifyIgnoringExpectedExpiry(token, expected)
}
func (p *LocalProvider) Delete(_ context.Context, ref ObjectRef) error {
	path, err := p.path(ref.ObjectKey)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (p *LocalProvider) sign(value localCapability) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(data)
	mac := hmac.New(sha256.New, p.key)
	_, _ = mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func (p *LocalProvider) verify(token string, expected localCapability) error {
	value, err := p.decode(token)
	if err != nil {
		return err
	}
	if value.ExpiresAt != expected.ExpiresAt {
		return ErrInvalidAuthorization
	}
	expected.ExpiresAt = value.ExpiresAt
	return p.compare(value, expected)
}
func (p *LocalProvider) verifyIgnoringExpectedExpiry(token string, expected localCapability) error {
	value, err := p.decode(token)
	if err != nil {
		return err
	}
	expected.ExpiresAt = value.ExpiresAt
	return p.compare(value, expected)
}
func (p *LocalProvider) decode(token string) (localCapability, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return localCapability{}, ErrInvalidAuthorization
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return localCapability{}, ErrInvalidAuthorization
	}
	mac := hmac.New(sha256.New, p.key)
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return localCapability{}, ErrInvalidAuthorization
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return localCapability{}, ErrInvalidAuthorization
	}
	var value localCapability
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return localCapability{}, ErrInvalidAuthorization
	}
	if value.ExpiresAt <= p.now().Unix() {
		return localCapability{}, ErrExpiredAuthorization
	}
	return value, nil
}
func (p *LocalProvider) compare(actual, expected localCapability) error {
	if actual != expected {
		return ErrInvalidAuthorization
	}
	return nil
}
func (p *LocalProvider) path(key string) (string, error) {
	if key == "" || filepath.IsAbs(key) || strings.Contains(key, "\\") {
		return "", ErrInvalidAuthorization
	}
	clean := filepath.Clean(filepath.FromSlash(key))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrInvalidAuthorization
	}
	result := filepath.Join(p.root, clean)
	rel, err := filepath.Rel(p.root, result)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrInvalidAuthorization
	}
	return result, nil
}
func normalizedMediaType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "application/octet-stream"
	}
	return value
}
func minDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(buffer)
	}
}
