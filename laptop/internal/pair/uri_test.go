package pair

import (
	"strings"
	"testing"
)

func TestBuildAndParseURI(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	raw := BuildURI(secret, []string{"192.168.1.10", "10.0.0.2"}, 10369, "laptop")

	info, err := ParseURI(raw)
	if err != nil {
		t.Fatalf("ParseURI() error = %v", err)
	}
	if string(info.Secret) != string(secret) {
		t.Fatalf("secret mismatch")
	}
	if info.Port != 10369 {
		t.Fatalf("port = %d, want 10369", info.Port)
	}
	if len(info.IPs) != 2 {
		t.Fatalf("IPs = %v, want two addresses", info.IPs)
	}
}

func TestParseURIRejectsInvalidPairingData(t *testing.T) {
	tests := []string{
		"clipconnect://pair?v=1&k=cw&h=192.168.1.10&p=10369",
		"clipconnect://pair?v=1&k=" + strings.Repeat("s", 42) + "&h=192.168.1.10&p=10369",
		"clipconnect://pair?v=1&k=" + strings.Repeat("c", 43) + "&h=not-an-ip&p=10369",
		"clipconnect://pair?v=1&k=" + strings.Repeat("c", 43) + "&h=192.168.1.10&p=10369x",
	}

	for _, raw := range tests {
		if _, err := ParseURI(raw); err == nil {
			t.Errorf("ParseURI(%q) succeeded; want error", raw)
		}
	}
}
