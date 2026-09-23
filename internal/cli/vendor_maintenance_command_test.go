package cli_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestVendorMaintenanceCommands(t *testing.T) {
	for _, tc := range []struct {
		name               string
		identity           config.IdentityKind
		args               []string
		method, path, body string
	}{
		{"personal create default identity", config.IdentityUser, []string{"create", "--data", `{"vendorText":"name"}`}, "POST", "/open-apis/contract/v1/mcp/vendors", `{"vendorText":"name"}`},
		{"personal patch", config.IdentityUser, []string{"patch", "9007199254740993", "--data", `{"shortText":null,"appendix":[]}`}, "PATCH", "/open-apis/contract/v1/mcp/vendors/9007199254740993", `{"shortText":null,"appendix":[]}`},
		{"app patch", config.IdentityApp, []string{"patch", "9007199254740993", "--user-id", "operator-1", "--data", `{"status":0,"shortText":null}`}, "PATCH", "/open-apis/mdm/v1/vendors/9007199254740993", `{"status":0,"shortText":null}`},
		{"enable", config.IdentityUser, []string{"enable", "9007199254740993"}, "PUT", "/open-apis/contract/v1/mcp/vendors/9007199254740993/status", `{"target_status":1}`},
		{"disable", config.IdentityUser, []string{"disable", "9007199254740993"}, "PUT", "/open-apis/contract/v1/mcp/vendors/9007199254740993/status", `{"target_status":0}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(tc.identity), true); err != nil {
				t.Fatal(err)
			}
			calls := 0
			app := cli.New(cli.Options{Store: store, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					calls++
					body, err := io.ReadAll(req.Body)
					if err != nil || req.Method != tc.method || req.URL.Path != tc.path || string(body) != tc.body {
						t.Fatalf("request: %s %s %s; error %v", req.Method, req.URL.Path, body, err)
					}
					if tc.identity == config.IdentityUser && req.URL.Query().Get("user_id") != "" {
						t.Fatal("personal actor must come from token")
					}
					if tc.identity == config.IdentityApp && req.URL.Query().Get("user_id") != "operator-1" {
						t.Fatal("missing app actor")
					}
					if req.URL.Query().Get("department_id_type") != "" {
						t.Fatal("department_id_type must be omitted unless explicitly requested")
					}
					return jsonResponse(`{"code":0,"data":{"outcome":"APPLIED","id":"9007199254740993","code":"V1","name":"name","status":1,"changed":[]}}`), nil
				}),
			}})
			args := append([]string{"mdm", "vendor"}, tc.args...)
			args = append(args, "--profile", "contract")
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("calls = %d", calls)
			}
		})
	}
}

func TestVendorMaintenanceRejectsInvalidCommandsBeforeHTTP(t *testing.T) {
	for _, tc := range []struct {
		identity config.IdentityKind
		args     []string
	}{
		{config.IdentityApp, []string{"patch", "1", "--data", `{"shortText":null}`}},
		{config.IdentityApp, []string{"enable", "1", "--as", "app"}},
		{config.IdentityUser, []string{"patch", "1", "--data", `{"id":"1"}`}},
		{config.IdentityUser, []string{"patch", "1", "--data", `{"status":null}`}},
		{config.IdentityUser, []string{"patch", "1", "--data", `{"vendor":null}`}},
		{config.IdentityUser, []string{"create", "--data", `{"isRisked":false}`}},
		{config.IdentityUser, []string{"create", "--data", `{"vendorText":"name","vendorAccounts":[{"bankId":"MDBK1"}]}`}},
		{config.IdentityUser, []string{"patch", "1", "--data", `{"vendorAccounts":[{"id":"2","bankId":"MDBK1"}]}`}},
		{config.IdentityUser, []string{"create", "--user-id", "other", "--data", `{"vendorText":"name"}`}},
		{config.IdentityUser, []string{"disable", "1", "--data", `{}`}},
		{config.IdentityUser, []string{"patch", "1", "--data", `{}`}},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(tc.identity), true); err != nil {
				t.Fatal(err)
			}
			app := cli.New(cli.Options{Store: store, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) { t.Fatal("unexpected HTTP call"); return nil, nil }),
			}})
			args := append([]string{"mdm", "vendor"}, tc.args...)
			if err := app.Run(context.Background(), args); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestPersonalVendorMaintenanceSupportsExplicitDepartmentIDType(t *testing.T) {
	for _, tc := range []struct {
		name             string
		departmentIDType string
		args             []string
	}{
		{"create open department", "open_department_id", []string{"create", "--department-id-type", "open_department_id", "--data", `{"vendorText":"name","ownerDepts":["od-1"]}`}},
		{"patch open department", "open_department_id", []string{"patch", "1", "--department-id-type", "open_department_id", "--data", `{"ownerDepts":["od-1"]}`}},
		{"patch internal department", "department_id", []string{"patch", "1", "--department-id-type", "department_id", "--data", `{"ownerDepts":["1001"]}`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
				t.Fatal(err)
			}
			calls := 0
			app := cli.New(cli.Options{Store: store, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					calls++
					if got := req.URL.Query().Get("department_id_type"); got != tc.departmentIDType {
						t.Fatalf("department_id_type = %q", got)
					}
					return jsonResponse(`{"code":0,"data":{"outcome":"APPLIED","id":"1","code":"V1","name":"name","status":1,"changed":[]}}`), nil
				}),
			}})
			args := append([]string{"mdm", "vendor"}, tc.args...)
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("calls = %d", calls)
			}
		})
	}
}

func TestVendorMaintenanceRejectsInvalidOrAppDepartmentIDTypeBeforeHTTP(t *testing.T) {
	for _, tc := range []struct {
		identity config.IdentityKind
		args     []string
	}{
		{config.IdentityUser, []string{"create", "--department-id-type", "invalid", "--data", `{"vendorText":"name"}`}},
		{config.IdentityUser, []string{"patch", "1", "--department-id-type", "invalid", "--data", `{"shortText":"name"}`}},
		{config.IdentityApp, []string{"create", "--department-id-type", "open_department_id", "--user-id", "operator-1", "--data", `{"vendorText":"name"}`}},
		{config.IdentityApp, []string{"patch", "1", "--department-id-type", "open_department_id", "--user-id", "operator-1", "--data", `{"shortText":"name"}`}},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(tc.identity), true); err != nil {
				t.Fatal(err)
			}
			app := cli.New(cli.Options{Store: store, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					t.Fatal("unexpected HTTP call")
					return nil, nil
				}),
			}})
			args := append([]string{"mdm", "vendor"}, tc.args...)
			if err := app.Run(context.Background(), args); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestVendorMaintenanceBusinessFailureReturnsErrorInEveryOutputMode(t *testing.T) {
	for _, flag := range [][]string{nil, {"--raw"}, {"--output", "json"}} {
		t.Run(strings.Join(flag, " "), func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
				t.Fatal(err)
			}
			app := cli.New(cli.Options{Store: store, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					return jsonResponse(`{"code":40001,"data":{"outcome":"FAILED"}}`), nil
				}),
			}})
			args := append([]string{"mdm", "vendor", "patch", "1", "--data", `{"shortText":null}`}, flag...)
			if err := app.Run(context.Background(), args); err == nil || !strings.Contains(err.Error(), "40001") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestVendorMaintenanceHelpDescribesIdentityAndPatchSemantics(t *testing.T) {
	for _, command := range []string{"patch", "enable", "disable"} {
		t.Run(command, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			app := cli.New(cli.Options{Stdout: stdout, Stderr: &bytes.Buffer{}, Store: config.NewStore(t.TempDir())})
			if err := app.Run(context.Background(), []string{"mdm", "vendor", command, "--help"}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stdout.String(), "mdm vendor "+command) || !strings.Contains(stdout.String(), "user") {
				t.Fatalf("missing maintenance help: %s", stdout.String())
			}
		})
	}
}
