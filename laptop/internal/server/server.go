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

// Server listens for incoming connections and manages sessions.
type Server struct {
	addr    string
	secret  []byte
	handler Handler
	logger  *slog.Logger

	mu      sync.Mutex
	clients map[*session.Session]bool
}

// New creates a new server.
func New(addr string, secret []byte, handler Handler, logger *slog.Logger) *Server {
	return &Server{
		addr:    addr,
		secret:  secret,
		handler: handler,
		logger:  logger,
		clients: make(map[*session.Session]bool),
	}
}

// Start begins listening. Returns when ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()

	s.logger.Info("server listening", "addr", s.addr)

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
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				s.logger.Error("accept", "err", err)
				continue
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

	// Separate reader and writer goroutines
	done := make(chan struct{})
	go s.readLoop(ctx, sess, done)
	go s.pingLoop(ctx, sess, done)

	// Wait for reader or context to finish
	<-done
	return nil
}

func (s *Server) readLoop(ctx context.Context, sess *session.Session, done chan struct{}) {
	msgCount := 0
	for {
		sess.SetReadDeadline(time.Now().Add(20 * time.Second))

		msg, err := sess.ReadMsg()
		if err != nil {
			s.logger.Info("read error", "err", err, "addr", sess.RemoteAddr())
			close(done)
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
			if s.handler != nil {
				s.handler(msg.Text)
			}
			if err := sess.WriteMsg(&session.Message{Type: "pong"}); err != nil {
				s.logger.Error("write pong", "err", err)
				close(done)
				return
			}
		case "ping":
			if err := sess.WriteMsg(&session.Message{Type: "pong"}); err != nil {
				s.logger.Error("write pong", "err", err)
				close(done)
				return
			}
		}
	}
}

func (s *Server) pingLoop(ctx context.Context, sess *session.Session, done chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	pingCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			pingCount++
			s.logger.Info("sending ping", "count", pingCount, "addr", sess.RemoteAddr())
			sess.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := sess.WriteMsg(&session.Message{Type: "ping"}); err != nil {
				s.logger.Error("ping failed", "err", err, "addr", sess.RemoteAddr())
				close(done)
				return
			}
			sess.SetWriteDeadline(time.Time{})
		}
	}
}

// Send sends a clip to all connected clients.
func (s *Server) Send(text string) error {
	msg := &session.Message{Type: "clip", Text: text}

	s.mu.Lock()
	defer s.mu.Unlock()

	for cl := range s.clients {
		cl.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := cl.WriteMsg(msg); err != nil {
			s.logger.Error("send clip", "err", err)
			cl.Close()
		}
		cl.SetWriteDeadline(time.Time{})
	}
	return nil
}

// ClientCount returns the number of connected clients.
func (s *Server) ClientCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.clients)
}
