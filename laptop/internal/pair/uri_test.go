package pair

import (
	"testing"
	"strings"
)

func TestBuildParseRoundtrip(t *testing.T) {
	secret := []byte("sO0ai9K4ES7GtoBe4DeQLeEbD-JuGyeGvgoY2eXcHSM")
	ips := []string{"192.168.0.110", "10.0.0.5"}
	uri := BuildURI(secret, ips, 10369, "stealth")

	parsed, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("ParseURI: %v", err)
	}
	if parsed.Version != 1 {
		t.Errorf("version = %d, want 1", parsed.Version)
	}
	if string(parsed.Secret) != string(secret) {
		t.Errorf("secret mismatch: %q vs %q", parsed.Secret, secret)
	}
	if parsed.Port != 10369 {
		t.Errorf("port = %d, want 10369", parsed.Port)
	}
	if parsed.Hostname != "stealth" {
		t.Errorf("hostname = %q, want stealth", parsed.Hostname)
	}
	if len(parsed.IPs) != 2 || parsed.IPs[0] != "192.168.0.110" || parsed.IPs[1] != "10.0.0.5" {
		t.Errorf("ips = %v, want [192.168.0.110 10.0.0.5]", parsed.IPs)
	}
}

func TestParseURIBadScheme(t *testing.T) {
	_, err := ParseURI("https://pair?v=1&k=AA==")
	if err == nil {
		t.Fatal("expected error for bad scheme")
	}
}

func TestParseURIBadHost(t *testing.T) {
	_, err := ParseURI("clipconnect://notpair?v=1&k=AA==")
	if err == nil {
		t.Fatal("expected error for bad host")
	}
}

func TestParseURIBadVersion(t *testing.T) {
	_, err := ParseURI("clipconnect://pair?v=2&k=AA==")
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestParseURISecretDecodes(t *testing.T) {
	// "hi" base64url (no padding) = "aGk"
	uri := "clipconnect://pair?v=1&k=aGk"
	parsed, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("ParseURI: %v", err)
	}
	if string(parsed.Secret) != "hi" {
		t.Errorf("secret = %q, want %q", parsed.Secret, "hi")
	}
}

func TestParseURIPortDefaultsWhenMissing(t *testing.T) {
	parsed, err := ParseURI("clipconnect://pair?v=1&k=AA==")
	if err != nil {
		t.Fatalf("ParseURI: %v", err)
	}
	if parsed.Port != 48123 {
		t.Errorf("port = %d, want default 48123", parsed.Port)
	}
}

func TestParseURIPortParsed(t *testing.T) {
	parsed, err := ParseURI("clipconnect://pair?v=1&k=AA==&p=10369")
	if err != nil {
		t.Fatalf("ParseURI: %v", err)
	}
	if parsed.Port != 10369 {
		t.Errorf("port = %d, want 10369", parsed.Port)
	}
}

func TestParseURIEmptyIPs(t *testing.T) {
	parsed, err := ParseURI("clipconnect://pair?v=1&k=AA==&h=")
	if err != nil {
		t.Fatalf("ParseURI: %v", err)
	}
	if len(parsed.IPs) != 0 {
		t.Errorf("ips = %v, want empty", parsed.IPs)
	}
}

func TestParseURITrimSpacesInIPs(t *testing.T) {
	parsed, err := ParseURI("clipconnect://pair?v=1&k=AA==&h= 192.168.0.1 , 10.0.0.2 ")
	if err != nil {
		t.Fatalf("ParseURI: %v", err)
	}
	if len(parsed.IPs) != 2 || parsed.IPs[0] != "192.168.0.1" || parsed.IPs[1] != "10.0.0.2" {
		t.Errorf("ips = %v, want [192.168.0.1 10.0.0.2]", parsed.IPs)
	}
}

func TestParseURIBadSecret(t *testing.T) {
	// A single base64 char "a" cannot form a valid byte even after padding, so
	// decoding must fail.
	if _, err := ParseURI("clipconnect://pair?v=1&k=a"); err == nil {
		t.Fatal("expected error for invalid base64url secret, got nil")
	}
}

func TestBuildURINoPadding(t *testing.T) {
	secret := []byte("hi")
	uri := BuildURI(secret, nil, 10369, "h")
	// Only the k= parameter value must be free of base64 padding; the URL
	// structure itself contains '=' as key/value separators.
	q := strings.SplitN(uri, "k=", 2)[1]
	q = strings.SplitN(q, "&", 2)[0]
	if strings.Contains(q, "=") {
		t.Fatalf("secret param %q should not contain base64 padding", q)
	}
	// And it must still parse back to the same secret.
	parsed, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("ParseURI: %v", err)
	}
	if string(parsed.Secret) != string(secret) {
		t.Errorf("roundtrip secret mismatch: %q vs %q", parsed.Secret, secret)
	}
}
