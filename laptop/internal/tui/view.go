package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4;249")).Render
	subtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render
	menuStyle     = lipgloss.NewStyle().PaddingLeft(2).Render
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true).PaddingLeft(2).Render
	helpStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1).MarginTop(1).Render
	statusGood    = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render
	statusBad     = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render
	statusMuted   = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render
	footerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Render
)

// View renders the current tab with Lip Gloss styling.
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
	b.WriteString(subtitleStyle("A CLI-based tool for Android↔Linux automated synced clipboard sharing"))
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
		return statusMuted("Not paired yet — select \"Pair Android device\"")
	}
	if m.status.phoneConnected {
		when := ""
		if m.status.phoneIP != "" {
			when = "  phone " + m.status.phoneIP
		}
		return statusGood("Paired • connected" + when)
	}
	return statusBad("Paired • phone not connected")
}

// viewPair renders the QR and live connection status.
func (m model) viewPair() string {
	var b strings.Builder
	b.WriteString(titleStyle("Pair Android device"))
	b.WriteString("\n\n")

	if m.qr != "" {
		b.WriteString(m.qr)
		b.WriteString("\n\n")
		b.WriteString(statusMuted("Scan the QR with the ClipConnect Android app"))
		b.WriteString("\n\n")
	}

	// Live status block.
	if m.status.paired {
		if m.status.phoneConnected {
			b.WriteString(statusGood("● Connected"))
		} else {
			b.WriteString(statusBad("○ Waiting for phone…"))
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("  Port:        %d\n", m.status.port))
	if m.status.phoneIP != "" {
		b.WriteString(fmt.Sprintf("  Phone IP:    %s\n", m.status.phoneIP))
	}
	if m.status.uptime > 0 {
		b.WriteString(fmt.Sprintf("  Uptime:      %s\n", formatUptime(m.status.uptime)))
	}

	b.WriteString("\n")
	b.WriteString(helpStyle("Press Esc to go back"))
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
		b.WriteString(truncate(m.logs, 24*40))
	}
	b.WriteString("\n\n")
	b.WriteString(helpStyle("Press Esc to go back"))
	return b.String()
}

// viewHelp renders the help/troubleshooting screen.
func (m model) viewHelp() string {
	return helpStyle(helpText)
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
