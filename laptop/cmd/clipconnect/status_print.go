package main

import (
	"fmt"
	"net"
	"time"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/pairing"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/status"
)

// printStatus renders the status snapshot in a human-readable report.
func printStatus(st status.State, pairedFallback, daemonFallback bool) {
	fmt.Println("ClipConnect status")
	fmt.Println("==================")

	fmt.Printf("Paired:            %s\n", yesNo(st.Paired || pairedFallback))
	fmt.Printf("Daemon running:    %s\n", yesNo(st.DaemonRunning || daemonFallback))
	fmt.Printf("Listen port:       %d\n", portOrDefault(st.Port))

	if st.IsConnected() {
		fmt.Printf("Phones connected:  %d\n", st.ClientCount)
		for _, addr := range st.PhoneAddrs {
			fmt.Printf("  - %s\n", addr)
		}
	} else {
		fmt.Println("Phones connected:  0")
	}

	if st.LastConnect != nil {
		fmt.Printf("Last connected:    %s\n", st.LastConnect.Format(time.RFC1123))
	} else {
		fmt.Println("Last connected:    never")
	}

	if st.UptimeSeconds > 0 {
		fmt.Printf("Daemon uptime:     %s\n", humanDuration(time.Duration(st.UptimeSeconds) * time.Second))
	}

	// Guidance when not connected.
	if !st.IsConnected() && (st.DaemonRunning || daemonFallback) && (st.Paired || pairedFallback) {
		fmt.Println()
		fmt.Println("No phone connected. On the phone: open ClipConnect, ensure it shows")
		fmt.Println("Connected, and keep it in the foreground or recent apps.")
	}
}

// probePhoneAddrs dials the daemon to confirm it is reachable. It cannot read
// the real peer IP from loopback, so it returns a single entry when the port
// answers and nil otherwise.
func probePhoneAddrs() []string {
	addr := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", pairing.FixedPort))
	conn, err := net.DialTimeout("tcp", addr, 500e6)
	if err != nil {
		return nil
	}
	conn.Close()
	return []string{addr + " (daemon reachable)"}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func portOrDefault(p int) int {
	if p <= 0 {
		return pairing.FixedPort
	}
	return p
}

func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
