package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

func NewCodeVerifier(reader io.Reader) (string, error) {
	if reader == nil {
		reader = rand.Reader
	}

	buf := make([]byte, 32)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return "", fmt.Errorf("read random verifier bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func NewState(reader io.Reader) (string, error) {
	return NewCodeVerifier(reader)
}

func S256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
