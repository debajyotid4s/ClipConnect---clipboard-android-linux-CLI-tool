// Package status defines the live status snapshot the TUI renders and the
// interface for the source that produces it (the running daemon). Keeping the
// snapshot a plain struct and the source an interface lets the TUI be tested
// against a fake and reused by non-interactive commands.
package status

import "time"

// Status is a point-in-time snapshot of the daemon. It is safe to copy.
type Status struct {
	Paired         bool
	PhoneConnected bool
	PhoneIP        string
	Port           int
	ClientCount    int
	Uptime         time.Duration
}

// Source produces fresh Status snapshots. The daemon implements it directly;
// tests provide a fake.
type Source interface {
	// Snapshot returns the current status.
	Snapshot() Status
}

// ServerLike is the subset of *server.Server the status source needs. It is
// satisfied by internal/server.Server.
type ServerLike interface {
	Port() int
	ClientCount() int
}

// FromServer builds a Source that reads live state from a running daemon.
func FromServer(s ServerLike, paired bool, port int) Source {
	return &serverSource{srv: s, paired: paired, port: port}
}

func (s *serverSource) Snapshot() Status {
	return Status{
		Paired:         s.paired,
		PhoneConnected: s.srv.ClientCount() > 0,
		Port:           s.srv.Port(),
		ClientCount:    s.srv.ClientCount(),
		Uptime:         time.Since(s.start),
	}
}

type serverSource struct {
	srv    ServerLike
	paired bool
	port   int
	start  time.Time
}
