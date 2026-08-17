package generate

import (
	"crypto/rand"
	"fmt"
)

// UUID returns a random (version 4) UUID per RFC 4122: 16 random bytes with
// six bits pinned, formatted as the canonical 8-4-4-4-12 hex string.
func UUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	b[6] = b[6]&0x0f | 0x40 // version 4 in the top nibble of byte 6
	b[8] = b[8]&0x3f | 0x80 // RFC 4122 variant: top two bits of byte 8 are 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
