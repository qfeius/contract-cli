package cli

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
)

func TestConfigAddRejectsBluePresetBeforeNetworkRequest(t *testing.T) {
	requestCount := 0
	app := New(Options{
		Store: config.NewStore(t.TempDir()),
		HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(*http.Request) (*http.Response, error) {
			requestCount++
			return nil, errors.New("network request must not run")
		})},
	})

	err := app.Run(context.Background(), []string{"config", "add", "--env", "blue"})
	if err == nil || err.Error() != `unsupported environment "blue"; supported environments: prod, test` {
		t.Fatalf("config add Blue error = %v", err)
	}
	if requestCount != 0 {
		t.Fatalf("network request count = %d, want 0", requestCount)
	}
}

func TestSelfConsistentProfilesUseDeclaredAddressesRegardlessOfLabel(t *testing.T) {
	for _, environment := range []string{"dev", "blue", "custom"} {
		t.Run(environment, func(t *testing.T) {
			profile := legacyNonProductionProfile(t, environment, "https://configured-api.example", "https://configured-account.example")
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(profile, true); err != nil {
				t.Fatal(err)
			}
			app := New(Options{
				Store: store, CredentialStore: &deviceMemoryCredentialStore{values: map[string]credential.DeviceCredential{}},
				HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(*http.Request) (*http.Response, error) {
					t.Fatal("loading the profile must not send a request")
					return nil, errors.New("unexpected request")
				})},
			})
			loaded, err := app.loadDeviceAwareProfile(profile.Name)
			if err != nil || loaded.Environment != environment || loaded.OpenPlatformBaseURL != profile.OpenPlatformBaseURL {
				t.Fatalf("self-consistent profile rejected or rerouted: %v", err)
			}
		})
	}
}

func TestSelfConsistentCustomDeviceSnapshotCanBeRecovered(t *testing.T) {
	profile := legacyNonProductionProfile(t, "blue", "https://open-b.qfei.cn", "https://myaccount-b.qfei.cn")
	store := config.NewStore(t.TempDir())
	workspace := t.TempDir()
	app := New(Options{
		Store: store, LookupEnv: func(name string) (string, bool) { return workspace, name == "SKILL_SESSION_WORKSPACE" },
		CredentialStore: &deviceMemoryCredentialStore{values: map[string]credential.DeviceCredential{
			profile.Name: {DeviceProfile: snapshotDeviceProfile(profile)},
		}},
		HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("recovering the snapshot must not send a request")
			return nil, errors.New("unexpected request")
		})},
	})
	loaded, err := app.loadDeviceAwareProfile(profile.Name)
	if err != nil || loaded.Environment != profile.Environment || loaded.Resource != profile.Resource ||
		loaded.Identities.User.TokenEndpoint != profile.Identities.User.TokenEndpoint {
		t.Fatalf("self-consistent snapshot rejected or rerouted: %v", err)
	}
	if _, found, err := store.LookupProfile(profile.Name); err != nil || !found {
		t.Fatalf("recovered profile not persisted: found=%v error=%v", found, err)
	}
}
