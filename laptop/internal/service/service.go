// Package service controls the ClipConnect daemon as a systemd user unit. The
// daemon must run as the normal user (it needs access to the user's graphical
// clipboard session), never as root, so every command here targets the user
// instance with `systemctl --user`.
package service

import (
	"fmt"
	"os/exec"
)

const unitName = "clipconnect.service"

// State reports whether the unit is active and, when paired, the connection
// state is tracked by the status package; here we only query systemd.

// Action is a systemd user-unit verb.
type Action string

const (
	Start   Action = "start"
	Stop    Action = "stop"
	Restart Action = "restart"
	Enable  Action = "enable"
)

// Run executes a systemd --user verb against the clipconnect unit. It returns
// the trimmed combined output (useful for status) and any error.
func Run(action Action) (string, error) {
	return run("systemctl", string(action), unitName)
}

// EnableNow enables the unit so it starts at login.
func EnableNow() error {
	_, err := run("systemctl", string(Enable))
	return err
}

// IsActive reports whether the unit is currently active (running).
func IsActive() bool {
	_, err := run("systemctl", "is-active --quiet", unitName)
	return err == nil
}

// StatusText returns the raw `systemctl --user status` output for display.
func StatusText() (string, error) {
	return run("systemctl", "status", unitName)
}

func run(name string, args ...string) (string, error) {
	full := []string{"--user"}
	full = append(full, args...)
	cmd := exec.Command(name, full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w: %s", name, err, string(out))
	}
	return string(out), nil
}
