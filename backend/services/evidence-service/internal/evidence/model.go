package evidence

import (
	"errors"
	"time"

	"omsbizfoundry/evidence-service/internal/storage"
)

var (
	ErrInvalid             = errors.New("invalid evidence input")
	ErrNotFound            = errors.New("evidence resource not found")
	ErrConflict            = errors.New("evidence state conflict")
	ErrIdempotencyConflict = errors.New("idempotency key payload conflict")
	ErrExpired             = errors.New("upload session expired")
)

type Actor struct{ TenantID, SubjectID, DisplayName, Kind string }

type ClientContext struct {
	OwnerRef string            `json:"owner_ref,omitempty"`
	Purpose  string            `json:"purpose,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}
type CreateUploadInput struct {
	FileName      string        `json:"file_name"`
	SizeBytes     int64         `json:"size_bytes"`
	MediaType     string        `json:"media_type,omitempty"`
	SHA256        string        `json:"sha256,omitempty"`
	ClientContext ClientContext `json:"client_context,omitempty"`
}
type CompleteUploadInput struct {
	ObservedSizeBytes int64           `json:"observed_size_bytes"`
	ObservedSHA256    string          `json:"observed_sha256,omitempty"`
	ETag              string          `json:"etag,omitempty"`
	Parts             []CompletedPart `json:"parts,omitempty"`
}
type CompletedPart struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
	SizeBytes  int64  `json:"size_bytes,omitempty"`
}
type AbortUploadInput struct {
	Reason string `json:"reason"`
}
type AccessInput struct {
	Disposition  string `json:"disposition"`
	DeliveryMode string `json:"delivery_mode"`
	FileName     string `json:"file_name,omitempty"`
}

type UploadSession struct {
	SessionID     string                      `json:"session_id"`
	FileID        string                      `json:"file_id"`
	Version       int                         `json:"version"`
	Status        string                      `json:"status"`
	ExpiresAt     time.Time                   `json:"expires_at"`
	Authorization storage.UploadAuthorization `json:"authorization"`
}

type Version struct {
	FileID    string            `json:"file_id"`
	Version   int               `json:"version"`
	Status    string            `json:"status"`
	FileName  string            `json:"file_name"`
	SizeBytes int64             `json:"size_bytes"`
	MediaType string            `json:"media_type"`
	SHA256    string            `json:"sha256"`
	CreatedAt time.Time         `json:"created_at"`
	Object    storage.ObjectRef `json:"-"`
}

type VersionPage struct {
	Items []Version `json:"items"`
}

type AccessGrant struct {
	FileID       string    `json:"file_id"`
	Version      int       `json:"version"`
	URL          string    `json:"url"`
	Method       string    `json:"method"`
	Disposition  string    `json:"disposition"`
	DeliveryMode string    `json:"delivery_mode"`
	ExpiresAt    time.Time `json:"expires_at"`
	RefreshAfter time.Time `json:"refresh_after"`
}

type CleanupResult struct {
	Examined  int `json:"examined"`
	Cleaned   int `json:"cleaned"`
	Failed    int `json:"failed"`
	Protected int `json:"protected"`
}
