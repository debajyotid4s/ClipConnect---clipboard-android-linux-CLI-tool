package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ensureUfwAllow makes sure the host firewall (ufw) permits inbound TCP on the
// fixed port. On a fresh host this requires sudo, so the user is prompted for
// their password once; on subsequent runs we detect the existing rule and skip
// the prompt entirely. Only the port is logged, never secrets or clipboard data.
//
// It returns an error only if the firewall could not be queried or updated.
func ensureUfwAllow() error {
	port := fixedPort
	if ufwRuleExists(port) {
		return nil
	}

	cmd := exec.Command("sudo", "ufw", "allow", fmt.Sprintf("%d/tcp", port))
	// Attach the controlling terminal so sudo can prompt for the password
	// interactively even when the daemon's own stdin is not a TTY.
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		cmd.Stdin = tty
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sudo ufw allow: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ufwRuleExists reports whether ufw already has an ALLOW rule for the port.
func ufwRuleExists(port int) bool {
	out, err := exec.Command("sudo", "ufw", "status").Output()
	if err != nil {
		// If ufw is inactive or unreadable, treat as "not present" so we attempt
		// to open it (ufw will prompt for the password and add the rule).
		return false
	}
	line := fmt.Sprintf("%d/tcp", port)
	for _, l := range strings.Split(string(out), "\n") {
		if strings.Contains(l, "ALLOW") && strings.Contains(l, line) {
			return true
		}
	}
	return false
}
