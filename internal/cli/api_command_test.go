package cli_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestAPICallUsesDefaultIdentityAndRendersJSON(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	store := config.NewStore(t.TempDir())
	profile := config.Profile{
		Name:                "contract-group",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityBot,
		Identities: config.Identities{
			Bot: config.BotIdentity{
				Token: &config.Token{
					AccessToken: "bot-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: stderr,
		Store:  store,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://dev-open.qtech.cn/open-apis/mdm/v1/vendors/1063197165850985296" {
					t.Fatalf("unexpected request url: %s", req.URL.String())
				}
				if req.Header.Get("Authorization") != "Bearer bot-token" {
					t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
				}
				return jsonResponse(`{"code":0,"data":{"vendorId":"1063197165850985296"}}`), nil
			}),
		},
	})

	if err := app.Run(context.Background(), []string{
		"api", "call", "GET", "/open-apis/mdm/v1/vendors/1063197165850985296", "--profile", "contract-group",
	}); err != nil {
		t.Fatalf("api call error = %v", err)
	}

	if !strings.Contains(stdout.String(), `"vendorId": "1063197165850985296"`) {
		t.Fatalf("unexpected api call output: %s", stdout.String())
	}
}

func TestAPICallSupportsRawOutputAndInputFileBody(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	bodyFile := filepath.Join(dir, "vendor.json")
	if err := os.WriteFile(bodyFile, []byte(`{"vendor":"V00108006"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	profile := config.Profile{
		Name:                "contract-group",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityBot,
		Identities: config.Identities{
			Bot: config.BotIdentity{
				Token: &config.Token{
					AccessToken: "bot-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: stderr,
		Store:  store,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost {
					t.Fatalf("method = %s", req.Method)
				}
				if req.Header.Get("Content-Type") != "application/json" {
					t.Fatalf("content-type = %q", req.Header.Get("Content-Type"))
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("ReadAll() error = %v", err)
				}
				if string(body) != `{"vendor":"V00108006"}` {
					t.Fatalf("body = %s", string(body))
				}
				return jsonResponse(`{"ok":true}`), nil
			}),
		},
	})

	if err := app.Run(context.Background(), []string{
		"api", "call", "POST", "/open-apis/mdm/v1/vendors", "--profile", "contract-group", "--input-file", bodyFile, "--raw",
	}); err != nil {
		t.Fatalf("api call error = %v", err)
	}

	if stdout.String() != `{"ok":true}` {
		t.Fatalf("unexpected raw output: %s", stdout.String())
	}
}

func TestAPICallRejectsAbsoluteURLAndMissingOpenPlatformBaseURL(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(config.Profile{
		Name:            "contract-group",
		Environment:     "dev",
		DefaultIdentity: config.IdentityBot,
		Identities: config.Identities{
			Bot: config.BotIdentity{
				Token: &config.Token{
					AccessToken: "bot-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: stderr,
		Store:  store,
	})

	err := app.Run(context.Background(), []string{
		"api", "call", "GET", "/open-apis/mdm/v1/vendors/1", "--profile", "contract-group",
	})
	if err == nil || !strings.Contains(err.Error(), "open platform base url is not configured") {
		t.Fatalf("unexpected missing-base-url error: %v", err)
	}

	err = app.Run(context.Background(), []string{
		"api", "call", "GET", "https://dev-open.qtech.cn/open-apis/mdm/v1/vendors/1", "--profile", "contract-group",
	})
	if err == nil || !strings.Contains(err.Error(), "must be a relative /open-apis/ path") {
		t.Fatalf("unexpected absolute-url error: %v", err)
	}
}

func TestAPICallRejectsConflictingBodyInputs(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	bodyFile := filepath.Join(dir, "vendor.json")
	if err := os.WriteFile(bodyFile, []byte(`{"vendor":"V00108006"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := store.UpsertProfile(config.Profile{
		Name:                "contract-group",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityBot,
		Identities: config.Identities{
			Bot: config.BotIdentity{
				Token: &config.Token{
					AccessToken: "bot-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: stderr,
		Store:  store,
	})

	err := app.Run(context.Background(), []string{
		"api", "call", "POST", "/open-apis/mdm/v1/vendors", "--profile", "contract-group", "--input-file", bodyFile, "--data", `{"vendor":"inline"}`,
	})
	if err == nil || !strings.Contains(err.Error(), "only one of --input-file or --data may be provided") {
		t.Fatalf("unexpected conflicting-body-input error: %v", err)
	}
}

func TestAPICallUserOnlyPathDefaultsToUserIdentity(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	store := config.NewStore(t.TempDir())
	profile := config.Profile{
		Name:                "contract-group",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityBot,
		Identities: config.Identities{
			User: config.UserIdentity{
				Token: &config.Token{
					AccessToken: "user-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
			Bot: config.BotIdentity{
				Token: &config.Token{
					AccessToken: "bot-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: stderr,
		Store:  store,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Header.Get("Authorization") != "Bearer user-token" {
					t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
				}
				return jsonResponse(`{"code":0,"data":{"vendor_id":"123"}}`), nil
			}),
		},
	})

	if err := app.Run(context.Background(), []string{
		"api", "call", "GET", "/open-apis/contract/v1/mcp/vendors/123", "--profile", "contract-group",
	}); err != nil {
		t.Fatalf("api call error = %v", err)
	}

	if !strings.Contains(stdout.String(), `"vendor_id": "123"`) {
		t.Fatalf("unexpected api call output: %s", stdout.String())
	}
}

func TestAPICallRejectsBotForUserOnlyPathAndLegacyFileFlag(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	dir := t.TempDir()
	store := config.NewStore(dir)
	bodyFile := filepath.Join(dir, "vendor.json")
	if err := os.WriteFile(bodyFile, []byte(`{"vendor":"V00108006"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := store.UpsertProfile(config.Profile{
		Name:                "contract-group",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityBot,
		Identities: config.Identities{
			User: config.UserIdentity{
				Token: &config.Token{
					AccessToken: "user-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
			Bot: config.BotIdentity{
				Token: &config.Token{
					AccessToken: "bot-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: stderr,
		Store:  store,
	})

	err := app.Run(context.Background(), []string{
		"api", "call", "GET", "/open-apis/contract/v1/mcp/vendors/123", "--profile", "contract-group", "--as", "bot",
	})
	if err == nil || !strings.Contains(err.Error(), "only supports --as user") {
		t.Fatalf("unexpected bot user-only error: %v", err)
	}

	err = app.Run(context.Background(), []string{
		"api", "call", "POST", "/open-apis/mdm/v1/vendors", "--profile", "contract-group", "--file", bodyFile,
	})
	if err == nil || !strings.Contains(err.Error(), `unknown flag "--file"`) {
		t.Fatalf("unexpected legacy file flag error: %v", err)
	}
}
