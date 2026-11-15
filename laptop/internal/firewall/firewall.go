// Package firewall manages the host firewall (UFW) so the daemon's TCP port is
// reachable from other devices on the LAN. It is best-effort: every failure
// is returned to the caller, which can show manual instructions and continue.
package firewall

import (
	"fmt"
	"os"
	"os/exec"
)

// UFWAllow ensures the host firewall (ufw) permits inbound TCP on the given port.
// On a fresh host this requires sudo, so the user is prompted for their password
// once via the controlling terminal; on subsequent runs we detect the existing
// rule and skip the prompt entirely. Only the port is logged, never secrets or
// clipboard data.
//
// It returns an error only if the firewall could not be queried or updated.
func UFWAllow(port int) error {
	if ufwRuleExists(port) {
		return nil
	}

	cmd := exec.Command("sudo", "ufw", "allow", fmt.Sprintf("%d/tcp", port))
	// Attach the controlling terminal so sudo can prompt for the password
	// interactively even when the command's own stdin is not a TTY.
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		cmd.Stdin = tty
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sudo ufw allow: %w: %s", err, stringsTrimRightNewline(string(out)))
	}
	return nil
}

// stringsTrimRightNewline trims trailing whitespace (local replacement for
// strings.TrimSpace, which is used elsewhere too).
func stringsTrimRightNewline(s string) string {
	n := len(s)
	for n > 0 {
		c := s[n-1]
		if c != ' ' && c != '\n' && c != '\r' && c != '\t' {
			break
		}
		n--
	}
	return s[:n]
}

// ufwRuleExists reports whether ufw already has an ALLOW rule for the port.
func ufwRuleExists(port int) bool {
	out, err := exec.Command("sudo", "ufw", "status").Output()
	if err != nil {
		// If ufw is inactive or unreadable, treat as "not present" so we attempt
		// to open it (ufw will prompt for the password and add the rule).
		return false
	}
	return ufwRuleLinePresent(fmt.Sprintf("%d/tcp", port), string(out))
}

// ufwRuleLinePresent reports whether status output contains an ALLOW rule for
// the given "port/proto" string. Pure, so it can be tested without sudo.
func ufwRuleLinePresent(line, status string) bool {
	for _, l := range splitLines(status) {
		if containsStr(l, "ALLOW") && containsStr(l, line) {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
		} else {
			cur += string(r)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// UFWRuleLine returns the one-line command that opens the port, for use in
// manual-instructions fallbacks.
func UFWRuleLine(port int) string {
	return fmt.Sprintf("sudo ufw allow %d/tcp", port)
}
