package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Palette inspired by the Claude Code terminal theme: a near-black blue-gray
// background with soft light text and muted colored accents.
var (
	bg        = lipgloss.Color("#1a1b26") // main background
	bgPanel   = lipgloss.Color("#222330") // cards / panels
	text      = lipgloss.Color("#d4d4d4") // primary text
	textDim   = lipgloss.Color("#8a8f98") // muted text
	accent    = lipgloss.Color("#9ece6a") // green accent (connected / ok)
	danger    = lipgloss.Color("#f7768e") // red accent (disconnected / bad)
	info      = lipgloss.Color("#7aa2f7") // blue accent (labels / titles)
	amber     = lipgloss.Color("#e0af68") // amber accent (warnings)
	borderCol = lipgloss.Color("#363744") // panel borders
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#bb9af7")).Padding(0, 1).Render
	subtitleStyle = lipgloss.NewStyle().Foreground(textDim).Render
	menuStyle     = lipgloss.NewStyle().Foreground(text).PaddingLeft(4).Render
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(bg)).Bold(true).Background(lipgloss.Color(accent)).PaddingLeft(4).PaddingRight(1).Render
	helpStyle     = lipgloss.NewStyle().Foreground(textDim).Background(lipgloss.Color(bgPanel)).Padding(1, 2).Render
	statusGood    = lipgloss.NewStyle().Foreground(lipgloss.Color(accent)).Render
	statusBad     = lipgloss.NewStyle().Foreground(lipgloss.Color(danger)).Render
	statusMuted   = lipgloss.NewStyle().Foreground(textDim).Render
	footerStyle   = lipgloss.NewStyle().Foreground(textDim).Render
	panelStyle    = lipgloss.NewStyle().Background(lipgloss.Color(bgPanel)).Padding(1, 2).Border(lipgloss.RoundedBorder()).Render
)

// View renders the current tab with Claude-style dark theming.
func (m model) View() string {
	switch m.tab {
	case tabPair:
		return m.viewPair()
	case tabLogs:
		return m.viewLogs()
	case tabHelp:
		return m.viewHelp()
	default:
		return m.viewMain()
	}
}

// viewMain renders the four-option keyboard menu.
func (m model) viewMain() string {
	var b strings.Builder
	b.WriteString(titleStyle("ClipConnect"))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle("Android↔Linux synced clipboard"))
	b.WriteString("\n\n")

	items := menu()
	for i, item := range items {
		if i == m.selected {
			b.WriteString(selectedStyle("> " + item))
		} else {
			b.WriteString(menuStyle("  " + item))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle("↑/↓ Navigate   Enter Select   q Quit"))

	// Live status footer.
	b.WriteString("\n\n")
	b.WriteString(m.statusFooter())

	return b.String()
}

// statusFooter renders a one-line connection summary.
func (m model) statusFooter() string {
	if !m.status.paired {
		return statusMuted("Not paired yet — choose \"Pair Android device\"")
	}
	if m.status.phoneConnected {
		if m.status.phoneIP != "" {
			return statusGood("● Connected") + statusMuted("   phone " + m.status.phoneIP)
		}
		return statusGood("Connected")
	}
	return statusBad("○ Not connected")
}

// viewPair renders the QR and live connection status.
func (m model) viewPair() string {
	var b strings.Builder
	b.WriteString(titleStyle("Pair Android device"))
	b.WriteString("\n\n")

	if m.qr != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(text)).Render("Scan with the ClipConnect Android app:"))
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(text)).Background(lipgloss.Color(bg)).Padding(0, 1).Render(m.qr))
		b.WriteString("\n\n")
	}

	b.WriteString(panelStyle(m.connectionPanel()))

	b.WriteString("\n\n")
	b.WriteString(helpStyle("Press Esc to go back"))
	return b.String()
}

// connectionPanel renders the connection status lines.
func (m model) connectionPanel() string {
	var b strings.Builder
	if m.status.paired {
		if m.status.phoneConnected {
			b.WriteString(statusGood("● Paired and connected"))
		} else {
			b.WriteString(statusBad("○ Paired — phone not connected"))
		}
	} else {
		b.WriteString(statusMuted("○ Not paired"))
	}
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s %d\n", labelStyle("Port:"), m.status.port))
	if m.status.phoneIP != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle("Phone IP:"), m.status.phoneIP))
	}
	if m.status.uptime > 0 {
		b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle("Uptime:"), formatUptime(m.status.uptime)))
	}
	return b.String()
}

// viewLogs renders the daemon log tail.
func (m model) viewLogs() string {
	var b strings.Builder
	b.WriteString(titleStyle("ClipConnect logs"))
	b.WriteString("\n\n")
	if m.logs == "" {
		b.WriteString(statusMuted("No logs available. Is the daemon running?"))
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(text)).Render(truncate(m.logs, 24*60)))
	}
	b.WriteString("\n\n")
	b.WriteString(helpStyle("Press Esc to go back"))
	return b.String()
}

// viewHelp renders the help/troubleshooting screen.
func (m model) viewHelp() string {
	return panelStyle(lipgloss.NewStyle().Foreground(lipgloss.Color(text)).Render(helpText))
}

// labelStyle renders a right-aligned key for status rows.
func labelStyle(key string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(info)).Width(len(key)).Render(key)
}

// truncate keeps the last n runes of s.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[len(r)-n:])
}

// formatUptime renders a duration as h/m/s.
func formatUptime(d time.Duration) string {
	total := int(d.Seconds())
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
