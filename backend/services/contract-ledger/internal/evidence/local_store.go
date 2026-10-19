package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

var storageKeyPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type LocalStore struct {
	root      string
	temporary string
	objects   string
}

func NewLocalStore(root string) (*LocalStore, error) {
	if root == "" {
		return nil, fmt.Errorf("evidence storage directory is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve evidence storage directory: %w", err)
	}
	if filepath.Clean(absolute) == string(filepath.Separator) {
		return nil, fmt.Errorf("evidence storage directory cannot be filesystem root")
	}
	store := &LocalStore{
		root:      absolute,
		temporary: filepath.Join(absolute, ".tmp"),
		objects:   filepath.Join(absolute, "objects"),
	}
	for _, directory := range []string{store.root, store.temporary, store.objects} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return nil, fmt.Errorf("create evidence storage directory: %w", err)
		}
	}
	return store, nil
}

func (s *LocalStore) Root() string { return s.root }

func (s *LocalStore) CheckWritable(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	probe, err := os.CreateTemp(s.temporary, ".writable-check-*")
	if err != nil {
		return fmt.Errorf("create evidence writable check: %w", err)
	}
	name := probe.Name()
	defer os.Remove(name)
	if err := probe.Chmod(0o600); err != nil {
		_ = probe.Close()
		return fmt.Errorf("restrict evidence writable check: %w", err)
	}
	if _, err := probe.Write([]byte("contract-ledger-evidence-check")); err != nil {
		_ = probe.Close()
		return fmt.Errorf("write evidence writable check: %w", err)
	}
	if err := probe.Sync(); err != nil {
		_ = probe.Close()
		return fmt.Errorf("sync evidence writable check: %w", err)
	}
	if err := probe.Close(); err != nil {
		return fmt.Errorf("close evidence writable check: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Remove(name); err != nil {
		return fmt.Errorf("remove evidence writable check: %w", err)
	}
	return nil
}

func (s *LocalStore) Put(ctx context.Context, storageKey string, source io.Reader, maxBytes int64) (StoredBlob, error) {
	if err := validateStorageKey(storageKey); err != nil {
		return StoredBlob{}, err
	}
	if maxBytes < 1 {
		return StoredBlob{}, fmt.Errorf("max evidence bytes must be positive")
	}
	if err := ctx.Err(); err != nil {
		return StoredBlob{}, err
	}
	temporary, err := os.CreateTemp(s.temporary, ".upload-*")
	if err != nil {
		return StoredBlob{}, fmt.Errorf("create evidence temporary file: %w", err)
	}
	temporaryName := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryName)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return StoredBlob{}, fmt.Errorf("restrict evidence temporary file: %w", err)
	}

	hash := sha256.New()
	prefix := &prefixWriter{limit: 512}
	readLimit := maxBytes
	if maxBytes < math.MaxInt64 {
		readLimit++
	}
	limited := io.LimitReader(source, readLimit)
	size, err := io.Copy(io.MultiWriter(temporary, hash, prefix), limited)
	if err != nil {
		return StoredBlob{}, fmt.Errorf("write evidence content: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return StoredBlob{}, err
	}
	if size == 0 {
		return StoredBlob{}, &EmptyFileError{}
	}
	if size > maxBytes {
		return StoredBlob{}, &FileTooLargeError{Limit: maxBytes}
	}
	if err := temporary.Sync(); err != nil {
		return StoredBlob{}, fmt.Errorf("sync evidence content: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return StoredBlob{}, fmt.Errorf("close evidence temporary file: %w", err)
	}

	destination, err := s.objectPath(storageKey, true)
	if err != nil {
		return StoredBlob{}, err
	}
	if _, err := os.Stat(destination); err == nil {
		return StoredBlob{}, fmt.Errorf("evidence storage key already exists")
	} else if !os.IsNotExist(err) {
		return StoredBlob{}, fmt.Errorf("check evidence destination: %w", err)
	}
	if err := os.Rename(temporaryName, destination); err != nil {
		return StoredBlob{}, fmt.Errorf("commit evidence content: %w", err)
	}
	committed = true
	return StoredBlob{
		StorageKey:        storageKey,
		SizeBytes:         size,
		SHA256:            hex.EncodeToString(hash.Sum(nil)),
		DetectedMediaType: http.DetectContentType(prefix.data),
	}, nil
}

func (s *LocalStore) Open(ctx context.Context, storageKey string) (ReadSeekCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	objectPath, err := s.objectPath(storageKey, false)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(objectPath)
	if err != nil {
		return nil, fmt.Errorf("open evidence content: %w", err)
	}
	return file, nil
}

func (s *LocalStore) Delete(ctx context.Context, storageKey string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	objectPath, err := s.objectPath(storageKey, false)
	if err != nil {
		return err
	}
	if err := os.Remove(objectPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete evidence content: %w", err)
	}
	return nil
}

func (s *LocalStore) objectPath(storageKey string, createDirectory bool) (string, error) {
	if err := validateStorageKey(storageKey); err != nil {
		return "", err
	}
	directory := filepath.Join(s.objects, storageKey[:2])
	if createDirectory {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return "", fmt.Errorf("create evidence object directory: %w", err)
		}
	}
	return filepath.Join(directory, storageKey), nil
}

func validateStorageKey(storageKey string) error {
	if !storageKeyPattern.MatchString(storageKey) {
		return fmt.Errorf("invalid evidence storage key")
	}
	return nil
}

type prefixWriter struct {
	data  []byte
	limit int
}

func (w *prefixWriter) Write(value []byte) (int, error) {
	remaining := w.limit - len(w.data)
	if remaining > 0 {
		if remaining > len(value) {
			remaining = len(value)
		}
		w.data = append(w.data, value[:remaining]...)
	}
	return len(value), nil
}
