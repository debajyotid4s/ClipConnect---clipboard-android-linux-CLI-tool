package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// refreshInterval is how often the model pulls a fresh daemon snapshot.
const refreshInterval = 1 * time.Second

// maxLog bounds the activity log to the most recent entries.
const maxLog = 200

// Tab identifies a view in the TUI.
type Tab int

const (
	TabOverview Tab = iota
	TabQR
	TabActivity
)

// String renders the tab for keyboard shortcut hints.
func (t Tab) String() string {
	switch t {
	case TabOverview:
		return "Overview"
	case TabQR:
		return "QR"
	case TabActivity:
		return "Activity"
	}
	return ""
}

// Status is an immutable snapshot of everything the Overview view needs. It is
// produced by the daemon-facing observer and consumed by the TUI so the two
// never share mutable state.
type Status struct {
	Paired         bool
	Backend        string
	Port           int
	PhoneConnected bool
	ClientCount    int
	Uptime         time.Duration
	SentCount      int
	ReceivedCount  int
}

type logEntry struct {
	time  time.Time
	text  string
}

// Model holds all TUI state. It is safe to render from View after any Update.
type Model struct {
	tab     Tab
	status  Status
	log     []logEntry
	cursor  int
	started time.Time
	observe <-chan Status
}

// NewModel creates a model primed to receive status updates on [ch].
func NewModel(ch <-chan Status) *Model {
	return &Model{
		tab:     TabOverview,
		status:  Status{},
		log:     make([]logEntry, 0, maxLog),
		started: time.Now(),
		observe: ch,
	}
}

// SetObserver replaces the status source channel. Primarily for tests.
func (m *Model) SetObserver(ch <-chan Status) {
	m.observe = ch
}

// Init satisfies bubbletea.Model by kicking off the refresh tick.
func (m *Model) Init() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

// poll reads one pending status snapshot, if any, applies it, and returns the
// message to feed back into Update. It returns nil when no new status is
// available so the tick can be discarded.
func (m *Model) poll() tea.Msg {
	select {
	case s := <-m.observe:
		m.applyStatus(s)
		return tickMsg{}
	default:
		return nil
	}
}

// applyStatus merges a new snapshot into the model and derives activity events
// from counter deltas so the Activity view shows sent/received without the
// daemon pushing each one.
func (m *Model) applyStatus(s Status) {
	if s.Paired != m.status.Paired {
		m.appendLog(fmt.Sprintf("paired: %t", s.Paired))
	}
	if s.PhoneConnected != m.status.PhoneConnected {
		m.appendLog(fmt.Sprintf("phone: %s", connectedLabel(s.PhoneConnected)))
	}
	if s.SentCount > m.status.SentCount {
		m.appendLog(fmt.Sprintf("sent clip (+%d)", s.SentCount-m.status.SentCount))
	}
	if s.ReceivedCount > m.status.ReceivedCount {
		m.appendLog(fmt.Sprintf("received clip (+%d)", s.ReceivedCount-m.status.ReceivedCount))
	}
	m.status = s
}

func connectedLabel(ok bool) string {
	if ok {
		return "connected"
	}
	return "disconnected"
}

// appendLog adds an entry, keeping the log bounded.
func (m *Model) appendLog(text string) {
	m.log = append(m.log, logEntry{time: time.Now(), text: text})
	if len(m.log) > maxLog {
		m.log = m.log[len(m.log)-maxLog:]
	}
}

// PushEvent appends a raw activity line. Exposed so the daemon can forward
// per-clip events directly if desired.
func (m *Model) PushEvent(text string) {
	m.appendLog(text)
}

// Status returns the latest snapshot.
func (m *Model) Status() Status {
	return m.status
}

// SetStatus replaces the snapshot directly (used by tests and by the daemon's
// one-shot observer bridge).
func (m *Model) SetStatus(s Status) {
	m.applyStatus(s)
}

// Tab returns the active tab.
func (m *Model) Tab() Tab {
	return m.tab
}

// SetTab switches the active view.
func (m *Model) SetTab(t Tab) {
	if t >= TabOverview && t <= TabActivity {
		m.tab = t
	}
}

// Scroll moves the activity-view cursor by delta, clamped to the log bounds.
func (m *Model) Scroll(delta int) {
	if m.tab != TabActivity || len(m.log) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor > len(m.log)-1 {
		m.cursor = len(m.log) - 1
	}
}
