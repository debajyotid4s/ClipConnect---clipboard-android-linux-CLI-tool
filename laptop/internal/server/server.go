package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

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

// New creates a new server.
func New(addr string, secret []byte, handler Handler, logger *slog.Logger) *Server {
	return &Server{
		addr:           addr,
		secret:         secret,
		handler:        handler,
		logger:         logger,
		clients:        make(map[*session.Session]bool),
		clientSeq:      make(map[string]uint64),
		ackedSeq:       make(map[string]uint64),
		outbox:         make(map[string]*phoneOutbox),
		outboxGrace:    15 * time.Minute,
		reaperInterval: 1 * time.Minute,
		stopReaper:     make(chan struct{}),
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
		s.Stop()
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

	// Start the outbox reaper so orphaned outboxes (phones that disconnected and
	// never came back, or came back under a fresh id) are eventually freed.
	go s.reaperLoop()

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

// handleConn accepts one connection: handshake, outbox replay, then read/ping.
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

	// Replay any durable clips that were not acknowledged before the phone
	// disconnected. The id is the per-connection id the server minted on connect,
	// so a reconnected phone (new id) cannot claim another phone's queue.
	if cid := sess.ConnID(); cid != "" {
		if clips := s.takeOutbox(cid); len(clips) > 0 {
			s.logger.Info("replaying buffered clips", "count", len(clips), "cid", cid)
			for _, c := range clips {
				if err := sess.WriteMsg(&session.Message{Type: "clip", Text: c.text, Seq: c.seq}); err != nil {
					s.logger.Error("replay clip", "err", err, "cid", cid)
					return err
				}
			}
		}
	}

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
