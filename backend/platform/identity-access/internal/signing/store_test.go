package signing

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreCreatesSignsRotatesAndPrunes(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "keys", "signing.json")
	store, err := Open(path, now)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("key file permission = %o", info.Mode().Perm())
	}
	firstKeys, err := store.PublicKeys(now)
	if err != nil || len(firstKeys) != 1 {
		t.Fatalf("initial public keys = %d, %v", len(firstKeys), err)
	}
	token := mustSign(t, store, map[string]string{"sub": "subject"})
	verifyCompact(t, token, firstKeys[0])
	secondID, err := store.Rotate(now.Add(time.Minute), 6*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if secondID == firstKeys[0].KeyID {
		t.Fatal("rotation reused key ID")
	}
	keys, err := store.PublicKeys(now.Add(2 * time.Minute))
	if err != nil || len(keys) != 2 {
		t.Fatalf("rotated public keys = %d, %v", len(keys), err)
	}
	if err := store.Prune(now.Add(8 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	keys, err = store.PublicKeys(now.Add(8 * time.Minute))
	if err != nil || len(keys) != 1 || keys[0].KeyID != secondID {
		t.Fatalf("pruned keys = %#v, %v", keys, err)
	}
	reopened, err := Open(path, now.Add(8*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	verifyCompact(t, mustSign(t, reopened, map[string]string{"aud": "contract-ledger"}), keys[0])
}

func TestStoreRejectsUnsafePermissionsAndShortRotationWindow(t *testing.T) {
	now := time.Now().UTC()
	path := filepath.Join(t.TempDir(), "signing.json")
	store, err := Open(path, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Rotate(now, 5*time.Minute); err == nil {
		t.Fatal("short verification window accepted")
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path, now); err == nil {
		t.Fatal("world-readable private key file accepted")
	}
}

func mustSign(t *testing.T, store *Store, payload any) string {
	t.Helper()
	token, err := store.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func verifyCompact(t *testing.T, token string, key PublicKey) {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token parts = %d", len(parts))
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	var header map[string]string
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		t.Fatal(err)
	}
	if header["alg"] != "EdDSA" || header["typ"] != "JWT" || header["kid"] != key.KeyID {
		t.Fatalf("unexpected header: %#v", header)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(key.PublicKey, []byte(parts[0]+"."+parts[1]), signature) {
		t.Fatal("signature did not verify")
	}
}
