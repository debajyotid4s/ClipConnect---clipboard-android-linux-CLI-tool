package config

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
)

// Config holds the pairing configuration.
type Config struct {
	Secret []byte `json:"secret"`
}

// SecretDir returns the directory where config files are stored.
func SecretDir() string {
	// Use XDG_CONFIG_HOME or fall back to ~/.config/clipconnect
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		home, _ := os.UserHomeDir()
		xdg = filepath.Join(home, ".config")
	}
	return filepath.Join(xdg, "clipconnect")
}

// SecretPath returns the path to the secret file.
func SecretPath() string {
	return filepath.Join(SecretDir(), "secret")
}

// LoadSecret reads the pairing secret from disk.
// Returns (nil, nil) if no secret exists.
func LoadSecret() ([]byte, error) {
	data, err := os.ReadFile(SecretPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	// Secret is stored as base64url (no padding).
	secret := make([]byte, base64.URLEncoding.DecodedLen(len(data)))
	n, err := base64.URLEncoding.Decode(secret, data)
	if err != nil {
		return nil, err
	}
	return secret[:n], nil
}

// SaveSecret writes the pairing secret to disk with mode 0600.
func SaveSecret(secret []byte) error {
	dir := SecretDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	encoded := make([]byte, base64.URLEncoding.EncodedLen(len(secret)))
	base64.URLEncoding.Encode(encoded, secret)

	return os.WriteFile(SecretPath(), encoded, 0600)
}

// DeleteSecret removes the pairing secret file.
func DeleteSecret() error {
	return os.Remove(SecretPath())
}

// GenerateSecret creates a new 32-byte random secret.
func GenerateSecret() ([]byte, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	return secret, nil
}
