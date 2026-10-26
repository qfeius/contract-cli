package cli

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/config"
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
	if err == nil || err.Error() != `unsupported environment "blue"; supported environments: prod` {
		t.Fatalf("config add Blue error = %v", err)
	}
	if requestCount != 0 {
		t.Fatalf("network request count = %d, want 0", requestCount)
	}
}

func TestProductionProfileRejectsBlueProfileBeforeNetworkRequest(t *testing.T) {
	store := config.NewStore(t.TempDir())
	profile := validProductionProfile()
	profile.Environment = "blue"
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatal(err)
	}
	requestCount := 0
	app := New(Options{
		Store: store,
		HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(*http.Request) (*http.Response, error) {
			requestCount++
			return nil, errors.New("network request must not run")
		})},
	})

	_, err := app.loadDeviceAwareProfile(profile.Name)
	if err == nil || err.Error() != productionProfileErrorMessage(profile.Name) {
		t.Fatalf("Blue profile error = %v", err)
	}
	if requestCount != 0 {
		t.Fatalf("network request count = %d, want 0", requestCount)
	}
}

func TestProductionHTTPClientBlocksBlueHosts(t *testing.T) {
	requestCount := 0
	app := New(Options{
		Store: config.NewStore(t.TempDir()),
		HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(*http.Request) (*http.Response, error) {
			requestCount++
			return productionResponse(http.StatusOK, `{}`), nil
		})},
	})

	for _, rawURL := range []string{"https://open-b.qfei.cn", "https://myaccount-b.qfei.cn"} {
		request, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = app.httpClient.Do(request)
		if err == nil || !strings.Contains(err.Error(), "production build blocks non-production host") {
			t.Fatalf("production request error = %v, want Blue host blocked", err)
		}
	}
	if requestCount != 0 {
		t.Fatalf("network request count = %d, want 0", requestCount)
	}
}
