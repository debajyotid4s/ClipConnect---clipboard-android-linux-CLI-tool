package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/session"
)

const srvTestSecretStr = "sO0ai9K4ES7GtoBe4DeQLeEbD-JuGyeGvgoY2eXcHSM"

var srvTestSecret = []byte(srvTestSecretStr)

// listenServer starts a Server on an ephemeral port and runs its accept loop in
// the background until ctx is cancelled. It returns the actual listener address.
func listenServer(t *testing.T, handler Handler) (addr string, cancel func()) {
	t.Helper()
	srv := New(":0", srvTestSecret, handler, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	go srv.Start(ctx)
	t.Cleanup(cancel)

	// Wait for the server to bind and report its port.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if p := srv.Port(); p != 0 {
			return fmt.Sprintf("127.0.0.1:%d", p), cancel
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	t.Fatal("server did not report a listening port")
	return "", cancel
}

// connectClient dials addr and completes the CCN1 handshake, returning the
// ready client session.
func connectClient(t *testing.T, addr string) *session.Session {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	sess, err := session.NewClient(conn, srvTestSecret)
	if err != nil {
		conn.Close()
		t.Fatalf("client handshake: %v", err)
	}
	return sess
}

func TestReadLoopAppliesClipOnce(t *testing.T) {
	var mu sync.Mutex
	var applied int
	var appliedText string
	addr, cancel := listenServer(t, func(text string) {
		mu.Lock()
		defer mu.Unlock()
		applied++
		appliedText = text
	})
	defer cancel()

	sess := connectClient(t, addr)
	defer sess.Close()

	// Send the same clip twice; the server dedups by hash and should apply once.
	if err := sess.WriteMsg(&session.Message{Type: "clip", Text: "duplicate-me", Seq: 1}); err != nil {
		t.Fatalf("write clip 1: %v", err)
	}
	if err := sess.WriteMsg(&session.Message{Type: "clip", Text: "duplicate-me", Seq: 2}); err != nil {
		t.Fatalf("write clip 2: %v", err)
	}

	// Wait for the handler to run exactly once.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := applied
		mu.Unlock()
		if n == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond) // allow any erroneous second call

	mu.Lock()
	defer mu.Unlock()
	if applied != 1 {
		t.Errorf("handler called %d times, want 1 (dedup)", applied)
	}
	if appliedText != "duplicate-me" {
		t.Errorf("applied text = %q, want %q", appliedText, "duplicate-me")
	}
}

func TestReadLoopDistinctClipsAllApplied(t *testing.T) {
	var mu sync.Mutex
	var count int
	addr, cancel := listenServer(t, func(string) {
		mu.Lock()
		count++
		mu.Unlock()
	})
	defer cancel()

	sess := connectClient(t, addr)
	defer sess.Close()

	for i := 0; i < 5; i++ {
		text := fmt.Sprintf("unique-%d", i)
		if err := sess.WriteMsg(&session.Message{Type: "clip", Text: text, Seq: uint64(i)}); err != nil {
			t.Fatalf("write clip %d: %v", i, err)
		}
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := count
		mu.Unlock()
		if n == 5 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if count != 5 {
		t.Errorf("handler called %d times, want 5", count)
	}
}

func TestReadLoopEmptyClipIgnored(t *testing.T) {
	var mu sync.Mutex
	var count int
	addr, cancel := listenServer(t, func(string) {
		mu.Lock()
		count++
		mu.Unlock()
	})
	defer cancel()

	sess := connectClient(t, addr)
	defer sess.Close()

	// Empty and whitespace-only clips are filtered by the server before applying.
	if err := sess.WriteMsg(&session.Message{Type: "clip", Text: ""}); err != nil {
		t.Fatalf("write empty: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if count != 0 {
		t.Errorf("empty clip should be ignored, handler called %d times", count)
	}
}

func TestClientCount(t *testing.T) {
	srv := New(":0", srvTestSecret, func(string) {}, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	go srv.Start(ctx)
	defer cancel()

	addr := waitPort(t, srv)
	c1 := connectClient(t, addr)
	defer c1.Close()
	c2 := connectClient(t, addr)
	defer c2.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := srv.ClientCount(); got == 2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := srv.ClientCount(); got != 2 {
		t.Errorf("client count = %d, want 2", got)
	}
}

// waitPort blocks until the server reports a bound port, then returns the dialable address.
func waitPort(t *testing.T, srv *Server) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if p := srv.Port(); p != 0 {
			return fmt.Sprintf("127.0.0.1:%d", p)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("server did not report a listening port")
	return ""
}
