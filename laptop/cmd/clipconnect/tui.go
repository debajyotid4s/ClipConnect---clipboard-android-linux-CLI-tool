package main

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/ui"
)

// tuiStatusSink is the subset of *server.Server the TUI observes. Keeping it a
// small interface lets tests inject a fake server.
type tuiStatusSink interface {
	Port() int
	ClientCount() int
}

var uiStartTime = time.Now()

// runTUI launches the bubbletea dashboard over a running daemon. A single
// polling goroutine pushes a fresh ui.Status once per second into the model's
// channel; the model's own tick pulls it via poll(). The TUI is read-only: it
// never mutates daemon state.
//
// [qr] optionally provides the pairing QR body; when empty the QR tab shows a
// placeholder.
func runTUI(sink tuiStatusSink, paired bool, qr string) {
	statusCh := make(chan ui.Status, 8)
	model := ui.NewModel(statusCh)

	cfg := ui.DefaultConfig()
	if qr != "" {
		q := qr
		cfg.QR = func() string { return q }
	}

	// Polling goroutine: push a fresh snapshot every second until the program
	// exits. The model pulls each value on its tick, so this channel never
	// blocks.
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			statusCh <- ui.Status{
				Paired:         paired,
				Port:           sink.Port(),
				PhoneConnected: sink.ClientCount() > 0,
				ClientCount:    sink.ClientCount(),
				Uptime:         time.Since(uiStartTime),
			}
		}
	}()

	p := tea.NewProgram(model, tea.WithAltScreen())
	if err := p.Start(); err != nil {
		// Not a TTY (e.g. piped or CI): fall back to a single static render so
		// the status is still visible.
		println(err.Error())
		println(model.View())
		return
	}
}
