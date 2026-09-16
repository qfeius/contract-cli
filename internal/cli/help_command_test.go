package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

/*
TestHelpRequestsRenderExpectedTopics 验证各级帮助请求展示当前命令参数与关键约束。
入参 t（*testing.T）为 Go 测试上下文。
返回值为空；失败通过 t.Fatalf 报告。
*/
func TestHelpRequestsRenderExpectedTopics(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		args        []string
		contains    []string
		notContains []string
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
				"contract-cli event outbound-ip list [flags]",
				"contract-cli rule table <subcommand> [flags]",
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
				"user/app",
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
			name: "contract download file help",
			args: []string{"contract", "download-file", "--help"},
			contains: []string{
				"contract download-file",
				"--output-file <path>",
				"--force",
				"默认拉起保存文件弹窗",
				"app-only",
			},
		},
		{
			name: "contract patch help",
			args: []string{"contract", "patch", "--help"},
			contains: []string{
				"contract patch",
				"contract-cli contract patch <contract-id> --input-file <path>|--data <json> [flags]",
				"PATCH /open-apis/contract/v1/contracts/{contract_id}",
				"app-only",
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
				"app: /open-apis/contract/v1/contracts/search",
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
			name: "contract search v2 app only",
			args: []string{"contract", "search-v2", "--help"},
			contains: []string{
				"contract search-v2",
				"POST /open-apis/contract/v1/contracts/searchV2",
				"app-only",
				"--input-file <path>",
			},
		},
		{
			name: "contract sign url get help",
			args: []string{"contract", "sign-url", "get", "--help"},
			contains: []string{
				"contract sign-url get",
				"GET /open-apis/contract/v1/contracts/{contract_id}/sign_url",
				"app-only",
				"不接受 --input-file / --data",
			},
		},
		{
			name: "contract cooperation search help",
			args: []string{"contract", "cooperation", "search", "--help"},
			contains: []string{
				"contract cooperation search",
				"POST /open-apis/contract/v1/cooperation/search",
				"--input-file <path>",
				"app-only",
			},
		},
		{
			name: "approval matrix import plan help",
			args: []string{"rule", "table", "import", "plan", "--help"},
			contains: []string{
				"rule table import plan",
				"--product-id <id>",
				"--table-id <id>",
				"--input-file <path>",
				"status=needs_confirmation",
				"不支持 --raw",
			},
		},
		{
			name: "approval matrix import apply help",
			args: []string{"rule", "table", "import", "apply", "--help"},
			contains: []string{
				"rule table import apply",
				"--plan-id <id>",
				"partial_success",
				"支持 --as user / --as app",
			},
			notContains: []string{
				"--product-id <id>",
				"--input-file <path>",
			},
		},
		{
			name: "mdm fixed exchange rate help",
			args: []string{"mdm", "fixed-exchange-rate", "get", "--help"},
			contains: []string{
				"mdm fixed-exchange-rate get",
				"--source-currency <code>",
				"GET /open-apis/mdm/v1/fixed_exchange_rate",
				"query 参数 date",
				"app-only",
			},
		},
		{
			name: "mdm vendor create help",
			args: []string{"mdm", "vendor", "create", "--help"},
			contains: []string{
				"mdm vendor create",
				"--user-id <id>",
				"必传 --user-id",
				"不要传后端生成的 vendor 编码",
			},
		},
		{
			name: "mdm legal update help",
			args: []string{"mdm", "legal", "update", "--help"},
			contains: []string{
				"mdm legal update",
				"必传 --user-id",
				"id 和 legalEntity",
				"不要用 legal_entity",
			},
		},
		{
			name: "event outbound ip help",
			args: []string{"event", "outbound-ip", "list", "--help"},
			contains: []string{
				"event outbound-ip list",
				"GET /open-apis/event/v1/outbound_ip",
				"--page-size <n>",
				"10-50",
				"app-only",
			},
		},
		{
			name: "rule table row create help",
			args: []string{"rule", "table", "row", "create", "--help"},
			contains: []string{
				"rule table row create",
				"--product-id <id>",
				"--table-id <id>",
				"POST /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_rows",
			},
		},
		{
			name: "rule table row search help",
			args: []string{"rule", "table", "row", "search", "--help"},
			contains: []string{
				"rule table row search",
				"--page-size <n>",
				"--page-token <token>",
				"--input-file <path>",
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
			name: "auth init restart guard",
			args: []string{"auth", "init", "--help"},
			contains: []string{
				"auth init",
				"qr_code_data_uri",
				"--restart",
				"仅在用户明确同意重新授权后替换旧 Device 会话",
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
			name: "config add environment defaults",
			args: []string{"config", "add", "--help"},
			contains: []string{
				"config add",
				"--env <prod>",
				"默认 prod",
				"--name <profile>",
				"默认 contract",
				"contract-cli config add --env prod --name contract",
			},
			notContains: []string{"--env <dev|prod>"},
		},
		{
			name: "update check production example",
			args: []string{"update", "check", "--help"},
			contains: []string{
				"update check",
				"--channel <latest|beta>",
				"--json",
				"contract-cli update check --channel latest --json",
			},
			notContains: []string{
				"contract-cli update check --channel beta",
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
			for _, forbidden := range tc.notContains {
				if strings.Contains(stdout.String(), forbidden) {
					t.Fatalf("help output should not contain %q:\n%s", forbidden, stdout.String())
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

/*
TestAllCurrentHelpTopicsRender 验证注册表中的现有帮助主题都能离线渲染。
入参 t（*testing.T）为 Go 测试上下文。
返回值为空；失败通过 t.Fatalf 报告。
*/
func TestAllCurrentHelpTopicsRender(t *testing.T) {
	t.Parallel()

	topics := []string{
		"version",
		"config",
		"config add",
		"auth",
		"auth init",
		"auth complete",
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
		"contract search-v2",
		"contract get",
		"contract sync-user-groups",
		"contract text",
		"contract create",
		"contract field",
		"contract field update",
		"contract sign",
		"contract sign switch-to-paper",
		"contract sign-url",
		"contract sign-url get",
		"contract form",
		"contract form attribute",
		"contract form attribute list",
		"contract authorization",
		"contract authorization grant",
		"contract esign",
		"contract esign personal-auth-url",
		"contract esign org-auth-url",
		"contract upload-file",
		"contract submit",
		"contract resubmit",
		"contract patch",
		"contract download-file",
		"contract delete",
		"contract print-file",
		"contract share",
		"contract share get",
		"contract share batch-create",
		"contract cooperation",
		"contract cooperation link get",
		"contract cooperation record get",
		"contract cooperation search",
		"contract cooperation file",
		"contract cooperation file get",
		"contract cooperation file download",
		"contract approval",
		"contract approval start",
		"contract approval get",
		"contract category",
		"contract category list",
		"contract template",
		"contract template list",
		"contract template get",
		"contract template instantiate",
		"contract enum",
		"contract enum list",
		"payment",
		"payment create",
		"payment update",
		"payment get",
		"payment list",
		"payment plan",
		"payment plan notify",
		"payment plan search",
		"payment record",
		"payment record create",
		"payment record update",
		"payment record get",
		"payment record list",
		"mdm",
		"mdm vendor",
		"mdm vendor list",
		"mdm vendor get",
		"mdm vendor create",
		"mdm vendor update",
		"mdm vendor list-all",
		"mdm vendor query-by-cert",
		"mdm legal",
		"mdm legal list",
		"mdm legal get",
		"mdm legal create",
		"mdm legal update",
		"mdm fields",
		"mdm fields list",
		"mdm fixed-exchange-rate",
		"mdm fixed-exchange-rate get",
		"mdm fixed-exchange-rate update",
		"mdm file",
		"mdm file download",
		"event",
		"event outbound-ip",
		"event outbound-ip list",
		"rule",
		"rule table",
		"rule table list",
		"rule table pre-release",
		"rule table release",
		"rule table column-headers",
		"rule table column-headers list",
		"rule table row",
		"rule table row create",
		"rule table row get",
		"rule table row list",
		"rule table row search",
		"rule table row update",
		"rule table row delete",
		"rule table import",
		"rule table import plan",
		"rule table import apply",
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
