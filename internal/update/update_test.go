package update

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckFindsNewBetaVersion(t *testing.T) {
	t.Parallel()

	result, err := Check(context.Background(), Options{
		HTTPClient:     jsonHTTPClient(`{"dist-tags":{"latest":"0.1.0","beta":"0.1.0-beta.2"}}`),
		RegistryURL:    "https://registry.test/@qfeius%2fcontract-cli",
		PackageName:    "@qfeius/contract-cli",
		CurrentVersion: "0.1.0-beta.1",
		Channel:        "beta",
		Now:            fixedNow,
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if !result.UpdateAvailable {
		t.Fatalf("UpdateAvailable = false, want true")
	}
	if result.LatestVersion != "0.1.0-beta.2" {
		t.Fatalf("LatestVersion = %q, want 0.1.0-beta.2", result.LatestVersion)
	}
	if result.InstallCommand != "npm install -g @qfeius/contract-cli@beta --registry https://registry.npmjs.org" {
		t.Fatalf("InstallCommand = %q", result.InstallCommand)
	}
}

func TestCheckReportsUpToDate(t *testing.T) {
	t.Parallel()

	result, err := Check(context.Background(), Options{
		HTTPClient:     jsonHTTPClient(`{"dist-tags":{"beta":"0.1.0-beta.1"}}`),
		RegistryURL:    "https://registry.test/@qfeius%2fcontract-cli",
		CurrentVersion: "0.1.0-beta.1",
		Channel:        "beta",
		Now:            fixedNow,
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.UpdateAvailable {
		t.Fatalf("UpdateAvailable = true, want false")
	}
}

func TestCheckSkipsNonSemanticCurrentVersion(t *testing.T) {
	t.Parallel()

	result, err := Check(context.Background(), Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatalf("non-semver current version should not hit registry")
				return nil, nil
			}),
		},
		RegistryURL:    "https://registry.test/@qfeius%2fcontract-cli",
		CurrentVersion: "b4a2091-dirty",
		Channel:        "beta",
		Now:            fixedNow,
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !result.Skipped {
		t.Fatalf("Skipped = false, want true")
	}
	if result.Reason == "" {
		t.Fatalf("Reason is empty")
	}
}

func TestCompareSemanticVersionsWithPrerelease(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "beta increments", a: "0.1.0-beta.1", b: "0.1.0-beta.2", want: -1},
		{name: "release wins prerelease", a: "0.1.0-beta.2", b: "0.1.0", want: -1},
		{name: "equal", a: "v0.1.0", b: "0.1.0", want: 0},
		{name: "major wins", a: "1.0.0", b: "0.9.9", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := CompareSemver(tt.a, tt.b)
			if err != nil {
				t.Fatalf("CompareSemver() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("CompareSemver(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCacheFreshOnlyWithinIntervalForSameVersionAndChannel(t *testing.T) {
	t.Parallel()

	cache := Cache{
		CheckedAt:      fixedNow().Add(-29 * time.Minute),
		Channel:        "beta",
		CurrentVersion: "0.1.0-beta.1",
		LatestVersion:  "0.1.0-beta.1",
	}

	if !CacheFresh(cache, fixedNow(), 30*time.Minute, "0.1.0-beta.1", "beta") {
		t.Fatalf("CacheFresh() = false, want true")
	}
	if CacheFresh(cache, fixedNow().Add(2*time.Minute), 30*time.Minute, "0.1.0-beta.1", "beta") {
		t.Fatalf("CacheFresh() after interval = true, want false")
	}
	if CacheFresh(cache, fixedNow(), 30*time.Minute, "0.1.0-beta.2", "beta") {
		t.Fatalf("CacheFresh() with different current version = true, want false")
	}
	if CacheFresh(cache, fixedNow(), 30*time.Minute, "0.1.0-beta.1", "latest") {
		t.Fatalf("CacheFresh() with different channel = true, want false")
	}
}

func TestCacheRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "update-check.json")
	cache := Cache{
		CheckedAt:       fixedNow(),
		Channel:         "beta",
		CurrentVersion:  "0.1.0-beta.1",
		LatestVersion:   "0.1.0-beta.2",
		UpdateAvailable: true,
		InstallCommand:  "npm install -g @qfeius/contract-cli@beta --registry https://registry.npmjs.org",
	}

	if err := SaveCache(path, cache); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	loaded, ok, err := LoadCache(path)
	if err != nil {
		t.Fatalf("LoadCache() error = %v", err)
	}
	if !ok {
		t.Fatalf("LoadCache() ok = false, want true")
	}
	if loaded.Channel != cache.Channel || loaded.LatestVersion != cache.LatestVersion || !loaded.UpdateAvailable {
		t.Fatalf("loaded cache = %+v, want %+v", loaded, cache)
	}

	if err := os.WriteFile(path, []byte(`{bad json`), 0o600); err != nil {
		t.Fatalf("WriteFile(bad json) error = %v", err)
	}
	if _, _, err := LoadCache(path); err == nil {
		t.Fatalf("LoadCache() with bad json error = nil, want error")
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 4, 20, 16, 0, 0, 0, time.FixedZone("CST", 8*60*60))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonHTTPClient(body string) *http.Client {
	return &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
}
