package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/config"
)

// logs flags.
var (
	logsFollow bool
	logsLines  int
	logsVerbose bool
)

// logsCmd is `clipconnect logs`.
var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show daemon logs (no clipboard contents or secrets)",
	Long: "Show daemon logs. Clipboard plaintext and secrets are never present.\n\n" +
		"Uses the systemd user journal when the daemon is active, otherwise the\n" +
		"daemon log file. Use --verbose to print which source is being read.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLogs()
	},
}

func init() {
	logsCmd.Flags().BoolVar(&logsFollow, "follow", false, "keep following new log lines")
	logsCmd.Flags().IntVar(&logsLines, "lines", 40, "number of recent lines to show")
	logsCmd.Flags().BoolVar(&logsVerbose, "verbose", false, "print which log source is being read")
	rootCmd.AddCommand(logsCmd)
}

// runLogs streams daemon logs. It prefers the systemd user journal and falls
// back to the daemon log file. Clipboard plaintext and secrets are never
// present in either source.
func runLogs() error {
	var cmd *exec.Cmd
	source := ""

	// Prefer the systemd user journal.
	if out, err := exec.Command("systemctl", "--user", "is-active", "clipconnect.service").Output(); err == nil && strings.TrimSpace(string(out)) == "active" {
		source = "systemd user journal"
		cmd = exec.Command("journalctl", "--user", "-u", "clipconnect.service", "-n", "0", "--follow", "-n", "0")
		// Use -f only when following; otherwise tail the requested number.
		if !logsFollow {
			cmd = exec.Command("journalctl", "--user", "-u", "clipconnect.service", "--lines", fmt.Sprintf("%d", logsLines))
		}
	} else {
		// Fall back to the daemon log file.
		if tailFile(logFilePath(), logsLines, logsFollow) != nil {
			source = "daemon log file"
		}
		cmd = tailFile(logFilePath(), logsLines, logsFollow)
	}

	if logsVerbose {
		if source == "" {
			fmt.Fprintln(os.Stderr, "logs: no log source available")
		} else {
			fmt.Fprintf(os.Stderr, "logs: reading from %s\n", source)
		}
	}

	if cmd == nil {
		fmt.Println("No logs available. Is the daemon running?")
		return nil
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil && err != io.EOF {
		return fmt.Errorf("logs: %w", err)
	}
	return nil
}

// logFilePath is where the daemon writes its log.
func logFilePath() string {
	return filepath.Join(config.SecretDir(), "daemon.log")
}

// tailFile returns a command that prints the last N lines of path, or follows
// it when follow is true. Returns nil if the file does not exist.
func tailFile(path string, lines int, follow bool) *exec.Cmd {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	var args []string
	if follow {
		args = []string{"-f"}
	} else {
		args = []string{"-n", fmt.Sprintf("%d", lines)}
	}
	args = append(args, path)
	cmd := exec.Command("tail", args...)
	return cmd
}
