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
	"reflect"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
	"cn.qfei/contract-cli/internal/invocation"
)

func TestBlueTraceJSONCustomHTTPSConfiguration(t *testing.T) {
	t.Parallel()
	target := blueTraceJSONCustomTarget(t)
	app, output, _, calls := blueTraceJSONApp(t, target, nil)
	if err := app.Run(context.Background(), blueTraceJSONConfigArgs(target, true)); err != nil {
		t.Fatalf("custom HTTPS configuration must support explicit JSON: %v", err)
	}
	data := blueTraceJSONResult(t, output.Bytes())
	if *calls != 1 || data["profile"] != target.Name || data["environment"] != "prod" || data["resource"] != target.Resource || data["default_identity"] != "user" || data["authentication_reset"] != false {
		t.Fatalf("custom JSON configuration lost its endpoint or safe result: calls=%d data=%#v", *calls, data)
	}
	saved, err := app.store.GetProfile(target.Name)
	if err != nil || saved.OpenPlatformBaseURL != target.OpenPlatformBaseURL || saved.AuthorizationServerMetadataURL != target.AuthorizationServerMetadataURL || saved.Resource != target.Resource || validateProductionProfile(saved) != nil {
		t.Fatalf("custom HTTPS addresses were replaced by preset addresses: %v", err)
	}
}

func TestBlueTraceJSONTargetChangeClearsOnlyBoundCredentials(t *testing.T) {
	t.Parallel()
	target := blueTraceJSONCustomTarget(t)
	app, output, _, calls := blueTraceJSONApp(t, target, nil)
	credentials, source, other := blueTraceJSONSeedCredentials(t, app)
	otherDevice, err := credentials.Load(other.Name)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background(), blueTraceJSONConfigArgs(target, true)); err != nil {
		t.Fatalf("same-name target change must support explicit JSON: %v", err)
	}
	data := blueTraceJSONResult(t, output.Bytes())
	if *calls != 1 || data["authentication_reset"] != true || data["resource"] != target.Resource {
		t.Fatalf("JSON target change did not report its credential reset: calls=%d data=%#v", *calls, data)
	}
	saved, err := app.store.GetProfile(source.Name)
	if err != nil || saved.OpenPlatformBaseURL != target.OpenPlatformBaseURL || saved.Identities.User.Token != nil || saved.Identities.User.ClientID != "" || saved.Identities.App.AppID != "" || saved.Identities.App.Token != nil || saved.DefaultIdentity != config.IdentityUser {
		t.Fatalf("same-name target change retained old credentials or wrong target: %v", err)
	}
	if saved.CredentialBinding == source.CredentialBinding || config.ValidateCredentialBinding(saved) != nil {
		t.Fatal("changed target retained the previous credential binding")
	}
	if _, err := credentials.Load(source.Name); !errors.Is(err, credential.ErrCredentialNotFound) {
		t.Fatalf("previous local Device credential survived target change: %v", err)
	}
	if _, found, err := app.secrets.Get(config.AppSecretKey(source.Name)); err != nil || found {
		t.Fatalf("previous App Secret survived target change: found=%t error=%v", found, err)
	}
	remaining, err := app.store.GetProfile(other.Name)
	if err != nil || !reflect.DeepEqual(remaining, other) {
		t.Fatalf("independent profile changed: %v", err)
	}
	remainingDevice, err := credentials.Load(other.Name)
	if err != nil || !reflect.DeepEqual(remainingDevice, otherDevice) {
		t.Fatalf("independent Device credential changed: %v", err)
	}
	if secret, found, err := app.secrets.Get(config.AppSecretKey(other.Name)); err != nil || !found || secret != other.Name+"-fixture-secret" {
		t.Fatalf("independent App Secret changed: found=%t error=%v", found, err)
	}
}

func TestBlueTraceJSONCancelledDiscoveryPreservesBoundState(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	target := blueTraceJSONCustomTarget(t)
	app, output, _, calls := blueTraceJSONApp(t, target, cancel)
	credentials, source, other := blueTraceJSONSeedCredentials(t, app)
	beforeConfig, err := os.ReadFile(app.store.Path())
	if err != nil {
		t.Fatal(err)
	}
	beforeSecrets, err := os.ReadFile(app.secrets.Path())
	if err != nil {
		t.Fatal(err)
	}
	beforeDevice, err := credentials.Load(source.Name)
	if err != nil {
		t.Fatal(err)
	}
	beforeOtherDevice, err := credentials.Load(other.Name)
	if err != nil {
		t.Fatal(err)
	}
	// Run and the default format exist on original blue, so this also exposes
	// cancellation after discovery instead of failing merely on a new symbol.
	err = app.Run(ctx, blueTraceJSONConfigArgs(target, false))
	if !errors.Is(ctx.Err(), context.Canceled) || !errors.Is(err, context.Canceled) || *calls != 1 || output.Len() != 0 {
		t.Errorf("cancelled custom discovery continued configuration: err=%v calls=%d stdout=%s", err, *calls, output)
	}
	afterConfig, readErr := os.ReadFile(app.store.Path())
	if readErr != nil || !bytes.Equal(beforeConfig, afterConfig) {
		t.Errorf("cancelled discovery replaced the old profile or binding: %v", readErr)
	}
	afterSecrets, readErr := os.ReadFile(app.secrets.Path())
	if readErr != nil || !bytes.Equal(beforeSecrets, afterSecrets) {
		t.Errorf("cancelled discovery cleared or rewrote previous App Secrets: %v", readErr)
	}
	afterDevice, readErr := credentials.Load(source.Name)
	if readErr != nil || !reflect.DeepEqual(afterDevice, beforeDevice) {
		t.Errorf("cancelled discovery cleared the previous Device binding: %v", readErr)
	}
	afterOtherDevice, readErr := credentials.Load(other.Name)
	if readErr != nil || !reflect.DeepEqual(afterOtherDevice, beforeOtherDevice) {
		t.Errorf("cancelled discovery changed the independent Device binding: %v", readErr)
	}
}

func blueTraceJSONCustomTarget(t *testing.T) config.Profile {
	t.Helper()
	profile := legacyNonProductionProfile(t, "prod", "https://api.blue-json.example", "https://accounts.blue-json.example")
	profile.Name = "contract"
	return profile
}

func blueTraceJSONConfigArgs(target config.Profile, jsonOutput bool) []string {
	args := []string{"config", "add", "--env", "prod", "--name", target.Name,
		"--open-platform-base-url", target.OpenPlatformBaseURL,
		"--authorization-server-metadata-url", target.AuthorizationServerMetadataURL}
	if jsonOutput {
		args = append(args, "--output", "json")
	}
	return args
}

func blueTraceJSONApp(t *testing.T, target config.Profile, cancel context.CancelFunc) (*App, *bytes.Buffer, *bytes.Buffer, *int) {
	t.Helper()
	state, home := t.TempDir(), t.TempDir()
	output, diagnostic := &bytes.Buffer{}, &bytes.Buffer{}
	calls := 0
	app := New(Options{
		Store: config.NewStore(state), Secrets: config.NewSecretsStore(state),
		Stdout: output, Stderr: diagnostic, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		CredentialKeyring: &localTestKeyring{values: map[string]string{}},
		LocalUserHome:     func() (string, error) { return home, nil },
		LookupEnv:         func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
		InspectEnvironment: func(context.Context, int) invocation.Result {
			return invocation.Result{Platform: "darwin", AgentSourceType: "codex", EvidenceType: "macos_code_signature", Confidence: "high", RuleID: "client.codex.signed-bundle"}
		},
		HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			if request.Method != http.MethodGet || request.URL.String() != target.AuthorizationServerMetadataURL {
				t.Errorf("request escaped the explicit custom metadata endpoint: %s", request.URL)
			}
			body, err := json.Marshal(map[string]string{
				"authorization_endpoint":        target.Identities.User.AuthorizationEndpoint,
				"device_authorization_endpoint": target.Identities.User.DeviceAuthorizationEndpoint,
				"token_endpoint":                target.Identities.User.TokenEndpoint,
				"revocation_endpoint":           target.Identities.User.RevocationEndpoint,
				"registration_endpoint":         target.Identities.User.RegistrationEndpoint,
			})
			if err != nil {
				return nil, err
			}
			response := productionResponse(http.StatusOK, string(body))
			if cancel != nil {
				response.Body = io.NopCloser(blueTraceJSONCancelReader{Reader: bytes.NewReader(body), cancel: cancel})
			}
			return response, nil
		})},
	})
	return app, output, diagnostic, &calls
}

func blueTraceJSONSeedCredentials(t *testing.T, app *App) (credential.Store, config.Profile, config.Profile) {
	t.Helper()
	store, err := app.deviceCredentials()
	if err != nil {
		t.Fatal(err)
	}
	var profiles []config.Profile
	for _, name := range []string{"contract", "independent"} {
		profile := validProductionProfile()
		profile.Name = name
		profile.DefaultIdentity = config.IdentityApp
		profile.Identities.User.Token = &config.Token{AccessToken: name + "-fixture-user-token"}
		profile.Identities.App = config.AppIdentity{AppID: name + "-fixture-app", Token: &config.Token{AccessToken: name + "-fixture-app-token"}}
		profile.CredentialBinding = config.CredentialTargetBinding(profile)
		if err := app.store.UpsertProfile(profile, name == "contract"); err != nil {
			t.Fatal(err)
		}
		profile, err := app.store.GetProfile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := app.secrets.Set(config.AppSecretKey(name), name+"-fixture-secret"); err != nil {
			t.Fatal(err)
		}
		if err := store.Save(name, credential.DeviceCredential{Token: &config.Token{AccessToken: name + "-fixture-device-token"}, DeviceProfile: snapshotDeviceProfile(profile)}); err != nil {
			t.Fatal(err)
		}
		profiles = append(profiles, profile)
	}
	return store, profiles[0], profiles[1]
}

func blueTraceJSONResult(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var result struct {
		OK   *bool          `json:"ok"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.OK == nil || !*result.OK || result.Data == nil {
		t.Fatalf("expected one JSON success result: %s (%v)", body, err)
	}
	for _, secret := range []string{"fixture-user-token", "fixture-app-token", "fixture-device-token", "fixture-secret"} {
		if strings.Contains(string(body), secret) {
			t.Fatal("custom configuration result leaked a credential")
		}
	}
	return result.Data
}

type blueTraceJSONCancelReader struct {
	io.Reader
	cancel context.CancelFunc
}

func (reader blueTraceJSONCancelReader) Read(body []byte) (int, error) {
	count, err := reader.Reader.Read(body)
	if count > 0 {
		reader.cancel()
	}
	return count, err
}
