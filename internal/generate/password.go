package generate

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const (
	passwordAlphanumerics = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	// no quotes, backslashes, or spaces, so passwords paste safely into
	// shells and config files
	passwordSymbols = "!@#$%^&*-_=+?"
)

// Password returns a random password of the given length, each character
// drawn uniformly from letters and digits, plus symbols unless disabled.
// rand.Int is used per character because the naive byte-modulo approach
// biases towards the start of the charset (256 is not a multiple of its
// length).
func Password(length int, symbols bool) (string, error) {
	if length < 1 {
		return "", fmt.Errorf("password length must be at least 1, got %d", length)
	}
	charset := passwordAlphanumerics
	if symbols {
		charset += passwordSymbols
	}
	size := big.NewInt(int64(len(charset)))
	b := make([]byte, length)
	for i := range b {
		idx, err := rand.Int(rand.Reader, size)
		if err != nil {
			return "", fmt.Errorf("reading random bytes: %w", err)
		}
		b[i] = charset[idx.Int64()]
	}
	return string(b), nil
}
