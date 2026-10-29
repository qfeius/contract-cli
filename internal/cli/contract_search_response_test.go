package cli_test

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestContractSearchFailureLogsDoNotCopyHTTPResponseBody(t *testing.T) {
	t.Parallel()
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
		t.Fatal(err)
	}
	logs := &bytes.Buffer{}
	const privateValue = "private-contract-field-echo"
	app := cli.New(cli.Options{Store: store, Stdout: &bytes.Buffer{}, Stderr: logs,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			response := jsonResponse(`{"message":"` + privateValue + `"}`)
			response.StatusCode = http.StatusBadRequest
			return response, nil
		})}})
	err := app.Run(context.Background(), []string{"contract", "search", "--as", "user", "--profile", "contract", "--debug"})
	if err == nil {
		t.Fatal("HTTP failure must return error")
	}
	if strings.Contains(logs.String(), privateValue) {
		t.Fatal("search logging copied private response content")
	}
	if !strings.Contains(logs.String(), "search contracts failed") {
		t.Fatal("missing search failure log")
	}
}

func TestContractSearchUserReportsBusinessOutcomeAndPreservesResponse(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		body      string
		wantError bool
	}{
		{"production invalid sort", `{"code":110000,"success":false,"msg":"参数有误","data":null}`, true},
		{"failure with zero code", `{"code":0,"success":false,"msg":"查询失败"}`, true},
		{"missing business code", `{"success":true,"data":{"items":[]}}`, true},
		{"empty success", `{"code":0,"success":true,"data":{"items":[],"has_more":false}}`, false},
		{"legacy success without boolean", `{"code":0,"data":{"items":[]}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, raw := range []bool{false, true} {
				store := config.NewStore(t.TempDir())
				if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
					t.Fatal(err)
				}
				stdout := &bytes.Buffer{}
				app := cli.New(cli.Options{Store: store, Stdout: stdout, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.URL.Path != "/open-apis/contract/v1/mcp/contracts/search" {
						t.Fatalf("wrong route: %s", req.URL.Path)
					}
					return jsonResponse(tc.body), nil
				})}})
				args := []string{"contract", "search", "--profile", "contract", "--as", "user", "--data", `{"page_size":1}`}
				if raw {
					args = append(args, "--raw")
				}
				err := app.Run(context.Background(), args)
				if (err != nil) != tc.wantError {
					t.Errorf("raw=%t error=%v, wantError=%t", raw, err, tc.wantError)
				}
				if raw && stdout.String() != tc.body {
					t.Errorf("response lost: %s", stdout)
				}
				if !bytes.Contains(stdout.Bytes(), []byte(`"code"`)) && tc.name != "missing business code" {
					t.Errorf("business response lost: %s", stdout)
				}
			}
		})
	}
}

func TestContractSearchAppPreservesExistingResponseContract(t *testing.T) {
	t.Parallel()
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatal(err)
	}
	body := `{"data":{"items":[]}}`
	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{Store: store, Stdout: stdout, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/open-apis/contract/v1/contracts/search" {
			t.Fatalf("wrong route: %s", req.URL.Path)
		}
		return jsonResponse(body), nil
	})}})
	if err := app.Run(context.Background(), []string{"contract", "search", "--profile", "contract", "--as", "app", "--raw"}); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != body {
		t.Errorf("app response changed: %s", stdout)
	}
}
