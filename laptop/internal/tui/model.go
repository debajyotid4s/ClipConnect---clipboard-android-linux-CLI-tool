package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// pollInterval is how often the model pulls a fresh daemon snapshot.
const pollInterval = 1 * time.Second

// tab identifies which full-screen view the TUI is showing.
type tab int

const (
	tabMain tab = iota // the four-option menu
	tabPair            // pairing + live connection status
	tabLogs            // daemon logs
	tabHelp            // help / troubleshooting
)

// statusMsg is a live status snapshot from the daemon poll channel.
type statusMsg struct {
	status *statusSnapshot
}

// tickMsg re-arms the poll ticker.
type tickMsg struct{}

// statusSnapshot is a plain copy of daemon state for rendering.
type statusSnapshot struct {
	paired         bool
	phoneConnected bool
	phoneIP        string
	port           int
	clientCount    int
	uptime         time.Duration
}

// model is the Bubble Tea model: the menu state machine plus the poll loop.
type model struct {
	tab      tab
	selected int // index into the main menu

	status statusSnapshot
	qr     string
	logs   string

	err error

	// statusCh receives live snapshots; nil means poll nothing.
	statusCh <-chan statusMsg
}

// menu returns the fixed four options for the main screen.
func menu() []string {
	return []string{
		"Pair Android device",
		"Unpair Android device",
		"View logs",
		"Help",
	}
}

// NewModel builds the initial model. statusCh (may be nil) receives live
// snapshots; qr is the pairing QR body; logs is an initial log tail.
func NewModel(statusCh <-chan statusMsg, qr string, logs string, paired bool) model {
	m := model{
		tab:      tabMain,
		selected: 0,
		statusCh: statusCh,
		logs:     logs,
	}
	if statusCh != nil {
		select {
		case s := <-statusCh:
			m.status = *s.status
		default:
		}
	} else {
		m.status = statusSnapshot{paired: paired}
	}
	if qr != "" {
		m.qr = qr
	}
	return m
}

// Init satisfies bubbletea.Model by kicking off the poll tick.
func (m model) Init() tea.Cmd {
	return tea.Tick(pollInterval, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

// Update implements bubbletea.Model.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		// Pull one pending snapshot, if any.
		select {
		case s := <-m.statusCh:
			m.status = *s.status
		default:
		}
		return m, tea.Tick(pollInterval, func(time.Time) tea.Msg {
			return tickMsg{}
		})
	case tea.KeyMsg:
		return m.updateKeys(msg)
	case tea.QuitMsg:
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateKeys(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.tab != tabMain {
			m.tab = tabMain
			m.selected = 0
			return m, nil
		}
		return m, tea.Quit
	}

	if m.tab == tabMain {
		return m.updateMenuKeys(k)
	}
	return m, nil
}

func (m model) updateMenuKeys(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := menu()
	switch k.String() {
	case "up", "k":
		if m.selected > 0 {
			m.selected--
		}
	case "down", "j":
		if m.selected < len(items)-1 {
			m.selected++
		}
	case "enter":
		return m.activate()
	}
	return m, nil
}

// activate runs the action for the selected menu item.
func (m model) activate() (tea.Model, tea.Cmd) {
	switch m.selected {
	case 0: // Pair
		m.tab = tabPair
		return m, nil
	case 1: // Unpair
		return m, nil
	case 2: // Logs
		m.tab = tabLogs
		return m, nil
	case 3: // Help
		m.tab = tabHelp
		return m, nil
	}
	return m, nil
}
