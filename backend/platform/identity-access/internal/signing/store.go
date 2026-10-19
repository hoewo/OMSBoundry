package signing

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const keyFileVersion = "1"

var keyIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type keyRecord struct {
	KeyID       string `json:"kid"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	RetireAfter string `json:"retire_after,omitempty"`
	PublicKey   string `json:"public_key"`
	PrivateKey  string `json:"private_key"`
}

type keyFile struct {
	Version     string      `json:"version"`
	ActiveKeyID string      `json:"active_kid"`
	Keys        []keyRecord `json:"keys"`
}

type PublicKey struct {
	KeyID     string
	Status    string
	PublicKey ed25519.PublicKey
	CreatedAt time.Time
	ExpiresAt time.Time
}

type Store struct {
	mu   sync.RWMutex
	path string
	file keyFile
}

func Open(path string, now time.Time) (*Store, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("signing key file path is required")
	}
	store := &Store{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := store.initialize(now.UTC()); err != nil {
			return nil, err
		}
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read signing key file: %w", err)
	}
	if info, statErr := os.Stat(path); statErr != nil {
		return nil, fmt.Errorf("stat signing key file: %w", statErr)
	} else if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("signing key file permissions %o expose private keys; expected 0600 or stricter", info.Mode().Perm())
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&store.file); err != nil {
		return nil, fmt.Errorf("decode signing key file: %w", err)
	}
	if err := validateFile(store.file); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) initialize(now time.Time) error {
	record, err := newKey(now)
	if err != nil {
		return err
	}
	s.file = keyFile{Version: keyFileVersion, ActiveKeyID: record.KeyID, Keys: []keyRecord{record}}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create signing key directory: %w", err)
	}
	return s.save(true)
}

func (s *Store) Rotate(now time.Time, verificationWindow time.Duration) (string, error) {
	if verificationWindow < 5*time.Minute+30*time.Second {
		return "", fmt.Errorf("verification window must be at least 5m30s")
	}
	now = now.UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.file.Keys {
		if s.file.Keys[index].KeyID == s.file.ActiveKeyID {
			s.file.Keys[index].Status = "retiring"
			s.file.Keys[index].RetireAfter = now.Add(verificationWindow).Format(time.RFC3339Nano)
		}
	}
	record, err := newKey(now)
	if err != nil {
		return "", err
	}
	s.file.ActiveKeyID = record.KeyID
	s.file.Keys = append(s.file.Keys, record)
	if err := s.save(false); err != nil {
		return "", err
	}
	return record.KeyID, nil
}

func (s *Store) Prune(now time.Time) error {
	now = now.UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := make([]keyRecord, 0, len(s.file.Keys))
	for _, record := range s.file.Keys {
		if record.Status == "active" {
			kept = append(kept, record)
			continue
		}
		retireAfter, err := time.Parse(time.RFC3339Nano, record.RetireAfter)
		if err != nil {
			return fmt.Errorf("parse retire time for %s: %w", record.KeyID, err)
		}
		if now.Before(retireAfter) {
			kept = append(kept, record)
		}
	}
	s.file.Keys = kept
	return s.save(false)
}

func (s *Store) Sign(payload any) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, found := findKey(s.file, s.file.ActiveKeyID)
	if !found || record.Status != "active" {
		return "", fmt.Errorf("active signing key is unavailable")
	}
	privateBytes, err := base64.RawURLEncoding.DecodeString(record.PrivateKey)
	if err != nil || len(privateBytes) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("active private key is invalid")
	}
	header, err := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": record.KeyID})
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode credential claims: %w", err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	signature := ed25519.Sign(ed25519.PrivateKey(privateBytes), []byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (s *Store) PublicKeys(now time.Time) ([]PublicKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]PublicKey, 0, len(s.file.Keys))
	for _, record := range s.file.Keys {
		publicBytes, err := base64.RawURLEncoding.DecodeString(record.PublicKey)
		if err != nil || len(publicBytes) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("public key %s is invalid", record.KeyID)
		}
		createdAt, err := time.Parse(time.RFC3339Nano, record.CreatedAt)
		if err != nil {
			return nil, err
		}
		item := PublicKey{KeyID: record.KeyID, Status: record.Status, PublicKey: ed25519.PublicKey(publicBytes), CreatedAt: createdAt}
		if record.RetireAfter != "" {
			item.ExpiresAt, err = time.Parse(time.RFC3339Nano, record.RetireAfter)
			if err != nil {
				return nil, err
			}
			if !now.Before(item.ExpiresAt) {
				continue
			}
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *Store) save(exclusive bool) error {
	data, err := json.MarshalIndent(s.file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode signing key file: %w", err)
	}
	data = append(data, '\n')
	if exclusive {
		file, err := os.OpenFile(s.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return fmt.Errorf("create signing key file: %w", err)
		}
		if _, err := file.Write(data); err != nil {
			_ = file.Close()
			return fmt.Errorf("write signing key file: %w", err)
		}
		if err := file.Sync(); err != nil {
			_ = file.Close()
			return fmt.Errorf("sync signing key file: %w", err)
		}
		return file.Close()
	}
	temporary, err := os.CreateTemp(filepath.Dir(s.path), ".signing-keys-*")
	if err != nil {
		return fmt.Errorf("create temporary signing key file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return fmt.Errorf("replace signing key file: %w", err)
	}
	return nil
}

func newKey(now time.Time) (keyRecord, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return keyRecord{}, fmt.Errorf("generate Ed25519 key: %w", err)
	}
	keyID, err := randomUUID()
	if err != nil {
		return keyRecord{}, err
	}
	return keyRecord{
		KeyID:      keyID,
		Status:     "active",
		CreatedAt:  now.Format(time.RFC3339Nano),
		PublicKey:  base64.RawURLEncoding.EncodeToString(publicKey),
		PrivateKey: base64.RawURLEncoding.EncodeToString(privateKey),
	}, nil
}

func validateFile(file keyFile) error {
	if file.Version != keyFileVersion || !keyIDPattern.MatchString(file.ActiveKeyID) || len(file.Keys) == 0 {
		return fmt.Errorf("signing key file identity is invalid")
	}
	seen := map[string]struct{}{}
	activeCount := 0
	for _, record := range file.Keys {
		if !keyIDPattern.MatchString(record.KeyID) {
			return fmt.Errorf("signing key ID is invalid")
		}
		if _, duplicate := seen[record.KeyID]; duplicate {
			return fmt.Errorf("duplicate signing key %s", record.KeyID)
		}
		seen[record.KeyID] = struct{}{}
		publicKey, publicErr := base64.RawURLEncoding.DecodeString(record.PublicKey)
		privateKey, privateErr := base64.RawURLEncoding.DecodeString(record.PrivateKey)
		if publicErr != nil || privateErr != nil || len(publicKey) != ed25519.PublicKeySize || len(privateKey) != ed25519.PrivateKeySize || !ed25519.PublicKey(publicKey).Equal(ed25519.PrivateKey(privateKey).Public()) {
			return fmt.Errorf("signing key %s material is invalid", record.KeyID)
		}
		if _, err := time.Parse(time.RFC3339Nano, record.CreatedAt); err != nil {
			return fmt.Errorf("signing key %s creation time is invalid", record.KeyID)
		}
		switch record.Status {
		case "active":
			if record.KeyID != file.ActiveKeyID || record.RetireAfter != "" {
				return fmt.Errorf("active signing key metadata is inconsistent")
			}
			activeCount++
		case "retiring":
			if _, err := time.Parse(time.RFC3339Nano, record.RetireAfter); err != nil {
				return fmt.Errorf("retiring key %s retirement time is invalid", record.KeyID)
			}
		default:
			return fmt.Errorf("signing key %s status is invalid", record.KeyID)
		}
	}
	if activeCount != 1 {
		return fmt.Errorf("signing key file must contain exactly one active key")
	}
	return nil
}

func findKey(file keyFile, keyID string) (keyRecord, bool) {
	for _, record := range file.Keys {
		if record.KeyID == keyID {
			return record, true
		}
	}
	return keyRecord{}, false
}

func randomUUID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate key ID: %w", err)
	}
	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		data[0:4], data[4:6], data[6:8], data[8:10], data[10:16]), nil
}
