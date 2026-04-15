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
		!strings.Contains(stdout.String(), "contract-cli auth login [flags]") ||
		!strings.Contains(stdout.String(), "contract-cli version") ||
		!strings.Contains(stdout.String(), "contract-cli api call [flags]") ||
		!strings.Contains(stdout.String(), "contract-cli mdm vendor <subcommand> [flags]") ||
		!strings.Contains(stdout.String(), "contract-cli mdm legal <subcommand> [flags]") ||
		!strings.Contains(stdout.String(), "contract-cli mdm fields list [flags]") {
		t.Fatalf("unexpected usage output: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "contract-cli vendor <subcommand> [flags]") ||
		strings.Contains(stdout.String(), "contract-cli entity <subcommand> [flags]") ||
		strings.Contains(stdout.String(), "contract-cli schema <subcommand> [flags]") ||
		strings.Contains(stdout.String(), "contract-cli mdm-vendor <subcommand> [flags]") ||
		strings.Contains(stdout.String(), "contract-cli mdm-legal <subcommand> [flags]") ||
		strings.Contains(stdout.String(), "contract-cli mdm-fields [flags]") {
		t.Fatalf("usage should not contain legacy command names: %s", stdout.String())
	}
}

func TestVersionCommandPrintsBuildInfo(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: stderr,
		Store:  config.NewStore(t.TempDir()),
	})

	if err := app.Run(context.Background(), []string{"version"}); err != nil {
		t.Fatalf("Run(version) error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "contract-cli") {
		t.Fatalf("version output should contain binary name: %s", output)
	}
	if !strings.Contains(output, "version dev") {
		t.Fatalf("version output should contain default version: %s", output)
	}
	if !strings.Contains(output, "commit unknown") {
		t.Fatalf("version output should contain default commit: %s", output)
	}
	if !strings.Contains(output, "built unknown") {
		t.Fatalf("version output should contain default build date: %s", output)
	}
}

func TestVersionFlagPrintsBuildInfo(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: stderr,
		Store:  config.NewStore(t.TempDir()),
	})

	if err := app.Run(context.Background(), []string{"--version"}); err != nil {
		t.Fatalf("Run(--version) error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "contract-cli") || !strings.Contains(output, "version dev") {
		t.Fatalf("unexpected version flag output: %s", output)
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
		"--resource-metadata-url", testServer.protectedResourceMetadataURL,
		"--redirect-url", "http://127.0.0.1:19090/callback",
	})
	if err != nil {
		t.Fatalf("config add error = %v", err)
	}

	if !strings.Contains(stdout.String(), `Profile "contract-group" saved`) {
		t.Fatalf("unexpected config add output: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "Server URL: ") {
		t.Fatalf("config add output should not contain removed server url: %s", stdout.String())
	}
	savedProfile, err := store.GetProfile("contract-group")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if savedProfile.BotTokenEndpoint != "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal" {
		t.Fatalf("bot token endpoint = %q", savedProfile.BotTokenEndpoint)
	}
	if savedProfile.OpenPlatformBaseURL != "https://dev-open.qtech.cn" {
		t.Fatalf("open platform base url = %q", savedProfile.OpenPlatformBaseURL)
	}

	stdout.Reset()
	if err := app.Run(context.Background(), []string{"auth", "status", "--profile", "contract-group"}); err != nil {
		t.Fatalf("auth status error = %v", err)
	}
	if !strings.Contains(stdout.String(), "Identity: user") || !strings.Contains(stdout.String(), "Authorization: unauthorized") {
		t.Fatalf("unexpected auth status output: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "Server URL: ") {
		t.Fatalf("auth status output should not contain removed server url: %s", stdout.String())
	}
}

func TestConfigAddUsesPublicDevPresetByDefault(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	store := config.NewStore(t.TempDir())

	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: stderr,
		Store:  store,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.String() {
				case "https://dev-myaccount.qtech.cn/.well-known/oauth-authorization-server/contract":
					return jsonResponse(`{"issuer":"common-organization-v2","authorization_endpoint":"https://example.test/oauth/authorize/contract","token_endpoint":"https://example.test/oauth/token/contract","registration_endpoint":"https://example.test/oauth/register/contract"}`), nil
				default:
					t.Fatalf("unexpected request url: %s", req.URL.String())
					return nil, nil
				}
			}),
		},
	})

	if err := app.Run(context.Background(), []string{"config", "add", "--name", "contract-group", "--env", "dev"}); err != nil {
		t.Fatalf("config add error = %v", err)
	}

	savedProfile, err := store.GetProfile("contract-group")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if savedProfile.ProtectedResourceMetadataURL != "" {
		t.Fatalf("protected resource metadata url = %q", savedProfile.ProtectedResourceMetadataURL)
	}
	if savedProfile.AuthorizationServerMetadataURL != "https://dev-myaccount.qtech.cn/.well-known/oauth-authorization-server/contract" {
		t.Fatalf("authorization server metadata url = %q", savedProfile.AuthorizationServerMetadataURL)
	}
	if savedProfile.Resource != "http://higress-gateway.higress-system/mcp-servers" {
		t.Fatalf("resource = %q", savedProfile.Resource)
	}

	configContent, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("ReadFile(config) error = %v", err)
	}
	if strings.Contains(string(configContent), "\"server_url\"") {
		t.Fatalf("config should not persist removed server_url field: %s", string(configContent))
	}
}

func TestConfigAddRejectsRemovedServerURLFlag(t *testing.T) {
	t.Parallel()

	app := cli.New(cli.Options{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Store:  config.NewStore(t.TempDir()),
	})

	err := app.Run(context.Background(), []string{
		"config", "add",
		"--server-url", "https://example.test/mcp-servers/contract-group",
	})
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -server-url") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuthLoginBotStoresCredentialsTokenAndSwitchesDefaultIdentity(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	secrets := config.NewSecretsStore(dir)

	profile := config.Profile{
		Name:             "contract-group",
		Environment:      "dev",
		BotTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
		DefaultIdentity:  config.IdentityUser,
		Identities: config.Identities{
			User: config.UserIdentity{},
		},
	}
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout:    stdout,
		Stderr:    stderr,
		Store:     store,
		Secrets:   secrets,
		LookupEnv: func(string) (string, bool) { return "", false },
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != profile.BotTokenEndpoint {
					t.Fatalf("unexpected request url: %s", req.URL.String())
				}
				return jsonResponse(`{"code":0,"expire":7200,"msg":"ok","tenant_access_token":"bot-token"}`), nil
			}),
		},
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
	if gotProfile.Identities.Bot.Token == nil || gotProfile.Identities.Bot.Token.AccessToken != "bot-token" {
		t.Fatalf("bot token = %+v", gotProfile.Identities.Bot.Token)
	}
	if gotProfile.Identities.Bot.Token.TokenType != "Bearer" {
		t.Fatalf("bot token type = %q", gotProfile.Identities.Bot.Token.TokenType)
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

	if !strings.Contains(stdout.String(), `Bot authorization succeeded for profile "contract-group".`) {
		t.Fatalf("unexpected bot login output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Access token expires at: ") {
		t.Fatalf("missing expiry output: %s", stdout.String())
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
		Name:             "contract-group",
		Environment:      "dev",
		BotTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
		DefaultIdentity:  config.IdentityUser,
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
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(`{"code":0,"expire":7200,"msg":"ok","tenant_access_token":"bot-token"}`), nil
			}),
		},
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
		Name:             "contract-group",
		Environment:      "dev",
		BotTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
		DefaultIdentity:  config.IdentityUser,
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
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(`{"code":0,"expire":7200,"msg":"ok","tenant_access_token":"legacy-bot-token"}`), nil
			}),
		},
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

func TestAuthLoginBotReturnsErrorWhenProfileMissesTokenEndpoint(t *testing.T) {
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

	err := app.Run(context.Background(), []string{
		"auth", "login",
		"--profile", "contract-group",
		"--as", "bot",
		"--app-id", "cli_bot_123",
		"--app-secret", "bot-secret",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "run `contract-cli config add --env dev --name contract-group` first") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuthLoginBotPersistsCredentialsWhenTokenExchangeFails(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	secrets := config.NewSecretsStore(dir)

	profile := config.Profile{
		Name:             "contract-group",
		Environment:      "dev",
		BotTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
		DefaultIdentity:  config.IdentityUser,
	}
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout:  stdout,
		Stderr:  stderr,
		Store:   store,
		Secrets: secrets,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(`{"code":999,"msg":"invalid app"}`), nil
			}),
		},
		LookupEnv: func(string) (string, bool) { return "", false },
	})

	err := app.Run(context.Background(), []string{
		"auth", "login",
		"--profile", "contract-group",
		"--as", "bot",
		"--app-id", "cli_bot_123",
		"--app-secret", "bot-secret",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "invalid app") {
		t.Fatalf("unexpected error: %v", err)
	}

	gotProfile, err := store.GetProfile("contract-group")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if gotProfile.DefaultIdentity != config.IdentityUser {
		t.Fatalf("default identity = %q, want %q", gotProfile.DefaultIdentity, config.IdentityUser)
	}
	if gotProfile.Identities.Bot.AppID != "cli_bot_123" {
		t.Fatalf("bot app_id = %q", gotProfile.Identities.Bot.AppID)
	}
	if gotProfile.Identities.Bot.Token != nil {
		t.Fatalf("bot token = %+v, want nil", gotProfile.Identities.Bot.Token)
	}

	secret, ok, err := secrets.Get(config.BotSecretKey("contract-group"))
	if err != nil {
		t.Fatalf("secrets.Get() error = %v", err)
	}
	if !ok || secret != "bot-secret" {
		t.Fatalf("stored secret mismatch: got (%q, %v)", secret, ok)
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
		Name:             "contract-group",
		Environment:      "dev",
		BotTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
		DefaultIdentity:  config.IdentityBot,
		Identities: config.Identities{
			Bot: config.BotIdentity{
				AuthMode:     config.BotAuthModeAppCredentials,
				AppID:        "bot-app-id",
				SecretRef:    config.BotSecretKey("contract-group"),
				ConfiguredAt: time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
				Token: &config.Token{
					AccessToken: "bot-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(2 * time.Hour),
				},
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
		!strings.Contains(stdout.String(), "Token Protocol: tenant_access_token/internal") ||
		!strings.Contains(stdout.String(), "Token Endpoint: https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal") ||
		!strings.Contains(stdout.String(), "Authorization: authorized") {
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
		Name:             "contract-group",
		Environment:      "dev",
		BotTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
		DefaultIdentity:  config.IdentityBot,
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

func TestAuthStatusBotHandlesConfiguredExpiredAndUnconfigured(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		profile           config.Profile
		seedSecret        string
		wantAuthorization string
		wantContains      []string
		wantNotContains   []string
	}{
		{
			name: "configured",
			profile: config.Profile{
				Name:             "contract-group",
				Environment:      "dev",
				BotTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
				Identities: config.Identities{
					Bot: config.BotIdentity{
						AuthMode:  config.BotAuthModeAppCredentials,
						AppID:     "bot-app-id",
						SecretRef: config.BotSecretKey("contract-group"),
					},
				},
			},
			seedSecret:        "bot-secret",
			wantAuthorization: "Authorization: configured",
			wantContains: []string{
				"Token Protocol: tenant_access_token/internal",
			},
		},
		{
			name: "expired",
			profile: config.Profile{
				Name:             "contract-group",
				Environment:      "dev",
				BotTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
				Identities: config.Identities{
					Bot: config.BotIdentity{
						AuthMode:  config.BotAuthModeAppCredentials,
						AppID:     "bot-app-id",
						SecretRef: config.BotSecretKey("contract-group"),
						Token: &config.Token{
							AccessToken: "expired-token",
							TokenType:   "Bearer",
							Expiry:      time.Now().Add(-1 * time.Hour),
						},
					},
				},
			},
			seedSecret:        "bot-secret",
			wantAuthorization: "Authorization: expired",
			wantContains: []string{
				"Expires At: ",
			},
		},
		{
			name: "unconfigured",
			profile: config.Profile{
				Name:        "contract-group",
				Environment: "dev",
			},
			wantAuthorization: "Authorization: unconfigured",
			wantContains: []string{
				"App ID: <not-configured>",
				"App Secret: missing",
			},
			wantNotContains: []string{
				"Expires At: ",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			dir := t.TempDir()
			store := config.NewStore(dir)
			secrets := config.NewSecretsStore(dir)
			if err := store.UpsertProfile(tc.profile, true); err != nil {
				t.Fatalf("UpsertProfile() error = %v", err)
			}
			if tc.seedSecret != "" {
				if err := secrets.Set(config.BotSecretKey("contract-group"), tc.seedSecret); err != nil {
					t.Fatalf("secrets.Set() error = %v", err)
				}
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
			if !strings.Contains(stdout.String(), tc.wantAuthorization) {
				t.Fatalf("unexpected bot status output: %s", stdout.String())
			}
			for _, want := range tc.wantContains {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("missing %q in output: %s", want, stdout.String())
				}
			}
			for _, want := range tc.wantNotContains {
				if strings.Contains(stdout.String(), want) {
					t.Fatalf("unexpected %q in output: %s", want, stdout.String())
				}
			}
		})
	}
}

func TestAuthLogoutBotKeepsUserTokenAndCredentials(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	secrets := config.NewSecretsStore(dir)
	profile := config.Profile{
		Name:             "contract-group",
		Environment:      "dev",
		BotTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
		DefaultIdentity:  config.IdentityBot,
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
	if gotProfile.Identities.Bot.AppID != "bot-app-id" || gotProfile.Identities.Bot.SecretRef != config.BotSecretKey("contract-group") {
		t.Fatalf("bot credentials should remain intact, got %+v", gotProfile.Identities.Bot)
	}
	if gotProfile.Identities.Bot.Token != nil {
		t.Fatalf("bot token should be cleared, got %+v", gotProfile.Identities.Bot.Token)
	}
	if gotProfile.DefaultIdentity != config.IdentityBot {
		t.Fatalf("default identity = %q, want %q", gotProfile.DefaultIdentity, config.IdentityBot)
	}
	_, ok, err := secrets.Get(config.BotSecretKey("contract-group"))
	if err != nil {
		t.Fatalf("secrets.Get() error = %v", err)
	}
	if !ok {
		t.Fatalf("bot secret should be retained")
	}
	if !strings.Contains(stdout.String(), `Logged out bot token for profile "contract-group" while keeping app credentials.`) {
		t.Fatalf("unexpected logout output: %s", stdout.String())
	}
}

type discoveryServer struct {
	protectedResourceMetadataURL string
}

func newDiscoveryServer(t *testing.T) discoveryServer {
	t.Helper()
	return discoveryServer{
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
