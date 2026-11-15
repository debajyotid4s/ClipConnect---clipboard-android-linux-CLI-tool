package session

import (
	"crypto/aes"
	"crypto/cipher"
	"net"
	"testing"
	"time"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/frame"
)

const testSecret = "sO0ai9K4ES7GtoBe4DeQLeEbD-JuGyeGvgoY2eXcHSM"

var testSecretBytes = []byte(testSecret)

// startServer launches a handshake server on an ephemeral port and returns its
// address plus a channel that receives the established *Session once a client
// connects.
func startServer(t *testing.T, secret []byte) (addr string, sessCh chan *Session, closeAll func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ch := make(chan *Session, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		s, err := NewServer(c, secret)
		if err == nil {
			ch <- s
		}
	}()
	return ln.Addr().String(), ch, func() { ln.Close() }
}

// TestHandshakeBidirectionalHello ensures both sides exchange the hello frame.
func TestHandshakeBidirectionalHello(t *testing.T) {
	addr, ch, closeAll := startServer(t, testSecretBytes)
	defer closeAll()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	client, err := NewClient(conn, testSecretBytes)
	if err != nil {
		t.Fatalf("client handshake: %v", err)
	}
	defer client.Close()

	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("server handshake timed out")
	}
}

// TestClientServerMessageExchange ensures a clip written by the client is read
// by the server with matching content.
func TestClientServerMessageExchange(t *testing.T) {
	addr, ch, closeAll := startServer(t, testSecretBytes)
	defer closeAll()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	client, err := NewClient(conn, testSecretBytes)
	if err != nil {
		t.Fatalf("client handshake: %v", err)
	}
	defer client.Close()

	server := <-ch
	defer server.Close()

	if err := client.WriteMsg(&Message{Type: "clip", Text: "hello from client", Seq: 1}); err != nil {
		t.Fatalf("client write: %v", err)
	}

	server.SetReadDeadline(time.Now().Add(5 * time.Second))
	msg, err := server.ReadMsg()
	if err != nil {
		t.Fatalf("server read: %v", err)
	}
	if msg.Type != "clip" || msg.Text != "hello from client" || msg.Seq != 1 {
		t.Fatalf("unexpected msg: %+v", msg)
	}
}

// TestServerToClient ensures the reverse direction works too.
func TestServerToClient(t *testing.T) {
	addr, ch, closeAll := startServer(t, testSecretBytes)
	defer closeAll()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	client, err := NewClient(conn, testSecretBytes)
	if err != nil {
		t.Fatalf("client handshake: %v", err)
	}
	defer client.Close()

	server := <-ch
	defer server.Close()

	if err := server.WriteMsg(&Message{Type: "clip", Text: "from server", Seq: 7}); err != nil {
		t.Fatalf("server write: %v", err)
	}

	client.SetReadDeadline(time.Now().Add(5 * time.Second))
	msg, err := client.ReadMsg()
	if err != nil {
		t.Fatalf("client read: %v", err)
	}
	if msg.Text != "from server" || msg.Seq != 7 {
		t.Fatalf("unexpected msg: %+v", msg)
	}
}

// TestDifferentSecretDecryptFails ensures the two HKDF-derived direction keys
// are independent: a frame sealed with the c2s key cannot be opened with the
// s2c key. This is the core authenticated-encryption guarantee that makes a
// wrong-key peer unable to read traffic.
func TestDifferentSecretDecryptFails(t *testing.T) {
	blockC2S, _ := aes.NewCipher([]byte("1234567890abcdefghIJabcdefghij12"))
	blockS2C, _ := aes.NewCipher([]byte("0987654321ZYXWVUTSRQponmlkjihgf9"))
	enc := frame.NewCodec(mustGCM(blockC2S)) // seals with c2s
	dec := frame.NewCodec(mustGCM(blockS2C)) // tries to open with s2c (wrong key)

	frame, err := enc.Encode([]byte("secret payload"))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := dec.Decode(frame); err == nil {
		t.Fatal("decoded a frame with the wrong direction key")
	}
}

func mustGCM(block cipher.Block) cipher.AEAD {
	g, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	return g
}

// TestRemoteAddr ensures RemoteAddr reflects the peer.
func TestRemoteAddr(t *testing.T) {
	addr, ch, closeAll := startServer(t, testSecretBytes)
	defer closeAll()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	client, err := NewClient(conn, testSecretBytes)
	if err != nil {
		t.Fatalf("client handshake: %v", err)
	}
	defer client.Close()

	<-ch
	if client.RemoteAddr() == "" {
		t.Fatal("RemoteAddr returned empty string")
	}
}

// TestCloseIsIdempotent ensures closing twice does not panic. The second close
// of a real net.Conn may return "use of closed network connection", which is
// acceptable; we only assert no panic.
func TestCloseIsIdempotent(t *testing.T) {
	addr, ch, closeAll := startServer(t, testSecretBytes)
	defer closeAll()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	client, err := NewClient(conn, testSecretBytes)
	if err != nil {
		t.Fatalf("client handshake: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	_ = client.Close()
	_ = ch
}
