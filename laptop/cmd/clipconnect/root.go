package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/config"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/tui"
)

func newRootCommand(logger *slog.Logger) *cobra.Command {
	root := &cobra.Command{
		Use:   "clipconnect",
		Short: "Secure Android/Linux clipboard synchronization",
		RunE: func(cmd *cobra.Command, args []string) error {
			action, err := tui.Run()
			if err != nil {
				return err
			}
			switch action {
			case tui.ActionPair:
				cmdPair(logger)
			case tui.ActionUnpair:
				cmdUnpair()
			case tui.ActionLogs:
				cmdLogs()
			case tui.ActionHelp:
				cmdDetailedHelp()
			}
			return nil
		},
	}

	root.AddCommand(
		&cobra.Command{
			Use:   "pair",
			Short: "Generate a new pairing and show its QR code",
			Run:   func(cmd *cobra.Command, args []string) { cmdPair(logger) },
		},
		&cobra.Command{
			Use:   "unpair",
			Short: "Stop the daemon and remove pairing credentials",
			Run:   func(cmd *cobra.Command, args []string) { cmdUnpair() },
		},
		&cobra.Command{
			Use:   "daemon",
			Short: "Run the clipboard synchronization daemon",
			Run:   func(cmd *cobra.Command, args []string) { cmdDaemon(logger) },
		},
		&cobra.Command{
			Use:   "status",
			Short: "Show daemon and phone connection status",
			Run:   func(cmd *cobra.Command, args []string) { cmdStatus(logger) },
		},
		&cobra.Command{
			Use:   "logs",
			Short: "Show daemon logs",
			Run:   func(cmd *cobra.Command, args []string) { cmdLogs() },
		},
		&cobra.Command{
			Use:    "setup-phone",
			Short:  "Show Android setup information",
			Hidden: true,
			Run:    func(cmd *cobra.Command, args []string) { cmdSetupPhone() },
		},
	)
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	return root
}

func cmdLogs() {
	data, err := os.ReadFile(filepath.Join(config.SecretDir(), "daemon.log"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not read logs: %v\n", err)
		return
	}
	fmt.Print(string(data))
}

func cmdDetailedHelp() {
	fmt.Println("ClipConnect")
	fmt.Println()
	fmt.Println("Secure Android/Linux clipboard synchronization over the local network.")
	fmt.Println()
	fmt.Println("Laptop:")
	fmt.Println("  Run 'clipconnect pair', scan the QR code, then run 'clipconnect daemon'.")
	fmt.Println("  The fixed TCP port is 10369.")
	fmt.Println()
	fmt.Println("Android:")
	fmt.Println("  Scan the QR code, or enter the laptop IP and pairing secret manually.")
	fmt.Println("  Allow ClipConnect to run in the background for continuous sync.")
}
