package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
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
	defaultPort = 48123
	pkgName     = "dev.clipconnect.app"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

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
	secret, err := config.LoadSecret()
	if err != nil {
		log.Fatalf("load secret: %v", err)
	}
	if secret == nil {
		log.Fatal("not paired. Run 'clipconnect pair' first.")
	}

	be, err := backend.Detect()
	if err != nil {
		log.Fatalf("detect backend: %v", err)
	}
	logger.Info("using backend", "type", backendName(be))

	srv := server.New(fmt.Sprintf(":%d", defaultPort), secret, func(text string) {
		logger.Info("received clip", "len", len(text))
	}, logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go watchClipboard(ctx, be, srv, logger)

	logger.Info("daemon starting", "port", defaultPort)
	if err := srv.Start(ctx); err != nil {
		log.Fatalf("server: %v", err)
	}
	logger.Info("daemon stopped")
}

func watchClipboard(ctx context.Context, be backend.Backend, srv *server.Server, logger *slog.Logger) {
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
		log.Fatalf("delete secret: %v", err)
	}
	fmt.Println("Unpaired. Secret removed.")
}

func cmdSetupPhone() {
	fmt.Println("Run these commands on your computer via adb:")
	fmt.Println()
	fmt.Printf("  adb shell pm grant %s android.permission.READ_LOGS\n", pkgName)
	fmt.Printf("  adb shell appops set %s SYSTEM_ALERT_WINDOW allow\n", pkgName)
	fmt.Println()
	fmt.Println("Then install the ClipConnect APK and run 'clipconnect pair'.")
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
