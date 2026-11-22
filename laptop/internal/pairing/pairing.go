package pairing

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/config"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/pair"
	"github.com/skip2/go-qrcode"
)

// FixedPort is the single port the listener, QR payload, firewall, Android app,
// tests, and docs all agree on.
const FixedPort = 10369

// Result is the outcome of a pairing pass, returned to the caller for display.
type Result struct {
	SecretB64URL string // base64url secret, no padding (for QR / manual entry)
	URI          string // full clipconnect://pair URI (shown once, never logged)
	QRString     string // ASCII art QR for the terminal
	IPs          []string
	Port         int
	Hostname     string
	FirewallOK   bool
	FirewallMsg  string
}

// Pair performs the full pairing workflow and returns everything the caller
// needs to display. It re-pairs unconditionally: any existing secret is deleted
// and a new one generated, which revokes previously paired phones.
func Pair(ctx context.Context) (*Result, error) {
	// 1. Stop any running daemon so we can cleanly swap the secret and restart.
	if err := StopDaemon(); err != nil {
		// Non-fatal: the daemon may simply not be running.
		fmt.Fprintf(os.Stderr, "note: could not stop daemon: %v\n", err)
	}

	// 2. Delete any existing pairing (revokes old phones) and generate a new secret.
	_ = config.DeleteSecret()
	secret, err := config.GenerateSecret()
	if err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}
	if err := config.SaveSecret(secret); err != nil {
		return nil, fmt.Errorf("save secret: %w", err)
	}

	// 3. LAN IPs for the QR payload (fallback if mDNS fails).
	ips := LanIPsForDisplay()
	hostname, _ := os.Hostname()

	// 4. Open the firewall (idempotent; prompts for sudo once if needed).
	fwOK, fwMsg := OpenFirewallResult()

	// 5. Build the QR and manual URI.
	uri := pair.BuildURI(secret, ips, FixedPort, hostname)
	qr, err := qrcode.New(uri, qrcode.Medium)
	if err != nil {
		return nil, fmt.Errorf("generate qr: %w", err)
	}

	return &Result{
		SecretB64URL: base64urlNoPad(secret),
		URI:          uri,
		QRString:     qr.ToSmallString(true),
		IPs:          ips,
		Port:         FixedPort,
		Hostname:     hostname,
		FirewallOK:   fwOK,
		FirewallMsg:  fwMsg,
	}, nil
}

func base64urlNoPad(src []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(src), "=")
}

// BuildURIString builds a pairing URI for the fixed port. Exposed so callers
// (e.g. the TUI) can display the manual-connect URI without importing pair.
func BuildURIString(secret []byte, ips []string, port int, hostname string) string {
	return pair.BuildURI(secret, ips, port, hostname)
}
