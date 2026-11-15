// Package pairing implements the "pair a device" workflow: stop any running
// daemon, replace (or create) the pairing secret, open the firewall for the
// fixed port, and restart the daemon so a fresh QR is valid immediately.
//
// It never logs the secret, the full pairing URI, or clipboard data — only
// lengths and event names.
package pairing

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/config"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/firewall"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/pair"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/service"
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
	ips := lanIPs()
	if len(ips) == 0 {
		ips = []string{"127.0.0.1"}
	}
	hostname, _ := os.Hostname()

	// 4. Open the firewall (idempotent; prompts for sudo once if needed).
	fwOK, fwMsg := ensureFirewall()

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

// StartDaemon (re)starts the user service so the fresh secret is live.
func StartDaemon() error {
	if _, err := service.Run(service.Start); err != nil {
		return fmt.Errorf("start service: %w", err)
	}
	return nil
}

// LanIPsForDisplay returns the private LAN IPs for the QR payload, or
// 127.0.0.1 if none are found. Exported so the CLI can echo them.
func LanIPsForDisplay() []string {
	ips := lanIPs()
	if len(ips) == 0 {
		return []string{"127.0.0.1"}
	}
	return ips
}

// OpenFirewall opens ufw for the fixed port (idempotent). It returns nil on
// success, or an error describing how to open it manually.
func OpenFirewall() error {
	if err := firewall.UFWAllow(FixedPort); err != nil {
		return fmt.Errorf("could not open firewall automatically: %v\nRun manually: %s", err, FirewallManualLine())
	}
	return nil
}

// FirewallManualLine returns the one-line ufw command to open the port.
func FirewallManualLine() string {
	return firewall.UFWRuleLine(FixedPort)
}

// RenderQR renders a pairing URI to ASCII-art QR for the terminal.
func RenderQR(uri string) (string, error) {
	qr, err := qrcode.New(uri, qrcode.Medium)
	if err != nil {
		return "", err
	}
	return qr.ToSmallString(true), nil
}

// StopDaemon stops the user service if it is running (non-fatal if not).
func StopDaemon() error {
	if _, err := service.Run(service.Stop); err != nil {
		return nil
	}
	return nil
}

func ensureFirewall() (bool, string) {
	if err := firewall.UFWAllow(FixedPort); err != nil {
		return false, fmt.Sprintf("could not open firewall automatically: %v\nRun manually: %s", err, firewall.UFWRuleLine(FixedPort))
	}
	return true, ""
}

func lanIPs() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ip, ok := addr.(*net.IPNet); ok && !ip.IP.IsLoopback() {
				if ip.IP.To4() != nil {
					ips = append(ips, ip.IP.String())
				}
			}
		}
	}
	return ips
}

func base64urlNoPad(src []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(src), "=")
}

// BuildURIString builds a pairing URI for the fixed port. Exposed so callers
// (e.g. the TUI) can display the manual-connect URI without importing pair.
func BuildURIString(secret []byte, ips []string, port int, hostname string) string {
	return pair.BuildURI(secret, ips, port, hostname)
}
