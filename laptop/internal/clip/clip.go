package clip

import (
	"crypto/sha256"
	"fmt"
	"sync"
)

// Guard prevents echoing the same clip back to the sender.
// It stores SHA-256 hashes of recently applied clips.
type Guard struct {
	mu      sync.Mutex
	hashes  map[string]bool
	maxSize int
}

// NewGuard creates a new echo guard with the given max tracked clips.
func NewGuard(maxSize int) *Guard {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &Guard{
		hashes:  make(map[string]bool, maxSize),
		maxSize: maxSize,
	}
}

// Record hashes a clip and returns true if it's a duplicate (echo detected).
func (g *Guard) Record(text string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	h := sha256.Sum256([]byte(text))
	hash := fmt.Sprintf("%x", h[:])

	if g.hashes[hash] {
		return true // echo detected
	}

	// Evict oldest if at capacity (simple round-robin via map iteration).
	if len(g.hashes) >= g.maxSize {
		// Remove a random entry (Go map iteration is random, but deterministic enough).
		for k := range g.hashes {
			delete(g.hashes, k)
			break
		}
	}

	g.hashes[hash] = true
	return false
}

// Reset clears all tracked hashes.
func (g *Guard) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.hashes = make(map[string]bool, g.maxSize)
}

// IsSensitive checks if text looks like it might be a sensitive clip
// (e.g., contains a password manager hint pattern).
func IsSensitive(text string) bool {
	// Check for common password manager markers.
	sensitivePatterns := []string{
		"x-kde-passwordManagerHint",
	}
	for _, p := range sensitivePatterns {
		if len(text) > len(p) {
			for i := 0; i <= len(text)-len(p); i++ {
				if text[i:i+len(p)] == p {
					return true
				}
			}
		}
	}
	return false
}

// IsEmptyOrTooLarge checks basic clip validity.
func IsEmptyOrTooLarge(text string) bool {
	if len(text) == 0 {
		return true
	}
	const maxClip = 256 * 1024 // 256 KiB
	return len(text) > maxClip
}
