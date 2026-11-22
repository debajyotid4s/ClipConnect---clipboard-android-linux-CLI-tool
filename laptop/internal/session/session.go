package session

import (
	"net"
	"sync"
	"time"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/frame"
)

const (
	magic       = "CCN1"
	nonceLen    = 16
	keyLen      = 32
	handshakeTO = 5 * time.Second
)

var (
	infoC2S = []byte("clipconnect v1 c2s")
	infoS2C = []byte("clipconnect v1 s2c")
	helloMsg = []byte(`{"t":"hello","v":1}`)
)

// Message is a JSON frame message.
type Message struct {
	Type string  `json:"t"`
	Text string  `json:"text,omitempty"`
	Ver  int     `json:"v,omitempty"`
	Seq  uint64  `json:"seq,omitempty"`
	Pid  string  `json:"pid,omitempty"`
	Cid  string  `json:"cid,omitempty"`
}

// Session holds an encrypted connection between two peers.
type Session struct {
	conn net.Conn
	recv *frame.Codec
	send *frame.Codec

	// connID is the per-connection identity used to key the durable outbox and
	// ack state. On the server it is a random id it generated for this
	// connection and handed to the client in its hello; on the client it is the
	// id it received from the server. Because a fresh random id is minted on
	// every connection, a reconnecting peer can never claim another peer's
	// queued clips, and a disconnected peer's id is orphaned and subject to
	// expiry. Empty only if the peer omitted it (older clients).
	connID string

	// isServer distinguishes the server side of a session. The server generates
	// a fresh random per-connection id and announces it in its hello so both
	// sides key the durable outbox on a value only it minted.
	isServer bool

	// writeMu serializes concurrent writers (server Send, sendAck, pingLoop)
	// so the AES-GCM counter nonce and the TCP frame stream are never
	// interleaved or advanced out of order.
	writeMu sync.Mutex
}

// NewServer performs the protocol handshake as the server and returns a ready Session.
func NewServer(conn net.Conn, secret []byte) (*Session, error) {
	s, err := handshake(conn, secret, false)
	if err != nil {
		conn.Close()
		return nil, err
	}
	// Generate a fresh random per-connection identity and announce it in the
	// server hello so both sides key the durable outbox on a value only the
	// server minted. A reconnecting client gets a new id each time and cannot
	// claim another peer's queued clips.
	if err := exchangeHello(s); err != nil {
		conn.Close()
		return nil, err
	}
	return s, nil
}

// NewClient performs the protocol handshake as the client and returns a ready Session.
func NewClient(conn net.Conn, secret []byte) (*Session, error) {
	s, err := handshake(conn, secret, true)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := exchangeHello(s); err != nil {
		conn.Close()
		return nil, err
	}
	return s, nil
}

// NewClientWithID performs the handshake like NewClient but sends a hello that
// carries the given identity ("pid"). It exists so tests (and future clients)
// can exercise identity-keyed behavior such as the durable outbox. An empty pid
// produces the same hello as NewClient.
func NewClientWithID(conn net.Conn, secret []byte, pid string) (*Session, error) {
	s, err := handshake(conn, secret, true)
	if err != nil {
		conn.Close()
		return nil, err
	}
	// Send a hello that optionally carries the identity.
	if pid != "" {
		if err := s.WriteMsg(&Message{Type: "hello", Ver: 1, Pid: pid}); err != nil {
			conn.Close()
			return nil, err
		}
	} else {
		if err := s.WriteMsg(&Message{Type: "hello", Ver: 1}); err != nil {
			conn.Close()
			return nil, err
		}
	}
	// Read the server's hello back. The server announces a fresh per-connection
	// id ("cid") which we adopt so both sides key the outbox on it.
	if _, err := s.ReadMsg(); err != nil {
		conn.Close()
		return nil, err
	}
	return s, nil
}

// ConnID returns the per-connection identity established during the hello
// exchange (server-generated on connect, empty if the peer omitted it).
func (s *Session) ConnID() string { return s.connID }

// SetConnID overrides the per-connection identity.
func (s *Session) SetConnID(id string) { s.connID = id }
