package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/config"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/pairing"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/service"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/status"
)

// statusCmd is `clipconnect status`: report paired/daemon-running/client count,
// phone IP, and last connection time from the daemon's live state.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show pairing and connection status (no clipboard contents or secrets)",
	Long: "Show whether the device is paired, whether the daemon is running, how\n" +
		"many phones are connected, the phone's IP, and the last connection time.\n\n" +
		"Reads the daemon's live status file plus pairing and systemd state. Never\n" +
		"prints clipboard data or the pairing secret.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStatus()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus() error {
	paired, _ := config.LoadSecret()
	daemonOn := service.IsActive()

	st, err := status.Read()
	if err != nil {
		// A read error (not "not found") is worth noting, but we still report
		// what we can.
		fmt.Fprintf(os.Stderr, "note: could not read daemon status file: %v\n", err)
	}

	// Fall back to live checks when the daemon has not written a state file yet
	// (e.g. freshly started, or running without the status file). If the daemon
	// is not active, the on-disk connection info is stale, so ignore it.
	if !daemonOn {
		st.ClientCount = 0
		st.PhoneAddrs = nil
		st.LastConnect = nil
		st.UptimeSeconds = 0
	}
	if st.Port == 0 && st.ClientCount == 0 && st.LastConnect == nil {
		st.Paired = paired != nil
		st.DaemonRunning = daemonOn
		st.Port = pairing.FixedPort
		if daemonOn {
			st.PhoneAddrs = probePhoneAddrs()
			st.ClientCount = len(st.PhoneAddrs)
		}
	}

	printStatus(st, paired != nil, daemonOn)
	return nil
}
