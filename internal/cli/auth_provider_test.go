package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
)

type fakeAuthorizationCallback struct {
	wait func(context.Context, string) (string, error)
}

func (f fakeAuthorizationCallback) Wait(ctx context.Context, state string) (string, error) {
	return f.wait(ctx, state)
}

func TestUserAuthLoginNoOpenBrowserPrintsAuthorizationURLBeforeWaiting(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	startedCallback := false
	provider := userAuthProvider{
		httpClient: &http.Client{},
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		openBrowser: func(string) error {
			t.Fatal("open browser should not be called when --no-open-browser is set")
			return nil
		},
		authorizationURLWriter: stdout,
		startCallbackServer: func(redirectURL string) (authorizationCallback, error) {
			startedCallback = true
			if redirectURL != "http://127.0.0.1:8000/callback" {
				t.Fatalf("redirect URL = %q", redirectURL)
			}
			return fakeAuthorizationCallback{
				wait: func(context.Context, string) (string, error) {
					if !strings.Contains(stdout.String(), "Open this URL and finish authorization:") {
						t.Fatalf("authorization URL should be printed before waiting, got: %s", stdout.String())
					}
					return "", errors.New("forced wait error")
				},
			}, nil
		},
	}
	profile := &config.Profile{
		Name:         "contract-group",
		ClientName:   "contract-cli",
		BusinessType: "contract",
		Scopes:       []string{"mcp:tools"},
		Identities: config.Identities{
			User: config.UserIdentity{
				ClientID:              "client-123",
				AuthorizationEndpoint: "https://example.test/oauth/authorize/contract",
				TokenEndpoint:         "https://example.test/oauth/token/contract",
				RegistrationEndpoint:  "https://example.test/oauth/register/contract",
				RedirectURL:           "http://127.0.0.1:8000/callback",
			},
		},
	}

	message, err := provider.Login(context.Background(), profile, authCommandOptions{
		Timeout:       time.Second,
		NoOpenBrowser: true,
	})
	if err == nil || !strings.Contains(err.Error(), "forced wait error") {
		t.Fatalf("Login() error = %v", err)
	}
	if message != "" {
		t.Fatalf("Login() message = %q, want empty on failure", message)
	}
	if !startedCallback {
		t.Fatal("callback server was not started")
	}

	output := stdout.String()
	for _, want := range []string{
		"Open this URL and finish authorization:",
		"https://example.test/oauth/authorize/contract",
		"client_id=client-123",
		"redirect_uri=http%3A%2F%2F127.0.0.1%3A8000%2Fcallback",
		"scope=mcp%3Atools",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("authorization URL output missing %q: %s", want, output)
		}
	}
}
