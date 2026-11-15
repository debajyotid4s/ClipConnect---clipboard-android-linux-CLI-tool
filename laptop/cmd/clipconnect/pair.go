package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/config"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/pairing"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/service"
)

// pairLogger prints numbered step-by-step traces to stderr for troubleshooting.
// It is silent unless verbose mode is on. It never prints the secret, the full
// URI, or clipboard data — only lengths, paths, event names, and statuses.
type pairLogger struct {
	debug bool
	n     int
}

func newPairLogger(debug bool) *pairLogger { return &pairLogger{debug: debug} }

func (l *pairLogger) step(format string, args ...any) {
	if !l.debug {
		return
	}
	l.n++
	fmt.Fprintf(os.Stderr, "[%d] %s\n", l.n, fmt.Sprintf(format, args...))
}

func (l *pairLogger) ok(format string, args ...any) {
	if !l.debug {
		return
	}
	fmt.Fprintf(os.Stderr, "    ok:   %s\n", fmt.Sprintf(format, args...))
}

func (l *pairLogger) warn(format string, args ...any) {
	if !l.debug {
		return
	}
	fmt.Fprintf(os.Stderr, "    warn: %s\n", fmt.Sprintf(format, args...))
}

func rows(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// pairCmd is `clipconnect pair`: run the full pairing workflow directly.
var pairCmd = &cobra.Command{
	Use:   "pair",
	Short: "Pair an Android device (regenerates the secret, opens the firewall)",
	Long: "Pair an Android device. Regenerates the secret, opens the firewall,\n" +
		"restarts the daemon, and prints a QR code.\n\n" +
		"Use --verbose to print each step as it runs (for troubleshooting).",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPair(context.Background(), pairVerbose)
	},
}

// pairVerbose enables step-by-step tracing to stderr.
var pairVerbose bool

func init() {
	pairCmd.Flags().BoolVar(&pairVerbose, "verbose", false, "print each pairing step as it runs")
}

func init() {
	rootCmd.AddCommand(pairCmd)
}

// runPair performs the pairing workflow and prints the QR + status. It re-pairs
// unconditionally (an existing pairing is replaced). debug traces each step.
func runPair(ctx context.Context, debug bool) error {
	logger := newPairLogger(debug)

	logger.step("stopping any running daemon")
	if err := pairing.StopDaemon(); err != nil {
		logger.warn("could not stop daemon: %v", err)
	}

	logger.step("deleting any existing pairing and generating a new secret")
	_ = config.DeleteSecret()
	secret, err := config.GenerateSecret()
	if err != nil {
		return fmt.Errorf("generate secret: %w", err)
	}
	if err := config.SaveSecret(secret); err != nil {
		return fmt.Errorf("save secret: %w", err)
	}
	logger.ok("secret written (%d bytes) to %s", len(secret), config.SecretPath())

	logger.step("collecting LAN IP addresses")
	ips := pairing.LanIPsForDisplay()
	if len(ips) == 0 {
		ips = []string{"127.0.0.1"}
	}
	logger.ok("lan IPs: %v", ips)

	hostname, _ := os.Hostname()
	logger.ok("hostname: %s", hostname)

	logger.step("opening firewall (ufw allow 10369/tcp)")
	fwErr := pairing.OpenFirewall()
	if fwErr != nil {
		logger.warn("firewall: %s", fwErr)
	} else {
		logger.ok("firewall opened (or already open)")
	}

	logger.step("building pairing QR")
	qrURI := pairing.BuildURIString(secret, ips, pairing.FixedPort, hostname)
	qrStr, err := pairing.RenderQR(qrURI)
	if err != nil {
		return fmt.Errorf("generate qr: %w", err)
	}
	logger.ok("qr rendered (%d rows)", rows(qrStr))

	if debug {
		logger.step("starting daemon via systemd")
		if _, err := service.Run(service.Start); err != nil {
			logger.warn("daemon not started via systemd: %v", err)
		} else {
			logger.ok("daemon started (systemctl --user)")
		}
	}

	// Display block (unchanged behavior).
	fmt.Println()
	fmt.Println("Scan this QR code with the ClipConnect Android app:")
	fmt.Println()
	fmt.Println(qrStr)
	fmt.Println()
	fmt.Printf("IP address:    %v\n", ips)
	fmt.Printf("Port:          %d\n", pairing.FixedPort)
	fmt.Printf("Hostname:      %s\n", hostname)
	fmt.Println("Protocol:      ClipConnect v1")
	fmt.Println("Encryption:    AES-256-GCM (HKDF-SHA256)")
	fmt.Println()
	fmt.Println("Connection status:")
	fmt.Println("  waiting for phone…")
	fmt.Println()
	if fwErr == nil {
		fmt.Println("Firewall:      opened automatically (ufw)")
	} else {
		fmt.Println("Firewall:      could not open automatically")
		fmt.Printf("  manual:      %s\n", pairing.FirewallManualLine())
	}
	fmt.Println()
	fmt.Println("The pairing secret is not logged. Do not share the QR.")
	return nil
}
