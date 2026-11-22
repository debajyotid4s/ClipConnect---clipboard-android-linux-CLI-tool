package tui

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// StatusProvider is the minimal interface the poll needs. It is a function
// type so callers can pass a closure without an import cycle.
type StatusProvider func() StatusSnapshot

// StatusSnapshot mirrors status.Snapshot for the TUI.
type StatusSnapshot = struct {
	Paired         bool
	PhoneConnected bool
	PhoneIP        string
	Port           int
	ClientCount    int
	Uptime         time.Duration
}

// Run wires a statusProvider into the TUI and launches it. A background
// goroutine snapshots the provider once per second and pushes into the channel
// the model reads. It blocks until the user quits.
func Run(src StatusProvider, qr string, logs string, paired bool) error {
	statusCh := make(chan statusMsg, 8)
	go func() {
		if src == nil {
			return
		}
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for range ticker.C {
			s := src()
			statusCh <- statusMsg{status: &statusSnapshot{
				paired:         s.Paired,
				phoneConnected: s.PhoneConnected,
				phoneIP:        s.PhoneIP,
				port:           s.Port,
				clientCount:    s.ClientCount,
				uptime:         s.Uptime,
			}}
		}
	}()

	m := NewModel(statusCh, qr, logs, paired)

	// When stdout is not a TTY (piped, CI, or the shell can't open /dev/tty),
	// the full-screen program cannot run. Render a single static frame so the
	// status and QR are still visible instead of failing.
	if !isTerminal() {
		fmt.Println(m.View())
		return nil
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println(m.View())
		return nil
	}
	return nil
}

// isTerminal reports whether stdout is an interactive terminal. When false the
// full-screen Bubble Tea program cannot run, so Run renders a static frame.
func isTerminal() bool {
	f, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return f.Mode()&os.ModeCharDevice != 0
}
