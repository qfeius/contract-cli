package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	updatecheck "cn.qfei/contract-cli/internal/update"
)

func TestUpdateCheckExplicitJSONPreservesLegacyPayload(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, current, latest, action string
		calls                         int
	}{
		{"update available", "1.9.7", "1.9.8", "update_available", 1},
		{"already current", "1.9.7", "1.9.7", "already_up_to_date", 1},
		{"development version skips HTTP", "dev", "", "skipped", 0},
	} {
		for _, flags := range [][]string{
			{"--json"},
			{"--output", "json"},
			{"--output=json"},
			{"--json=false", "--output", "json"},
			{"--output=json", "--json=false"},
			{"--json", "--output", ""},
		} {
			t.Run(test.name+"/"+strings.Join(flags, " "), func(t *testing.T) {
				t.Parallel()
				calls := 0
				app, stdout, stderr, _ := newUpdateJSONOutputApp(t, test.current, func(*http.Request) (*http.Response, error) {
					calls++
					return jsonResponse(`{"dist-tags":{"latest":"` + test.latest + `"}}`), nil
				})
				if code := app.Execute(context.Background(), append([]string{"update", "check"}, flags...)); code != 0 {
					t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
				}
				var result map[string]any
				if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
					t.Fatalf("update JSON is not one object: %s: %v", stdout, err)
				}
				if calls != test.calls || stderr.Len() != 0 || result["ok"] != true || result["package"] != "@qfeius/contract-cli" ||
					result["previous_version"] != test.current || result["current_version"] != test.current || result["latest_version"] != test.latest ||
					result["channel"] != "latest" || result["action"] != test.action || result["message"] == "" {
					t.Fatalf("legacy update payload changed: calls=%d stdout=%s stderr=%s", calls, stdout, stderr)
				}
				if _, nested := result["data"]; nested {
					t.Fatalf("legacy update payload must remain flat: %s", stdout)
				}
				if _, notice := result["_notice"]; notice {
					t.Fatalf("manual update check must not inject _notice: %s", stdout)
				}
				if test.action == "update_available" && (result["auto_update"] != false || result["command"] != updatecheck.InstallCommand("@qfeius/contract-cli", "latest")) {
					t.Fatalf("update install instructions changed: %s", stdout)
				}
				if test.action == "skipped" && result["reason"] != "current version is dev/unknown; skip remote update check" {
					t.Fatalf("skip reason changed: %s", stdout)
				}
			})
		}
	}
}

func TestUpdateCheckDefaultAndFalseAliasKeepText(t *testing.T) {
	t.Parallel()
	for _, flags := range [][]string{nil, {"--json=false"}} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			t.Parallel()
			app, stdout, stderr, _ := newUpdateJSONOutputApp(t, "1.9.7", func(*http.Request) (*http.Response, error) {
				return jsonResponse(`{"dist-tags":{"latest":"1.9.8"}}`), nil
			})
			if code := app.Execute(context.Background(), append([]string{"update", "check"}, flags...)); code != 0 {
				t.Fatalf("code=%d stderr=%s", code, stderr)
			}
			want := "Update available: contract-cli 1.9.7 -> 1.9.8\nRun: " + updatecheck.InstallCommand("@qfeius/contract-cli", "latest") + "\n"
			if stdout.String() != want || stderr.Len() != 0 {
				t.Fatalf("default update text changed: stdout=%s stderr=%s", stdout, stderr)
			}
		})
	}
}

func TestUpdateCheckExplicitJSONKeepsDebugLogsJSONWhenAliasFalse(t *testing.T) {
	t.Parallel()
	for _, flags := range [][]string{{"--output=json", "--json=false"}, {"--json=false", "--output", "json"}, {"--json", "--output", ""}} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			t.Parallel()
			app, stdout, stderr, _ := newUpdateJSONOutputApp(t, "1.9.7", func(*http.Request) (*http.Response, error) {
				return jsonResponse(`{"dist-tags":{"latest":"1.9.8"}}`), nil
			})
			args := append([]string{"update", "check", "--debug"}, flags...)
			if code := app.Execute(context.Background(), args); code != 0 || !json.Valid(stdout.Bytes()) || stderr.Len() == 0 {
				t.Fatalf("JSON debug execution failed: code=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
			for _, line := range bytes.Split(bytes.TrimSpace(stderr.Bytes()), []byte("\n")) {
				if !json.Valid(line) {
					t.Fatalf("explicit --output json must win over --json=false for diagnostics: %s", stderr)
				}
			}
		})
	}
}

func TestUpdateCheckOutputValidationPrecedesHTTP(t *testing.T) {
	t.Parallel()
	for _, format := range []string{"yaml", "table", "JSON"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			calls := 0
			app, stdout, stderr, _ := newUpdateJSONOutputApp(t, "1.9.7", func(*http.Request) (*http.Response, error) {
				calls++
				return jsonResponse(`{"dist-tags":{"latest":"1.9.8"}}`), nil
			})
			code := app.Execute(context.Background(), []string{"update", "check", "--json", "--output", format})
			if code != 1 || calls != 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "unsupported output format") {
				t.Fatalf("format validation did not precede HTTP: code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
			}
		})
	}
}

func TestUpdateCheckJSONFailureAndCancellation(t *testing.T) {
	t.Parallel()
	for _, flags := range [][]string{{"--output", "json"}, {"--output=json"}, {"--json"}, {"--json", "--output", ""}} {
		for _, failure := range []string{"HTTP", "transport", "cancelled", "invalid flag"} {
			t.Run(failure+"/"+strings.Join(flags, " "), func(t *testing.T) {
				t.Parallel()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				calls := 0
				app, stdout, stderr, _ := newUpdateJSONOutputApp(t, "1.9.7", func(request *http.Request) (*http.Response, error) {
					calls++
					switch failure {
					case "HTTP":
						response := jsonResponse(`{"message":"fixture registry unavailable"}`)
						response.StatusCode = http.StatusServiceUnavailable
						return response, nil
					case "transport":
						return nil, errors.New("fixture transport unavailable")
					case "cancelled":
						cancel()
						return nil, request.Context().Err()
					default:
						t.Fatal("invalid flags reached HTTP")
						return nil, nil
					}
				})
				args := append([]string{"update", "check"}, flags...)
				if failure == "invalid flag" {
					args = append(args, "--unknown")
				}
				wantCode, wantCalls := 1, 1
				if failure == "cancelled" {
					wantCode = 130
				}
				if failure == "invalid flag" {
					wantCalls = 0
				}
				code := app.Execute(ctx, args)
				var result struct {
					OK    bool `json:"ok"`
					Error struct {
						Message   string `json:"message"`
						Cancelled bool   `json:"cancelled"`
					} `json:"error"`
				}
				if err := json.Unmarshal(stderr.Bytes(), &result); err != nil {
					t.Fatalf("failure must be one JSON object: code=%d calls=%d stderr=%s: %v", code, calls, stderr, err)
				}
				if code != wantCode || calls != wantCalls || stdout.Len() != 0 || result.OK || result.Error.Message == "" || result.Error.Cancelled != (failure == "cancelled") {
					t.Fatalf("wrong update failure: code=%d calls=%d stdout=%s stderr=%s", code, calls, stdout, stderr)
				}
				if failure == "HTTP" && !strings.Contains(result.Error.Message, "npm registry returned status 503") {
					t.Fatalf("registry HTTP error was replaced: %s", stderr)
				}
			})
		}
	}
}

func TestUpdateCheckJSONDoesNotInjectCachedNotice(t *testing.T) {
	t.Parallel()
	app, stdout, stderr, state := newUpdateJSONOutputApp(t, "1.9.7", func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"dist-tags":{"latest":"1.9.8"}}`), nil
	})
	cache := updatecheck.Cache{CheckedAt: time.Now(), Channel: "latest", CurrentVersion: "1.9.7", LatestVersion: "9.9.9", UpdateAvailable: true}
	if err := updatecheck.SaveCache(filepath.Join(state, "update-check.json"), cache); err != nil {
		t.Fatal(err)
	}
	if code := app.Execute(context.Background(), []string{"update", "check", "--output", "json"}); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if _, notice := result["_notice"]; notice || result["latest_version"] != "1.9.8" || stderr.Len() != 0 {
		t.Fatalf("manual update used automatic cached notice: stdout=%s stderr=%s", stdout, stderr)
	}
}

func newUpdateJSONOutputApp(t *testing.T, current string, transport roundTripFunc) (*cli.App, *bytes.Buffer, *bytes.Buffer, string) {
	t.Helper()
	state := t.TempDir()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	app := cli.New(cli.Options{
		Store: config.NewStore(state), Stdout: stdout, Stderr: stderr,
		HTTPClient:           &http.Client{Transport: transport},
		UpdateCurrentVersion: current, UpdateRegistryURL: "https://registry.example.test/package",
		LookupEnv: func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
	})
	return app, stdout, stderr, state
}
