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
	"syscall"
	"time"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/clip"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/clip/backend"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/config"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/mdns"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/pair"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/server"

	"github.com/skip2/go-qrcode"
)

const (
	defaultPort = 48123
	pkgName     = "dev.clipconnect.app"

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
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// The daemon logger writes to a file, not the terminal: the terminal is
	// reserved for the interactive sudo prompt and the QR code, so the two can
	// never interleave and garble the output.
	logger := newFileLogger()
	switch os.Args[1] {
	case "pair":
		cmdPair(logger)
	case "daemon":
		cmdDaemon(logger)
	case "status":
		cmdStatus(logger)
	case "unpair":
		cmdUnpair()
	case "setup-phone":
		cmdSetupPhone()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprint(os.Stderr, `Usage: clipconnect <command>

Commands:
  pair         Generate pairing secret and show QR code
  daemon       Start the clipboard sync daemon
  status       Show pairing and connection status
  unpair       Remove the pairing secret
  setup-phone  Print adb commands to set up the phone
`)
}

func cmdPair(logger *slog.Logger) {
	existing, _ := config.LoadSecret()
	if existing != nil {
		fmt.Println("Already paired. Run 'clipconnect unpair' first to re-pair.")
		return
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
	uri := pair.BuildURI(secret, ips, defaultPort, hostname)

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
	fmt.Printf("Port: %d\n", defaultPort)
	fmt.Println()
	fmt.Println("Firewall rules:")
	fmt.Println("  ufw: sudo ufw allow 48123/tcp")
	fmt.Println("  firewalld: sudo firewall-cmd --add-port=48123/tcp --permanent && sudo firewall-cmd --reload")
}

func cmdDaemon(logger *slog.Logger) {
	// Pair only on first start: reuse the existing secret so the QR stays
	// stable across restarts. The user changes it only via 'unpair'.
	secret, _ := config.LoadSecret()
	if secret == nil {
		cmdUnpair()
		var err error
		secret, err = config.GenerateSecret()
		if err != nil {
			log.Fatalf("generate secret: %v", err)
		}
		if err := config.SaveSecret(secret); err != nil {
			log.Fatalf("save secret: %v", err)
		}
	}

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
	var lastAppliedIncoming string

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
		lastAppliedIncoming = text
	}, logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go watchClipboard(ctx, be, srv, logger, &lastAppliedIncoming)

	// Start mDNS advertiser
	ifaces := mdns.PickInterfaces()
	hostname, _ := os.Hostname()
	var advertiser *mdns.Advertiser
	if adv, err := mdns.NewAdvertiser(secret, defaultPort, hostname, ifaces, logger); err != nil {
		logger.Warn("mDNS advertisement failed, continuing without it", "err", err)
	} else {
		advertiser = adv
		go advertiser.Run(ctx)
		logger.Info("mDNS advertiser started")
	}

	// Run the server in the background so we can capture the OS-assigned port
	// and print the pairing QR before blocking on shutdown.
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

	if advertiser != nil {
		advertiser.AdvertisePort(port)
	}
	logger.Info("daemon listening", "port", port)

	// Open the firewall BEFORE showing the QR, so the sudo prompt (if any) is
	// out of the way and the QR prints cleanly on its own. A clear line marks
	// where the password prompt will appear.
	fmt.Println()
	fmt.Println("Opening firewall (enter sudo password when prompted)…")
	if err := ensureUfwAllow(); err != nil {
		logger.Warn("firewall (manual steps required)", "err", err)
		fmt.Printf("Auto-allow failed; run manually:  sudo ufw allow %d/tcp\n", fixedPort)
	} else {
		logger.Info("firewall open", "port", fixedPort)
	}

	// Build and print the pairing QR with the real IP:port+secret.
	printPairingQR(logger, secret, port)

	<-ctx.Done()
}

// printPairingQR builds a pairing URI with the live IP list and port, prints the
// QR, and echoes the manual URI. The phone reads host + port + secret from it.
func printPairingQR(logger *slog.Logger, secret []byte, port int) {
	ips := lanIPs()
	if len(ips) == 0 {
		ips = []string{"127.0.0.1"}
	}
	hostname, _ := os.Hostname()
	uri := pair.BuildURI(secret, ips, port, hostname)

	qr, err := qrcode.New(uri, qrcode.Medium)
	if err != nil {
		log.Fatalf("generate qr: %v", err)
	}

	fmt.Println()
	fmt.Println("Scan this QR code with the ClipConnect Android app:")
	fmt.Println()
	fmt.Println(qr.ToSmallString(true))
	fmt.Println()
	fmt.Println("Or connect manually with:")
	fmt.Println(uri)
	fmt.Println()
	fmt.Printf("LAN IPs: %s\n", strings.Join(ips, ", "))
	fmt.Printf("Port: %d\n", port)
	fmt.Println()
	fmt.Println("The firewall is opened automatically on first start (one sudo prompt).")
}

func watchClipboard(ctx context.Context, be backend.Backend, srv *server.Server, logger *slog.Logger, lastAppliedIncoming *string) {
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
			if lastAppliedIncoming != nil && text == *lastAppliedIncoming {
				*lastAppliedIncoming = ""
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
	fmt.Println("Paired: yes")

	be, err := backend.Detect()
	if err != nil {
		fmt.Printf("Backend: %v\n", err)
	} else {
		fmt.Printf("Backend: %s\n", backendName(be))
	}

	fmt.Printf("Port: %d\n", defaultPort)

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", defaultPort), 2*time.Second)
	if err != nil {
		fmt.Println("Phone connected: no")
	} else {
		conn.Close()
		fmt.Println("Phone connected: yes")
	}
}

func cmdUnpair() {
	if err := config.DeleteSecret(); err != nil {
		// Not paired yet: nothing to remove. Safe to ignore.
		return
	}
	fmt.Println("Unpaired. Secret removed.")
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
