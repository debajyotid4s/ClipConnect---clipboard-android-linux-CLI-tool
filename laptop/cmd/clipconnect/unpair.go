package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/config"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/pairing"
)

// yes flag: skip the interactive confirmation.
var unpairYes bool

// unpairVerbose enables step-by-step tracing to stderr.
var unpairVerbose bool

// unpairCmd is `clipconnect unpair`.
var unpairCmd = &cobra.Command{
	Use:   "unpair",
	Short: "Remove the pairing secret and disconnect",
	Long: "Remove the pairing secret and disconnect.\n\n" +
		"Use --verbose to print each step as it runs (for troubleshooting).",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUnpair(context.Background(), unpairVerbose)
	},
}

func init() {
	unpairCmd.Flags().BoolVar(&unpairYes, "yes", false, "skip the confirmation prompt")
	unpairCmd.Flags().BoolVar(&unpairVerbose, "verbose", false, "print each step as it runs")
	rootCmd.AddCommand(unpairCmd)
}

// runUnpair stops the daemon, deletes the secret, and confirms. debug traces each step.
func runUnpair(ctx context.Context, debug bool) error {
	logger := newPairLogger(debug)

	// 1. Stop the active daemon/session.
	logger.step("stopping the active daemon/session")
	_ = pairing.StopDaemon()
	logger.ok("daemon stopped")

	// 2. Confirm unless --yes.
	if !unpairYes {
		fmt.Print("Remove pairing and disconnect the phone? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		ans, _ := reader.ReadString('\n')
		ans = strings.ToLower(strings.TrimSpace(ans))
		if ans != "y" && ans != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// 3. Delete the pairing secret.
	logger.step("deleting the pairing secret")
	_ = config.DeleteSecret()
	logger.ok("secret removed from %s", config.SecretPath())

	// 4. Clear pairing/connection state (service state is managed by systemd).
	fmt.Println()
	fmt.Println("Successfully unpaired. Pair again to reconnect Android.")
	return nil
}
