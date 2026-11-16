package status

import (
	"sync"
	"time"
)

// ServerLike is the subset of *server.Server the manager observes.
type ServerLike interface {
	ClientCount() int
	ClientAddrs() []string
	Port() int
}

// Manager tracks the daemon's live connection state and persists it to the
// status file so external commands can report real state. It is safe for
// concurrent use. It never records clipboard data or secrets.
type Manager struct {
	mu       sync.Mutex
	srv      ServerLike
	paired   bool
	daemonOn bool
	started  time.Time

	phoneAddrs  []string
	clientCount int
	lastConnect *time.Time
}

// NewManager creates a Manager observing srv. started is the daemon start time.
func NewManager(srv ServerLike, started time.Time) *Manager {
	return &Manager{srv: srv, started: started}
}

// SetPaired records whether a pairing secret exists on disk.
func (m *Manager) SetPaired(paired bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.paired = paired
}

// SetDaemonRunning records whether the systemd unit is active.
func (m *Manager) SetDaemonRunning(on bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.daemonOn = on
}

// TrackConnect records the time and address of a newly established session.
func (m *Manager) TrackConnect(addr string) {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastConnect = &now
	if addr != "" {
		// Keep only the most recent connection addr in the primary field;
		// the full live list comes from the server on the next Refresh.
		m.phoneAddrs = []string{addr}
	}
}

// Snapshot reads the current observed state.
func (m *Manager) Snapshot() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := State{
		Paired:        m.paired,
		DaemonRunning: m.daemonOn,
		Port:          m.port(),
		ClientCount:   m.clientCount,
		PhoneAddrs:    append([]string(nil), m.phoneAddrs...),
		UptimeSeconds: int64(time.Since(m.started).Seconds()),
	}
	if m.lastConnect != nil {
		tc := *m.lastConnect
		st.LastConnect = &tc
	}
	return st
}

func (m *Manager) port() int {
	if m.srv == nil {
		return 0
	}
	return m.srv.Port()
}

// Refresh updates the observed connection state from the server and writes the
// state file. It is intended to be called on a ticker. Errors are returned so
// the caller can log them but keep running.
func (m *Manager) Refresh() (State, error) {
	m.mu.Lock()
	if m.srv != nil {
		m.clientCount = m.srv.ClientCount()
		m.phoneAddrs = m.srv.ClientAddrs()
	}
	st := State{
		Paired:        m.paired,
		DaemonRunning: m.daemonOn,
		Port:          m.port(),
		ClientCount:   m.clientCount,
		PhoneAddrs:    append([]string(nil), m.phoneAddrs...),
		UptimeSeconds: int64(time.Since(m.started).Seconds()),
	}
	if m.lastConnect != nil {
		tc := *m.lastConnect
		st.LastConnect = &tc
	}
	m.mu.Unlock()

	if err := Write(st); err != nil {
		return st, err
	}
	return st, nil
}

// trackConnect records a new established session so LastConnect is fresh.
func (m *Manager) trackConnect() {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastConnect = &now
}

