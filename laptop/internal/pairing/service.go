package pairing

import (
	"fmt"
	"net"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/firewall"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/service"
	"github.com/skip2/go-qrcode"
)

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

// OpenFirewallResult is like OpenFirewall but returns a bool+message for the
// pairing Result struct.
func OpenFirewallResult() (bool, string) {
	if err := firewall.UFWAllow(FixedPort); err != nil {
		return false, fmt.Sprintf("could not open firewall automatically: %v\nRun manually: %s", err, firewall.UFWRuleLine(FixedPort))
	}
	return true, ""
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
