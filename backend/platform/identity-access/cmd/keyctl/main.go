package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"omsbizfoundry/identity-access/internal/signing"
)

type jwk struct {
	KTY    string `json:"kty"`
	CRV    string `json:"crv"`
	Use    string `json:"use"`
	ALG    string `json:"alg"`
	KID    string `json:"kid"`
	X      string `json:"x"`
	Status string `json:"status"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	if len(arguments) < 1 || len(arguments) > 2 {
		return fmt.Errorf("usage: identity-access-keyctl <public [output-file]|rotate|prune>")
	}
	path := os.Getenv("SIGNING_KEY_FILE")
	if path == "" {
		return fmt.Errorf("SIGNING_KEY_FILE is required")
	}
	now := time.Now().UTC()
	store, err := signing.Open(path, now)
	if err != nil {
		return err
	}
	switch arguments[0] {
	case "public":
		data, err := publicJSON(store, now)
		if err != nil {
			return err
		}
		if len(arguments) == 1 {
			_, err = os.Stdout.Write(data)
			return err
		}
		return writeAtomic(arguments[1], data, 0o644)
	case "rotate":
		if len(arguments) != 1 {
			return fmt.Errorf("rotate does not accept an output path")
		}
		keyID, err := store.Rotate(now, 5*time.Minute+30*time.Second)
		if err != nil {
			return err
		}
		fmt.Println(keyID)
		return nil
	case "prune":
		if len(arguments) != 1 {
			return fmt.Errorf("prune does not accept an output path")
		}
		return store.Prune(now)
	default:
		return fmt.Errorf("unknown key operation %q", arguments[0])
	}
}

func publicJSON(store *signing.Store, now time.Time) ([]byte, error) {
	keys, err := store.PublicKeys(now)
	if err != nil {
		return nil, err
	}
	values := make([]jwk, 0, len(keys))
	for _, key := range keys {
		values = append(values, jwk{KTY: "OKP", CRV: "Ed25519", Use: "sig", ALG: "EdDSA", KID: key.KeyID, X: base64.RawURLEncoding.EncodeToString(key.PublicKey), Status: key.Status})
	}
	data, err := json.MarshalIndent(map[string]any{"keys": values}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".public-keys-*")
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
