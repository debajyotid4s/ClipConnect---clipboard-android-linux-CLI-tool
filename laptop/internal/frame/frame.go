package frame

import (
	"crypto/cipher"
	"encoding/binary"
	"fmt"
)

const (
	// HeaderSize is the size of the length prefix.
	HeaderSize = 4
	// MaxPlaintext is the maximum plaintext size per frame (1 MiB).
	MaxPlaintext = 1024 * 1024
	// TagSize is the AES-GCM tag size.
	TagSize = 16
	// NonceSize is the GCM nonce size.
	NonceSize = 12
)

// Codec handles encryption/decryption of frames using AES-256-GCM.
type Codec struct {
	ae      cipher.AEAD
	counter uint64
}

// NewCodec creates a new frame codec.
func NewCodec(ae cipher.AEAD) *Codec {
	return &Codec{ae: ae}
}

// Encode encrypts a plaintext message and returns the wire frame.
func (c *Codec) Encode(plaintext []byte) ([]byte, error) {
	if len(plaintext) > MaxPlaintext {
		return nil, fmt.Errorf("plaintext too large: %d bytes", len(plaintext))
	}

	nonce := make([]byte, NonceSize)
	binary.BigEndian.PutUint64(nonce[4:], c.counter)
	c.counter++

	ciphertext := c.ae.Seal(nil, nonce, plaintext, nil)

	length := uint32(len(ciphertext))
	result := make([]byte, HeaderSize+len(ciphertext))
	binary.BigEndian.PutUint32(result, length)
	copy(result[HeaderSize:], ciphertext)
	return result, nil
}

// Decode decrypts a received frame. buf must contain [4-byte length ‖ ciphertext‖tag].
func (c *Codec) Decode(buf []byte) ([]byte, error) {
	if len(buf) < HeaderSize {
		return nil, fmt.Errorf("insufficient data for frame header")
	}

	length := binary.BigEndian.Uint32(buf[:HeaderSize])
	if length > uint32(MaxPlaintext+TagSize) {
		return nil, fmt.Errorf("frame too large: %d bytes", length)
	}

	frameTotal := HeaderSize + int(length)
	if len(buf) < frameTotal {
		return nil, fmt.Errorf("incomplete frame")
	}

	ciphertext := buf[HeaderSize:frameTotal]

	nonce := make([]byte, NonceSize)
	binary.BigEndian.PutUint64(nonce[4:], c.counter)
	c.counter++

	plaintext, err := c.ae.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}
