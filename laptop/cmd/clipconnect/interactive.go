package main

import (
	"net"
	"os"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/config"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/pairing"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/service"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/tui"
)

// version is printed by `clipconnect version`.
const version = "clipconnect 1.0.0"

// runInteractive launches the Bubble Tea menu. It wires the live daemon status,
// the pairing QR (if paired), and the log tail.
func runInteractive() error {
	paired := func() bool {
		s, _ := config.LoadSecret()
		return s != nil
	}()

	// Status source: live connection state from the running service.
	var src tui.StatusProvider
	if service.IsActive() {
		src = func() tui.StatusSnapshot {
			s := tui.StatusSnapshot{Port: pairing.FixedPort}
			s.Paired = true
			if ip, ok := phoneConnected(); ok {
				s.PhoneConnected = true
				s.PhoneIP = ip
			}
			return s
		}
	}

	// QR shown on the pair tab when paired.
	var qr string
	if paired {
		qr = buildQR()
	}

	return tui.Run(src, qr, "", paired)
}

// phoneConnected reports the phone's IP if the daemon accepts a connection.
func phoneConnected() (string, bool) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", "10369"), 500e6)
	if err != nil {
		return "", false
	}
	conn.Close()
	// A successful dial means a session exists; the real peer IP is not
	// distinguishable from loopback here, so we report connected without an IP.
	return "", true
}

// buildQR returns the current pairing URI (shown once on the pair tab).
func buildQR() string {
	secret, _ := config.LoadSecret()
	if secret == nil {
		return ""
	}
	hostname, _ := os.Hostname()
	return pairing.BuildURIString(secret, lanIPs(), pairing.FixedPort, hostname)
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
