package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// PortFile returns the path of the persisted port file.
func PortFile() string {
	return filepath.Join(SecretDir(), "port")
}

// LoadPort reads the persisted TCP port. Returns (0, nil) if none is stored or
// the stored value is invalid.
func LoadPort() (int, error) {
	data, err := os.ReadFile(PortFile())
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || port <= 0 || port > 65535 {
		return 0, nil
	}
	return port, nil
}

// SavePort persists the chosen TCP port (mode 0600, alongside the secret).
func SavePort(port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port %d", port)
	}
	if err := os.MkdirAll(SecretDir(), 0700); err != nil {
		return err
	}
	return os.WriteFile(PortFile(), []byte(strconv.Itoa(port)), 0600)
}
