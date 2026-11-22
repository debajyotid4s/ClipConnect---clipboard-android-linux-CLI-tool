package server

import (
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/session"
)

// Server listens for incoming connections and manages sessions.
type Server struct {
	addr    string
	secret  []byte
	handler Handler
	onEstablished OnEstablished
	logger  *slog.Logger
	port    int

	// ln is stored so Start can close it on shutdown, which unblocks Accept()
	// (a plain ctx cancel does not stop a blocking Accept).
	ln   net.Listener
	mu   sync.Mutex
	clients map[*session.Session]bool

	// Per-client seq tracking
	seqMu     sync.Mutex
	clientSeq map[string]uint64

	// Per-connection highest acked seq (laptop->phone direction), so the outbox
	// can be pruned when a phone acknowledges delivery.
	ackedMu   sync.Mutex
	ackedSeq  map[string]uint64

	// Outbox: per-connection durable queue of laptop->phone clips that have not
	// yet been acknowledged. It survives a disconnect for [outboxGrace] so a clip
	// sent just before the phone drops is replayed when the phone reconnects,
	// instead of being lost (the previous fire-and-forget behavior). Each entry
	// is keyed by a fresh per-connection id minted by the server on connect, so a
	// disconnected phone's id is orphaned and eventually pruned by the reaper.
	outboxMu     sync.Mutex
	outbox       map[string]*phoneOutbox
	outboxGrace  time.Duration
	reaperInterval time.Duration
	stopReaper   chan struct{}
}
