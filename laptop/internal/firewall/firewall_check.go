package firewall

import (
	"fmt"
	"os/exec"
)

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
