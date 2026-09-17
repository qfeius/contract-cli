package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(payload string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(payload)),
	}
}

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
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/oauth/register/contract" {
				t.Fatalf("unexpected request: %s", req.URL)
			}
			return jsonResponse(`{"client_id":"client-new"}`), nil
		})},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
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
		"client_id=client-new",
		"redirect_uri=http%3A%2F%2F127.0.0.1%3A8000%2Fcallback",
		"scope=mcp%3Atools",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("authorization URL output missing %q: %s", want, output)
		}
	}
}

func TestUserAuthLoginRegistersClientEveryTime(t *testing.T) {
	for _, initialClientID := range []string{"", "client-old"} {
		for _, noOpenBrowser := range []bool{false, true} {
			t.Run(fmt.Sprintf("existing=%t/noOpenBrowser=%t", initialClientID != "", noOpenBrowser), func(t *testing.T) {
				t.Parallel()
				profile := config.Profile{
					Name: "test", ClientName: "contract-cli",
					Identities: config.Identities{User: config.UserIdentity{
						ClientID:              initialClientID,
						RegistrationEndpoint:  "https://example.test/register",
						AuthorizationEndpoint: "https://example.test/authorize",
						TokenEndpoint:         "https://example.test/token",
						RedirectURL:           "http://127.0.0.1:8000/callback",
					}},
				}
				var registrations, exchanges int
				var browserURL string
				stdout := &bytes.Buffer{}
				provider := userAuthProvider{
					logger:                 slog.New(slog.NewTextHandler(io.Discard, nil)),
					authorizationURLWriter: stdout,
					openBrowser:            func(value string) error { browserURL = value; return nil },
					httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						if req.Method != http.MethodPost {
							t.Fatalf("method = %s, want POST", req.Method)
						}
						switch req.URL.Path {
						case "/register":
							registrations++
							return jsonResponse(fmt.Sprintf(`{"client_id":"client-%d"}`, registrations)), nil
						case "/token":
							exchanges++
							if err := req.ParseForm(); err != nil {
								t.Fatal(err)
							}
							if got, want := req.Form.Get("client_id"), fmt.Sprintf("client-%d", exchanges); got != want {
								t.Fatalf("token client_id = %q, want %q", got, want)
							}
							return jsonResponse(`{"access_token":"test-token","token_type":"Bearer"}`), nil
						default:
							t.Fatalf("unexpected request: %s", req.URL)
							return nil, errors.New("unexpected request")
						}
					})},
					startCallbackServer: func(string) (authorizationCallback, error) {
						return fakeAuthorizationCallback{wait: func(_ context.Context, state string) (string, error) {
							authURL := browserURL
							if noOpenBrowser {
								authURL = strings.TrimSpace(strings.TrimPrefix(stdout.String(), "Open this URL and finish authorization:\n"))
							}
							parsed, err := url.Parse(authURL)
							if err != nil {
								t.Fatal(err)
							}
							if got, want := parsed.Query().Get("client_id"), fmt.Sprintf("client-%d", exchanges+1); got != want {
								t.Fatalf("authorization client_id = %q, want %q", got, want)
							}
							if parsed.Query().Get("state") != state {
								t.Fatal("authorization state mismatch")
							}
							return "test-code", nil
						}}, nil
					},
				}
				for attempt := 1; attempt <= 2; attempt++ {
					stdout.Reset()
					if _, err := provider.Login(context.Background(), &profile, authCommandOptions{Timeout: time.Second, NoOpenBrowser: noOpenBrowser}); err != nil {
						t.Fatal(err)
					}
					if registrations != attempt || exchanges != attempt {
						t.Fatalf("registrations=%d exchanges=%d, want %d each", registrations, exchanges, attempt)
					}
					if profile.Identities.User.ClientID != fmt.Sprintf("client-%d", attempt) {
						t.Fatalf("stored client_id = %q", profile.Identities.User.ClientID)
					}
					if profile.Identities.User.Token == nil || profile.Identities.User.Token.AccessToken != "test-token" {
						t.Fatal("login token was not updated")
					}
				}
			})
		}
	}
}

func TestUserAuthLoginRegistrationFailureDoesNotReuseClient(t *testing.T) {
	t.Parallel()
	profile := config.Profile{Identities: config.Identities{User: config.UserIdentity{
		ClientID:              "client-old",
		RegistrationEndpoint:  "https://example.test/register",
		AuthorizationEndpoint: "https://example.test/authorize",
		TokenEndpoint:         "https://example.test/token",
		RedirectURL:           "http://127.0.0.1:8000/callback",
		Token:                 &config.Token{AccessToken: "old-token"},
	}}}
	registrationErr := errors.New("registration unavailable")
	provider := userAuthProvider{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/register" {
				t.Fatalf("unexpected request: %s", req.URL)
			}
			return nil, registrationErr
		})},
		startCallbackServer: func(string) (authorizationCallback, error) {
			t.Fatal("authorization must not start after registration failure")
			return nil, nil
		},
	}
	message, err := provider.Login(context.Background(), &profile, authCommandOptions{Timeout: time.Second})
	if !errors.Is(err, registrationErr) || message != "" {
		t.Fatalf("Login() = %q, %v", message, err)
	}
	if profile.Identities.User.ClientID != "client-old" || profile.Identities.User.Token.AccessToken != "old-token" {
		t.Fatal("registration failure changed existing credentials")
	}
}

func TestUserAuthLoginPrintsAuthorizationURLBeforeAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name          string
		noOpenBrowser bool
		browserErr    error
	}{
		{name: "browser opens"},
		{name: "browser fails", browserErr: errors.New("browser unavailable")},
		{name: "manual authorization", noOpenBrowser: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			output := &bytes.Buffer{}
			browserCalls := 0
			profile := config.Profile{Name: "test", Identities: config.Identities{User: config.UserIdentity{
				RegistrationEndpoint:  "https://example.test/register",
				AuthorizationEndpoint: "https://example.test/authorize",
				TokenEndpoint:         "https://example.test/token",
				RedirectURL:           "http://127.0.0.1:8000/callback",
			}}}
			provider := userAuthProvider{
				logger:                 slog.New(slog.NewTextHandler(output, nil)),
				authorizationURLWriter: output,
				openBrowser: func(authURL string) error {
					browserCalls++
					if !strings.Contains(output.String(), authorizationURLMessage(authURL)) {
						t.Fatal("authorization URL must be printed before opening browser")
					}
					return tc.browserErr
				},
				startCallbackServer: func(string) (authorizationCallback, error) {
					return fakeAuthorizationCallback{wait: func(context.Context, string) (string, error) {
						if !strings.Contains(output.String(), "Open this URL and finish authorization:\nhttps://example.test/authorize?") {
							t.Fatal("authorization URL must be printed before waiting for callback")
						}
						return "test-code", nil
					}}, nil
				},
				httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch req.URL.Path {
					case "/register":
						return jsonResponse(`{"client_id":"client-new"}`), nil
					case "/token":
						return jsonResponse(`{"access_token":"test-token","expires_in":3600}`), nil
					default:
						t.Fatalf("unexpected request: %s", req.URL)
						return nil, errors.New("unexpected request")
					}
				})},
			}
			message, err := provider.Login(context.Background(), &profile, authCommandOptions{Timeout: time.Second, NoOpenBrowser: tc.noOpenBrowser})
			if err != nil {
				t.Fatal(err)
			}
			wantBrowserCalls := 1
			if tc.noOpenBrowser {
				wantBrowserCalls = 0
			}
			if browserCalls != wantBrowserCalls {
				t.Fatalf("browser calls = %d, want %d", browserCalls, wantBrowserCalls)
			}
			if !strings.HasPrefix(message, `Authorization succeeded for profile "test".`) || !strings.Contains(message, "Access token expires at:") {
				t.Fatalf("unexpected success message: %s", message)
			}
			if strings.Contains(message, "https://example.test/authorize") || strings.Contains(message, "Open this URL") {
				t.Fatalf("success message repeats authorization URL: %s", message)
			}
			if strings.Count(output.String(), "Open this URL and finish authorization:") != 1 {
				t.Fatalf("authorization URL must be printed exactly once: %s", output.String())
			}
			urlIndex := strings.Index(output.String(), "Open this URL")
			completedIndex := strings.Index(output.String(), "user auth login completed")
			if completedIndex < urlIndex {
				t.Fatalf("completion log appeared before authorization URL: %s", output.String())
			}
		})
	}
}
