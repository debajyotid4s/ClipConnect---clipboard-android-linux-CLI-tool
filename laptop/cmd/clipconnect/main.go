package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/clip"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/clip/backend"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/config"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/pair"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/server"

	"github.com/skip2/go-qrcode"
)

const (
	pkgName = "dev.clipconnect.app"

	// fixedPort is the single, always-on listening port for the daemon. It is
	// persisted to disk (config.PortFile) so the QR and the phone's saved port
	// stay in lockstep across restarts and fresh installs.
	fixedPort = 10369
)

// newFileLogger returns a slog logger that writes to ~/.config/clipconnect/
// daemon.log (or $XDG_CONFIG_HOME/clipconnect/daemon.log). Keeping logs off the
// terminal leaves it free for the interactive sudo password prompt and the QR
// code so the two never interleave and garble each other.
func newFileLogger() *slog.Logger {
	logPath := filepath.Join(config.SecretDir(), "daemon.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		// Fall back to stderr if the log file can't be opened.
		return slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	return slog.New(slog.NewTextHandler(f, nil))
}

func main() {
	logger := newFileLogger()
	if err := newRootCommand(logger).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func cmdPair(logger *slog.Logger) {
	if err := stopDaemon(); err != nil {
		log.Fatalf("stop daemon before pairing: %v", err)
	}

	existing, err := config.LoadSecret()
	if err != nil {
		log.Fatalf("load pairing secret: %v", err)
	}
	if existing != nil {
		if err := config.DeleteSecret(); err != nil {
			log.Fatalf("remove existing pairing: %v", err)
		}
		fmt.Println("Existing pairing removed.")
	}

	fmt.Println("Checking firewall (enter sudo password if required)…")
	if err := ensureUfwAllow(); err != nil {
		logger.Warn("firewall (manual steps required)", "err", err)
		fmt.Printf("Auto-allow failed; run manually: sudo ufw allow %d/tcp\n", fixedPort)
	} else {
		fmt.Println("Firewall ready.")
	}

	secret, err := config.GenerateSecret()
	if err != nil {
		log.Fatalf("generate secret: %v", err)
	}
	if err := config.SaveSecret(secret); err != nil {
		log.Fatalf("save secret: %v", err)
	}

	ips := lanIPs()
	if len(ips) == 0 {
		ips = []string{"127.0.0.1"}
	}

	hostname, _ := os.Hostname()
	uri := pair.BuildURI(secret, ips, fixedPort, hostname)

	qr, err := qrcode.New(uri, qrcode.Medium)
	if err != nil {
		log.Fatalf("generate qr: %v", err)
	}

	fmt.Println("Scan this QR code with the ClipConnect Android app:")
	fmt.Println()
	fmt.Println(qr.ToSmallString(true))
	fmt.Println()
	fmt.Println("Or visit manually:")
	fmt.Println(uri)
	fmt.Println()
	fmt.Printf("LAN IPs: %s\n", strings.Join(ips, ", "))
	fmt.Printf("Port: %d\n", fixedPort)
}

func cmdDaemon(logger *slog.Logger) {
	// Pairing owns secret generation and firewall setup. The daemon only runs
	// after an explicit pairing has been completed.
	secret, err := config.LoadSecret()
	if err != nil {
		log.Fatalf("load pairing secret: %v", err)
	}
	if secret == nil {
		log.Fatal("not paired; run 'clipconnect pair' first")
	}
	if err := config.SavePID(os.Getpid()); err != nil {
		log.Fatalf("save daemon pid: %v", err)
	}
	defer config.DeletePID()

	be, err := backend.Detect()
	if err != nil {
		log.Fatalf("detect backend: %v", err)
	}
	logger.Info("using backend", "type", backendName(be))

	// The listening port is always fixedPort, persisted to disk so the QR and
	// the phone's saved port stay in lockstep across restarts and fresh installs.
	addr := fmt.Sprintf("0.0.0.0:%d", fixedPort)

	// lastAppliedIncoming holds the most recent clip pasted INTO the OS
	// clipboard from a phone. watchClipboard skips it so an incoming clip is
	// never echoed back out to the phone (feedback loop).
	var lastAppliedIncoming incomingClipGuard

	srv := server.New(addr, secret, func(text string) {
		// A clip arrived from a paired phone. Paste it into the OS clipboard so
		// the user can Ctrl+V it anywhere. We log only the length, never content.
		if err := be.Set(context.Background(), text); err != nil {
			logger.Error("clipboard set (incoming)", "err", err)
			return
		}
		logger.Info("received clip", "len", len(text))
		// Remember what we just pasted so watchClipboard does not echo it back
		// out to the phone (feedback loop).
		lastAppliedIncoming.Mark(text)
	}, logger)
	srv.SetStatusCallback(func(addrs []string) {
		status := config.RuntimeStatus{
			PID:       os.Getpid(),
			Port:      fixedPort,
			Clients:   addrs,
			UpdatedAt: time.Now(),
		}
		if err := config.SaveRuntimeStatus(status); err != nil {
			logger.Warn("save runtime status", "err", err)
		}
	})
	defer config.DeleteRuntimeStatus()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go watchClipboard(ctx, be, srv, logger, &lastAppliedIncoming)

	// Run the server in the background before blocking on shutdown.
	go func() {
		if err := srv.Start(ctx); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	// srv.port is set synchronously inside Start right after the listener
	// binds, before the accept loop blocks. Poll until it is non-zero.
	port := 0
	for port == 0 {
		port = srv.Port()
		time.Sleep(20 * time.Millisecond)
	}

	// Persist the chosen port so the QR and the phone's saved port stay stable
	// across restarts and fresh installs.
	if port != fixedPort {
		logger.Info("adopting bound port", "port", port)
		port = fixedPort
	}
	if err := config.SavePort(fixedPort); err != nil {
		logger.Warn("persist port", "err", err)
	}

	logger.Info("daemon listening", "port", port)

	<-ctx.Done()
}

type incomingClipGuard struct {
	mu   sync.Mutex
	text string
}

func (g *incomingClipGuard) Mark(text string) {
	g.mu.Lock()
	g.text = text
	g.mu.Unlock()
}

func (g *incomingClipGuard) Consume(text string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.text != text {
		return false
	}
	g.text = ""
	return true
}

func watchClipboard(ctx context.Context, be backend.Backend, srv *server.Server, logger *slog.Logger, lastAppliedIncoming *incomingClipGuard) {
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
				logger.Error("clipboard get", "err", err)
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

			// Skip a clip we just pasted from a phone so it is not echoed back.
			if lastAppliedIncoming != nil && lastAppliedIncoming.Consume(text) {
				continue
			}

			lastClip = text
			lastTime = t

			if err := srv.Send(text); err != nil {
				logger.Error("send clip", "err", err)
			} else {
				logger.Info("sent clip", "len", len(text))
			}
		}
	}
}

func cmdStatus(logger *slog.Logger) {
	secret, err := config.LoadSecret()
	if err != nil {
		fmt.Printf("Error reading config: %v\n", err)
		os.Exit(1)
	}
	if secret == nil {
		fmt.Println("Not paired. Run 'clipconnect pair' first.")
		return
	}
	pid, err := config.LoadPID()
	if err != nil {
		fmt.Printf("Daemon: unknown (%v)\n", err)
		pid = 0
	}
	running := pid > 0 && syscall.Kill(pid, 0) == nil
	if !running {
		fmt.Println("Daemon: stopped")
	} else {
		fmt.Println("Daemon: running")
	}

	be, err := backend.Detect()
	if err != nil {
		fmt.Printf("Backend: %v\n", err)
	} else {
		fmt.Printf("Backend: %s\n", backendName(be))
	}

	fmt.Printf("Port: %d\n", fixedPort)

	status, err := config.LoadRuntimeStatus()
	if err != nil || !running || status.PID != pid {
		fmt.Println("Phone connected: no")
	} else {
		fmt.Printf("Phone connected: %s\n", strings.Join(status.Clients, ", "))
	}
}

func cmdUnpair() {
	if err := stopDaemon(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not stop daemon: %v\n", err)
	}
	if err := config.DeleteSecret(); err != nil {
		// Not paired yet: nothing to remove. Safe to ignore.
		return
	}
	fmt.Println("Unpaired. Secret removed.")
}

func stopDaemon() error {
	pid, err := config.LoadPID()
	if err != nil {
		return err
	}
	if pid == 0 || pid == os.Getpid() {
		return config.DeletePID()
	}

	if err := syscall.Kill(pid, 0); err != nil {
		if err := config.DeletePID(); err != nil {
			return err
		}
		return nil
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM to pid %d: %w", pid, err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err != nil {
			return config.DeletePID()
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("daemon pid %d did not stop", pid)
}

func cmdSetupPhone() {
	fmt.Println("Run these commands on your computer via adb:")
	fmt.Println()
	fmt.Printf("  adb install ClipConnect-debug.apk\n")
	fmt.Printf("  adb shell pm grant %s android.permission.READ_LOGS\n", pkgName)
	fmt.Printf("  adb shell appops set %s SYSTEM_ALERT_WINDOW allow\n", pkgName)
	fmt.Println()
	fmt.Println("Then on your phone, enable Accessibility:")
	fmt.Printf("  Settings → Accessibility → ClipConnect → Enable\n")
	fmt.Println()
	fmt.Println("For automatic start on login, install the systemd service:")
	fmt.Println("  sudo cp laptop/clipconnect.service /etc/systemd/system/")
	fmt.Println("  sudo systemctl enable --now clipconnect")
	fmt.Println()
	fmt.Println("Or for XDG autostart (desktop environments):")
	fmt.Println("  cp laptop/clipconnect.desktop ~/.config/autostart/")
	fmt.Println()
	fmt.Println("Finally, run 'clipconnect pair' to pair with your phone.")
}

func lanIPs() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ip, ok := addr.(*net.IPNet); ok && !ip.IP.IsLoopback() {
				if ip.IP.To4() != nil {
					ips = append(ips, ip.IP.String())
				}
			}
		}
	}
	return ips
}

func backendName(be backend.Backend) string {
	switch be.(type) {
	case *backend.WaylandBackend:
		return "wayland (wl-paste/wl-copy)"
	case *backend.X11Backend:
		return "x11 (xclip/xsel)"
	default:
		return "unknown"
	}
}
