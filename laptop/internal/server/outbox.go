package server

import "time"

// phoneOutbox holds unacknowledged clips for one logical connection (keyed by its
// server-minted per-connection id), oldest first.
type phoneOutbox struct {
	phonestring string
	clips       []outboxClip
	lastSeen    time.Time
}

// outboxClip is one pending laptop->phone clip awaiting acknowledgement.
type outboxClip struct {
	seq  uint64
	text string
}

// MaxOutboxForPhone caps the durable outbox per connection so a misbehaving or
// stalled client cannot grow server memory without bound.
const MaxOutboxForPhone = 200

// recordOutbox tracks a laptop->phone clip that is awaiting acknowledgement so
// it can be replayed if the connection drops before the ack arrives. Bounded to
// prevent unbounded memory growth if a phone stays connected without acking.
func (s *Server) recordOutbox(phonestring string, seq uint64, text string) {
	if phonestring == "" {
		return
	}
	s.outboxMu.Lock()
	defer s.outboxMu.Unlock()
	bo := s.outbox[phonestring]
	if bo == nil {
		bo = &phoneOutbox{phonestring: phonestring}
		s.outbox[phonestring] = bo
	}
	bo.clips = append(bo.clips, outboxClip{seq: seq, text: text})
	bo.lastSeen = time.Now()
	if len(bo.clips) > MaxOutboxForPhone {
		bo.clips = bo.clips[len(bo.clips)-MaxOutboxForPhone:]
	}
}

// clearOutboxForSeq removes all queued clips with seq <= ackSeq (they have been
// delivered and acknowledged). Called when an ack arrives.
func (s *Server) clearOutboxForSeq(phonestring string, ackSeq uint64) {
	if phonestring == "" {
		return
	}
	s.outboxMu.Lock()
	defer s.outboxMu.Unlock()
	bo := s.outbox[phonestring]
	if bo == nil || len(bo.clips) == 0 {
		return
	}
	i := 0
	for i < len(bo.clips) && bo.clips[i].seq <= ackSeq {
		i++
	}
	if i > 0 {
		bo.clips = bo.clips[i:]
		if len(bo.clips) == 0 {
			delete(s.outbox, phonestring)
		}
	}
}

// takeOutbox returns (and clears) the durable outbox for a connection so it can
// be replayed to a freshly reconnected session. Returns nil if there is nothing
// to replay.
func (s *Server) takeOutbox(phonestring string) []outboxClip {
	s.outboxMu.Lock()
	defer s.outboxMu.Unlock()
	bo := s.outbox[phonestring]
	if bo == nil {
		return nil
	}
	if len(bo.clips) == 0 {
		delete(s.outbox, phonestring)
		return nil
	}
	clips := bo.clips
	s.outbox[phonestring] = &phoneOutbox{phonestring: phonestring}
	return clips
}

// pruneOutbox drops outboxes whose phone has been disconnected longer than the
// grace window. Called on connect and by the reaper so a stale phone's queue is
// eventually freed. Returns the number of outboxes pruned.
func (s *Server) pruneOutbox(now time.Time) int {
	s.outboxMu.Lock()
	defer s.outboxMu.Unlock()
	n := 0
	for id, bo := range s.outbox {
		if now.Sub(bo.lastSeen) > s.outboxGrace {
			delete(s.outbox, id)
			n++
		}
	}
	return n
}

// reaperLoop periodically prunes orphaned outboxes so a phone that disconnects
// (or reconnects under a fresh id) cannot cause unbounded memory growth. The
// reaper fires on a fixed interval and drops any outbox whose most recent clip
// is older than the grace window.
func (s *Server) reaperLoop() {
	ticker := time.NewTicker(s.reaperInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopReaper:
			return
		case <-ticker.C:
			n := s.pruneOutbox(time.Now())
			if n > 0 {
				s.logger.Info("reaped stale outboxes", "count", n)
			}
		}
	}
}

// Stop tears down the server and signals the reaper goroutine to exit.
func (s *Server) Stop() {
	select {
	case <-s.stopReaper:
		// already stopped
	default:
		close(s.stopReaper)
	}
}

// OutboxLenForTest returns the number of buffered clips for a connection. Test
// helper only.
func (s *Server) OutboxLenForTest(pid string) int {
	s.outboxMu.Lock()
	defer s.outboxMu.Unlock()
	bo := s.outbox[pid]
	if bo == nil {
		return 0
	}
	return len(bo.clips)
}

// SetOutboxGraceForTest overrides the outbox grace window. Test helper only.
func (s *Server) SetOutboxGraceForTest(d time.Duration) {
	s.outboxMu.Lock()
	s.outboxGrace = d
	s.outboxMu.Unlock()
}

// SetReaperIntervalForTest overrides the reaper poll interval. Test helper only.
func (s *Server) SetReaperIntervalForTest(d time.Duration) {
	s.outboxMu.Lock()
	s.reaperInterval = d
	s.outboxMu.Unlock()
}
