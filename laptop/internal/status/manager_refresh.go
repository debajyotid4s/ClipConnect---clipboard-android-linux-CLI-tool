package status

import "time"

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
