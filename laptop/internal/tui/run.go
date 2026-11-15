package tui

import (
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
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
