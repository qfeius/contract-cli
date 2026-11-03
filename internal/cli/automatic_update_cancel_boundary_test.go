package cli_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/cli"
	updatecheck "cn.qfei/contract-cli/internal/update"
)

func TestAutomaticUpdateCancelledResponseDoesNotSaveCacheOrDispatch(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(map[bool]string{false: "no-cache", true: "stale-cache"}[cached], func(t *testing.T) {
			fixture := newNonBusinessJSONFixture(t)
			before, err := os.ReadFile(fixture.store.Path())
			if err != nil {
				t.Fatal(err)
			}
			cachePath := filepath.Join(filepath.Dir(fixture.store.Path()), "update-check.json")
			var previous []byte
			if cached {
				if err := updatecheck.SaveCache(cachePath, updatecheck.Cache{CheckedAt: time.Unix(0, 0), Channel: "latest", CurrentVersion: "1.9.7", LatestVersion: "1.9.7"}); err != nil {
					t.Fatal(err)
				}
				previous, err = os.ReadFile(cachePath)
				if err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			fixture.options.LookupEnv = func(string) (string, bool) { return "", false }
			fixture.options.UpdateCurrentVersion = "1.9.7"
			fixture.options.UpdateRegistryURL = "https://registry.example.test/package"
			calls := 0
			fixture.options.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				response := jsonResponse(`{"dist-tags":{"latest":"1.9.8"}}`)
				response.Body = &nonBusinessBoundaryResponseBody{reader: strings.NewReader(`{"dist-tags":{"latest":"1.9.8"}}`), cancel: cancel}
				return response, nil
			})}
			code := cli.New(fixture.options).Execute(ctx, []string{"auth", "use", "--as", "app", "--profile", "contract", "--output", "json"})
			nonBusinessBoundaryAssertCancelled(t, code, fixture.stdout.Bytes(), fixture.stderr.Bytes(), "json")
			after, err := os.ReadFile(fixture.store.Path())
			if err != nil || calls != 1 || !bytes.Equal(before, after) {
				t.Fatalf("cancelled automatic update dispatched the command: calls=%d err=%v", calls, err)
			}
			current, err := os.ReadFile(cachePath)
			if !cached && !errors.Is(err, os.ErrNotExist) || cached && (err != nil || !bytes.Equal(previous, current)) {
				t.Fatalf("cancelled automatic update saved a cache: cached=%t err=%v body=%s", cached, err, current)
			}
		})
	}
}
