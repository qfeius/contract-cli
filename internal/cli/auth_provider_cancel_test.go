package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
)

func TestAppAuthProviderCancellationPreservesPreviousProfileAndSecret(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"before-request", "success-response", "business-response", "http-response", "transport"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			profile := providerCancellationProfile()
			before := providerProfileBytes(t, profile)
			secrets := config.NewSecretsStore(t.TempDir())
			if err := secrets.Set(config.AppSecretKey(profile.Name), "previous-secret"); err != nil {
				t.Fatal(err)
			}
			secretBefore, err := os.ReadFile(secrets.Path())
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if phase == "before-request" {
				cancel()
			}
			calls := 0
			provider := appAuthProvider{logger: providerCancellationLogger(), secrets: secrets,
				lookupEnv: func(string) (string, bool) { return "", false },
				httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					calls++
					if phase == "transport" {
						cancel()
						return nil, ctx.Err()
					}
					body := `{"code":0,"expire":3600,"tenant_access_token":"replacement-token"}`
					if phase == "business-response" {
						body = `{"code":999,"msg":"fixture invalid app"}`
					}
					response := providerCancelledResponse(body, cancel)
					if phase == "http-response" {
						response.StatusCode = http.StatusServiceUnavailable
					}
					return response, nil
				})},
			}
			message, err := provider.Login(ctx, &profile, authCommandOptions{AppID: "replacement-app", AppSecret: "replacement-secret"})
			var committed *appCredentialsCommittedError
			if !errors.Is(err, context.Canceled) || message != "" {
				t.Fatalf("cancelled provider returned success: message=%q error=%v", message, err)
			}
			wantCalls := 1
			if phase == "before-request" {
				wantCalls = 0
			}
			if calls != wantCalls || !bytes.Equal(before, providerProfileBytes(t, profile)) {
				t.Errorf("cancelled provider changed profile or sent work: calls=%d want=%d", calls, wantCalls)
			}
			if errors.As(err, &committed) {
				t.Error("cancellation before Secret write incorrectly required a profile save")
			}
			secretAfter, err := os.ReadFile(secrets.Path())
			if err != nil || !bytes.Equal(secretBefore, secretAfter) {
				t.Errorf("cancelled provider changed the previous Secret file: %v", err)
			}
		})
	}
}

func TestAppAuthProviderOrdinaryExchangeFailureStillSavesCredentials(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"business", "http", "transport", "request-timeout"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			profile := providerCancellationProfile()
			secrets := config.NewSecretsStore(t.TempDir())
			provider := appAuthProvider{logger: providerCancellationLogger(), secrets: secrets,
				lookupEnv: func(string) (string, bool) { return "", false },
				httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					if failure == "request-timeout" {
						return nil, context.DeadlineExceeded
					}
					if failure == "transport" {
						return nil, errors.New("fixture transport unavailable")
					}
					response := jsonResponse(`{"code":999,"msg":"fixture invalid app"}`)
					if failure == "http" {
						response.StatusCode = http.StatusServiceUnavailable
					}
					return response, nil
				})},
			}
			message, err := provider.Login(context.Background(), &profile, authCommandOptions{AppID: "replacement-app", AppSecret: "replacement-secret"})
			var committed *appCredentialsCommittedError
			if err == nil || message != "" || profile.Identities.App.AppID != "replacement-app" || profile.Identities.App.SecretRef != config.AppSecretKey(profile.Name) || profile.Identities.App.Token != nil {
				t.Fatalf("ordinary failure changed credential-saving contract: message=%q error=%v", message, err)
			}
			if !errors.As(err, &committed) {
				t.Error("saved credentials require a matching profile save after ordinary exchange failure")
			}
			secret, found, err := secrets.Get(config.AppSecretKey(profile.Name))
			if err != nil || !found || secret != "replacement-secret" || profile.DefaultIdentity != config.IdentityUser || profile.Identities.User.Token.AccessToken != "previous-user-token" {
				t.Fatalf("ordinary failure did not preserve credential/identity boundaries: found=%t error=%v", found, err)
			}
		})
	}
}

func TestAppAuthProviderSecretSaveFailureLeavesProfileUntouched(t *testing.T) {
	t.Parallel()
	profile := providerCancellationProfile()
	before := providerProfileBytes(t, profile)
	secrets := config.NewSecretsStore(t.TempDir())
	if err := os.Mkdir(secrets.Path(), 0700); err != nil {
		t.Fatal(err)
	}
	calls := 0
	provider := appAuthProvider{logger: providerCancellationLogger(), secrets: secrets,
		lookupEnv: func(string) (string, bool) { return "", false },
		httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return jsonResponse(`{"code":0,"expire":3600,"tenant_access_token":"replacement-token"}`), nil
		})},
	}
	message, err := provider.Login(context.Background(), &profile, authCommandOptions{AppID: "replacement-app", AppSecret: "replacement-secret"})
	if err == nil || !strings.Contains(err.Error(), "read secrets") || message != "" || calls != 1 || !bytes.Equal(before, providerProfileBytes(t, profile)) {
		t.Fatalf("Secret failure changed profile or did not follow the exchange: calls=%d message=%q error=%v", calls, message, err)
	}
}

func TestAppAuthProviderCancellationAfterSecretCommitKeepsMatchingProfile(t *testing.T) {
	t.Parallel()
	for _, exchange := range []string{"success", "business", "request-timeout"} {
		t.Run(exchange, func(t *testing.T) {
			t.Parallel()
			profile := providerCancellationProfile()
			secrets := config.NewSecretsStore(t.TempDir())
			if err := secrets.Set(config.AppSecretKey(profile.Name), "previous-secret"); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{ReplaceAttr: func(_ []string, attribute slog.Attr) slog.Attr {
				if attribute.Key == slog.MessageKey && attribute.Value.String() == "app credentials saved" {
					cancel()
				}
				return attribute
			}}))
			provider := appAuthProvider{logger: logger, secrets: secrets,
				lookupEnv: func(string) (string, bool) { return "", false },
				httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					if exchange == "request-timeout" {
						return nil, context.DeadlineExceeded
					}
					if exchange == "business" {
						return jsonResponse(`{"code":999,"msg":"fixture invalid app"}`), nil
					}
					return jsonResponse(`{"code":0,"tenant_access_token":"replacement-token"}`), nil
				})},
			}
			message, err := provider.Login(ctx, &profile, authCommandOptions{AppID: "replacement-app", AppSecret: "replacement-secret"})
			var committed *appCredentialsCommittedError
			if !errors.Is(err, context.Canceled) || message != "" {
				t.Errorf("late cancellation returned ordinary/success result: message=%q error=%v", message, err)
			}
			if !errors.As(err, &committed) {
				t.Error("late cancellation lost the Secret commit marker")
			}
			if exchange == "request-timeout" && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("late cancellation discarded original exchange failure: %v", err)
			}
			secret, found, readErr := secrets.Get(config.AppSecretKey(profile.Name))
			if readErr != nil || !found || secret != "replacement-secret" || profile.Identities.App.AppID != "replacement-app" || profile.Identities.App.SecretRef != config.AppSecretKey(profile.Name) || profile.DefaultIdentity != config.IdentityUser {
				t.Errorf("committed Secret has no matching profile for necessary save: found=%t error=%v", found, readErr)
			}
			if exchange == "success" {
				if profile.Identities.App.Token == nil || profile.Identities.App.Token.AccessToken != "replacement-token" {
					t.Error("late cancellation lost the completed token exchange result")
				}
			} else if profile.Identities.App.Token != nil {
				t.Error("failed token exchange retained an old token beside the new Secret")
			}
		})
	}
}

func TestAuthProviderCancellationPreservesRequestFailureCause(t *testing.T) {
	t.Parallel()
	for _, identity := range []string{"app", "user"} {
		t.Run(identity, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			requestFailure := errors.New("fixture request failure")
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				cancel()
				return nil, requestFailure
			})}
			var provider authProvider = userAuthProvider{logger: providerCancellationLogger(), httpClient: client}
			if identity == "app" {
				provider = appAuthProvider{logger: providerCancellationLogger(), httpClient: client, secrets: config.NewSecretsStore(t.TempDir()), lookupEnv: func(string) (string, bool) { return "", false }}
			}
			profile := providerCancellationProfile()
			before := providerProfileBytes(t, profile)
			message, err := provider.Login(ctx, &profile, authCommandOptions{AppID: "replacement-app", AppSecret: "replacement-secret"})
			if message != "" || !errors.Is(err, context.Canceled) || !errors.Is(err, requestFailure) || !bytes.Equal(before, providerProfileBytes(t, profile)) {
				t.Fatalf("cancellation lost the original failure or changed profile: message=%q error=%v", message, err)
			}
		})
	}
}

func TestAuthProviderCancellationAtRequestLogStopsHTTP(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ identity, message string }{
		{"app", "app token exchange started"},
		{"user", "register oauth client"},
	} {
		t.Run(test.identity, func(t *testing.T) {
			t.Parallel()
			profile := providerCancellationProfile()
			before := providerProfileBytes(t, profile)
			secrets := config.NewSecretsStore(t.TempDir())
			if err := secrets.Set(config.AppSecretKey(profile.Name), "previous-secret"); err != nil {
				t.Fatal(err)
			}
			secretBefore, err := os.ReadFile(secrets.Path())
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			triggered := false
			logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{ReplaceAttr: func(_ []string, attribute slog.Attr) slog.Attr {
				if attribute.Key == slog.MessageKey && attribute.Value.String() == test.message {
					triggered = true
					cancel()
				}
				return attribute
			}}))
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				if test.identity == "app" {
					return jsonResponse(`{"code":0,"tenant_access_token":"replacement-token"}`), nil
				}
				return jsonResponse(`{"client_id":"replacement-client"}`), nil
			})}
			var provider authProvider = userAuthProvider{logger: logger, httpClient: client}
			if test.identity == "app" {
				provider = appAuthProvider{logger: logger, httpClient: client, secrets: secrets, lookupEnv: func(string) (string, bool) { return "", false }}
			}
			message, err := provider.Login(ctx, &profile, authCommandOptions{AppID: "replacement-app", AppSecret: "replacement-secret"})
			if !triggered || !errors.Is(err, context.Canceled) || message != "" || calls != 0 || !bytes.Equal(before, providerProfileBytes(t, profile)) {
				t.Errorf("request log cancellation still dispatched HTTP or changed credentials: triggered=%t calls=%d message=%q error=%v", triggered, calls, message, err)
			}
			secretAfter, err := os.ReadFile(secrets.Path())
			if err != nil || !bytes.Equal(secretBefore, secretAfter) {
				t.Errorf("request log cancellation changed Secret state: %v", err)
			}
		})
	}
}

func TestUserAuthProviderCancellationPreservesPreviousProfile(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"before-request", "registration-response", "before-browser", "callback-response", "token-response"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			profile := providerCancellationProfile()
			before := providerProfileBytes(t, profile)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if phase == "before-request" {
				cancel()
			}
			registrations, exchanges, browsers, callbacks, waits, closes := 0, 0, 0, 0, 0, 0
			provider := userAuthProvider{logger: providerCancellationLogger(),
				httpClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					switch request.URL.Path {
					case "/register":
						registrations++
						if phase == "registration-response" {
							return providerCancelledResponse(`{"client_id":"replacement-client"}`, cancel), nil
						}
						return jsonResponse(`{"client_id":"replacement-client"}`), nil
					case "/token":
						exchanges++
						return providerCancelledResponse(`{"access_token":"replacement-token","token_type":"Bearer"}`, cancel), nil
					default:
						t.Fatalf("unexpected request %s", request.URL)
						return nil, errors.New("unexpected request")
					}
				})},
				openBrowser: func(string) error { browsers++; return nil },
				startCallbackServer: func(string) (authorizationCallback, error) {
					callbacks++
					if phase == "before-browser" {
						cancel()
					}
					return providerCancellationCallback{
						close: func() { closes++ },
						wait: func(context.Context, string) (string, error) {
							waits++
							if phase == "callback-response" {
								cancel()
							}
							return "replacement-code", nil
						},
					}, nil
				},
			}
			message, err := provider.Login(ctx, &profile, authCommandOptions{Timeout: time.Second})
			if !errors.Is(err, context.Canceled) || message != "" || !bytes.Equal(before, providerProfileBytes(t, profile)) {
				t.Errorf("cancelled user provider changed credentials or returned success: message=%q error=%v", message, err)
			}
			wantRegistrations, wantCallbacks, wantBrowsers, wantWaits, wantExchanges := 1, 1, 1, 1, 0
			switch phase {
			case "before-request":
				wantRegistrations, wantCallbacks, wantBrowsers, wantWaits = 0, 0, 0, 0
			case "registration-response":
				wantCallbacks, wantBrowsers, wantWaits = 0, 0, 0
			case "before-browser":
				wantBrowsers, wantWaits = 0, 0
			case "token-response":
				wantExchanges = 1
			}
			if registrations != wantRegistrations || callbacks != wantCallbacks || browsers != wantBrowsers || waits != wantWaits || exchanges != wantExchanges || closes != wantCallbacks {
				t.Errorf("cancelled user provider continued work or leaked callback: registrations=%d callbacks=%d browsers=%d waits=%d exchanges=%d closes=%d", registrations, callbacks, browsers, waits, exchanges, closes)
			}
		})
	}
}

type providerCancellationCallback struct {
	wait  func(context.Context, string) (string, error)
	close func()
}

func (callback providerCancellationCallback) Wait(ctx context.Context, state string) (string, error) {
	return callback.wait(ctx, state)
}

func (callback providerCancellationCallback) Close() { callback.close() }

type providerCancellationReader struct {
	io.Reader
	cancel context.CancelFunc
}

func (reader providerCancellationReader) Read(body []byte) (int, error) {
	count, err := reader.Reader.Read(body)
	if count > 0 {
		reader.cancel()
	}
	return count, err
}

func providerCancelledResponse(body string, cancel context.CancelFunc) *http.Response {
	response := jsonResponse(body)
	response.Body = io.NopCloser(providerCancellationReader{Reader: strings.NewReader(body), cancel: cancel})
	return response
}

func providerCancellationProfile() config.Profile {
	return config.Profile{
		Name: "fixture", ClientName: "contract-cli", DefaultIdentity: config.IdentityUser,
		AppTokenEndpoint: "https://example.test/app-token",
		Identities: config.Identities{
			User: config.UserIdentity{AuthMode: config.UserAuthModeDevice, ClientID: "previous-client",
				RegistrationEndpoint: "https://example.test/register", AuthorizationEndpoint: "https://example.test/authorize",
				TokenEndpoint: "https://example.test/token", RedirectURL: "http://127.0.0.1:8000/callback",
				Token: &config.Token{AccessToken: "previous-user-token"}},
			App: config.AppIdentity{AuthMode: config.AppAuthModeAppCredentials, AppID: "previous-app", SecretRef: config.AppSecretKey("fixture"),
				ConfiguredAt: time.Unix(1, 0).UTC(), Token: &config.Token{AccessToken: "previous-app-token"}},
		},
	}
}

func providerProfileBytes(t *testing.T, profile config.Profile) []byte {
	t.Helper()
	body, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func providerCancellationLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
