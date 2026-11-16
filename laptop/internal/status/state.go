// Package status defines the live status snapshot the daemon exposes and the
// commands/TUI render. The daemon writes a small JSON state file that any
// command can read to report paired/daemon-running/client-count/phone-IP/last
// connection time without re-implementing the connection probe.
package status

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/config"
)

// State is the daemon's live connection state, persisted to disk so external
// commands (the `status` subcommand, the TUI) can read it without reaching into
// the running process. It never contains clipboard data or the pairing secret.
type State struct {
	// Paired is true when a pairing secret exists on disk.
	Paired bool `json:"paired"`
	// DaemonRunning is true when the systemd user unit is active.
	DaemonRunning bool `json:"daemonRunning"`
	// Port is the TCP port the daemon listens on.
	Port int `json:"port"`
	// ClientCount is the number of phones with an established session.
	ClientCount int `json:"clientCount"`
	// PhoneAddrs are the remote addrs of connected phones.
	PhoneAddrs []string `json:"phoneAddrs,omitempty"`
	// LastConnect is the time the most recent session was established.
	LastConnect *time.Time `json:"lastConnect,omitempty"`
	// Uptime is how long the current daemon process has run.
	UptimeSeconds int64 `json:"uptimeSeconds"`
}

// IsConnected reports whether at least one phone is connected.
func (s State) IsConnected() bool { return s.ClientCount > 0 }

// FilePath returns the path of the persisted state file.
func FilePath() string {
	return config.StateFile()
}

// Write serializes st to the state file (mode 0600). Errors are returned so the
// daemon can log them but keep running.
func Write(st State) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(config.StateFile()), 0700); err != nil {
		return err
	}
	return os.WriteFile(config.StateFile(), data, 0600)
}

// Read loads the state file. It returns (State{}, nil) when no file exists yet,
// so callers can distinguish "not written" from an error.
func Read() (State, error) {
	var st State
	data, err := os.ReadFile(config.StateFile())
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return st, err
	}
	if len(data) == 0 {
		return st, nil
	}
	err = json.Unmarshal(data, &st)
	return st, err
}
