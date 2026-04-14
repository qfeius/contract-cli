package cli_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestRunWithoutArgsPrintsContractCLIUsage(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: stderr,
		Store:  config.NewStore(t.TempDir()),
	})

	if err := app.Run(context.Background(), nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "contract-cli config add [flags]") ||
		!strings.Contains(stdout.String(), "contract-cli auth login [flags]") {
		t.Fatalf("unexpected usage output: %s", stdout.String())
	}
}

func TestConfigAddAndAuthStatus(t *testing.T) {
	t.Parallel()

	testServer := newDiscoveryServer(t)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	store := config.NewStore(t.TempDir())

	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: stderr,
		Store:  store,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/.well-known/oauth-protected-resource":
					return jsonResponse(`{"resource":"https://example.test/mcp-servers","authorization_servers":["https://example.test/contract"],"scopes_supported":["mcp:tools","mcp:resources"]}`), nil
				case "/.well-known/oauth-authorization-server/contract":
					return jsonResponse(`{"issuer":"common-organization-v2","authorization_endpoint":"https://example.test/oauth/authorize/contract","token_endpoint":"https://example.test/oauth/token/contract","registration_endpoint":"https://example.test/oauth/register/contract"}`), nil
				default:
					t.Fatalf("unexpected request path: %s", req.URL.Path)
					return nil, nil
				}
			}),
		},
	})

	err := app.Run(context.Background(), []string{
		"config", "add",
		"--name", "contract-group",
		"--env", "dev",
		"--server-url", testServer.serverURL,
		"--resource-metadata-url", testServer.protectedResourceMetadataURL,
		"--redirect-url", "http://127.0.0.1:19090/callback",
	})
	if err != nil {
		t.Fatalf("config add error = %v", err)
	}

	if !strings.Contains(stdout.String(), `Profile "contract-group" saved`) {
		t.Fatalf("unexpected config add output: %s", stdout.String())
	}

	stdout.Reset()
	if err := app.Run(context.Background(), []string{"auth", "status", "--profile", "contract-group"}); err != nil {
		t.Fatalf("auth status error = %v", err)
	}
	if !strings.Contains(stdout.String(), "Identity: user") || !strings.Contains(stdout.String(), "Authorization: unauthorized") {
		t.Fatalf("unexpected auth status output: %s", stdout.String())
	}
}

func TestAuthLoginBotStoresCredentialsAndSwitchesDefaultIdentity(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	secrets := config.NewSecretsStore(dir)

	profile := config.Profile{
		Name:            "contract-group",
		Environment:     "dev",
		ServerURL:       "https://example.test/mcp-servers/contract-group",
		DefaultIdentity: config.IdentityUser,
		Identities: config.Identities{
			User: config.UserIdentity{},
		},
	}
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout:      stdout,
		Stderr:      stderr,
		Store:       store,
		Secrets:     secrets,
		LookupEnv:   func(string) (string, bool) { return "", false },
		HTTPClient:  &http.Client{},
		OpenBrowser: func(string) error { return nil },
	})

	if err := app.Run(context.Background(), []string{
		"auth", "login",
		"--profile", "contract-group",
		"--as", "bot",
		"--app-id", "cli_bot_123",
		"--app-secret", "bot-secret",
	}); err != nil {
		t.Fatalf("auth login --as bot error = %v", err)
	}

	gotProfile, err := store.GetProfile("contract-group")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if gotProfile.DefaultIdentity != config.IdentityBot {
		t.Fatalf("default identity = %q, want %q", gotProfile.DefaultIdentity, config.IdentityBot)
	}
	if gotProfile.Identities.Bot.AppID != "cli_bot_123" {
		t.Fatalf("bot app_id = %q", gotProfile.Identities.Bot.AppID)
	}
	if gotProfile.Identities.Bot.SecretRef == "" {
		t.Fatalf("expected bot secret ref to be saved")
	}
	secret, ok, err := secrets.Get(gotProfile.Identities.Bot.SecretRef)
	if err != nil {
		t.Fatalf("secrets.Get() error = %v", err)
	}
	if !ok || secret != "bot-secret" {
		t.Fatalf("stored secret mismatch: got (%q, %v)", secret, ok)
	}

	configContent, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("ReadFile(config) error = %v", err)
	}
	if strings.Contains(string(configContent), "bot-secret") {
		t.Fatalf("main config should not contain bot secret: %s", string(configContent))
	}

	if !strings.Contains(stdout.String(), "token exchange not implemented yet") {
		t.Fatalf("unexpected bot login output: %s", stdout.String())
	}
}

func TestAuthLoginBotCredentialPriority(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	secrets := config.NewSecretsStore(dir)

	profile := config.Profile{
		Name:            "contract-group",
		Environment:     "dev",
		DefaultIdentity: config.IdentityUser,
		Identities: config.Identities{
			Bot: config.BotIdentity{
				AuthMode:  config.BotAuthModeAppCredentials,
				AppID:     "local-app-id",
				SecretRef: config.BotSecretKey("contract-group"),
			},
		},
	}
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	if err := secrets.Set(config.BotSecretKey("contract-group"), "local-secret"); err != nil {
		t.Fatalf("secrets.Set() error = %v", err)
	}

	env := map[string]string{
		"CONTRACT_CLI_BOT_APP_ID":     "env-app-id",
		"CONTRACT_CLI_BOT_APP_SECRET": "env-secret",
	}
	app := cli.New(cli.Options{
		Stdout:  stdout,
		Stderr:  stderr,
		Store:   store,
		Secrets: secrets,
		LookupEnv: func(key string) (string, bool) {
			value, ok := env[key]
			return value, ok
		},
	})

	if err := app.Run(context.Background(), []string{
		"auth", "login",
		"--profile", "contract-group",
		"--as", "bot",
		"--app-id", "flag-app-id",
	}); err != nil {
		t.Fatalf("auth login --as bot error = %v", err)
	}

	gotProfile, err := store.GetProfile("contract-group")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if gotProfile.Identities.Bot.AppID != "flag-app-id" {
		t.Fatalf("bot app_id = %q, want flag-app-id", gotProfile.Identities.Bot.AppID)
	}
	secret, ok, err := secrets.Get(gotProfile.Identities.Bot.SecretRef)
	if err != nil {
		t.Fatalf("secrets.Get() error = %v", err)
	}
	if !ok || secret != "env-secret" {
		t.Fatalf("stored secret mismatch: got (%q, %v), want (env-secret, true)", secret, ok)
	}
}

func TestAuthLoginBotFallsBackToLegacyEnvVariables(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	secrets := config.NewSecretsStore(dir)

	profile := config.Profile{
		Name:            "contract-group",
		Environment:     "dev",
		DefaultIdentity: config.IdentityUser,
		Identities: config.Identities{
			Bot: config.BotIdentity{
				AuthMode:  config.BotAuthModeAppCredentials,
				AppID:     "local-app-id",
				SecretRef: config.BotSecretKey("contract-group"),
			},
		},
	}
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	env := map[string]string{
		"DEMOCLI_BOT_APP_ID":     "legacy-env-app-id",
		"DEMOCLI_BOT_APP_SECRET": "legacy-env-secret",
	}
	app := cli.New(cli.Options{
		Stdout:  stdout,
		Stderr:  stderr,
		Store:   store,
		Secrets: secrets,
		LookupEnv: func(key string) (string, bool) {
			value, ok := env[key]
			return value, ok
		},
	})

	if err := app.Run(context.Background(), []string{
		"auth", "login",
		"--profile", "contract-group",
		"--as", "bot",
	}); err != nil {
		t.Fatalf("auth login --as bot error = %v", err)
	}

	gotProfile, err := store.GetProfile("contract-group")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if gotProfile.Identities.Bot.AppID != "legacy-env-app-id" {
		t.Fatalf("bot app_id = %q, want legacy-env-app-id", gotProfile.Identities.Bot.AppID)
	}
	secret, ok, err := secrets.Get(gotProfile.Identities.Bot.SecretRef)
	if err != nil {
		t.Fatalf("secrets.Get() error = %v", err)
	}
	if !ok || secret != "legacy-env-secret" {
		t.Fatalf("stored secret mismatch: got (%q, %v), want (legacy-env-secret, true)", secret, ok)
	}
}

func TestAuthStatusBotAndAuthUse(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	secrets := config.NewSecretsStore(dir)
	profile := config.Profile{
		Name:            "contract-group",
		Environment:     "dev",
		ServerURL:       "https://example.test/mcp-servers/contract-group",
		DefaultIdentity: config.IdentityBot,
		Identities: config.Identities{
			Bot: config.BotIdentity{
				AuthMode:     config.BotAuthModeAppCredentials,
				AppID:        "bot-app-id",
				SecretRef:    config.BotSecretKey("contract-group"),
				ConfiguredAt: time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
			},
		},
	}
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	if err := secrets.Set(config.BotSecretKey("contract-group"), "bot-secret"); err != nil {
		t.Fatalf("secrets.Set() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout:    stdout,
		Stderr:    stderr,
		Store:     store,
		Secrets:   secrets,
		LookupEnv: func(string) (string, bool) { return "", false },
	})

	if err := app.Run(context.Background(), []string{"auth", "status", "--profile", "contract-group", "--as", "bot"}); err != nil {
		t.Fatalf("auth status --as bot error = %v", err)
	}
	if !strings.Contains(stdout.String(), "Identity: bot") ||
		!strings.Contains(stdout.String(), "Credential Source: secrets") ||
		!strings.Contains(stdout.String(), "Token Protocol: not_implemented") ||
		!strings.Contains(stdout.String(), "Authorization: configured") {
		t.Fatalf("unexpected bot status output: %s", stdout.String())
	}

	stdout.Reset()
	if err := app.Run(context.Background(), []string{"auth", "use", "--profile", "contract-group", "--as", "user"}); err != nil {
		t.Fatalf("auth use error = %v", err)
	}

	gotProfile, err := store.GetProfile("contract-group")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if gotProfile.DefaultIdentity != config.IdentityUser {
		t.Fatalf("default identity = %q, want %q", gotProfile.DefaultIdentity, config.IdentityUser)
	}
}

func TestAuthStatusDefaultsToUserEvenWhenDefaultIdentityIsBot(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	secrets := config.NewSecretsStore(dir)
	profile := config.Profile{
		Name:            "contract-group",
		Environment:     "dev",
		ServerURL:       "https://example.test/mcp-servers/contract-group",
		DefaultIdentity: config.IdentityBot,
		Identities: config.Identities{
			Bot: config.BotIdentity{
				AuthMode:  config.BotAuthModeAppCredentials,
				AppID:     "bot-app-id",
				SecretRef: config.BotSecretKey("contract-group"),
			},
		},
	}
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	if err := secrets.Set(config.BotSecretKey("contract-group"), "bot-secret"); err != nil {
		t.Fatalf("secrets.Set() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout:    stdout,
		Stderr:    stderr,
		Store:     store,
		Secrets:   secrets,
		LookupEnv: func(string) (string, bool) { return "", false },
	})

	if err := app.Run(context.Background(), []string{"auth", "status", "--profile", "contract-group"}); err != nil {
		t.Fatalf("auth status error = %v", err)
	}
	if !strings.Contains(stdout.String(), "\nIdentity: user\n") || strings.Contains(stdout.String(), "\nIdentity: bot\n") {
		t.Fatalf("unexpected default status identity output: %s", stdout.String())
	}
}

func TestAuthLogoutBotKeepsUserToken(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	secrets := config.NewSecretsStore(dir)
	profile := config.Profile{
		Name:            "contract-group",
		Environment:     "dev",
		DefaultIdentity: config.IdentityBot,
		Identities: config.Identities{
			User: config.UserIdentity{
				Token: &config.Token{AccessToken: "user-token"},
			},
			Bot: config.BotIdentity{
				AuthMode:  config.BotAuthModeAppCredentials,
				AppID:     "bot-app-id",
				SecretRef: config.BotSecretKey("contract-group"),
				Token:     &config.Token{AccessToken: "bot-token"},
			},
		},
	}
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	if err := secrets.Set(config.BotSecretKey("contract-group"), "bot-secret"); err != nil {
		t.Fatalf("secrets.Set() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout:    stdout,
		Stderr:    stderr,
		Store:     store,
		Secrets:   secrets,
		LookupEnv: func(string) (string, bool) { return "", false },
	})

	if err := app.Run(context.Background(), []string{"auth", "logout", "--profile", "contract-group", "--as", "bot"}); err != nil {
		t.Fatalf("auth logout --as bot error = %v", err)
	}

	gotProfile, err := store.GetProfile("contract-group")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if gotProfile.Identities.User.Token == nil || gotProfile.Identities.User.Token.AccessToken != "user-token" {
		t.Fatalf("user token should remain intact, got %+v", gotProfile.Identities.User.Token)
	}
	if gotProfile.Identities.Bot.AppID != "" || gotProfile.Identities.Bot.SecretRef != "" || gotProfile.Identities.Bot.Token != nil {
		t.Fatalf("bot identity should be cleared, got %+v", gotProfile.Identities.Bot)
	}
	_, ok, err := secrets.Get(config.BotSecretKey("contract-group"))
	if err != nil {
		t.Fatalf("secrets.Get() error = %v", err)
	}
	if ok {
		t.Fatalf("bot secret should be deleted")
	}
}

type discoveryServer struct {
	serverURL                    string
	protectedResourceMetadataURL string
}

func newDiscoveryServer(t *testing.T) discoveryServer {
	t.Helper()
	return discoveryServer{
		serverURL:                    "https://example.test/mcp-servers/contract-group",
		protectedResourceMetadataURL: "https://example.test/.well-known/oauth-protected-resource",
	}
}

func jsonResponse(payload string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(payload)),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
