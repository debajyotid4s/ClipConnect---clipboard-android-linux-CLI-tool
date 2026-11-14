package session

import (
	"net"
	"testing"
	"time"
)

func TestSelfHandshake(t *testing.T) {
	secret := []byte("sO0ai9K4ES7GtoBe4DeQLeEbD-JuGyeGvgoY2eXcHSM")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { t.Fatal(err) }
	defer ln.Close()
	done := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil { done <- err; return }
		_, err = NewServer(c, secret)
		done <- err
	}()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil { t.Fatal(err) }
	_, err = NewClient(conn, secret)
	if err != nil {
		t.Fatalf("client handshake failed: %v", err)
	}
	select {
	case err := <-done:
		if err != nil { t.Fatalf("server handshake failed: %v", err) }
	case <-time.After(5 * time.Second):
		t.Fatal("server handshake timed out")
	}
	t.Log("self handshake OK")
}
