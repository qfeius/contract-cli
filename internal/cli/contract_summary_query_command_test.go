package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestContractSummaryQueryUsesUserIdentityAndContractNumber(t *testing.T) {
	t.Parallel()

	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
		Store:  store,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost || req.URL.Path != "/open-apis/contract/v1/summary/query" {
					t.Fatalf("request = %s %s", req.Method, req.URL.Path)
				}
				if got := req.Header.Get("Authorization"); got != "Bearer user-token" {
					t.Fatalf("authorization = %q", got)
				}
				if got := req.URL.Query().Get("user_id_type"); got != "user_id" {
					t.Fatalf("user_id_type = %q", got)
				}
				assertJSONNumberRequestBody(t, req, `{"contractNumber":"CN-001"}`)
				return jsonResponse(`{"code":0,"data":{"contractNumber":"CN-001","status":2,"overview":"摘要"},"msg":"success"}`), nil
			}),
		},
	})

	err := app.Run(context.Background(), []string{
		"contract", "summary", "query",
		"--profile", "contract",
		"--as", "user",
		"--contract-number", "CN-001",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(stdout.String(), `"overview": "摘要"`) {
		t.Fatalf("generated summary was hidden: %s", stdout.String())
	}
}

func TestContractSummaryQueryShowsOnlyGeneratingStatus(t *testing.T) {
	for _, outputFlag := range [][]string{nil, {"--raw"}} {
		name := "formatted"
		if len(outputFlag) > 0 {
			name = "raw"
		}
		t.Run(name, func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
				t.Fatalf("UpsertProfile() error = %v", err)
			}
			stdout := &bytes.Buffer{}
			calls := 0
			app := cli.New(cli.Options{
				Stdout: stdout,
				Stderr: &bytes.Buffer{},
				Store:  store,
				HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					calls++
					return jsonResponse(`{"code":0,"msg":"success","success":true,"data":{"status":1,"contractNumber":"CN-001","overview":"过期概览","summary":"过期摘要","structuredSummary":{"section":"过期内容"}}}`), nil
				})},
			})
			args := append([]string{"contract", "summary", "query", "--profile", "contract", "--as", "user", "--contract-number", "CN-001"}, outputFlag...)
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if calls != 1 {
				t.Fatalf("summary query calls = %d, want one request without automatic waiting", calls)
			}
			var response struct {
				Code    int    `json:"code"`
				Msg     string `json:"msg"`
				Success bool   `json:"success"`
				Data    struct {
					Status int `json:"status"`
				} `json:"data"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
				t.Fatalf("output is not JSON: %v; output=%s", err, stdout.String())
			}
			if response.Code != 0 || response.Msg != "摘要正在生成中，请稍后再试" || !response.Success || response.Data.Status != 1 {
				t.Fatalf("unexpected generating response: %+v", response)
			}
			for _, forbidden := range []string{"过期概览", "过期摘要", "过期内容", "contractNumber", "structuredSummary"} {
				if strings.Contains(stdout.String(), forbidden) {
					t.Fatalf("generating response exposed %q: %s", forbidden, stdout.String())
				}
			}
		})
	}
}

func TestContractSummaryQueryRejectsUnsupportedOutputBeforeHTTP(t *testing.T) {
	t.Parallel()

	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
		Store:  store,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			}),
		},
	})

	err := app.Run(context.Background(), []string{
		"contract", "summary", "query",
		"--profile", "contract",
		"--as", "user",
		"--contract-number", "CN-001",
		"--output", "unsupported-format",
	})
	if err == nil || !strings.Contains(err.Error(), `unsupported output format "unsupported-format"`) {
		t.Fatalf("Run() error = %v, want unsupported output rejection", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestContractSummaryQuerySupportsExactContractName(t *testing.T) {
	t.Parallel()

	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Store:  store,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				assertJSONNumberRequestBody(t, req, `{"contractName":"采购合同"}`)
				return jsonResponse(`{"code":0,"data":{"contractName":"采购合同","status":2},"msg":"success"}`), nil
			}),
		},
	})

	err := app.Run(context.Background(), []string{
		"contract", "summary", "query",
		"--profile", "contract",
		"--as", "user",
		"--contract-name", "采购合同",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestContractSummaryQueryRejectsAmbiguousSelectorsBeforeHTTP(t *testing.T) {
	t.Parallel()

	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	app := cli.New(cli.Options{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Store:  store,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			}),
		},
	})

	err := app.Run(context.Background(), []string{
		"contract", "summary", "query",
		"--profile", "contract",
		"--as", "user",
		"--contract-number", "CN-001",
		"--contract-name", "采购合同",
	})
	if err == nil || !strings.Contains(err.Error(), "exactly one of --contract-number or --contract-name") {
		t.Fatalf("Run() error = %v, want selector validation", err)
	}
}

func TestContractSummaryQueryRejectsAppIdentityBeforeHTTP(t *testing.T) {
	t.Parallel()

	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	app := cli.New(cli.Options{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Store:  store,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			}),
		},
	})

	err := app.Run(context.Background(), []string{
		"contract", "summary", "query",
		"--profile", "contract",
		"--as", "app",
		"--contract-number", "CN-001",
	})
	if err == nil || !strings.Contains(err.Error(), "only supports --as user") {
		t.Fatalf("Run() error = %v, want user-only identity rejection", err)
	}
}
