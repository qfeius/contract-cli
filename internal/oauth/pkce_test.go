package oauth_test

import (
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/oauth"
)

func TestNewCodeVerifier(t *testing.T) {
	t.Parallel()

	verifier, err := oauth.NewCodeVerifier(strings.NewReader(strings.Repeat("a", 64)))
	if err != nil {
		t.Fatalf("NewCodeVerifier() error = %v", err)
	}
	if got := len(verifier); got < 43 || got > 128 {
		t.Fatalf("verifier length = %d, want between 43 and 128", got)
	}
}

func TestS256Challenge(t *testing.T) {
	t.Parallel()

	const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	const want = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"

	if got := oauth.S256Challenge(verifier); got != want {
		t.Fatalf("S256Challenge() = %q, want %q", got, want)
	}
}
