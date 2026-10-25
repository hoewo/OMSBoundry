package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrNotFound             = errors.New("stored object not found")
	ErrExpiredAuthorization = errors.New("storage authorization expired")
	ErrInvalidAuthorization = errors.New("invalid storage authorization")
	ErrObjectExists         = errors.New("stored object already exists")
)

type ObjectRef struct {
	TenantID  string
	Provider  string
	Bucket    string
	ObjectKey string
}

type UploadIntent struct {
	SessionID         string
	FileID            string
	Version           int
	FileName          string
	MediaType         string
	ExpectedSHA256    string
	ExpectedSize      int64
	ValidationProfile string
	CreatedBySubject  string
	ExpiresAt         time.Time
	Object            ObjectRef
}

type UploadAuthorization struct {
	Provider        string             `json:"provider"`
	Region          string             `json:"region,omitempty"`
	Endpoint        string             `json:"endpoint,omitempty"`
	EndpointIsCName bool               `json:"endpoint_is_cname,omitempty"`
	Bucket          string             `json:"bucket,omitempty"`
	ObjectKey       string             `json:"object_key"`
	AccessKeyID     string             `json:"access_key_id,omitempty"`
	AccessKeySecret string             `json:"access_key_secret,omitempty"`
	SecurityToken   string             `json:"security_token,omitempty"`
	UploadURL       string             `json:"upload_url,omitempty"`
	Method          string             `json:"method,omitempty"`
	ExpiresAt       time.Time          `json:"expires_at"`
	Multipart       *MultipartOptions  `json:"multipart,omitempty"`
	Scope           AuthorizationScope `json:"scope"`
	Headers         map[string]string  `json:"headers,omitempty"`
}

type MultipartOptions struct {
	ThresholdBytes int64 `json:"threshold_bytes"`
	PartSizeBytes  int64 `json:"part_size_bytes"`
	Parallel       int   `json:"parallel"`
}
type AuthorizationScope struct {
	Bucket         string   `json:"bucket,omitempty"`
	ObjectKey      string   `json:"object_key"`
	AllowedActions []string `json:"allowed_actions"`
}

type ObjectMetadata struct {
	SizeBytes    int64
	ETag         string
	LastModified time.Time
}
type ObjectReader struct {
	Reader   io.ReadCloser
	Metadata ObjectMetadata
}

type AccessIntent struct {
	FileID      string
	Version     int
	FileName    string
	Disposition string
	Object      ObjectRef
}
type AccessGrant struct {
	URL          string
	ExpiresAt    time.Time
	RefreshAfter time.Time
}

type Provider interface {
	Name() string
	Ready(context.Context) error
	AuthorizeUpload(context.Context, UploadIntent) (UploadAuthorization, error)
	Open(context.Context, ObjectRef) (ObjectReader, error)
	AuthorizeAccess(context.Context, AccessIntent) (AccessGrant, error)
	Delete(context.Context, ObjectRef) error
}

type LocalDataPlane interface {
	VerifyUploadToken(token string, intent UploadIntent) error
	Put(context.Context, UploadIntent, io.Reader, int64) (string, error)
	VerifyAccessToken(token string, intent AccessIntent) error
}
