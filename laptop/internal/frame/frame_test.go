package frame

import (
	"crypto/aes"
	"crypto/cipher"
	"testing"
)

func TestCodecRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	block1, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher error: %v", err)
	}
	gcm1, err := cipher.NewGCM(block1)
	if err != nil {
		t.Fatalf("NewGCM error: %v", err)
	}

	block2, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher error: %v", err)
	}
	gcm2, err := cipher.NewGCM(block2)
	if err != nil {
		t.Fatalf("NewGCM error: %v", err)
	}

	// Create separate codecs for encode and decode (simulates two directions)
	// Both start with counter=0, so first frame nonces match
	encCodec := NewCodec(gcm1)
	decCodec := NewCodec(gcm2)

	plaintext := []byte("Hello, ClipConnect!")
	encoded, err := encCodec.Encode(plaintext)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	decoded, err := decCodec.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if string(decoded) != string(plaintext) {
		t.Errorf("decoded = %q, want %q", decoded, plaintext)
	}
}

func TestCodecMultipleFrames(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	block1, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher error: %v", err)
	}
	gcm1, err := cipher.NewGCM(block1)
	if err != nil {
		t.Fatalf("NewGCM error: %v", err)
	}

	block2, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher error: %v", err)
	}
	gcm2, err := cipher.NewGCM(block2)
	if err != nil {
		t.Fatalf("NewGCM error: %v", err)
	}

	// Use separate codecs for encode/decode
	encCodec := NewCodec(gcm1)
	decCodec := NewCodec(gcm2)

	// Send multiple frames
	plaintexts := [][]byte{
		[]byte("frame 1"),
		[]byte("frame 2"),
		[]byte("frame 3 with longer content"),
	}

	for i, pt := range plaintexts {
		encoded, err := encCodec.Encode(pt)
		if err != nil {
			t.Fatalf("Encode frame %d error: %v", i, err)
		}

		decoded, err := decCodec.Decode(encoded)
		if err != nil {
			t.Fatalf("Decode frame %d error: %v", i, err)
		}

		if string(decoded) != string(pt) {
			t.Errorf("frame %d: decoded = %q, want %q", i, decoded, pt)
		}
	}
}

func TestCodecLargeFrame(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	block1, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher error: %v", err)
	}
	gcm1, err := cipher.NewGCM(block1)
	if err != nil {
		t.Fatalf("NewGCM error: %v", err)
	}

	block2, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher error: %v", err)
	}
	gcm2, err := cipher.NewGCM(block2)
	if err != nil {
		t.Fatalf("NewGCM error: %v", err)
	}

	// Use separate codecs for encode/decode
	encCodec := NewCodec(gcm1)
	decCodec := NewCodec(gcm2)

	// 100KB frame
	plaintext := make([]byte, 102400)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	encoded, err := encCodec.Encode(plaintext)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	decoded, err := decCodec.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if len(decoded) != len(plaintext) {
		t.Errorf("decoded length = %d, want %d", len(decoded), len(plaintext))
	}

	for i := range plaintext {
		if decoded[i] != plaintext[i] {
			t.Errorf("byte %d: got %d, want %d", i, decoded[i], plaintext[i])
			break
		}
	}
}

func TestCodecMaxPlaintextLimit(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	block1, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher error: %v", err)
	}
	gcm1, err := cipher.NewGCM(block1)
	if err != nil {
		t.Fatalf("NewGCM error: %v", err)
	}

	block2, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher error: %v", err)
	}
	gcm2, err := cipher.NewGCM(block2)
	if err != nil {
		t.Fatalf("NewGCM error: %v", err)
	}

	// Use separate codecs for encode/decode
	encCodec := NewCodec(gcm1)
	decCodec := NewCodec(gcm2)

	// Max allowed size
	plaintext := make([]byte, MaxPlaintext)
	encoded, err := encCodec.Encode(plaintext)
	if err != nil {
		t.Fatalf("Encode max size error: %v", err)
	}

	decoded, err := decCodec.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode max size error: %v", err)
	}

	if len(decoded) != MaxPlaintext {
		t.Errorf("decoded length = %d, want %d", len(decoded), MaxPlaintext)
	}

	// Over max size
	overMax := make([]byte, MaxPlaintext+1)
	_, err = encCodec.Encode(overMax)
	if err == nil {
		t.Error("expected error for frame over MaxPlaintext, got nil")
	}
}
