package frame

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"testing"
)

// newTestCodec returns a fresh AES-GCM AEAD for codec tests.
func newTestCodec(t *testing.T) cipher.AEAD {
	t.Helper()
	block, err := aes.NewCipher(make([]byte, 32))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("new gcm: %v", err)
	}
	return gcm
}

// TestNonceFormat verifies the codec drives its nonce from an incrementing
// counter and that Encode/Decode round-trips across consecutive messages.
// (The nonce is implicit in the counter, not carried on the wire.)
func TestNonceFormat(t *testing.T) {
	enc := NewCodec(newTestCodec(t))
	dec := NewCodec(newTestCodec(t)) // independent counter starting at 0

	for i := uint64(0); i < 4; i++ {
		out, err := enc.Encode([]byte{byte(i)})
		if err != nil {
			t.Fatalf("encode %d: %v", i, err)
		}
		got, err := dec.Decode(out)
		if err != nil {
			t.Fatalf("decode %d: %v", i, err)
		}
		if len(got) != 1 || got[0] != byte(i) {
			t.Fatalf("decode %d: got %v, want [%d]", i, got, i)
		}
	}
}

// TestNonceDistinctPerMessage ensures consecutive encodes use different
// nonces, which for GCM means different ciphertext even for identical plaintext.
func TestNonceDistinctPerMessage(t *testing.T) {
	c0 := NewCodec(newTestCodec(t))
	first, err := c0.Encode([]byte("same"))
	if err != nil {
		t.Fatalf("encode 1: %v", err)
	}

	// Second codec advanced to counter==1, so it encodes "same" with a
	// different nonce than c0's first message (counter==0).
	c1 := NewCodec(newTestCodec(t))
	if _, err := c1.Encode([]byte("x")); err != nil {
		t.Fatalf("prime c1: %v", err)
	}
	second, err := c1.Encode([]byte("same"))
	if err != nil {
		t.Fatalf("encode 2: %v", err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("expected different ciphertext for different nonces")
	}
}

// TestCounterMonotonicPerCodec ensures each direction's counter advances by 1.
func TestCounterMonotonicPerCodec(t *testing.T) {
	ae := newTestCodec(t)
	c := NewCodec(ae)
	for i := uint64(0); i < 5; i++ {
		if c.counter != i {
			t.Fatalf("before encode %d: counter = %d", i, c.counter)
		}
		if _, err := c.Encode([]byte("data")); err != nil {
			t.Fatalf("encode %d: %v", i, err)
		}
	}
	if c.counter != 5 {
		t.Fatalf("counter = %d, want 5", c.counter)
	}
}

// TestDecodeCounterAdvances ensures decode also advances its own counter, even
// when decryption fails (auth failure must still consume the nonce/counter).
func TestDecodeCounterAdvances(t *testing.T) {
	enc := NewCodec(mustGCM(mustKey(0x11)))
	dec := NewCodec(mustGCM(mustKey(0x22))) // different key -> auth failure

	frame, err := enc.Encode([]byte(""))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := dec.Decode(frame); err == nil {
		t.Fatal("decode with different key should fail")
	}
	if dec.counter != 1 {
		t.Fatalf("counter = %d, want 1", dec.counter)
	}
}

// TestEncodeRejectsOversized ensures frames above MaxPlaintext are rejected.
func TestEncodeRejectsOversized(t *testing.T) {
	ae := newTestCodec(t)
	c := NewCodec(ae)
	overflow := make([]byte, MaxPlaintext+1)
	if _, err := c.Encode(overflow); err == nil {
		t.Fatal("expected error for oversized plaintext, got nil")
	}
	if _, err := c.Encode(make([]byte, MaxPlaintext)); err != nil {
		t.Fatalf("exact-max plaintext should encode: %v", err)
	}
}

// TestDecodeRejectsOversizedLength ensures a bogus large length field is
// rejected before allocation (no DoS via huge malloc).
func TestDecodeRejectsOversizedLength(t *testing.T) {
	ae := newTestCodec(t)
	d := NewCodec(ae)
	frame := make([]byte, HeaderSize)
	binary.BigEndian.PutUint32(frame, uint32(MaxPlaintext+TagSize+1))
	if _, err := d.Decode(frame); err == nil {
		t.Fatal("expected error for oversized length field, got nil")
	}
}

// TestDecodeShortHeader ensures a truncated header is rejected.
func TestDecodeShortHeader(t *testing.T) {
	ae := newTestCodec(t)
	d := NewCodec(ae)
	if _, err := d.Decode([]byte{0x00}); err == nil {
		t.Fatal("expected error for short header, got nil")
	}
}

// TestDecodeIncompleteBody ensures a header claiming more bytes than present
// is rejected.
func TestDecodeIncompleteBody(t *testing.T) {
	ae := newTestCodec(t)
	d := NewCodec(ae)
	frame := make([]byte, HeaderSize+2)
	binary.BigEndian.PutUint32(frame, uint32(100)) // claims 100 bytes body
	if _, err := d.Decode(frame); err == nil {
		t.Fatal("expected error for incomplete body, got nil")
	}
}

// TestWrongKeyDecryptFails ensures a codec with a different key cannot decrypt
// (authenticated encryption guarantees integrity).
func TestWrongKeyDecryptFails(t *testing.T) {
	enc := NewCodec(mustGCM(mustKey(0x11)))
	dec := NewCodec(mustGCM(mustKey(0x22)))

	frame, err := enc.Encode([]byte("secret payload"))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := dec.Decode(frame); err == nil {
		t.Fatal("decode with wrong key should fail, got nil")
	}
}

// mustKey returns a deterministic 32-byte AES key filled with the given byte.
func mustKey(b byte) []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = b
	}
	return k
}

func mustGCM(key []byte) cipher.AEAD {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	return g
}
