package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"testing"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
	"cn.qfei/contract-cli/internal/invocation"
)

func TestDeviceCredentialStoreAvailableForLocalCodexWithoutSessionVariables(t *testing.T) {
	app := localEnvironmentResetApp(t)
	seed, err := app.deviceCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if err := seed.Save("contract", credential.DeviceCredential{Token: &config.Token{AccessToken: "fixture-access"}}); err != nil {
		t.Fatal(err)
	}

	store, available, err := app.deviceCredentialStoreIfAvailable()
	if err != nil || !available || store == nil {
		t.Fatalf("local credential store unavailable: available=%v err=%v", available, err)
	}
	stored, err := store.Load("contract")
	if err != nil || stored.Token == nil || stored.Token.AccessToken != "fixture-access" {
		t.Fatalf("local credential was not found: err=%v", err)
	}
}

func TestDeviceCredentialStoreUnavailableWithoutSessionOrDesktopEvidence(t *testing.T) {
	app := localEnvironmentResetApp(t)
	app.inspectEnvironment = func(context.Context, int) invocation.Result {
		return invocation.Result{Platform: "darwin", AgentSourceType: "unknown"}
	}
	store, available, err := app.deviceCredentialStoreIfAvailable()
	if err != nil || available || store != nil {
		t.Fatalf("unknown runtime acquired a store: available=%v err=%v", available, err)
	}
}

func TestLocalConfigEndpointChangeResetsOnlySelectedProfile(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for _, sameName := range []bool{false, true} {
			source, target := validProductionProfile(), validTestProfile()
			if reverse {
				source, target = target, source
			}
			source.Environment, target.Environment = "prod", "prod"
			target.Name = "contract-other"
			if sameName {
				target.Name = source.Name
			}
			t.Run(source.OpenPlatformBaseURL+"/"+target.Name, func(t *testing.T) {
				checkLocalConfigEndpointChange(t, source, target, sameName)
			})
		}
	}
}

func checkLocalConfigEndpointChange(t *testing.T, source, target config.Profile, sameName bool) {
	t.Helper()
	app := localEnvironmentResetApp(t)
	source.Identities.User.Token = &config.Token{AccessToken: "fixture-user"}
	source.Identities.App = config.AppIdentity{AppID: "fixture-app", Token: &config.Token{AccessToken: "fixture-app-token"}}
	source.CredentialBinding = config.CredentialTargetBinding(source)
	if err := app.store.UpsertProfile(source, true); err != nil {
		t.Fatal(err)
	}
	if err := app.secrets.Set(config.AppSecretKey(source.Name), "fixture-app-secret"); err != nil {
		t.Fatal(err)
	}
	store, err := app.deviceCredentials()
	if err != nil {
		t.Fatal(err)
	}
	stored := credential.DeviceCredential{Token: &config.Token{AccessToken: "fixture-device"}, DeviceProfile: snapshotDeviceProfile(source)}
	if err := store.Save(source.Name, stored); err != nil {
		t.Fatal(err)
	}
	app.httpClient = withProductionNetworkGuard(&http.Client{Transport: productionRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != target.AuthorizationServerMetadataURL {
			t.Fatalf("unexpected metadata endpoint: %s", request.URL.String())
		}
		metadata, err := json.Marshal(map[string]string{
			"authorization_endpoint":        target.Identities.User.AuthorizationEndpoint,
			"device_authorization_endpoint": target.Identities.User.DeviceAuthorizationEndpoint,
			"token_endpoint":                target.Identities.User.TokenEndpoint,
			"revocation_endpoint":           target.Identities.User.RevocationEndpoint,
			"registration_endpoint":         target.Identities.User.RegistrationEndpoint,
		})
		if err != nil {
			t.Fatal(err)
		}
		return productionResponse(http.StatusOK, string(metadata)), nil
	})}, app.logger)
	if err := app.Run(context.Background(), []string{"config", "add", "--env", "prod", "--name", target.Name,
		"--open-platform-base-url", target.OpenPlatformBaseURL,
		"--authorization-server-metadata-url", target.AuthorizationServerMetadataURL}); err != nil {
		t.Fatal(err)
	}
	configured, err := app.store.GetProfile(target.Name)
	if err != nil || configured.Environment != "prod" || configured.OpenPlatformBaseURL != target.OpenPlatformBaseURL ||
		configured.Resource != target.Resource || configured.Identities.User.Token != nil || configured.Identities.App.AppID != "" {
		t.Fatalf("changed profile has wrong endpoints or retained identity: err=%v", err)
	}
	remaining, err := store.Load(source.Name)
	if sameName && !errors.Is(err, credential.ErrCredentialNotFound) {
		t.Fatalf("old local credential survived endpoint change: err=%v", err)
	}
	if !sameName && (err != nil || !reflect.DeepEqual(remaining, stored)) {
		t.Fatalf("independent local credential changed: err=%v", err)
	}
	_, secretFound, err := app.secrets.Get(config.AppSecretKey(source.Name))
	if err != nil || secretFound == sameName {
		t.Fatalf("old App Secret state incorrect: found=%v err=%v", secretFound, err)
	}
	if !sameName {
		preserved, err := app.store.GetProfile(source.Name)
		if err != nil || !reflect.DeepEqual(preserved, source) {
			t.Fatalf("independent profile changed: err=%v", err)
		}
	}
}

func localEnvironmentResetApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	return New(Options{
		Store: config.NewStore(dir), Secrets: config.NewSecretsStore(dir),
		Stdout: io.Discard, Stderr: io.Discard,
		LookupEnv:         func(string) (string, bool) { return "", false },
		CredentialKeyring: &localTestKeyring{values: map[string]string{}},
		InspectEnvironment: func(context.Context, int) invocation.Result {
			return invocation.Result{
				Platform: "darwin", AgentSourceType: "codex", EvidenceType: "macos_code_signature",
				Confidence: "high", RuleID: "client.codex.signed-bundle",
			}
		},
	})
}
