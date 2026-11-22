package server

import (
	"context"
	"time"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/clip"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/session"
)

// readLoop consumes client messages for one session. It routes clips (with
// privacy-sensitive and oversize drops), tracks per-client seq, handles acks
// (which prune the outbox), and captures a follow-up hello's cid.
func (s *Server) readLoop(ctx context.Context, sess *session.Session) {
	msgCount := 0
	// cid is the per-connection identity the server minted on connect. It keys
	// the durable outbox so a clip sent before a disconnect is replayed on
	// reconnect. Because it is fresh per connection, a reconnected peer cannot
	// claim another peer's queue.
	cid := sess.ConnID()
	if cid != "" {
		s.pruneOutbox(time.Now())
	}

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
				// Privacy: never log the content. Log only that a sensitive clip
				// was dropped, so an operator can tell a privacy skip from a
				// transport error without leaking the clip.
				s.logger.Info("dropped sensitive clip", "len", len(msg.Text), "addr", sess.RemoteAddr())
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
			// The phone acknowledged delivery of all laptop->phone clips up to
			// seq. Clear the durable outbox so a replayed clip is not sent twice.
			s.ackedMu.Lock()
			if prev, ok := s.ackedSeq[cid]; ok && msg.Seq <= prev {
				s.ackedMu.Unlock()
			} else {
				s.ackedSeq[cid] = msg.Seq
				s.ackedMu.Unlock()
				s.clearOutboxForSeq(cid, msg.Seq)
			}
		case "hello":
			// A follow-up hello can carry the connection id. Capture it so the
			// outbox is keyed correctly even if the handshake hello lacked a cid.
			if msg.Cid != "" && msg.Cid != cid {
				cid = msg.Cid
				sess.SetConnID(msg.Cid)
				s.logger.Info("captured conn id from follow-up hello", "cid", msg.Cid, "addr", sess.RemoteAddr())
			}
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

// pingLoop sends periodic pings so a dead peer is detected even without traffic.
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
//
// Each clip is also recorded in the per-connection durable outbox so that, if the
// connection drops before the phone acknowledges it, the clip is replayed when
// the phone reconnects. This closes the previous fire-and-forget gap where a
// clip sent just before a disconnect was silently lost.
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
			// Record for replay even on write failure: the phone may still be
			// connected and will replay on reconnect if it never acks.
			if cid := cl.ConnID(); cid != "" {
				s.recordOutbox(cid, seq, text)
			}
			cl.Close()
			continue
		}
		cl.SetWriteDeadline(time.Time{})
		// Track for durable replay until the phone acks delivery.
		if cid := cl.ConnID(); cid != "" {
			s.recordOutbox(cid, seq, text)
		}
	}
	s.mu.Unlock()
	return nil
}
