package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/clip"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/session"
)

// Handler is called when a clip message is received.
type Handler func(text string)

// OnEstablished is an optional callback invoked once per successfully
// established session (after handshake + hello). It may be nil.
type OnEstablished func(addr string)

// portFromAddr extracts the port from a "host:port" address string.
func portFromAddr(addr string) (int, error) {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}
	var p int
	_, err = fmt.Sscanf(portStr, "%d", &p)
	return p, err
}

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
}

// New creates a new server.
func New(addr string, secret []byte, handler Handler, logger *slog.Logger) *Server {
	return &Server{
		addr:       addr,
		secret:     secret,
		handler:    handler,
		logger:     logger,
		clients:    make(map[*session.Session]bool),
		clientSeq:  make(map[string]uint64),
	}
}

// SetOnEstablished registers the per-session callback. Safe to call before Start.
func (s *Server) SetOnEstablished(f OnEstablished) {
	s.onEstablished = f
}

// Port returns the actual listening port. Valid after Start returns.
func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

// Start begins listening. Returns when ctx is cancelled or the listener is
// closed via Close.
func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	s.mu.Lock()
	s.ln = ln
	s.port, _ = portFromAddr(ln.Addr().String())
	s.mu.Unlock()
	s.logger.Info("server listening", "addr", ln.Addr().String())

	defer func() {
		s.mu.Lock()
		s.ln = nil
		s.mu.Unlock()
		ln.Close()
	}()

	var wg sync.WaitGroup
	defer func() {
		wg.Wait()
		s.mu.Lock()
		for cl := range s.clients {
			cl.Close()
		}
		s.mu.Unlock()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			// Shutdown path: listener closed or ctx cancelled.
			select {
			case <-ctx.Done():
				return nil
			default:
				// Accept error for another reason (e.g. listener closed by
				// Close); report it rather than looping.
				return fmt.Errorf("accept: %w", err)
			}
		}

		// Enable TCP keepalive
		if tc, ok := conn.(*net.TCPConn); ok {
			tc.SetKeepAlive(true)
			tc.SetKeepAlivePeriod(15 * time.Second)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.handleConn(ctx, conn); err != nil {
				s.logger.Error("handle conn", "err", err)
			}
		}()
	}
}

// Close stops the listener, unblocking Accept() so Start returns. It is safe to
// call multiple times and from another goroutine (e.g. a signal handler).
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln == nil {
		return nil
	}
	err := s.ln.Close()
	s.ln = nil
	return err
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) error {
	defer conn.Close()

	s.logger.Info("new connection", "addr", conn.RemoteAddr())

	sess, err := session.NewServer(conn, s.secret)
	if err != nil {
		s.logger.Error("handshake", "err", err, "addr", conn.RemoteAddr())
		return err
	}

	s.mu.Lock()
	s.clients[sess] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, sess)
		s.mu.Unlock()
		sess.Close()
	}()

	s.logger.Info("session established", "addr", conn.RemoteAddr())

	if s.onEstablished != nil {
		s.onEstablished(conn.RemoteAddr().String())
	}

	// Use context for signaling shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Separate reader and writer goroutines
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); s.readLoop(ctx, sess) }()
	go func() { defer wg.Done(); s.pingLoop(ctx, sess) }()

	wg.Wait()
	return nil
}

func (s *Server) readLoop(ctx context.Context, sess *session.Session) {
	msgCount := 0


	// buf := make([]byte, 1024)
	// for {
	// 	n, err := conn.Read(buf)
	// 	buf = buf[n:]
	// }


	for {
		sess.SetReadDeadline(time.Now().Add(45 * time.Second))

		msg, err := sess.ReadMsg()
		if err != nil {
			s.logger.Info("read error", "err", err, "addr", sess.RemoteAddr())
			return
		}

		sess.SetReadDeadline(time.Time{})
		msgCount++
		s.logger.Info("read msg", "type", msg.Type, "count", msgCount, "addr", sess.RemoteAddr())

		switch msg.Type {
		case "clip":
			if clip.IsEmptyOrTooLarge(msg.Text) {
				continue
			}
			if clip.IsSensitive(msg.Text) {
				continue
			}

		// Seq/ack: track sequence number
			s.seqMu.Lock()
			if msg.Seq > 0 {
				addr := sess.RemoteAddr()
				s.clientSeq[addr] = msg.Seq
			}
			s.seqMu.Unlock()

			if s.handler != nil {
				s.handler(msg.Text)
			}
			s.sendAck(sess, msg.Seq)
		case "ack":
			s.logger.Info("received ack", "seq", msg.Seq, "addr", sess.RemoteAddr())
		case "ping":
			if err := sess.WriteMsg(&session.Message{Type: "pong"}); err != nil {
				s.logger.Error("write pong", "err", err)
				return
			}
		}
	}
}

// sendAck sends an acknowledgment for a clip sequence number.
func (s *Server) sendAck(sess *session.Session, seq uint64) {
	if seq > 0 {
		if err := sess.WriteMsg(&session.Message{Type: "ack", Seq: seq}); err != nil {
			s.logger.Error("write ack", "err", err, "seq", seq)
		}
	}
}

func (s *Server) pingLoop(ctx context.Context, sess *session.Session) {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	pingCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCount++
			s.logger.Info("sending ping", "count", pingCount, "addr", sess.RemoteAddr())
			sess.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := sess.WriteMsg(&session.Message{Type: "ping"}); err != nil {
				s.logger.Error("ping failed", "err", err, "addr", sess.RemoteAddr())
				return
			}
			sess.SetWriteDeadline(time.Time{})
		}
	}
}

// Send sends a clip to all connected clients with seq/ack.
func (s *Server) Send(text string) error {
	s.mu.Lock()
	// Increment seq per client
	for cl := range s.clients {
		addr := cl.RemoteAddr()
		s.seqMu.Lock()
		seq := s.clientSeq[addr] + 1
		s.clientSeq[addr] = seq
		s.seqMu.Unlock()

		msg := &session.Message{Type: "clip", Text: text, Seq: seq}
		cl.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := cl.WriteMsg(msg); err != nil {
			s.logger.Error("send clip", "err", err)
			cl.Close()
		}
		cl.SetWriteDeadline(time.Time{})
	}
	s.mu.Unlock()
	return nil
}

// ClientCount returns the number of connected clients.
func (s *Server) ClientCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.clients)
}

// ClientAddrs returns the remote addrs of connected clients. An empty slice
// means no phone is connected.
func (s *Server) ClientAddrs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.clients) == 0 {
		return nil
	}
	addrs := make([]string, 0, len(s.clients))
	for cl := range s.clients {
		addrs = append(addrs, cl.RemoteAddr())
	}
	return addrs
}
