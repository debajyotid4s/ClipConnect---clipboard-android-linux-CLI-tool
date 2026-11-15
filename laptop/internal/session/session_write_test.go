package session

import (
	"crypto/aes"
	"crypto/cipher"
	"net"
	"sync"
	"testing"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/frame"
)

func TestWriteMsgSerializesConcurrentWriters(t *testing.T) {
	block, err := aes.NewCipher(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	ae, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	sender := &Session{
		conn: clientConn,
		send: frameCodecForTest(t, ae),
	}
	receiver := frameCodecForTest(t, ae)

	const messages = 100
	readErr := make(chan error, 1)
	go func() {
		for i := 0; i < messages; i++ {
			if _, err := readOneFrame(serverConn, receiver); err != nil {
				readErr <- err
				return
			}
		}
		readErr <- nil
	}()

	var writers sync.WaitGroup
	for i := 0; i < messages; i++ {
		writers.Add(1)
		go func(seq int) {
			defer writers.Done()
			if err := sender.WriteMsg(&Message{Type: "clip", Seq: uint64(seq + 1)}); err != nil {
				t.Errorf("write message %d: %v", seq, err)
			}
		}(i)
	}
	writers.Wait()

	if err := <-readErr; err != nil {
		t.Fatalf("read concurrent frames: %v", err)
	}
}

func frameCodecForTest(t *testing.T, ae cipher.AEAD) *frame.Codec {
	t.Helper()
	return frame.NewCodec(ae)
}
