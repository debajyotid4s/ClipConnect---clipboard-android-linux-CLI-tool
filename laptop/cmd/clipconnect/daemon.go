package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
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

// runDaemon starts the sync server, the clipboard watcher, and the status
// refresh loop, then blocks until an interrupt/TERM signal.
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
	started := timeNow()
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
	go refreshStatus(ctx, statusMgr)

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

// refreshStatus writes the status file on a 2s tick until shutdown.
func refreshStatus(ctx context.Context, mgr *status.Manager) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	if _, err := mgr.Refresh(); err != nil {
		fmt.Fprintf(os.Stderr, "status write: %v\n", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mgr.SetDaemonRunning(service.IsActive())
			if _, err := mgr.Refresh(); err != nil {
				fmt.Fprintf(os.Stderr, "status write: %v\n", err)
			}
		}
	}
}
