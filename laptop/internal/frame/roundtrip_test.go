package frame

import (
	"crypto/aes"
	"crypto/cipher"
	"testing"

)

func TestCodecRoundtrip(t *testing.T) {
	secret := []byte("sO0ai9K4ES7GtoBe4DeQLeEbD-JuGyeGvgoY2eXcHSM")
	salt := make([]byte, 32)
	for i := range salt { salt[i] = byte(i) }
	kC2S := hkdfSha256(secret, salt, "clipconnect v1 c2s")
	kS2C := hkdfSha256(secret, salt, "clipconnect v1 s2c")
	c2sBlock, _ := aes.NewCipher(kC2S)
	s2cBlock, _ := aes.NewCipher(kS2C)
	c2s, _ := cipher.NewGCM(c2sBlock)
	s2c, _ := cipher.NewGCM(s2cBlock)
	clientSend := NewCodec(c2s) // c2s
	_ = NewCodec(s2c) // s2c
	_ = NewCodec(s2c)
	serverRecv := NewCodec(c2s) // c2s
	// client encrypts hello with c2s, server decrypts with c2s
	msg := []byte(`{"t":"hello","v":1}`)
	frame, err := clientSend.Encode(msg)
	if err != nil { t.Fatalf("encode: %v", err) }
	dec, err := serverRecv.Decode(frame)
	if err != nil { t.Fatalf("server decrypt client hello: %v", err) }
	if string(dec) != string(msg) { t.Fatalf("mismatch: %q vs %q", dec, msg) }
	t.Log("codec roundtrip OK")
}
