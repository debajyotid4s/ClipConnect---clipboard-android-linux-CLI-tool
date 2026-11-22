package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/config"
)

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
