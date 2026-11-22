package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/config"
)

// pairingFixedPort returns the fixed daemon port.
func pairingFixedPort() int { return 10369 }

// newDaemonLogger returns a slog logger writing to the daemon log file (never
// the terminal), so logs never mix with the QR/prompt and never contain
// clipboard plaintext or secrets.
func newDaemonLogger() *slog.Logger {
	path := filepath.Join(config.SecretDir(), "daemon.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	return slog.New(slog.NewTextHandler(f, nil))
}

// timeNow is a small indirection so tests can observe elapsed uptime.
func timeNow() time.Time { return time.Now() }
