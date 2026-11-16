package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/clip"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/clip/backend"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/config"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/server"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/service"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/status"
	"github.com/spf13/cobra"
)

// daemonCmd is `clipconnect daemon`: run the sync server. Invoked by the
// systemd user unit. It stays running until signalled.
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run the clipboard sync daemon (server + clipboard watch)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDaemon()
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}

func runDaemon() error {
	secret, err := config.LoadSecret()
	if err != nil {
		return fmt.Errorf("load secret: %w", err)
	}
	if secret == nil {
		return fmt.Errorf("not paired: run 'clipconnect pair' first")
	}

	be, err := backend.Detect()
	if err != nil {
		return fmt.Errorf("detect backend: %w", err)
	}

	// echoGuard tracks clips applied from the laptop to the phone's clipboard so
	// the outgoing watcher does not echo them back. It is shared between the
	// server's readLoop goroutine (writes) and watchClipboard (reads); clip.Guard
	// is internally mutex-protected, so sharing it across goroutines is safe.
	echoGuard := clip.NewGuard(100)

	srv := server.New(fmt.Sprintf("0.0.0.0:%d", pairingFixedPort()), secret, func(text string) {
		if setErr := be.Set(context.Background(), text); setErr != nil {
			fmt.Fprintf(os.Stderr, "clipboard set (incoming): %v\n", setErr)
			return
		}
		// Record the applied clip so the outgoing watcher can skip echoing it
		// back. Guard is internally mutex-protected, so this is safe to call
		// from the server readLoop goroutine while watchClipboard reads it.
		echoGuard.Record(text)
		fmt.Printf("received clip len=%d\n", len(text))
	}, newDaemonLogger())

	// statusManager exposes the daemon's live connection state (client count,
	// phone IPs, last connect time, uptime) via a JSON file that the `status`
	// subcommand and TUI read. It never stores clipboard data or secrets.
	started := time.Now()
	statusMgr := status.NewManager(srv, started)
	statusMgr.SetPaired(true)
	statusMgr.SetDaemonRunning(service.IsActive())
	srv.SetOnEstablished(func(addr string) {
		statusMgr.TrackConnect(addr)
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go watchClipboard(ctx, be, srv, echoGuard)

	// Refresh the status file periodically so external readers see a fresh
	// client count / phone IP / uptime.
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		// Write an initial snapshot immediately.
		if _, err := statusMgr.Refresh(); err != nil {
			fmt.Fprintf(os.Stderr, "status write: %v\n", err)
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				statusMgr.SetDaemonRunning(service.IsActive())
				if _, err := statusMgr.Refresh(); err != nil {
					fmt.Fprintf(os.Stderr, "status write: %v\n", err)
				}
			}
		}
	}()

	go func() {
		if startErr := srv.Start(ctx); startErr != nil {
			fmt.Fprintf(os.Stderr, "server: %v\n", startErr)
		}
	}()

	// On signal, stop accepting new connections so the daemon exits cleanly.
	go func() {
		<-ctx.Done()
		if err := srv.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "server close: %v\n", err)
		}
	}()

	fmt.Printf("clipconnect daemon listening on port %d\n", pairingFixedPort())
	<-ctx.Done()
	return nil
}

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

// watchClipboard polls the OS clipboard and sends changed clips to paired phones.
// echoGuard is shared with the incoming handler so a clip applied from the
// laptop (phone side) is not echoed back; clip.Guard is mutex-protected and safe
// to share across goroutines.
func watchClipboard(ctx context.Context, be backend.Backend, srv *server.Server, echoGuard *clip.Guard) {
	var lastClip string
	var lastTime time.Time

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			text, err := be.Get(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "clipboard get: %v\n", err)
				continue
			}
			if clip.IsEmptyOrTooLarge(text) || clip.IsSensitive(text) {
				continue
			}
			if t.Sub(lastTime) < 150*time.Millisecond {
				continue
			}
			// Echo guard: skip a clip the laptop just applied to this machine.
			if echoGuard.Record(text) {
				continue
			}
			if text == lastClip {
				continue
			}
			lastClip = text
			lastTime = t
			if sendErr := srv.Send(text); sendErr != nil {
				fmt.Fprintf(os.Stderr, "send clip: %v\n", sendErr)
			}
		}
	}
}
