package cli_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"cn.qfei/contract-cli/internal/build"
	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
	updatecheck "cn.qfei/contract-cli/internal/update"
)

func TestRunWithoutArgsPrintsTopLevelHelp(t *testing.T) {
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

	if !strings.Contains(stdout.String(), "Name:\n  contract-cli") ||
		!strings.Contains(stdout.String(), "Usage:") ||
		!strings.Contains(stdout.String(), "contract-cli config add [flags]") ||
		!strings.Contains(stdout.String(), "contract-cli auth login [flags]") ||
		!strings.Contains(stdout.String(), "contract-cli version") ||
		!strings.Contains(stdout.String(), "contract-cli skills list") ||
		!strings.Contains(stdout.String(), "contract-cli skills install [flags]") ||
		!strings.Contains(stdout.String(), "contract-cli update check [flags]") ||
		!strings.Contains(stdout.String(), "contract-cli environment inspect [flags]") ||
		!strings.Contains(stdout.String(), "contract-cli mdm vendor <subcommand> [flags]") ||
		!strings.Contains(stdout.String(), "contract-cli mdm legal <subcommand> [flags]") ||
		!strings.Contains(stdout.String(), "contract-cli mdm fields list [flags]") {
		t.Fatalf("unexpected usage output: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "contract-cli vendor <subcommand> [flags]") ||
		strings.Contains(stdout.String(), "contract-cli entity <subcommand> [flags]") ||
		strings.Contains(stdout.String(), "contract-cli schema <subcommand> [flags]") ||
		strings.Contains(stdout.String(), "contract-cli api call [flags]") ||
		strings.Contains(stdout.String(), "contract-cli mdm-vendor <subcommand> [flags]") ||
		strings.Contains(stdout.String(), "contract-cli mdm-legal <subcommand> [flags]") ||
		strings.Contains(stdout.String(), "contract-cli mdm-fields [flags]") {
		t.Fatalf("usage should not contain legacy command names: %s", stdout.String())
	}
}

func TestRunCommandLogRedactsSensitiveArguments(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		args    []string
		secrets []string
	}{
		{
			name:    "separate app secret",
			args:    []string{"unknown-command", "--app-secret", "cli-secret-value", "--profile", "contract"},
			secrets: []string{"cli-secret-value"},
		},
		{
			name:    "equals app secret",
			args:    []string{"unknown-command", "--app-secret=equals-secret-value"},
			secrets: []string{"equals-secret-value"},
		},
		{
			name:    "underscore app secret",
			args:    []string{"unknown-command", "--app_secret=underscore-secret-value"},
			secrets: []string{"underscore-secret-value"},
		},
		{
			name:    "authorization header",
			args:    []string{"unknown-command", "--header", "Authorization: Bearer header-token-value"},
			secrets: []string{"header-token-value"},
		},
		{
			name:    "inline request body",
			args:    []string{"unknown-command", "--data", `{"access_token":"body-token-value","password":"body-password-value"}`},
			secrets: []string{"body-token-value", "body-password-value"},
		},
		{
			name: "generic credential flags",
			args: []string{
				"unknown-command",
				"--client-secret", "client-secret-value",
				"--access-token=access-token-value",
				"--refresh-token", "refresh-token-value",
				"--token", "generic-token-value",
				"--password", "password-value",
				"--authorization", "Bearer authorization-value",
			},
			secrets: []string{
				"client-secret-value",
				"access-token-value",
				"refresh-token-value",
				"generic-token-value",
				"password-value",
				"authorization-value",
			},
		},
		{
			name:    "nested command logger",
			args:    []string{"skills", "list", "--app-secret", "nested-secret-value"},
			secrets: []string{"nested-secret-value"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stderr := &bytes.Buffer{}
			app := cli.New(cli.Options{
				Stdout: io.Discard,
				Stderr: stderr,
				Store:  config.NewStore(t.TempDir()),
				LookupEnv: func(key string) (string, bool) {
					if key == "CONTRACT_CLI_NO_UPDATE_CHECK" {
						return "1", true
					}
					return "", false
				},
			})

			if err := app.Run(context.Background(), tc.args); err == nil {
				t.Fatal("Run() error = nil, want unknown command error")
			}

			logOutput := stderr.String()
			if !strings.Contains(logOutput, "[REDACTED]") {
				t.Fatalf("log should contain redaction marker: %s", logOutput)
			}
			for _, secret := range tc.secrets {
				if strings.Contains(logOutput, secret) {
					t.Fatalf("log exposes sensitive value %q: %s", secret, logOutput)
				}
			}
		})
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

func TestUpdateCheckDefaultReportsAvailableBetaVersionAsText(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	store := config.NewStore(t.TempDir())

	app := cli.New(cli.Options{
		Stdout:               stdout,
		Stderr:               stderr,
		Store:                store,
		UpdateRegistryURL:    "https://registry.test/@qfeius%2fcontract-cli",
		UpdateCurrentVersion: "0.1.0-beta.1",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("method = %s, want GET", req.Method)
				}
				if req.URL.String() != "https://registry.test/@qfeius%2fcontract-cli" {
					t.Fatalf("unexpected update registry URL: %s", req.URL.String())
				}
				return jsonResponse(`{"dist-tags":{"latest":"0.1.0","beta":"0.1.0-beta.2"}}`), nil
			}),
		},
	})

	if err := app.Run(context.Background(), []string{"update", "check", "--channel", "beta"}); err != nil {
		t.Fatalf("update check error = %v", err)
	}

	output := stdout.String()
	for _, want := range []string{
		"Update available: contract-cli 0.1.0-beta.1 -> 0.1.0-beta.2",
		"Run: npm install -g @qfeius/contract-cli@beta --registry https://registry.npmjs.org",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout missing %q: %s", want, output)
		}
		if strings.Contains(stderr.String(), want) {
			t.Fatalf("default update check should write normal result to stdout, not stderr: %s", stderr.String())
		}
	}
	cacheContent, err := os.ReadFile(filepath.Join(filepath.Dir(store.Path()), "update-check.json"))
	if err != nil {
		t.Fatalf("ReadFile(update cache) error = %v", err)
	}
	if !strings.Contains(string(cacheContent), `"latest_version": "0.1.0-beta.2"`) {
		t.Fatalf("unexpected update cache: %s", string(cacheContent))
	}
}

func TestUpdateCheckJSONReportsAvailableBetaVersion(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	store := config.NewStore(t.TempDir())

	app := cli.New(cli.Options{
		Stdout:               stdout,
		Stderr:               stderr,
		Store:                store,
		UpdateRegistryURL:    "https://registry.test/@qfeius%2fcontract-cli",
		UpdateCurrentVersion: "0.1.0-beta.1",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("method = %s, want GET", req.Method)
				}
				if req.URL.String() != "https://registry.test/@qfeius%2fcontract-cli" {
					t.Fatalf("unexpected update registry URL: %s", req.URL.String())
				}
				return jsonResponse(`{"dist-tags":{"latest":"0.1.0","beta":"0.1.0-beta.2"}}`), nil
			}),
		},
	})

	if err := app.Run(context.Background(), []string{"update", "check", "--channel", "beta", "--json"}); err != nil {
		t.Fatalf("update check error = %v", err)
	}

	output := decodeJSONObject(t, stdout.Bytes())
	if output["ok"] != true {
		t.Fatalf("ok = %v, want true: %+v", output["ok"], output)
	}
	if output["action"] != "update_available" {
		t.Fatalf("action = %v, want update_available: %+v", output["action"], output)
	}
	if output["current_version"] != "0.1.0-beta.1" || output["latest_version"] != "0.1.0-beta.2" {
		t.Fatalf("unexpected update output: %+v", output)
	}
	if output["command"] != "npm install -g @qfeius/contract-cli@beta --registry https://registry.npmjs.org" {
		t.Fatalf("missing install command: %+v", output)
	}
	if _, ok := output["_notice"]; ok {
		t.Fatalf("manual update check JSON should not include _notice: %+v", output)
	}
	cacheContent, err := os.ReadFile(filepath.Join(filepath.Dir(store.Path()), "update-check.json"))
	if err != nil {
		t.Fatalf("ReadFile(update cache) error = %v", err)
	}
	if !strings.Contains(string(cacheContent), `"latest_version": "0.1.0-beta.2"`) {
		t.Fatalf("unexpected update cache: %s", string(cacheContent))
	}
}

func TestAutomaticUpdateNoticeRefreshesStaleCacheSynchronously(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	apiRequests := 0
	registryRequests := 0
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	cachePath := filepath.Join(filepath.Dir(store.Path()), "update-check.json")
	if err := updatecheck.SaveCache(cachePath, updatecheck.Cache{
		CheckedAt:       fixedCLINow().Add(-25 * time.Hour),
		Channel:         "beta",
		CurrentVersion:  "0.1.0-beta.1",
		LatestVersion:   "0.1.0-beta.2",
		UpdateAvailable: true,
		InstallCommand:  "npm install -g @qfeius/contract-cli@beta --registry https://registry.npmjs.org",
	}); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout:               stdout,
		Stderr:               stderr,
		Store:                store,
		SkillsFS:             testSkillsFS(),
		UpdateRegistryURL:    "https://registry.test/@qfeius%2fcontract-cli",
		UpdateCurrentVersion: "0.1.0-beta.1",
		Now:                  fixedCLINow,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host == "registry.test" {
					registryRequests++
					return jsonResponse(`{"dist-tags":{"beta":"0.1.0-beta.3"}}`), nil
				}
				apiRequests++
				if req.URL.Path != "/open-apis/contract/v1/contracts/contract-1" {
					t.Fatalf("unexpected API path: %s", req.URL.Path)
				}
				return jsonResponse(`{"code":0,"data":{"contract":{"contract_id":"contract-1"}}}`), nil
			}),
		},
	})

	if err := app.Run(context.Background(), []string{"contract", "get", "contract-1", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatalf("contract get error = %v", err)
	}
	if strings.Contains(stderr.String(), "A new contract-cli version is available") {
		t.Fatalf("stderr should not contain legacy update notice: %s", stderr.String())
	}
	output := decodeJSONObject(t, stdout.Bytes())
	updateNotice := output["_notice"].(map[string]any)["update"].(map[string]any)
	if updateNotice["current"] != "0.1.0-beta.1" || updateNotice["latest"] != "0.1.0-beta.3" {
		t.Fatalf("unexpected update notice: %+v", updateNotice)
	}
	if apiRequests != 1 {
		t.Fatalf("api requests = %d, want 1", apiRequests)
	}
	if registryRequests != 1 {
		t.Fatalf("registry requests = %d, want 1", registryRequests)
	}
	cache, ok, err := updatecheck.LoadCache(cachePath)
	if err != nil {
		t.Fatalf("LoadCache() error = %v", err)
	}
	if !ok || cache.LatestVersion != "0.1.0-beta.3" || !cache.UpdateAvailable {
		t.Fatalf("cache after synchronous refresh = %+v, ok=%v", cache, ok)
	}
}

func TestAutomaticUpdateNoticeDoesNotUseStaleCacheWhenRefreshFails(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	apiRequests := 0
	registryRequests := 0
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	cachePath := filepath.Join(filepath.Dir(store.Path()), "update-check.json")
	if err := updatecheck.SaveCache(cachePath, updatecheck.Cache{
		CheckedAt:       fixedCLINow().Add(-25 * time.Hour),
		Channel:         "beta",
		CurrentVersion:  "0.1.0-beta.1",
		LatestVersion:   "0.1.0-beta.2",
		UpdateAvailable: true,
		InstallCommand:  "npm install -g @qfeius/contract-cli@beta --registry https://registry.npmjs.org",
	}); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout:               stdout,
		Stderr:               stderr,
		Store:                store,
		SkillsFS:             testSkillsFS(),
		UpdateRegistryURL:    "https://registry.test/@qfeius%2fcontract-cli",
		UpdateCurrentVersion: "0.1.0-beta.1",
		Now:                  fixedCLINow,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host == "registry.test" {
					registryRequests++
					return nil, errors.New("registry unavailable")
				}
				apiRequests++
				return jsonResponse(`{"code":0,"data":{"contract":{"contract_id":"contract-1"}}}`), nil
			}),
		},
	})

	if err := app.Run(context.Background(), []string{"contract", "get", "contract-1", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatalf("contract get error = %v", err)
	}
	if apiRequests != 1 {
		t.Fatalf("api requests = %d, want 1", apiRequests)
	}
	output := decodeJSONObject(t, stdout.Bytes())
	if _, ok := output["_notice"]; ok {
		t.Fatalf("stale cache should not be injected when refresh fails: %+v", output)
	}
	if registryRequests != 1 {
		t.Fatalf("registry requests = %d, want 1", registryRequests)
	}
	cache, ok, err := updatecheck.LoadCache(cachePath)
	if err != nil {
		t.Fatalf("LoadCache() error = %v", err)
	}
	if !ok || cache.LatestVersion != "0.1.0-beta.2" {
		t.Fatalf("failed refresh should preserve existing cache, got %+v ok=%v", cache, ok)
	}
}

func TestAutomaticUpdateNoticeKeepsFreshNoUpdateCache(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	apiRequests := 0
	registryRequests := 0
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	cachePath := filepath.Join(filepath.Dir(store.Path()), "update-check.json")
	if err := updatecheck.SaveCache(cachePath, updatecheck.Cache{
		CheckedAt:       fixedCLINow(),
		Channel:         "beta",
		CurrentVersion:  "0.3.3-beta.3",
		LatestVersion:   "0.3.3-beta.3",
		UpdateAvailable: false,
		InstallCommand:  "npm install -g @qfeius/contract-cli@beta --registry https://registry.npmjs.org",
	}); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout:               stdout,
		Stderr:               stderr,
		Store:                store,
		SkillsFS:             testSkillsFS(),
		UpdateRegistryURL:    "https://registry.test/@qfeius%2fcontract-cli",
		UpdateCurrentVersion: "0.3.3-beta.5",
		Now:                  fixedCLINow,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host == "registry.test" {
					registryRequests++
					return jsonResponse(`{"dist-tags":{"beta":"0.3.3-beta.6"}}`), nil
				}
				apiRequests++
				return jsonResponse(`{"code":0,"data":{"contract":{"contract_id":"contract-1"}}}`), nil
			}),
		},
	})

	if err := app.Run(context.Background(), []string{"contract", "get", "contract-1", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatalf("contract get error = %v", err)
	}
	if apiRequests != 1 {
		t.Fatalf("api requests = %d, want 1", apiRequests)
	}
	output := decodeJSONObject(t, stdout.Bytes())
	if _, ok := output["_notice"]; ok {
		t.Fatalf("fresh no-update cache should not inject notice: %+v", output)
	}
	if registryRequests != 0 {
		t.Fatalf("fresh cache should not request registry, got %d", registryRequests)
	}
}

func TestAutomaticUpdateNoticeWritesMissingCacheSynchronously(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	apiRequests := 0
	registryRequests := 0
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	cachePath := filepath.Join(filepath.Dir(store.Path()), "update-check.json")

	app := cli.New(cli.Options{
		Stdout:               stdout,
		Stderr:               stderr,
		Store:                store,
		SkillsFS:             testSkillsFS(),
		UpdateRegistryURL:    "https://registry.test/@qfeius%2fcontract-cli",
		UpdateCurrentVersion: "0.1.0-beta.1",
		Now:                  fixedCLINow,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host == "registry.test" {
					registryRequests++
					return jsonResponse(`{"dist-tags":{"beta":"0.1.0-beta.2"}}`), nil
				}
				apiRequests++
				return jsonResponse(`{"code":0,"data":{"contract":{"contract_id":"contract-1"}}}`), nil
			}),
		},
	})

	if err := app.Run(context.Background(), []string{"contract", "get", "contract-1", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatalf("contract get error = %v", err)
	}
	if apiRequests != 1 {
		t.Fatalf("api requests = %d, want 1", apiRequests)
	}
	if registryRequests != 1 {
		t.Fatalf("registry requests = %d, want 1", registryRequests)
	}
	output := decodeJSONObject(t, stdout.Bytes())
	updateNotice := output["_notice"].(map[string]any)["update"].(map[string]any)
	if updateNotice["latest"] != "0.1.0-beta.2" {
		t.Fatalf("unexpected update notice: %+v", updateNotice)
	}
	cache, ok, err := updatecheck.LoadCache(cachePath)
	if err != nil {
		t.Fatalf("LoadCache() error = %v", err)
	}
	if !ok || cache.LatestVersion != "0.1.0-beta.2" || !cache.UpdateAvailable {
		t.Fatalf("cache after missing-cache refresh = %+v, ok=%v", cache, ok)
	}
}

func TestAutomaticUpdateNoticeCanBeDisabledByEnv(t *testing.T) {
	requests := 0
	stdout := &bytes.Buffer{}
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	app := cli.New(cli.Options{
		Stdout:               stdout,
		Stderr:               &bytes.Buffer{},
		Store:                store,
		SkillsFS:             testSkillsFS(),
		UpdateRegistryURL:    "https://registry.test/@qfeius%2fcontract-cli",
		UpdateCurrentVersion: "0.1.0-beta.1",
		LookupEnv: func(key string) (string, bool) {
			if key == "CONTRACT_CLI_NO_UPDATE_CHECK" {
				return "1", true
			}
			return "", false
		},
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host == "registry.test" {
					requests++
					return jsonResponse(`{"dist-tags":{"beta":"0.1.0-beta.2"}}`), nil
				}
				return jsonResponse(`{"code":0,"data":{"contract":{"contract_id":"contract-1"}}}`), nil
			}),
		},
	})

	if err := app.Run(context.Background(), []string{"contract", "get", "contract-1", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatalf("contract get error = %v", err)
	}
	if requests != 0 {
		t.Fatalf("disabled update check sent %d requests, want 0", requests)
	}
	if strings.Contains(stdout.String(), "_notice") {
		t.Fatalf("disabled update check should not inject notice: %s", stdout.String())
	}
}

func TestAutomaticUpdateNoticeRetriesFailureEveryRun(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	registryRequests := 0

	app := cli.New(cli.Options{
		Stdout:               stdout,
		Stderr:               stderr,
		Store:                config.NewStore(t.TempDir()),
		SkillsFS:             testSkillsFS(),
		UpdateRegistryURL:    "https://registry.test/@qfeius%2fcontract-cli",
		UpdateCurrentVersion: "0.1.0-beta.1",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				registryRequests++
				return nil, errors.New("registry unavailable")
			}),
		},
	})

	if err := app.Run(context.Background(), []string{"skills", "list"}); err != nil {
		t.Fatalf("skills list error = %v", err)
	}
	if err := app.Run(context.Background(), []string{"skills", "list"}); err != nil {
		t.Fatalf("second skills list error = %v", err)
	}
	if registryRequests != 2 {
		t.Fatalf("failed update check requests = %d, want 2", registryRequests)
	}
	if strings.Contains(stderr.String(), "A new contract-cli version is available") {
		t.Fatalf("failed update check should not print notice: %s", stderr.String())
	}
}

func TestUpdateCheckUsesBuildVersionWhenNotOverridden(t *testing.T) {
	originalVersion := build.Version
	build.Version = "0.1.0-beta.1"
	t.Cleanup(func() { build.Version = originalVersion })

	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{
		Stdout:            stdout,
		Stderr:            &bytes.Buffer{},
		Store:             config.NewStore(t.TempDir()),
		UpdateRegistryURL: "https://registry.test/@qfeius%2fcontract-cli",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonResponse(`{"dist-tags":{"beta":"0.1.0-beta.1"}}`), nil
			}),
		},
	})

	if err := app.Run(context.Background(), []string{"update", "check", "--channel", "beta", "--json"}); err != nil {
		t.Fatalf("update check error = %v", err)
	}
	output := decodeJSONObject(t, stdout.Bytes())
	if output["action"] != "already_up_to_date" || output["current_version"] != "0.1.0-beta.1" {
		t.Fatalf("unexpected update output: %+v", output)
	}
}

const testAuthSkill = `---
name: auth
version: 1.1.0
description: "contract-cli auth skill"
---

# Auth
`

/*
testSkillsFS 构造安装测试使用的包内 Skill 文件集合，覆盖普通规则技能和被隐藏的接口技能。
入参为空；返回 fstest.MapFS 为无需访问真实文件系统的 Skill 测试源。
*/
func testSkillsFS() fstest.MapFS {
	return fstest.MapFS{
		"auth/SKILL.md": {
			Data: []byte(testAuthSkill),
		},
		"auth/agents/openai.yaml": {
			Data: []byte("name: auth\n"),
		},
		"contract-cli-contract/SKILL.md": {
			Data: []byte(`---
name: contract-cli-contract
version: 1.0.0
description: "contract commands skill"
---

# Contract
`),
		},
		"contract-cli-contract/agents/openai.yaml": {
			Data: []byte("name: contract-cli-contract\n"),
		},
		"contract-cli-contract/references/commands.md": {
			Data: []byte("# Commands\n"),
		},
		"contract-cli-rule/SKILL.md": {
			Data: []byte(`---
name: contract-cli-rule
version: 1.0.0
description: "rule commands skill"
---

# Rule

contract-cli rule table import get|pause|resume|verify|cancel
`),
		},
		"contract-cli-api-call/SKILL.md": {
			Data: []byte(`---
name: contract-cli-api-call
version: 1.0.0
description: "api call skill"
---

# API Call
`),
		},
	}
}

func TestSkillsListDisplaysCurrentCLIVersion(t *testing.T) {
	originalVersion := build.Version
	build.Version = "1.2.3"
	t.Cleanup(func() { build.Version = originalVersion })

	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{
		Stdout:   stdout,
		Stderr:   &bytes.Buffer{},
		Store:    config.NewStore(t.TempDir()),
		SkillsFS: testSkillsFS(),
	})

	if err := app.Run(context.Background(), []string{"skills", "list"}); err != nil {
		t.Fatalf("skills list error = %v", err)
	}

	output := stdout.String()
	for _, want := range []string{
		"Built-in skills:",
		"auth\t1.2.3\tcontract-cli auth skill",
		"contract-cli-contract\t1.2.3\tcontract commands skill",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("skills list output missing %q: %s", want, output)
		}
	}
	for _, localSkillVersion := range []string{"auth\t1.1.0", "contract-cli-contract\t1.0.0"} {
		if strings.Contains(output, localSkillVersion) {
			t.Fatalf("skills list should display CLI version instead of SKILL.md version %q: %s", localSkillVersion, output)
		}
	}
	if strings.Contains(output, "contract-cli-api-call") {
		t.Fatalf("skills list should hide disabled api call skill: %s", output)
	}
}

func TestSkillsInstallCopiesBundledSkillsAndSkipsExisting(t *testing.T) {
	originalVersion := build.Version
	build.Version = "1.2.3"
	t.Cleanup(func() { build.Version = originalVersion })

	target := filepath.Join(t.TempDir(), "skills")
	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{
		Stdout:   stdout,
		Stderr:   &bytes.Buffer{},
		Store:    config.NewStore(t.TempDir()),
		SkillsFS: testSkillsFS(),
	})

	if err := app.Run(context.Background(), []string{"skills", "install", "--target", target}); err != nil {
		t.Fatalf("skills install error = %v", err)
	}
	assertFileContent(t, filepath.Join(target, "auth", "SKILL.md"), skillWithVersion(testAuthSkill, "1.2.3"))
	assertFileContent(t, filepath.Join(target, "auth", "agents", "openai.yaml"), "name: auth\n")
	assertFileContent(t, filepath.Join(target, "contract-cli-contract", "references", "commands.md"), "# Commands\n")
	if _, err := os.Stat(filepath.Join(target, "contract-cli-api-call")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("api call skill should not be installed, stat error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(target, "auth", "SKILL.md"), []byte("local custom skill\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(local custom skill) error = %v", err)
	}
	stdout.Reset()
	if err := app.Run(context.Background(), []string{"skills", "install", "--target", target}); err != nil {
		t.Fatalf("second skills install error = %v", err)
	}
	assertFileContent(t, filepath.Join(target, "auth", "SKILL.md"), "local custom skill\n")
	if !strings.Contains(stdout.String(), "Skipped existing skill: auth (not synchronized; use --force to overwrite)") {
		t.Fatalf("expected skip output, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "contract-cli skills install --target ") ||
		!strings.Contains(stdout.String(), " --force") {
		t.Fatalf("expected actionable skill synchronization command, got: %s", stdout.String())
	}
}

func TestSkillsInstallForceOverwritesExistingSkill(t *testing.T) {
	originalVersion := build.Version
	build.Version = "1.2.3"
	t.Cleanup(func() { build.Version = originalVersion })

	target := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(filepath.Join(target, "auth"), 0o755); err != nil {
		t.Fatalf("MkdirAll(auth) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "auth", "SKILL.md"), []byte("local custom skill\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(local custom skill) error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
		Store:    config.NewStore(t.TempDir()),
		SkillsFS: testSkillsFS(),
	})

	if err := app.Run(context.Background(), []string{"skills", "install", "--target", target, "--force"}); err != nil {
		t.Fatalf("skills install --force error = %v", err)
	}
	assertFileContent(t, filepath.Join(target, "auth", "SKILL.md"), skillWithVersion(testAuthSkill, "1.2.3"))
}

/*
TestSkillsInstallForceOnlySelectedSkill 验证定向更新规则技能时仅替换目标目录并写入当前 CLI 版本。
入参 t（*testing.T）为测试上下文；返回值为空，断言失败时报告测试错误。
*/
func TestSkillsInstallForceOnlySelectedSkill(t *testing.T) {
	originalVersion := build.Version
	build.Version = "1.2.3"
	t.Cleanup(func() { build.Version = originalVersion })

	target := filepath.Join(t.TempDir(), "skills")
	for name, content := range map[string]string{"auth": "local custom auth\n", "contract-cli-rule": "stale rule skill\n"} {
		dir := filepath.Join(target, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{Stdout: stdout, Stderr: &bytes.Buffer{}, Store: config.NewStore(t.TempDir()), SkillsFS: testSkillsFS()})
	if err := app.Run(context.Background(), []string{"skills", "install", "--target", target, "--name", "contract-cli-rule", "--force"}); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, filepath.Join(target, "auth", "SKILL.md"), "local custom auth\n")
	assertFileContent(t, filepath.Join(target, "contract-cli-rule", "SKILL.md"), "---\nname: contract-cli-rule\nversion: 1.2.3\ndescription: \"rule commands skill\"\n---\n\n# Rule\n\ncontract-cli rule table import get|pause|resume|verify|cancel\n")
	if strings.Contains(stdout.String(), "Installed skill: auth") || !strings.Contains(stdout.String(), "Installed skill: contract-cli-rule") {
		t.Fatalf("unexpected install output: %s", stdout.String())
	}
}

/*
TestSkillsInstallRejectsUnknownNameWithoutCreatingTarget 验证未知或路径型 Skill 名称会在创建目标目录前被拒绝。
入参 t（*testing.T）为测试上下文；返回值为空，断言失败时报告测试错误。
*/
func TestSkillsInstallRejectsUnknownNameWithoutCreatingTarget(t *testing.T) {
	for _, name := range []string{"missing-skill", "../auth"} {
		t.Run(name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "skills")
			app := cli.New(cli.Options{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Store: config.NewStore(t.TempDir()), SkillsFS: testSkillsFS()})
			err := app.Run(context.Background(), []string{"skills", "install", "--target", target, "--name", name, "--force"})
			if err == nil || !strings.Contains(err.Error(), "not found") {
				t.Fatalf("unknown skill error=%v", err)
			}
			if _, statErr := os.Stat(target); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("unknown skill created target: %v", statErr)
			}
		})
	}
}

func TestSkillsInstallDefaultsToCodexHome(t *testing.T) {
	originalVersion := build.Version
	build.Version = "1.2.3"
	t.Cleanup(func() { build.Version = originalVersion })

	codexHome := t.TempDir()
	app := cli.New(cli.Options{
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
		Store:    config.NewStore(t.TempDir()),
		SkillsFS: testSkillsFS(),
		LookupEnv: func(key string) (string, bool) {
			if key == "CODEX_HOME" {
				return codexHome, true
			}
			return "", false
		},
	})

	if err := app.Run(context.Background(), []string{"skills", "install"}); err != nil {
		t.Fatalf("skills install error = %v", err)
	}
	assertFileContent(t, filepath.Join(codexHome, "skills", "auth", "SKILL.md"), skillWithVersion(testAuthSkill, "1.2.3"))
}

func skillWithVersion(content string, version string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "version: ") {
			lines[i] = "version: " + version
			break
		}
	}
	return strings.Join(lines, "\n")
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	if string(content) != want {
		t.Fatalf("%s content = %q, want %q", path, string(content), want)
	}
}

func TestConfigAddAndAuthStatus(t *testing.T) {
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
				switch req.URL.Path {
				case "/.well-known/oauth-protected-resource":
					return jsonResponse(`{"resource":"https://open.qfei.cn","authorization_servers":["https://myaccount.qfei.cn/contract"],"scopes_supported":["mcp:tools","mcp:resources"]}`), nil
				case "/.well-known/oauth-authorization-server/contract":
					return jsonResponse(`{"issuer":"common-organization-v2","authorization_endpoint":"https://myaccount.qfei.cn/oauth/authorize/contract","token_endpoint":"https://myaccount.qfei.cn/oauth/token/contract","registration_endpoint":"https://myaccount.qfei.cn/oauth/register/contract"}`), nil
				default:
					t.Fatalf("unexpected request path: %s", req.URL.Path)
					return nil, nil
				}
			}),
		},
	})

	err := app.Run(context.Background(), []string{
		"config", "add",
		"--name", "contract",
		"--env", "prod",
		"--resource-metadata-url", "https://open.qfei.cn/.well-known/oauth-protected-resource",
		"--redirect-url", "http://127.0.0.1:19090/callback",
	})
	if err != nil {
		t.Fatalf("config add error = %v", err)
	}

	if !strings.Contains(stdout.String(), `Profile "contract" saved`) {
		t.Fatalf("unexpected config add output: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "Server URL: ") {
		t.Fatalf("config add output should not contain removed server url: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "Open Platform URL:") || strings.Contains(stdout.String(), "Authorization endpoint:") {
		t.Fatalf("config add output should not expose endpoint URLs: %s", stdout.String())
	}
	savedProfile, err := store.GetProfile("contract")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if savedProfile.AppTokenEndpoint != "https://open.qfei.cn/open-apis/auth/v3/tenant_access_token/internal" {
		t.Fatalf("app token endpoint = %q", savedProfile.AppTokenEndpoint)
	}
	if savedProfile.OpenPlatformBaseURL != "https://open.qfei.cn" {
		t.Fatalf("open platform base url = %q", savedProfile.OpenPlatformBaseURL)
	}

	stdout.Reset()
	if err := app.Run(context.Background(), []string{"auth", "status", "--profile", "contract"}); err != nil {
		t.Fatalf("auth status error = %v", err)
	}
	if !strings.Contains(stdout.String(), "Identity: user") || !strings.Contains(stdout.String(), "Authorization: unauthorized") {
		t.Fatalf("unexpected auth status output: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "Server URL: ") {
		t.Fatalf("auth status output should not contain removed server url: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "Open Platform URL:") {
		t.Fatalf("auth status output should not expose open platform URL: %s", stdout.String())
	}
}

func TestConfigAddUsesProdPresetByDefault(t *testing.T) {
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
				case "https://myaccount.qfei.cn/.well-known/oauth-authorization-server/contract":
					return jsonResponse(`{"issuer":"common-organization-v2","authorization_endpoint":"https://myaccount.qfei.cn/api/public/oauth/authorize/contract","device_authorization_endpoint":"https://myaccount.qfei.cn/api/public/oauth/device-authorization/contract","token_endpoint":"https://myaccount.qfei.cn/api/public/oauth/token/contract","revocation_endpoint":"https://myaccount.qfei.cn/api/public/oauth/revoke/contract","registration_endpoint":"https://myaccount.qfei.cn/api/public/oauth/register/contract"}`), nil
				default:
					t.Fatalf("unexpected request url: %s", req.URL.String())
					return nil, nil
				}
			}),
		},
	})

	if err := app.Run(context.Background(), []string{"config", "add", "--name", "contract"}); err != nil {
		t.Fatalf("config add error = %v", err)
	}

	savedProfile, err := store.GetProfile("contract")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if savedProfile.ProtectedResourceMetadataURL != "" {
		t.Fatalf("protected resource metadata url = %q", savedProfile.ProtectedResourceMetadataURL)
	}
	if savedProfile.AuthorizationServerMetadataURL != "https://myaccount.qfei.cn/.well-known/oauth-authorization-server/contract" {
		t.Fatalf("authorization server metadata url = %q", savedProfile.AuthorizationServerMetadataURL)
	}
	if savedProfile.Resource != "https://open.qfei.cn" {
		t.Fatalf("resource = %q", savedProfile.Resource)
	}
	if savedProfile.OpenPlatformBaseURL != "https://open.qfei.cn" {
		t.Fatalf("open platform base url = %q", savedProfile.OpenPlatformBaseURL)
	}
	if savedProfile.Identities.User.DeviceClientID != "zscli_892efdadc11a3f53" {
		t.Fatalf("device client id = %q", savedProfile.Identities.User.DeviceClientID)
	}
	if savedProfile.Identities.User.DeviceScope != "contract:full contract-review:full" {
		t.Fatalf("device scope = %q", savedProfile.Identities.User.DeviceScope)
	}
	if savedProfile.Identities.User.DeviceAuthorizationEndpoint != "https://myaccount.qfei.cn/api/public/oauth/device-authorization/contract" {
		t.Fatalf("device authorization endpoint = %q", savedProfile.Identities.User.DeviceAuthorizationEndpoint)
	}
	if savedProfile.Identities.User.TokenEndpoint != "https://myaccount.qfei.cn/api/public/oauth/token/contract" {
		t.Fatalf("token endpoint = %q", savedProfile.Identities.User.TokenEndpoint)
	}
	if savedProfile.Identities.User.RevocationEndpoint != "https://myaccount.qfei.cn/api/public/oauth/revoke/contract" {
		t.Fatalf("revocation endpoint = %q", savedProfile.Identities.User.RevocationEndpoint)
	}

	configContent, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("ReadFile(config) error = %v", err)
	}
	if strings.Contains(string(configContent), "\"server_url\"") {
		t.Fatalf("config should not persist removed server_url field: %s", string(configContent))
	}
}

func TestConfigAddRejectsDevPresetBeforeNetworkRequest(t *testing.T) {
	t.Parallel()

	requestCount := 0
	app := cli.New(cli.Options{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Store:  config.NewStore(t.TempDir()),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestCount++
			t.Fatalf("dev preset must be rejected before network request: %s", req.URL.String())
			return nil, nil
		})},
	})

	err := app.Run(context.Background(), []string{"config", "add", "--name", "contract", "--env", "dev"})
	if err == nil || err.Error() != `unsupported environment "dev"; supported environments: prod` {
		t.Fatalf("config add dev error = %v", err)
	}
	if requestCount != 0 {
		t.Fatalf("network request count = %d, want 0", requestCount)
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
		"--server-url", "https://example.test/mcp-servers/contract",
	})
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -server-url") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuthLoginAppStoresCredentialsTokenAndSwitchesDefaultIdentity(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	secrets := config.NewSecretsStore(dir)

	profile := config.Profile{
		Name:             "contract",
		Environment:      "dev",
		AppTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
		DefaultIdentity:  config.IdentityUser,
		Identities: config.Identities{
			User: config.UserIdentity{},
		},
	}
	if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
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
				if req.URL.String() != productionProfileFixture(profile).AppTokenEndpoint {
					t.Fatalf("unexpected request url: %s", req.URL.String())
				}
				return jsonResponse(`{"code":0,"expire":7200,"msg":"ok","tenant_access_token":"app-token"}`), nil
			}),
		},
		OpenBrowser: func(string) error { return nil },
	})

	if err := app.Run(context.Background(), []string{
		"auth", "login",
		"--profile", "contract",
		"--as", "app",
		"--app-id", "cli_app_123",
		"--app-secret", "app-secret",
	}); err != nil {
		t.Fatalf("auth login --as app error = %v", err)
	}

	gotProfile, err := store.GetProfile("contract")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if gotProfile.DefaultIdentity != config.IdentityApp {
		t.Fatalf("default identity = %q, want %q", gotProfile.DefaultIdentity, config.IdentityApp)
	}
	if gotProfile.Identities.App.AppID != "cli_app_123" {
		t.Fatalf("app app_id = %q", gotProfile.Identities.App.AppID)
	}
	if gotProfile.Identities.App.SecretRef == "" {
		t.Fatalf("expected app secret ref to be saved")
	}
	if gotProfile.Identities.App.Token == nil || gotProfile.Identities.App.Token.AccessToken != "app-token" {
		t.Fatalf("app token = %+v", gotProfile.Identities.App.Token)
	}
	if gotProfile.Identities.App.Token.TokenType != "Bearer" {
		t.Fatalf("app token type = %q", gotProfile.Identities.App.Token.TokenType)
	}
	secret, ok, err := secrets.Get(gotProfile.Identities.App.SecretRef)
	if err != nil {
		t.Fatalf("secrets.Get() error = %v", err)
	}
	if !ok || secret != "app-secret" {
		t.Fatalf("stored secret mismatch: got (%q, %v)", secret, ok)
	}

	configContent, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("ReadFile(config) error = %v", err)
	}
	if strings.Contains(string(configContent), "app-secret") {
		t.Fatalf("main config should not contain app secret: %s", string(configContent))
	}

	if !strings.Contains(stdout.String(), `App authorization succeeded for profile "contract".`) {
		t.Fatalf("unexpected app login output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Access token expires at: ") {
		t.Fatalf("missing expiry output: %s", stdout.String())
	}
}

func TestAuthLoginAcceptsLegacyBotIdentityAsApp(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	secrets := config.NewSecretsStore(dir)
	profile := config.Profile{
		Name:             "contract",
		Environment:      "dev",
		AppTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
		DefaultIdentity:  config.IdentityUser,
	}
	if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout:  stdout,
		Stderr:  &bytes.Buffer{},
		Store:   store,
		Secrets: secrets,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(`{"code":0,"expire":7200,"msg":"ok","tenant_access_token":"legacy-app-token"}`), nil
			}),
		},
	})

	err := app.Run(context.Background(), []string{
		"auth", "login",
		"--profile", "contract",
		"--as", "bot",
		"--app-id", "legacy-app-id",
		"--app-secret", "legacy-secret",
	})
	if err != nil {
		t.Fatalf("auth login --as bot error = %v", err)
	}

	gotProfile, err := store.GetProfile("contract")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if gotProfile.DefaultIdentity != config.IdentityApp {
		t.Fatalf("default identity = %q, want %q", gotProfile.DefaultIdentity, config.IdentityApp)
	}
	if gotProfile.Identities.App.AppID != "legacy-app-id" {
		t.Fatalf("app id = %q", gotProfile.Identities.App.AppID)
	}
	if gotProfile.Identities.App.Token == nil || gotProfile.Identities.App.Token.AccessToken != "legacy-app-token" {
		t.Fatalf("app token = %+v", gotProfile.Identities.App.Token)
	}
	if !strings.Contains(stdout.String(), `App authorization succeeded for profile "contract".`) {
		t.Fatalf("unexpected legacy login output: %s", stdout.String())
	}
}

func TestAuthLoginAppCredentialPriority(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	secrets := config.NewSecretsStore(dir)

	profile := config.Profile{
		Name:             "contract",
		Environment:      "dev",
		AppTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
		DefaultIdentity:  config.IdentityUser,
		Identities: config.Identities{
			App: config.AppIdentity{
				AuthMode:  config.AppAuthModeAppCredentials,
				AppID:     "local-app-id",
				SecretRef: config.AppSecretKey("contract"),
			},
		},
	}
	if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	if err := secrets.Set(config.AppSecretKey("contract"), "local-secret"); err != nil {
		t.Fatalf("secrets.Set() error = %v", err)
	}

	env := map[string]string{
		"CONTRACT_CLI_APP_ID":         "env-app-id",
		"CONTRACT_CLI_APP_SECRET":     "env-secret",
		"CONTRACT_CLI_BOT_APP_ID":     "old-env-app-id",
		"CONTRACT_CLI_BOT_APP_SECRET": "old-env-secret",
	}
	app := cli.New(cli.Options{
		Stdout:  stdout,
		Stderr:  stderr,
		Store:   store,
		Secrets: secrets,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(`{"code":0,"expire":7200,"msg":"ok","tenant_access_token":"app-token"}`), nil
			}),
		},
		LookupEnv: func(key string) (string, bool) {
			value, ok := env[key]
			return value, ok
		},
	})

	if err := app.Run(context.Background(), []string{
		"auth", "login",
		"--profile", "contract",
		"--as", "app",
		"--app-id", "flag-app-id",
	}); err != nil {
		t.Fatalf("auth login --as app error = %v", err)
	}

	gotProfile, err := store.GetProfile("contract")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if gotProfile.Identities.App.AppID != "flag-app-id" {
		t.Fatalf("app app_id = %q, want flag-app-id", gotProfile.Identities.App.AppID)
	}
	secret, ok, err := secrets.Get(gotProfile.Identities.App.SecretRef)
	if err != nil {
		t.Fatalf("secrets.Get() error = %v", err)
	}
	if !ok || secret != "env-secret" {
		t.Fatalf("stored secret mismatch: got (%q, %v), want (env-secret, true)", secret, ok)
	}
}

func TestAuthLoginAppFallsBackToLegacyEnvVariables(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		env  map[string]string
	}{
		{
			name: "contract cli bot env",
			env: map[string]string{
				"CONTRACT_CLI_BOT_APP_ID":     "legacy-env-app-id",
				"CONTRACT_CLI_BOT_APP_SECRET": "legacy-env-secret",
			},
		},
		{
			name: "democli bot env",
			env: map[string]string{
				"DEMOCLI_BOT_APP_ID":     "legacy-env-app-id",
				"DEMOCLI_BOT_APP_SECRET": "legacy-env-secret",
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			dir := t.TempDir()
			store := config.NewStore(dir)
			secrets := config.NewSecretsStore(dir)

			profile := config.Profile{
				Name:             "contract",
				Environment:      "dev",
				AppTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
				DefaultIdentity:  config.IdentityUser,
				Identities: config.Identities{
					App: config.AppIdentity{
						AuthMode:  config.AppAuthModeAppCredentials,
						AppID:     "local-app-id",
						SecretRef: config.AppSecretKey("contract"),
					},
				},
			}
			if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
				t.Fatalf("UpsertProfile() error = %v", err)
			}

			app := cli.New(cli.Options{
				Stdout:  stdout,
				Stderr:  stderr,
				Store:   store,
				Secrets: secrets,
				HTTPClient: &http.Client{
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						return jsonResponse(`{"code":0,"expire":7200,"msg":"ok","tenant_access_token":"legacy-app-token"}`), nil
					}),
				},
				LookupEnv: func(key string) (string, bool) {
					value, ok := tc.env[key]
					return value, ok
				},
			})

			if err := app.Run(context.Background(), []string{
				"auth", "login",
				"--profile", "contract",
				"--as", "app",
			}); err != nil {
				t.Fatalf("auth login --as app error = %v", err)
			}

			gotProfile, err := store.GetProfile("contract")
			if err != nil {
				t.Fatalf("GetProfile() error = %v", err)
			}
			if gotProfile.Identities.App.AppID != "legacy-env-app-id" {
				t.Fatalf("app app_id = %q, want legacy-env-app-id", gotProfile.Identities.App.AppID)
			}
			secret, ok, err := secrets.Get(gotProfile.Identities.App.SecretRef)
			if err != nil {
				t.Fatalf("secrets.Get() error = %v", err)
			}
			if !ok || secret != "legacy-env-secret" {
				t.Fatalf("stored secret mismatch: got (%q, %v), want (legacy-env-secret, true)", secret, ok)
			}
		})
	}
}

func TestAuthLoginAppReturnsErrorWhenProfileMissesTokenEndpoint(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	secrets := config.NewSecretsStore(dir)

	profile := config.Profile{
		Name:            "contract",
		Environment:     "dev",
		DefaultIdentity: config.IdentityUser,
	}
	if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
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
		"--profile", "contract",
		"--as", "app",
		"--app-id", "cli_app_123",
		"--app-secret", "app-secret",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "run `contract-cli config add --env prod --name contract` first") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuthLoginAppPersistsCredentialsWhenTokenExchangeFails(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	secrets := config.NewSecretsStore(dir)

	profile := config.Profile{
		Name:             "contract",
		Environment:      "dev",
		AppTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
		DefaultIdentity:  config.IdentityUser,
	}
	if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
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
		"--profile", "contract",
		"--as", "app",
		"--app-id", "cli_app_123",
		"--app-secret", "app-secret",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "invalid app") {
		t.Fatalf("unexpected error: %v", err)
	}

	gotProfile, err := store.GetProfile("contract")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if gotProfile.DefaultIdentity != config.IdentityUser {
		t.Fatalf("default identity = %q, want %q", gotProfile.DefaultIdentity, config.IdentityUser)
	}
	if gotProfile.Identities.App.AppID != "cli_app_123" {
		t.Fatalf("app app_id = %q", gotProfile.Identities.App.AppID)
	}
	if gotProfile.Identities.App.Token != nil {
		t.Fatalf("app token = %+v, want nil", gotProfile.Identities.App.Token)
	}

	secret, ok, err := secrets.Get(config.AppSecretKey("contract"))
	if err != nil {
		t.Fatalf("secrets.Get() error = %v", err)
	}
	if !ok || secret != "app-secret" {
		t.Fatalf("stored secret mismatch: got (%q, %v)", secret, ok)
	}
}

func TestAuthStatusAppAndAuthUse(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	secrets := config.NewSecretsStore(dir)
	profile := config.Profile{
		Name:             "contract",
		Environment:      "dev",
		AppTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
		DefaultIdentity:  config.IdentityApp,
		Identities: config.Identities{
			App: config.AppIdentity{
				AuthMode:     config.AppAuthModeAppCredentials,
				AppID:        "app-id",
				SecretRef:    config.AppSecretKey("contract"),
				ConfiguredAt: time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
				Token: &config.Token{
					AccessToken: "app-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(2 * time.Hour),
				},
			},
		},
	}
	if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	if err := secrets.Set(config.AppSecretKey("contract"), "app-secret"); err != nil {
		t.Fatalf("secrets.Set() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout:    stdout,
		Stderr:    stderr,
		Store:     store,
		Secrets:   secrets,
		LookupEnv: func(string) (string, bool) { return "", false },
	})

	if err := app.Run(context.Background(), []string{"auth", "status", "--profile", "contract", "--as", "app"}); err != nil {
		t.Fatalf("auth status --as app error = %v", err)
	}
	if !strings.Contains(stdout.String(), "Identity: app") ||
		!strings.Contains(stdout.String(), "Credential Source: secrets") ||
		!strings.Contains(stdout.String(), "Token Protocol: tenant_access_token/internal") ||
		!strings.Contains(stdout.String(), "Authorization: authorized") {
		t.Fatalf("unexpected app status output: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "Open Platform URL:") || strings.Contains(stdout.String(), "Token Endpoint:") {
		t.Fatalf("app status output should not expose endpoint URLs: %s", stdout.String())
	}

	stdout.Reset()
	if err := app.Run(context.Background(), []string{"auth", "use", "--profile", "contract", "--as", "user"}); err != nil {
		t.Fatalf("auth use error = %v", err)
	}

	gotProfile, err := store.GetProfile("contract")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if gotProfile.DefaultIdentity != config.IdentityUser {
		t.Fatalf("default identity = %q, want %q", gotProfile.DefaultIdentity, config.IdentityUser)
	}
}

func TestAuthStatusDefaultsToUserEvenWhenDefaultIdentityIsApp(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	secrets := config.NewSecretsStore(dir)
	profile := config.Profile{
		Name:             "contract",
		Environment:      "dev",
		AppTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
		DefaultIdentity:  config.IdentityApp,
		Identities: config.Identities{
			App: config.AppIdentity{
				AuthMode:  config.AppAuthModeAppCredentials,
				AppID:     "app-id",
				SecretRef: config.AppSecretKey("contract"),
			},
		},
	}
	if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	if err := secrets.Set(config.AppSecretKey("contract"), "app-secret"); err != nil {
		t.Fatalf("secrets.Set() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout:    stdout,
		Stderr:    stderr,
		Store:     store,
		Secrets:   secrets,
		LookupEnv: func(string) (string, bool) { return "", false },
	})

	if err := app.Run(context.Background(), []string{"auth", "status", "--profile", "contract"}); err != nil {
		t.Fatalf("auth status error = %v", err)
	}
	if !strings.Contains(stdout.String(), "\nIdentity: user\n") || strings.Contains(stdout.String(), "\nIdentity: app\n") {
		t.Fatalf("unexpected default status identity output: %s", stdout.String())
	}
}

func TestAuthStatusUserMarksExpiredToken(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	profile := config.Profile{
		Name:        "contract",
		Environment: "dev",
		Identities: config.Identities{
			User: config.UserIdentity{
				ClientID: "client-id",
				Token: &config.Token{
					AccessToken: "expired-user-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(-1 * time.Hour),
				},
			},
		},
	}
	if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	secrets := config.NewSecretsStore(dir)
	app := cli.New(cli.Options{
		Stdout:    stdout,
		Stderr:    stderr,
		Store:     store,
		Secrets:   secrets,
		LookupEnv: func(string) (string, bool) { return "", false },
	})

	if err := app.Run(context.Background(), []string{"auth", "status", "--profile", "contract", "--as", "user"}); err != nil {
		t.Fatalf("auth status --as user error = %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "Authorization: expired") || !strings.Contains(output, "Expires At: ") {
		t.Fatalf("unexpected expired user status output: %s", output)
	}
}

func TestAuthStatusAppHandlesConfiguredExpiredAndUnconfigured(t *testing.T) {
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
				Name:             "contract",
				Environment:      "dev",
				AppTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
				Identities: config.Identities{
					App: config.AppIdentity{
						AuthMode:  config.AppAuthModeAppCredentials,
						AppID:     "app-id",
						SecretRef: config.AppSecretKey("contract"),
					},
				},
			},
			seedSecret:        "app-secret",
			wantAuthorization: "Authorization: configured",
			wantContains: []string{
				"Token Protocol: tenant_access_token/internal",
			},
		},
		{
			name: "expired",
			profile: config.Profile{
				Name:             "contract",
				Environment:      "dev",
				AppTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
				Identities: config.Identities{
					App: config.AppIdentity{
						AuthMode:  config.AppAuthModeAppCredentials,
						AppID:     "app-id",
						SecretRef: config.AppSecretKey("contract"),
						Token: &config.Token{
							AccessToken: "expired-token",
							TokenType:   "Bearer",
							Expiry:      time.Now().Add(-1 * time.Hour),
						},
					},
				},
			},
			seedSecret:        "app-secret",
			wantAuthorization: "Authorization: expired",
			wantContains: []string{
				"Expires At: ",
			},
		},
		{
			name: "unconfigured",
			profile: config.Profile{
				Name:        "contract",
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
			if err := store.UpsertProfile(productionProfileFixture(tc.profile), true); err != nil {
				t.Fatalf("UpsertProfile() error = %v", err)
			}
			if tc.seedSecret != "" {
				if err := secrets.Set(config.AppSecretKey("contract"), tc.seedSecret); err != nil {
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

			if err := app.Run(context.Background(), []string{"auth", "status", "--profile", "contract", "--as", "app"}); err != nil {
				t.Fatalf("auth status --as app error = %v", err)
			}
			if !strings.Contains(stdout.String(), tc.wantAuthorization) {
				t.Fatalf("unexpected app status output: %s", stdout.String())
			}
			if strings.Contains(stdout.String(), "Open Platform URL:") || strings.Contains(stdout.String(), "Token Endpoint:") {
				t.Fatalf("app status output should not expose endpoint URLs: %s", stdout.String())
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

func TestAuthLogoutAppKeepsUserTokenAndCredentials(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	secrets := config.NewSecretsStore(dir)
	profile := config.Profile{
		Name:             "contract",
		Environment:      "dev",
		AppTokenEndpoint: "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
		DefaultIdentity:  config.IdentityApp,
		Identities: config.Identities{
			User: config.UserIdentity{
				Token: &config.Token{AccessToken: "user-token"},
			},
			App: config.AppIdentity{
				AuthMode:  config.AppAuthModeAppCredentials,
				AppID:     "app-id",
				SecretRef: config.AppSecretKey("contract"),
				Token:     &config.Token{AccessToken: "app-token"},
			},
		},
	}
	if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	if err := secrets.Set(config.AppSecretKey("contract"), "app-secret"); err != nil {
		t.Fatalf("secrets.Set() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout:    stdout,
		Stderr:    stderr,
		Store:     store,
		Secrets:   secrets,
		LookupEnv: func(string) (string, bool) { return "", false },
	})

	if err := app.Run(context.Background(), []string{"auth", "logout", "--profile", "contract", "--as", "app"}); err != nil {
		t.Fatalf("auth logout --as app error = %v", err)
	}

	gotProfile, err := store.GetProfile("contract")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if gotProfile.Identities.User.Token == nil || gotProfile.Identities.User.Token.AccessToken != "user-token" {
		t.Fatalf("user token should remain intact, got %+v", gotProfile.Identities.User.Token)
	}
	if gotProfile.Identities.App.AppID != "app-id" || gotProfile.Identities.App.SecretRef != config.AppSecretKey("contract") {
		t.Fatalf("app credentials should remain intact, got %+v", gotProfile.Identities.App)
	}
	if gotProfile.Identities.App.Token != nil {
		t.Fatalf("app token should be cleared, got %+v", gotProfile.Identities.App.Token)
	}
	if gotProfile.DefaultIdentity != config.IdentityApp {
		t.Fatalf("default identity = %q, want %q", gotProfile.DefaultIdentity, config.IdentityApp)
	}
	_, ok, err := secrets.Get(config.AppSecretKey("contract"))
	if err != nil {
		t.Fatalf("secrets.Get() error = %v", err)
	}
	if !ok {
		t.Fatalf("app secret should be retained")
	}
	if !strings.Contains(stdout.String(), `Logged out app token for profile "contract" while keeping app credentials.`) {
		t.Fatalf("unexpected logout output: %s", stdout.String())
	}
}

func TestAuthDeviceInitPreflightsCredentialStoreBeforeRemoteRequestOrProfileMutation(t *testing.T) {
	store := config.NewStore(t.TempDir())
	legacyToken := &config.Token{AccessToken: "legacy-access", Expiry: fixedCLINow().Add(time.Hour)}
	profile := config.Profile{
		Name: "contract", Environment: "prod", Resource: "https://open.qfei.cn", DefaultIdentity: config.IdentityUser,
		Identities: config.Identities{User: config.UserIdentity{
			AuthMode: config.UserAuthModeAuthorizationCode, Token: legacyToken,
			DeviceAuthorizationEndpoint: "https://myaccount.qfei.cn/device", TokenEndpoint: "https://myaccount.qfei.cn/token/contract",
			DeviceClientID: "device-client", DeviceScope: "contract:full",
		}},
	}
	if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
		t.Fatal(err)
	}
	credentialErr := errors.New("credential backend unavailable")
	credentials := &memoryDeviceCredentialStore{values: map[string]credential.DeviceCredential{}, loadErr: credentialErr}
	requestCount := 0
	app := cli.New(cli.Options{
		Store: store, CredentialStore: credentials, Now: fixedCLINow,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			requestCount++
			return nil, errors.New("remote request must not run")
		})},
	})

	err := app.Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json"})
	if !errors.Is(err, credentialErr) {
		t.Fatalf("auth init error = %v, want credential error", err)
	}
	if requestCount != 0 {
		t.Fatalf("remote request count = %d, want 0", requestCount)
	}
	got, err := store.GetProfile("contract")
	if err != nil {
		t.Fatal(err)
	}
	if got.Identities.User.AuthMode != config.UserAuthModeAuthorizationCode || got.Identities.User.Token == nil || got.Identities.User.Token.AccessToken != "legacy-access" {
		t.Fatalf("legacy profile was mutated: %+v", got.Identities.User)
	}
}

func TestAuthDeviceInitQRCodeFailureDoesNotSwitchLegacyProfile(t *testing.T) {
	store := config.NewStore(t.TempDir())
	profile := config.Profile{
		Name: "contract", Environment: "prod", Resource: "https://open.qfei.cn", DefaultIdentity: config.IdentityUser,
		Identities: config.Identities{User: config.UserIdentity{
			AuthMode:                    config.UserAuthModeAuthorizationCode,
			Token:                       &config.Token{AccessToken: "legacy-access", Expiry: fixedCLINow().Add(time.Hour)},
			DeviceAuthorizationEndpoint: "https://myaccount.qfei.cn/device", TokenEndpoint: "https://myaccount.qfei.cn/token/contract",
			DeviceClientID: "device-client", DeviceScope: "contract:full",
		}},
	}
	if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
		t.Fatal(err)
	}
	workspaceFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(workspaceFile, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	credentials := &memoryDeviceCredentialStore{values: map[string]credential.DeviceCredential{}}
	app := cli.New(cli.Options{
		Store: store, CredentialStore: credentials, Now: fixedCLINow,
		LookupEnv: func(name string) (string, bool) {
			if name == "SKILL_SESSION_WORKSPACE" {
				return workspaceFile, true
			}
			return "", false
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return jsonResponse(`{"device_code":"secret-device","user_code":"user-a","verification_uri":"https://myaccount.qfei.cn/device","verification_uri_complete":"https://myaccount.qfei.cn/device?user_code=user-a","expires_in":600}`), nil
		})},
	})

	if err := app.Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json"}); err == nil {
		t.Fatal("auth init should fail when qr directory cannot be created")
	}
	got, err := store.GetProfile("contract")
	if err != nil {
		t.Fatal(err)
	}
	if got.Identities.User.AuthMode != config.UserAuthModeAuthorizationCode || got.Identities.User.Token == nil || got.Identities.User.Token.AccessToken != "legacy-access" {
		t.Fatalf("legacy profile was mutated: %+v", got.Identities.User)
	}
}

func TestDoubaoWorkTaskAuthInitRejectsUnwritableCredentialRootBeforeRemoteRequest(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	if err := os.WriteFile(filepath.Join(workspace, ".contract-cli"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := config.NewStore(t.TempDir())
	profile := config.Profile{
		Name: "contract", Environment: "prod", Resource: "https://open.qfei.cn", BusinessType: "contract",
		Identities: config.Identities{User: config.UserIdentity{
			DeviceAuthorizationEndpoint: "https://myaccount.qfei.cn/device", TokenEndpoint: "https://myaccount.qfei.cn/token/contract",
			DeviceClientID: "device-client", DeviceScope: "contract:full",
		}},
	}
	if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
		t.Fatal(err)
	}
	requestCount := 0
	app := cli.New(cli.Options{
		Store: store,
		LookupEnv: func(name string) (string, bool) {
			if name == "SESSION_ID" {
				return "doubao-task-a", true
			}
			return "", false
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			requestCount++
			return nil, errors.New("remote request must not run")
		})},
	})
	if err := app.Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json"}); err == nil {
		t.Fatal("auth init unexpectedly accepted an unwritable credential root")
	}
	if requestCount != 0 {
		t.Fatalf("remote request count = %d, want 0", requestCount)
	}
}

func TestAuthDeviceInitAndCompleteDoNotExposeCredentialSecrets(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	store := config.NewStore(t.TempDir())
	credentials := &memoryDeviceCredentialStore{values: map[string]credential.DeviceCredential{}}
	profile := config.Profile{
		Name: "contract", Environment: "prod", Resource: "https://open.qfei.cn",
		OpenPlatformBaseURL: "https://open.qfei.cn", BusinessType: "contract",
		Identities: config.Identities{User: config.UserIdentity{
			DeviceAuthorizationEndpoint: "https://myaccount.qfei.cn/device",
			TokenEndpoint:               "https://myaccount.qfei.cn/token/contract",
			DeviceClientID:              "zscli_892efdadc11a3f53",
			DeviceScope:                 "contract:full",
		}},
	}
	if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	app := cli.New(cli.Options{
		Stdout: stdout, Stderr: stderr, Store: store, CredentialStore: credentials, Now: fixedCLINow,
		LookupEnv: func(name string) (string, bool) {
			if name == "SKILL_SESSION_WORKSPACE" {
				return workspace, true
			}
			return "", false
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/device":
				return jsonResponse(`{"device_code":"secret-device","user_code":"user-a","verification_uri":"https://myaccount.qfei.cn/device","verification_uri_complete":"https://myaccount.qfei.cn/device?user_code=user-a","expires_in":600}`), nil
			case "/token/contract":
				return jsonResponse(`{"access_token":"secret-access","refresh_token":"secret-refresh","token_type":"Bearer","scope":"contract:full","expires_in":3600}`), nil
			default:
				t.Fatalf("unexpected URL: %s", req.URL)
				return nil, nil
			}
		})},
	})

	if err := app.Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	initOutput := stdout.String() + stderr.String()
	if strings.Contains(initOutput, "secret-device") {
		t.Fatalf("auth init exposed device code: %s", initOutput)
	}
	stdout.Reset()
	stderr.Reset()
	if err := app.Run(context.Background(), []string{"auth", "complete", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	completeOutput := stdout.String() + stderr.String()
	for _, secret := range []string{"secret-device", "secret-access", "secret-refresh"} {
		if strings.Contains(completeOutput, secret) {
			t.Fatalf("auth complete exposed %q: %s", secret, completeOutput)
		}
	}
	if !strings.Contains(stdout.String(), `"status":"succeeded"`) {
		t.Fatalf("unexpected complete output: %s", stdout.String())
	}
	stored, err := credentials.Load("contract")
	if err != nil || stored.Token == nil || stored.Token.AccessToken != "secret-access" || stored.Pending != nil {
		t.Fatalf("stored credential = %+v, err=%v", stored, err)
	}
	profileAfter, err := store.GetProfile("contract")
	if err != nil {
		t.Fatal(err)
	}
	if profileAfter.Identities.User.Token != nil {
		t.Fatal("device token must not be written to profile")
	}
}

func TestDoubaoRebuildRestoresEncryptedDeviceProfileAndContinuesBusinessRequest(t *testing.T) {
	workspace := t.TempDir()
	key := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	lookupEnv := func(name string) (string, bool) {
		switch name {
		case "SKILL_SESSION_WORKSPACE":
			return workspace, true
		case "CONTRACT_CLI_CREDENTIAL_KEY_V1":
			return key, true
		default:
			return "", false
		}
	}
	credentials, err := credential.NewStore(credential.Options{LookupEnv: lookupEnv})
	if err != nil {
		t.Fatal(err)
	}
	firstStore := config.NewStore(t.TempDir())
	profile := config.Profile{
		Name: "contract", Environment: "prod", Resource: "https://open.qfei.cn",
		OpenPlatformBaseURL: "https://open.qfei.cn", BusinessType: "contract", ClientName: "contract-cli",
		DefaultIdentity: config.IdentityApp,
		Identities: config.Identities{
			User: config.UserIdentity{
				AuthMode:                    config.UserAuthModeAuthorizationCode,
				Token:                       &config.Token{AccessToken: "legacy-user-access"},
				DeviceAuthorizationEndpoint: "https://myaccount.qfei.cn/device",
				TokenEndpoint:               "https://myaccount.qfei.cn/token/contract",
				RevocationEndpoint:          "https://myaccount.qfei.cn/revoke/contract",
				DeviceClientID:              "device-client",
				DeviceScope:                 "contract:full",
			},
			App: config.AppIdentity{
				AppID: "app-id", SecretRef: config.AppSecretKey("contract"),
				Token: &config.Token{AccessToken: "app-access"},
			},
		},
	}
	if err := firstStore.UpsertProfile(productionProfileFixture(profile), true); err != nil {
		t.Fatal(err)
	}
	firstApp := cli.New(cli.Options{
		Store: firstStore, CredentialStore: credentials, LookupEnv: lookupEnv, Now: fixedCLINow,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/device" {
				t.Fatalf("unexpected init URL: %s", req.URL)
			}
			return jsonResponse(`{"device_code":"secret-device","user_code":"user-a","verification_uri":"https://myaccount.qfei.cn/device","verification_uri_complete":"https://myaccount.qfei.cn/device?user_code=user-a","expires_in":600}`), nil
		})},
	})
	if err := firstApp.Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatal(err)
	}

	secondStore := config.NewStore(t.TempDir())
	stdout := &bytes.Buffer{}
	secondApp := cli.New(cli.Options{
		Stdout: stdout, Store: secondStore, CredentialStore: credentials, LookupEnv: lookupEnv, Now: fixedCLINow,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/token/contract":
				return jsonResponse(`{"access_token":"secret-access","refresh_token":"secret-refresh","token_type":"Bearer","scope":"contract:full","expires_in":3600}`), nil
			case "/open-apis/contract/v1/mcp/contracts/contract-1":
				if req.Header.Get("Authorization") != "Bearer secret-access" {
					t.Fatalf("unexpected authorization header: %q", req.Header.Get("Authorization"))
				}
				return jsonResponse(`{"code":0,"data":{"contract":{"contract_id":"contract-1"}}}`), nil
			default:
				t.Fatalf("unexpected rebuilt URL: %s", req.URL)
				return nil, nil
			}
		})},
	})
	if err := secondApp.Run(context.Background(), []string{"auth", "complete", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := secondApp.Run(context.Background(), []string{"contract", "get", "contract-1", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	output := decodeJSONObject(t, stdout.Bytes())
	data, ok := output["data"].(map[string]any)
	if !ok {
		t.Fatalf("business output data = %#v", output["data"])
	}
	contract, ok := data["contract"].(map[string]any)
	if !ok || contract["contract_id"] != "contract-1" {
		t.Fatalf("unexpected business output: %s", stdout.String())
	}

	restored, err := secondStore.GetProfile("contract")
	if err != nil {
		t.Fatal(err)
	}
	if restored.DefaultIdentity != config.IdentityUser || restored.Identities.User.AuthMode != config.UserAuthModeDevice {
		t.Fatalf("restored profile is not a Device user profile: %+v", restored)
	}
	if restored.Identities.User.Token != nil || restored.Identities.App != (config.AppIdentity{}) {
		t.Fatalf("restored profile contains credential-bearing identities: %+v", restored.Identities)
	}
	credentialFiles, err := os.ReadDir(filepath.Join(workspace, ".contract-cli", "credentials"))
	if err != nil {
		t.Fatal(err)
	}
	if len(credentialFiles) != 1 {
		t.Fatalf("credential file count = %d, want 1", len(credentialFiles))
	}
	rawCredential, err := os.ReadFile(filepath.Join(workspace, ".contract-cli", "credentials", credentialFiles[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	for _, plaintext := range []string{"secret-device", "secret-access", "secret-refresh", "legacy-user-access", "app-access", "app-id"} {
		if strings.Contains(string(rawCredential), plaintext) {
			t.Fatalf("workspace credential contains plaintext %q", plaintext)
		}
	}
}

func TestDoubaoWorkTaskRestoresDeviceProfileFromTaskCredential(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	lookupEnv := func(name string) (string, bool) {
		if name == "SESSION_ID" {
			return "doubao-task-a", true
		}
		return "", false
	}
	credentials, err := credential.NewStore(credential.Options{LookupEnv: lookupEnv})
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := fixedCLINow().Add(10 * time.Minute)
	if err := credentials.Save("contract", credential.DeviceCredential{
		Pending: &credential.PendingTransaction{
			Status: credential.PendingStatusPending, DeviceCode: "secret-device", ClientID: "device-client",
			TokenEndpoint: "https://myaccount.qfei.cn/token/contract", ExpiresAt: expiresAt,
		},
		DeviceProfile: &credential.DeviceProfile{
			Name: "contract", Environment: "prod", OpenPlatformBaseURL: "https://open.qfei.cn",
			Resource: "https://open.qfei.cn", BusinessType: "contract", ClientName: "contract-cli",
			DeviceClientID: "device-client", DeviceScope: "contract:full",
			DeviceAuthorizationEndpoint: "https://myaccount.qfei.cn/device",
			TokenEndpoint:               "https://myaccount.qfei.cn/token/contract", RevocationEndpoint: "https://myaccount.qfei.cn/revoke/contract",
		},
	}); err != nil {
		t.Fatal(err)
	}

	store := config.NewStore(t.TempDir())
	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{
		Stdout: stdout, Store: store, LookupEnv: lookupEnv, Now: fixedCLINow,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/token/contract" {
				t.Fatalf("unexpected URL: %s", req.URL)
			}
			return jsonResponse(`{"access_token":"secret-access","refresh_token":"secret-refresh","token_type":"Bearer","scope":"contract:full","expires_in":3600}`), nil
		})},
	})
	if err := app.Run(context.Background(), []string{"auth", "complete", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"status":"succeeded"`) {
		t.Fatalf("unexpected auth output: %s", stdout.String())
	}
	restored, err := store.GetProfile("contract")
	if err != nil {
		t.Fatal(err)
	}
	if restored.Identities.User.AuthMode != config.UserAuthModeDevice || restored.Identities.User.Token != nil {
		t.Fatalf("unexpected restored profile: %+v", restored)
	}
}

func TestDoubaoWorkTaskReusesPendingWithinTaskAndIsolatesNewTask(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	store := config.NewStore(t.TempDir())
	profile := config.Profile{
		Name: "contract", Environment: "prod", Resource: "https://open.qfei.cn", BusinessType: "contract",
		Identities: config.Identities{User: config.UserIdentity{
			DeviceAuthorizationEndpoint: "https://myaccount.qfei.cn/device", TokenEndpoint: "https://myaccount.qfei.cn/token/contract",
			DeviceClientID: "device-client", DeviceScope: "contract:full",
		}},
	}
	if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
		t.Fatal(err)
	}
	sessionID := "doubao-task-a"
	lookupEnv := func(name string) (string, bool) {
		if name == "SESSION_ID" {
			return sessionID, true
		}
		return "", false
	}
	requestCount := 0
	newApp := func() *cli.App {
		return cli.New(cli.Options{
			Stdout: &bytes.Buffer{}, Store: store, LookupEnv: lookupEnv, Now: fixedCLINow,
			HTTPClient: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				requestCount++
				return jsonResponse(fmt.Sprintf(
					`{"device_code":"device-%d","user_code":"user-%d","verification_uri":"https://myaccount.qfei.cn/device","verification_uri_complete":"https://myaccount.qfei.cn/device?user_code=user-%d","expires_in":600}`,
					requestCount, requestCount, requestCount,
				)), nil
			})},
		})
	}

	for range 2 {
		if err := newApp().Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json"}); err != nil {
			t.Fatal(err)
		}
	}
	if requestCount != 1 {
		t.Fatalf("same-task init requests = %d, want 1", requestCount)
	}

	sessionID = "doubao-task-b"
	if err := newApp().Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	if requestCount != 2 {
		t.Fatalf("new-task init requests = %d, want 2", requestCount)
	}
}

func TestDoubaoRebuildRejectsMissingOrInvalidDeviceProfileSnapshot(t *testing.T) {
	validSnapshot := &credential.DeviceProfile{
		Name:                        "contract",
		Environment:                 "prod",
		OpenPlatformBaseURL:         "https://open.qfei.cn",
		Resource:                    "https://open.qfei.cn",
		BusinessType:                "contract",
		DeviceClientID:              "device-client",
		DeviceScope:                 "contract:full",
		DeviceAuthorizationEndpoint: "https://myaccount.qfei.cn/device",
		TokenEndpoint:               "https://myaccount.qfei.cn/token/contract",
	}
	tests := []struct {
		name     string
		snapshot *credential.DeviceProfile
	}{
		{name: "missing snapshot"},
		{name: "mismatched profile", snapshot: func() *credential.DeviceProfile {
			value := *validSnapshot
			value.Name = "another-profile"
			return &value
		}()},
		{name: "missing required field", snapshot: func() *credential.DeviceProfile {
			value := *validSnapshot
			value.DeviceClientID = ""
			return &value
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credentials := &memoryDeviceCredentialStore{values: map[string]credential.DeviceCredential{
				"contract": {
					Pending: &credential.PendingTransaction{
						DeviceCode: "secret-device", TokenEndpoint: "https://myaccount.qfei.cn/token/contract",
						ClientID: "device-client", ExpiresAt: fixedCLINow().Add(time.Minute),
					},
					DeviceProfile: tt.snapshot,
				},
			}}
			calls := 0
			app := cli.New(cli.Options{
				Store:           config.NewStore(t.TempDir()),
				CredentialStore: credentials,
				LookupEnv: func(name string) (string, bool) {
					if name == "SKILL_SESSION_WORKSPACE" {
						return t.TempDir(), true
					}
					return "", false
				},
				HTTPClient: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
					calls++
					return nil, errors.New("unexpected HTTP request")
				})},
			})

			err := app.Run(context.Background(), []string{"auth", "complete", "--profile", "contract", "--output", "json"})
			if err == nil || !strings.Contains(err.Error(), "cannot restore Device profile") || !strings.Contains(err.Error(), "config add") {
				t.Fatalf("error = %v", err)
			}
			if calls != 0 {
				t.Fatalf("HTTP calls = %d, want 0", calls)
			}
		})
	}
}

func TestAuthDeviceCompleteMapsSlowDownToPendingWithoutRetry(t *testing.T) {
	stdout := &bytes.Buffer{}
	store := config.NewStore(t.TempDir())
	profile := config.Profile{Name: "contract", Environment: "prod", Identities: config.Identities{User: config.UserIdentity{AuthMode: config.UserAuthModeDevice}}}
	if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
		t.Fatal(err)
	}
	credentials := &memoryDeviceCredentialStore{values: map[string]credential.DeviceCredential{
		"contract": {Pending: &credential.PendingTransaction{
			DeviceCode: "device-a", TokenEndpoint: "https://myaccount.qfei.cn/token/contract", ClientID: "client-a", ExpiresAt: fixedCLINow().Add(time.Minute),
		}},
	}}
	calls := 0
	app := cli.New(cli.Options{
		Stdout: stdout, Store: store, CredentialStore: credentials, Now: fixedCLINow,
		LookupEnv: func(name string) (string, bool) {
			if name == "SKILL_SESSION_WORKSPACE" {
				return t.TempDir(), true
			}
			return "", false
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			calls++
			response := jsonResponse(`{"error":"slow_down","error_description":"too many requests"}`)
			response.StatusCode = http.StatusTooManyRequests
			return response, nil
		})},
	})

	if err := app.Run(context.Background(), []string{"auth", "complete", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !strings.Contains(stdout.String(), `"status":"pending"`) {
		t.Fatalf("calls=%d output=%s", calls, stdout.String())
	}
}

func TestAuthDeviceCompleteInvalidGrantKeepsTerminalStateAndRequiresExplicitRestart(t *testing.T) {
	stdout := &bytes.Buffer{}
	store := config.NewStore(t.TempDir())
	profile := config.Profile{Name: "contract", Environment: "prod", Identities: config.Identities{User: config.UserIdentity{AuthMode: config.UserAuthModeDevice}}}
	if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
		t.Fatal(err)
	}
	credentials := &memoryDeviceCredentialStore{values: map[string]credential.DeviceCredential{
		"contract": {Pending: &credential.PendingTransaction{
			DeviceCode: "device-a", TokenEndpoint: "https://myaccount.qfei.cn/token/contract", ClientID: "client-a", ExpiresAt: fixedCLINow().Add(time.Minute),
		}},
	}}
	workspace := t.TempDir()
	app := cli.New(cli.Options{
		Stdout: stdout, Store: store, CredentialStore: credentials, Now: fixedCLINow,
		LookupEnv: func(name string) (string, bool) {
			if name == "SKILL_SESSION_WORKSPACE" {
				return workspace, true
			}
			return "", false
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			response := jsonResponse(`{"error":"invalid_grant","error_description":"already consumed"}`)
			response.StatusCode = http.StatusBadRequest
			return response, nil
		})},
	})
	if err := app.Run(context.Background(), []string{"auth", "complete", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"status":"restart_required"`) {
		t.Fatalf("output = %s", stdout.String())
	}
	stored, loadErr := credentials.Load("contract")
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if stored.Pending == nil || stored.Pending.EffectiveStatus() != credential.PendingStatusInvalidGrant {
		t.Fatalf("consumed pending transaction should remain terminal: %+v", stored.Pending)
	}
}

func TestAuthDeviceCompleteSaveFailureRequiresFreshAuthorizationWithoutRetry(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	store := config.NewStore(t.TempDir())
	profile := config.Profile{Name: "contract", Environment: "prod", Identities: config.Identities{User: config.UserIdentity{AuthMode: config.UserAuthModeDevice}}}
	if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
		t.Fatal(err)
	}
	credentials := &memoryDeviceCredentialStore{
		values: map[string]credential.DeviceCredential{
			"contract": {Pending: &credential.PendingTransaction{
				DeviceCode: "secret-device", TokenEndpoint: "https://myaccount.qfei.cn/token/contract", ClientID: "client-a", ExpiresAt: fixedCLINow().Add(time.Minute),
			}},
		},
		saveErr:       errors.New("secure credential store unavailable"),
		saveErrOnCall: 2,
	}
	calls := 0
	app := cli.New(cli.Options{
		Stdout: stdout, Stderr: stderr, Store: store, CredentialStore: credentials, Now: fixedCLINow,
		LookupEnv: func(name string) (string, bool) {
			if name == "SKILL_SESSION_WORKSPACE" {
				return t.TempDir(), true
			}
			return "", false
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			calls++
			return jsonResponse(`{"access_token":"secret-access","refresh_token":"secret-refresh","token_type":"Bearer","scope":"contract:full","expires_in":3600}`), nil
		})},
	})

	err := app.Run(context.Background(), []string{"auth", "complete", "--profile", "contract", "--output", "json"})
	if err == nil || !strings.Contains(err.Error(), "authorization result is uncertain") || !strings.Contains(err.Error(), "do not retry auth complete") {
		t.Fatalf("error = %v, want uncertain authorization instruction", err)
	}
	if calls != 1 {
		t.Fatalf("token endpoint calls = %d, want 1", calls)
	}
	combinedOutput := err.Error() + stdout.String() + stderr.String()
	for _, secret := range []string{"secret-device", "secret-access", "secret-refresh"} {
		if strings.Contains(combinedOutput, secret) {
			t.Fatalf("auth complete save failure exposed %q: %s", secret, combinedOutput)
		}
	}
}

func TestAuthDeviceStatusAndLogoutUseCredentialStoreAndRevoke(t *testing.T) {
	stdout := &bytes.Buffer{}
	store := config.NewStore(t.TempDir())
	workspace := t.TempDir()
	credentials := &memoryDeviceCredentialStore{values: map[string]credential.DeviceCredential{
		"contract": {Token: &config.Token{
			AccessToken: "secret-access", RefreshToken: "secret-refresh", Scope: "contract:full contract-review:full", Expiry: fixedCLINow().Add(time.Hour),
		}},
	}}
	profile := config.Profile{
		Name: "contract", Environment: "prod", DefaultIdentity: config.IdentityUser,
		Identities: config.Identities{User: config.UserIdentity{
			AuthMode: config.UserAuthModeDevice, DeviceClientID: "zscli_892efdadc11a3f53",
			RevocationEndpoint: "https://myaccount.qfei.cn/revoke/contract",
		}},
	}
	if err := store.UpsertProfile(productionProfileFixture(profile), true); err != nil {
		t.Fatal(err)
	}
	revokeCalls := 0
	app := cli.New(cli.Options{
		Stdout: stdout, Store: store, CredentialStore: credentials, Now: fixedCLINow,
		LookupEnv: func(name string) (string, bool) {
			if name == "SKILL_SESSION_WORKSPACE" {
				return workspace, true
			}
			return "", false
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			revokeCalls++
			if err := req.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if req.Form.Get("token") != "secret-refresh" || req.Form.Get("client_id") != "zscli_892efdadc11a3f53" {
				t.Fatalf("unexpected revoke form: %v", req.Form)
			}
			return &http.Response{StatusCode: http.StatusNoContent, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
		})},
	})
	if err := app.Run(context.Background(), []string{"auth", "status", "--profile", "contract", "--as", "user"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Authorization: authorized") ||
		!strings.Contains(stdout.String(), "Scope: contract:full contract-review:full") ||
		strings.Contains(stdout.String(), "secret-") {
		t.Fatalf("status output = %s", stdout.String())
	}
	stdout.Reset()
	if err := app.Run(context.Background(), []string{"auth", "logout", "--profile", "contract", "--as", "user"}); err != nil {
		t.Fatal(err)
	}
	if revokeCalls != 1 {
		t.Fatalf("revoke calls = %d", revokeCalls)
	}
	if _, err := credentials.Load("contract"); !errors.Is(err, credential.ErrCredentialNotFound) {
		t.Fatalf("credential must be deleted, err=%v", err)
	}
	if strings.Contains(stdout.String(), "secret-") {
		t.Fatalf("logout output exposed credential: %s", stdout.String())
	}
}

type memoryDeviceCredentialStore struct {
	values        map[string]credential.DeviceCredential
	loadErr       error
	saveErr       error
	saveErrOnCall int
	saveCalls     int
}

func (s *memoryDeviceCredentialStore) Load(profileName string) (credential.DeviceCredential, error) {
	if s.loadErr != nil {
		return credential.DeviceCredential{}, s.loadErr
	}
	value, ok := s.values[profileName]
	if !ok {
		return credential.DeviceCredential{}, credential.ErrCredentialNotFound
	}
	return value, nil
}

func (s *memoryDeviceCredentialStore) Save(profileName string, value credential.DeviceCredential) error {
	s.saveCalls++
	if s.saveErr != nil && (s.saveErrOnCall == 0 || s.saveCalls == s.saveErrOnCall) {
		return s.saveErr
	}
	s.values[profileName] = value
	return nil
}

func (s *memoryDeviceCredentialStore) Delete(profileName string) error {
	delete(s.values, profileName)
	return nil
}

func fixedCLINow() time.Time {
	return time.Date(2026, 4, 20, 16, 0, 0, 0, time.FixedZone("CST", 8*60*60))
}

func decodeJSONObject(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("Unmarshal(%s) error = %v", string(data), err)
	}
	return value
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
