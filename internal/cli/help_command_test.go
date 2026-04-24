package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestHelpRequestsRenderExpectedTopics(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		args     []string
		contains []string
	}{
		{
			name: "top level help flag",
			args: []string{"--help"},
			contains: []string{
				"Name:",
				"contract-cli",
				"Usage:",
				"contract-cli <command> [flags]",
				"Commands:",
				"contract-cli contract <subcommand> [flags]",
				"contract-cli mdm vendor <subcommand> [flags]",
			},
		},
		{
			name: "help command path",
			args: []string{"help", "contract", "upload-file"},
			contains: []string{
				"Name:",
				"contract upload-file",
				"Usage:",
				"contract-cli contract upload-file --file <path> --file-type <type> [flags]",
				"--file <path>",
				"--file-type <type>",
				"bot-only",
				"200MB",
				"不接受 --input-file / --data",
			},
		},
		{
			name: "subcommand help flag",
			args: []string{"contract", "upload-file", "--help"},
			contains: []string{
				"contract upload-file",
				"--file-name <name>",
				"multipart/form-data",
			},
		},
		{
			name: "leaf help ignores positional example",
			args: []string{"contract", "get", "contract-1", "--help"},
			contains: []string{
				"Name:",
				"contract get",
				"Usage:",
				"contract-cli contract get <contract-id> [flags]",
				"--user-id-type <type>",
			},
		},
		{
			name: "contract search explains identity routing",
			args: []string{"contract", "search", "--help"},
			contains: []string{
				"contract search",
				"--contract-number <number>",
				"user: /open-apis/contract/v1/mcp/contracts/search",
				"bot: /open-apis/contract/v1/contracts/search",
			},
		},
		{
			name: "contract enum list user only",
			args: []string{"contract", "enum", "list", "--help"},
			contains: []string{
				"contract enum list",
				"--type <enum-type>",
				"仅支持 --as user",
			},
		},
		{
			name: "mdm vendor list flags",
			args: []string{"mdm", "vendor", "list", "--help"},
			contains: []string{
				"mdm vendor list",
				"--name <name>",
				"--page-size <n>",
				"--page-token <token>",
			},
		},
		{
			name: "auth login flags",
			args: []string{"auth", "login", "--help"},
			contains: []string{
				"auth login",
				"--timeout <duration>",
				"--app-id <id>",
				"--app-secret <secret>",
			},
		},
		{
			name: "skills install flags",
			args: []string{"skills", "install", "--help"},
			contains: []string{
				"skills install",
				"--target <dir>",
				"--force",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stdout := &bytes.Buffer{}
			app := cli.New(cli.Options{
				Stdout: stdout,
				Stderr: &bytes.Buffer{},
				Store:  config.NewStore(t.TempDir()),
			})

			if err := app.Run(context.Background(), tc.args); err != nil {
				t.Fatalf("Run(%v) error = %v", tc.args, err)
			}
			for _, want := range tc.contains {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("help output missing %q:\n%s", want, stdout.String())
				}
			}
		})
	}
}

func TestHelpDoesNotTriggerProfilesHTTPUpdateOrLogs(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	requests := 0
	store := config.NewStore(t.TempDir())
	app := cli.New(cli.Options{
		Stdout:               stdout,
		Stderr:               stderr,
		Store:                store,
		UpdateRegistryURL:    "https://registry.test/@qfeius%2fcontract-cli",
		UpdateCurrentVersion: "0.1.0-beta.1",
		IsTerminal:           func(io.Writer) bool { return true },
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				return nil, fmt.Errorf("help must not send HTTP request to %s", req.URL.String())
			}),
		},
	})

	if err := app.Run(context.Background(), []string{"contract", "upload-file", "--help"}); err != nil {
		t.Fatalf("Run(help) error = %v", err)
	}
	if requests != 0 {
		t.Fatalf("help sent %d HTTP requests, want 0", requests)
	}
	if stderr.String() != "" {
		t.Fatalf("help should not log to stderr, got: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "contract upload-file") {
		t.Fatalf("missing help output: %s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(store.Path()), "update-check.json")); !os.IsNotExist(err) {
		t.Fatalf("help should not write update cache, stat error = %v", err)
	}
}

func TestAllCurrentHelpTopicsRender(t *testing.T) {
	t.Parallel()

	topics := []string{
		"version",
		"config",
		"config add",
		"auth",
		"auth login",
		"auth status",
		"auth logout",
		"auth use",
		"skills",
		"skills list",
		"skills install",
		"update",
		"update check",
		"contract",
		"contract search",
		"contract get",
		"contract sync-user-groups",
		"contract text",
		"contract create",
		"contract upload-file",
		"contract category",
		"contract category list",
		"contract template",
		"contract template list",
		"contract template get",
		"contract template instantiate",
		"contract enum",
		"contract enum list",
		"mdm",
		"mdm vendor",
		"mdm vendor list",
		"mdm vendor get",
		"mdm legal",
		"mdm legal list",
		"mdm legal get",
		"mdm fields",
		"mdm fields list",
	}

	for _, topic := range topics {
		topic := topic
		t.Run(topic, func(t *testing.T) {
			t.Parallel()

			stdout := &bytes.Buffer{}
			app := cli.New(cli.Options{
				Stdout: stdout,
				Stderr: &bytes.Buffer{},
				Store:  config.NewStore(t.TempDir()),
			})
			args := append([]string{"help"}, strings.Fields(topic)...)
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatalf("Run(%v) error = %v", args, err)
			}
			if !strings.Contains(stdout.String(), "Name:\n  "+topic) {
				t.Fatalf("help output for %q has unexpected name:\n%s", topic, stdout.String())
			}
		})
	}
}

func TestUnknownHelpTopicReturnsClearError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		args    []string
		wantErr string
	}{
		{
			args:    []string{"help", "vendor"},
			wantErr: `unknown help topic "vendor"`,
		},
		{
			args:    []string{"contract", "unknown", "--help"},
			wantErr: `unknown help topic "contract unknown"`,
		},
		{
			args:    []string{"help", "api", "call"},
			wantErr: `unknown help topic "api call"`,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			t.Parallel()

			app := cli.New(cli.Options{
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
				Store:  config.NewStore(t.TempDir()),
			})

			err := app.Run(context.Background(), tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), "contract-cli help") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
