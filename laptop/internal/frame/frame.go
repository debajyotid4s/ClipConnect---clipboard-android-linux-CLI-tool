package frame

import (
	"crypto/cipher"
	"encoding/binary"
	"fmt"
)

const (
	HeaderSize = 4
	MaxPlaintext = 1024 * 1024
	TagSize = 16
	NonceSize = 12
)

type Codec struct {
	ae      cipher.AEAD
	counter uint64
}

func NewCodec(ae cipher.AEAD) *Codec {
	return &Codec{ae: ae}
}

func (c *Codec) Encode(plaintext []byte) ([]byte, error) {
	if len(plaintext) > MaxPlaintext {
		return nil, fmt.Errorf("plaintext too large: %d bytes", len(plaintext))
	}

	nonce := make([]byte, NonceSize)
	binary.BigEndian.PutUint64(nonce[4:], c.counter)
	c.counter++

	// Seal returns new slice, don't pass pre-allocated output
	ciphertext := c.ae.Seal(nil, nonce, plaintext, nil)

	length := uint32(len(ciphertext))
	result := make([]byte, HeaderSize+len(ciphertext))
	binary.BigEndian.PutUint32(result, length)
	copy(result[HeaderSize:], ciphertext)
	return result, nil
}

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
