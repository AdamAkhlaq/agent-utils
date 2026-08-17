// Package digest computes cryptographic checksums. It is named digest rather
// than hash to avoid colliding with the stdlib hash package it builds on.
package digest

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
)

// Sum hashes r with the named algorithm and returns the lowercase hex digest.
// Input is streamed through the hasher, so arbitrarily large inputs hash in
// constant memory.
func Sum(algo string, r io.Reader) (string, error) {
	var h hash.Hash
	switch algo {
	case "sha256":
		h = sha256.New()
	case "sha1":
		h = sha1.New()
	case "sha512":
		h = sha512.New()
	case "md5":
		h = md5.New()
	default:
		return "", fmt.Errorf("unknown algorithm %q (valid: sha256, sha1, sha512, md5)", algo)
	}
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("hashing input: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
