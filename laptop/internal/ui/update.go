package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// tickMsg is returned by the refresh tick; it carries no payload because the
// status is read from the observer channel in poll().
type tickMsg struct{}

// Update implements bubbletea.Model. It handles keyboard navigation, the
// refresh tick, and external status pushes.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		return m, nil

	case tickMsg:
		// Refresh on the interval. The returned command re-arms the tick.
		fn := func(time.Time) tea.Msg {
			return tickMsg{}
		}
		if m.poll() != nil {
			return m, tea.Tick(refreshInterval, fn)
		}
		return m, tea.Tick(refreshInterval, fn)

	case tea.KeyMsg:
		return m.handleKey(v)

	case tea.QuitMsg:
		return m, tea.Quit
	}
	return m, nil
}

// handleKey routes key presses to tab switches and quit.
func (m *Model) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
	case "1":
		m.tab = TabOverview
		return m, nil
	case "2":
		m.tab = TabQR
		return m, nil
	case "3":
		m.tab = TabActivity
		return m, nil
	case "right", "l":
		m.tab = (m.tab + 1) % 3
		return m, nil
	case "left", "h":
		m.tab = (m.tab + 2) % 3
		return m, nil
	case "up", "k":
		m.Scroll(-1)
		return m, nil
	case "down", "j":
		m.Scroll(1)
		return m, nil
	}
	return m, nil
}
