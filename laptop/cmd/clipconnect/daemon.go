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
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/mdns"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/server"
	"github.com/spf13/cobra"
)

// daemonCmd is `clipconnect daemon`: run the sync server. Invoked by the
// systemd user unit. It stays running until signalled.
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run the clipboard sync daemon (server + mDNS + clipboard watch)",
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

	srv := server.New(fmt.Sprintf("0.0.0.0:%d", pairingFixedPort()), secret, func(text string) {
		if setErr := be.Set(context.Background(), text); setErr != nil {
			fmt.Fprintf(os.Stderr, "clipboard set (incoming): %v\n", setErr)
			return
		}
		fmt.Printf("received clip len=%d\n", len(text))
	}, newDaemonLogger())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var lastAppliedIncoming string
	go watchClipboard(ctx, be, srv, &lastAppliedIncoming)

	ifaces := mdns.PickInterfaces()
	hostname, _ := os.Hostname()
	if adv, err := mdns.NewAdvertiser(secret, pairingFixedPort(), hostname, ifaces, newDaemonLogger()); err == nil {
		go adv.Run(ctx)
	}

	go func() {
		if startErr := srv.Start(ctx); startErr != nil {
			fmt.Fprintf(os.Stderr, "server: %v\n", startErr)
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
func watchClipboard(ctx context.Context, be backend.Backend, srv *server.Server, lastAppliedIncoming *string) {
	guard := clip.NewGuard(100)
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
			if guard.Record(text) {
				continue
			}
			if text == lastClip {
				continue
			}
			if lastAppliedIncoming != nil && text == *lastAppliedIncoming {
				*lastAppliedIncoming = ""
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
