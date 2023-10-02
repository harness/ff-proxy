package hash

import (
	"crypto/sha256"
	"fmt"
	"io"
)

// Hasher an interface for generating a hash
type Hasher interface {
	Hash(string) string
}

// Sha256 is a Hasher that generates a sha256 hash
type Sha256 struct {
}

// NewSha256 returns a pointer to a Sha256
func NewSha256() *Sha256 {
	return &Sha256{}
}

// Hash takes a string and generates a sha256 checksum from it and returns the
// base16 formatting of it as a string
func (s *Sha256) Hash(v string) string {
	h := sha256.New()
	_, err := io.WriteString(h, v)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
