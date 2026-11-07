package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"fmt"
)

// DeriveKey derives a 32-byte AES-GCM key from the pairing secret using SHA-256.
func DeriveKey(secret []byte) []byte {
	hash := sha256.Sum256(secret)
	return hash[:]
}

// NewGCM creates an AES-256-GCM cipher.
func NewGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	return cipher.NewGCM(block)
}
