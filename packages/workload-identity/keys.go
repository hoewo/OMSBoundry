package workloadidentity

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
	"time"
)

const keyFileVersion = "1"

var (
	uuidPattern       = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,63}$`)
)

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
	WorkloadID  string      `json:"workload_id"`
	SubjectID   string      `json:"subject_id"`
	ActiveKeyID string      `json:"active_kid"`
	Keys        []keyRecord `json:"keys"`
}

type publicKey struct {
	KTY         string `json:"kty"`
	CRV         string `json:"crv"`
	Use         string `json:"use"`
	ALG         string `json:"alg"`
	KeyID       string `json:"kid"`
	X           string `json:"x"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	RetireAfter string `json:"retire_after,omitempty"`
}

type publicBundle struct {
	Version    string      `json:"version"`
	WorkloadID string      `json:"workload_id"`
	SubjectID  string      `json:"subject_id"`
	Keys       []publicKey `json:"keys"`
}

type assertionHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid"`
}

type AssertionClaims struct {
	Version       string `json:"ver"`
	Issuer        string `json:"iss"`
	Subject       string `json:"sub"`
	Tenant        string `json:"tid"`
	Audience      string `json:"aud"`
	Purpose       string `json:"purpose"`
	RequestSHA256 string `json:"request_sha256"`
	IssuedAt      int64  `json:"iat"`
	NotBefore     int64  `json:"nbf"`
	ExpiresAt     int64  `json:"exp"`
	ID            string `json:"jti"`
}

type KeyStore struct {
	path       string
	workloadID string
	subjectID  string
}

func OpenKeyStore(path, workloadID, subjectID string) (*KeyStore, error) {
	path, workloadID, subjectID = strings.TrimSpace(path), strings.TrimSpace(workloadID), strings.ToLower(strings.TrimSpace(subjectID))
	if path == "" || !identifierPattern.MatchString(workloadID) || !uuidPattern.MatchString(subjectID) {
		return nil, fmt.Errorf("workload key configuration is invalid")
	}
	store := &KeyStore{path: path, workloadID: workloadID, subjectID: subjectID}
	if _, err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func Initialize(privatePath, publicPath, workloadID, subjectID string, now time.Time) error {
	if strings.TrimSpace(privatePath) == "" || strings.TrimSpace(publicPath) == "" || !identifierPattern.MatchString(workloadID) || !uuidPattern.MatchString(subjectID) {
		return fmt.Errorf("workload key configuration is invalid")
	}
	value, err := loadKeyFile(privatePath)
	if errors.Is(err, os.ErrNotExist) {
		record, keyErr := newKey(now.UTC())
		if keyErr != nil {
			return keyErr
		}
		value = keyFile{Version: keyFileVersion, WorkloadID: workloadID, SubjectID: subjectID, ActiveKeyID: record.KeyID, Keys: []keyRecord{record}}
		if err := saveKeyFile(privatePath, value, true); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if value.WorkloadID != workloadID || value.SubjectID != subjectID {
		return fmt.Errorf("existing workload key identity does not match configuration")
	}
	return writePublicBundle(publicPath, value, now.UTC())
}

func Rotate(privatePath, publicPath, workloadID, subjectID string, now time.Time, verificationWindow time.Duration) (string, error) {
	if verificationWindow < 90*time.Second {
		return "", fmt.Errorf("verification window must be at least 90s")
	}
	value, err := loadKeyFile(privatePath)
	if err != nil {
		return "", err
	}
	if value.WorkloadID != workloadID || value.SubjectID != subjectID {
		return "", fmt.Errorf("workload key identity does not match configuration")
	}
	now = now.UTC()
	for index := range value.Keys {
		if value.Keys[index].KeyID == value.ActiveKeyID {
			value.Keys[index].Status = "retiring"
			value.Keys[index].RetireAfter = now.Add(verificationWindow).Format(time.RFC3339Nano)
		}
	}
	record, err := newKey(now)
	if err != nil {
		return "", err
	}
	value.ActiveKeyID = record.KeyID
	value.Keys = append(value.Keys, record)
	if err := saveKeyFile(privatePath, value, false); err != nil {
		return "", err
	}
	if err := writePublicBundle(publicPath, value, now); err != nil {
		return "", err
	}
	return record.KeyID, nil
}

func Prune(privatePath, publicPath, workloadID, subjectID string, now time.Time) error {
	value, err := loadKeyFile(privatePath)
	if err != nil {
		return err
	}
	if value.WorkloadID != workloadID || value.SubjectID != subjectID {
		return fmt.Errorf("workload key identity does not match configuration")
	}
	now = now.UTC()
	kept := make([]keyRecord, 0, len(value.Keys))
	for _, record := range value.Keys {
		if record.Status == "active" {
			kept = append(kept, record)
			continue
		}
		retireAfter, parseErr := time.Parse(time.RFC3339Nano, record.RetireAfter)
		if parseErr != nil {
			return parseErr
		}
		if now.Before(retireAfter) {
			kept = append(kept, record)
		}
	}
	value.Keys = kept
	if err := saveKeyFile(privatePath, value, false); err != nil {
		return err
	}
	return writePublicBundle(publicPath, value, now)
}

func (s *KeyStore) SignAssertion(tenantID, requestSHA256 string, now time.Time) (string, string, error) {
	if !uuidPattern.MatchString(strings.ToLower(strings.TrimSpace(tenantID))) || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(requestSHA256) {
		return "", "", fmt.Errorf("workload assertion input is invalid")
	}
	value, err := s.load()
	if err != nil {
		return "", "", err
	}
	record, ok := findKey(value, value.ActiveKeyID)
	if !ok || record.Status != "active" {
		return "", "", fmt.Errorf("active workload key is unavailable")
	}
	privateKey, err := base64.RawURLEncoding.DecodeString(record.PrivateKey)
	if err != nil || len(privateKey) != ed25519.PrivateKeySize {
		return "", "", fmt.Errorf("active workload private key is invalid")
	}
	id, err := randomUUID()
	if err != nil {
		return "", "", err
	}
	now = now.UTC()
	claims := AssertionClaims{
		Version: "1", Issuer: s.workloadID, Subject: s.subjectID, Tenant: tenantID,
		Audience: "identity-access", Purpose: "capability-exchange", RequestSHA256: requestSHA256,
		IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(), ID: id,
	}
	header, _ := json.Marshal(assertionHeader{Algorithm: "EdDSA", Type: "JWT", KeyID: record.KeyID})
	body, err := json.Marshal(claims)
	if err != nil {
		return "", "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	signature := ed25519.Sign(ed25519.PrivateKey(privateKey), []byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), record.KeyID, nil
}

func (s *KeyStore) load() (keyFile, error) {
	value, err := loadKeyFile(s.path)
	if err != nil {
		return keyFile{}, err
	}
	if value.WorkloadID != s.workloadID || value.SubjectID != s.subjectID {
		return keyFile{}, fmt.Errorf("workload key identity does not match configuration")
	}
	return value, nil
}

func loadKeyFile(path string) (keyFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return keyFile{}, err
	}
	if info, statErr := os.Stat(path); statErr != nil {
		return keyFile{}, statErr
	} else if info.Mode().Perm()&0o077 != 0 {
		return keyFile{}, fmt.Errorf("workload private key file permissions %o expose private keys", info.Mode().Perm())
	}
	var value keyFile
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return keyFile{}, err
	}
	if err := validateKeyFile(value); err != nil {
		return keyFile{}, err
	}
	return value, nil
}

func validateKeyFile(value keyFile) error {
	if value.Version != keyFileVersion || !identifierPattern.MatchString(value.WorkloadID) || !uuidPattern.MatchString(value.SubjectID) || !uuidPattern.MatchString(value.ActiveKeyID) || len(value.Keys) == 0 {
		return fmt.Errorf("workload key file identity is invalid")
	}
	active := 0
	seen := map[string]bool{}
	for _, record := range value.Keys {
		if !uuidPattern.MatchString(record.KeyID) || seen[record.KeyID] {
			return fmt.Errorf("workload key ID is invalid or duplicated")
		}
		seen[record.KeyID] = true
		publicValue, publicErr := base64.RawURLEncoding.DecodeString(record.PublicKey)
		privateValue, privateErr := base64.RawURLEncoding.DecodeString(record.PrivateKey)
		if publicErr != nil || privateErr != nil || len(publicValue) != ed25519.PublicKeySize || len(privateValue) != ed25519.PrivateKeySize || !ed25519.PublicKey(publicValue).Equal(ed25519.PrivateKey(privateValue).Public()) {
			return fmt.Errorf("workload key material is invalid")
		}
		if _, err := time.Parse(time.RFC3339Nano, record.CreatedAt); err != nil {
			return err
		}
		switch record.Status {
		case "active":
			if record.KeyID != value.ActiveKeyID || record.RetireAfter != "" {
				return fmt.Errorf("active workload key metadata is inconsistent")
			}
			active++
		case "retiring":
			if _, err := time.Parse(time.RFC3339Nano, record.RetireAfter); err != nil {
				return err
			}
		default:
			return fmt.Errorf("workload key status is invalid")
		}
	}
	if active != 1 {
		return fmt.Errorf("workload key file must contain exactly one active key")
	}
	return nil
}

func newKey(now time.Time) (keyRecord, error) {
	publicValue, privateValue, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return keyRecord{}, err
	}
	id, err := randomUUID()
	if err != nil {
		return keyRecord{}, err
	}
	return keyRecord{KeyID: id, Status: "active", CreatedAt: now.Format(time.RFC3339Nano), PublicKey: base64.RawURLEncoding.EncodeToString(publicValue), PrivateKey: base64.RawURLEncoding.EncodeToString(privateValue)}, nil
}

func findKey(value keyFile, id string) (keyRecord, bool) {
	for _, record := range value.Keys {
		if record.KeyID == id {
			return record, true
		}
	}
	return keyRecord{}, false
}

func writePublicBundle(path string, value keyFile, now time.Time) error {
	keys := make([]publicKey, 0, len(value.Keys))
	for _, record := range value.Keys {
		if record.RetireAfter != "" {
			retireAfter, err := time.Parse(time.RFC3339Nano, record.RetireAfter)
			if err != nil {
				return err
			}
			if !now.Before(retireAfter) {
				continue
			}
		}
		keys = append(keys, publicKey{KTY: "OKP", CRV: "Ed25519", Use: "sig", ALG: "EdDSA", KeyID: record.KeyID, X: record.PublicKey, Status: record.Status, CreatedAt: record.CreatedAt, RetireAfter: record.RetireAfter})
	}
	data, err := json.MarshalIndent(publicBundle{Version: keyFileVersion, WorkloadID: value.WorkloadID, SubjectID: value.SubjectID, Keys: keys}, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append(data, '\n'), 0o644, false)
}

func saveKeyFile(path string, value keyFile, exclusive bool) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append(data, '\n'), 0o600, exclusive)
}

func writeAtomic(path string, data []byte, mode os.FileMode, exclusive bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if exclusive {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			return err
		}
		if _, err := file.Write(data); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Sync(); err != nil {
			_ = file.Close()
			return err
		}
		return file.Close()
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".workload-key-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(mode); err != nil {
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
	return os.Rename(temporaryPath, path)
}

func randomUUID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}
