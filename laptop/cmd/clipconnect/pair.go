package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
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
