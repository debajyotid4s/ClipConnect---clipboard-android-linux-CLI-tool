package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// PortFile returns the path of the persisted port file.
func PortFile() string {
	return filepath.Join(SecretDir(), "port")
}

// StateFile returns the path of the daemon's live status state file.
func StateFile() string {
	return filepath.Join(SecretDir(), "status.json")
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
