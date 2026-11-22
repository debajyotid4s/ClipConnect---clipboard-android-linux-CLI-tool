package ui

import (
	"fmt"
	"strings"
	"time"
)

// qrSource abstracts QR generation so the view can be tested without a real
// QR library and so the daemon can supply whatever it already has.
type qrSource func() string

// ViewConfig holds rendering options and the optional QR provider.
type ViewConfig struct {
	QR   qrSource
	Now  func() time.Time
}

// DefaultConfig returns sensible defaults (clock = time.Now, no QR provider).
func DefaultConfig() ViewConfig {
	return ViewConfig{Now: time.Now}
}

// View satisfies bubbletea.Model. It renders with default config.
func (m *Model) View() string {
	return m.Render(DefaultConfig())
}

// Render renders the model to a string. It is pure: no terminal I/O, which makes
// it directly unit-testable. [cfg.QR] is optional; when nil the QR tab shows a
// placeholder.
func (m *Model) Render(cfg ViewConfig) string {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	switch m.tab {
	case TabQR:
		return m.viewQR(cfg)
	case TabActivity:
		return m.viewActivity(cfg)
	case TabOverview:
		fallthrough
	default:
		return m.viewOverview(cfg)
	}
}

func (m *Model) viewOverview(cfg ViewConfig) string {
	s := m.status

	var b strings.Builder
	b.WriteString(headerStyle.Render(" ClipConnect "))
	b.WriteString(fmt.Sprintf("  uptime %s\n", formatUptime(s.Uptime, cfg.Now())))

	b.WriteString(row("Paired", boolLabel(s.Paired)))
	b.WriteString(row("Backend", nonDash(s.Backend)))
	b.WriteString(row("Port", intOrDash(s.Port)))
	b.WriteString(row("Phone", connectedLabel(s.PhoneConnected)))
	if s.PhoneConnected {
		b.WriteString(fmt.Sprintf("    clients: %d\n", s.ClientCount))
	}
	b.WriteString(row("Sent", itoa(s.SentCount)))
	b.WriteString(row("Received", itoa(s.ReceivedCount)))

	b.WriteString("\n")
	b.WriteString(helpStyle.Render(" [1] Overview  [2] QR  [3] Activity   j/k ↑↓   q quit "))
	return b.String()
}

func (m *Model) viewQR(cfg ViewConfig) string {
	s := m.status
	var b strings.Builder
	b.WriteString(headerStyle.Render(" Pairing "))

	if !s.Paired {
		b.WriteString("\n  Not paired yet.\n")
		b.WriteString(mutedStyle.Render("  Run 'clipconnect pair' to generate a secret.\n"))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render(" [2] QR   q quit "))
		return b.String()
	}

	if cfg.QR == nil {
		b.WriteString("\n  QR unavailable (daemon did not expose one).\n")
		b.WriteString(mutedStyle.Render("  Pair via 'clipconnect pair' instead.\n"))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render(" [2] QR   q quit "))
		return b.String()
	}

	qr := cfg.QR()
	b.WriteString("\n")
	b.WriteString(qrStyle.Render(qr))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(" [2] QR   q quit "))
	return b.String()
}

func (m *Model) viewActivity(cfg ViewConfig) string {
	var b strings.Builder
	b.WriteString(headerStyle.Render(" Activity "))
	b.WriteString(fmt.Sprintf("  %d entries\n", len(m.log)))

	if len(m.log) == 0 {
		b.WriteString("\n  No activity yet.\n")
		b.WriteString(helpStyle.Render(" [3] Activity   q quit "))
		return b.String()
	}

	// Show the tail of the log around the cursor.
	start := m.cursor
	const window = 12
	if start > len(m.log)-window {
		start = len(m.log) - window
	}
	if start < 0 {
		start = 0
	}
	for i := start; i < len(m.log); i++ {
		entry := m.log[i]
		ts := entry.time.Format("15:04:05")
		line := fmt.Sprintf("  %s  %s", mutedStyle.Render(ts), entry.text)
		if i == m.cursor {
			line = activeStyle.Render("▸ " + line)
		}
		b.WriteString(line + "\n")
	}

	b.WriteString(helpStyle.Render(" [3] Activity   j/k ↑↓   q quit "))
	return b.String()
}

// row renders a two-column status row.
func row(label, value string) string {
	return fmt.Sprintf("  %-9s %s\n", mutedStyle.Render(label)+":", value)
}

func boolLabel(b bool) string {
	if b {
		return greenStyle.Render("yes")
	}
	return redStyle.Render("no")
}

func nonDash(s string) string {
	if s == "" {
		return dash
	}
	return s
}

func intOrDash(n int) string {
	if n == 0 {
		return dash
	}
	return itoa(n)
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

// formatUptime renders a duration as h/m/s, dropping leading zero units.
func formatUptime(d time.Duration, now time.Time) string {
	if d < 0 {
		d = 0
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
