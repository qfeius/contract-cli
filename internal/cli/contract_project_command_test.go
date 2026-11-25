package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestContractProjectSearchUsesMatchingIdentity(t *testing.T) {
	t.Parallel()
	const projectID = "1200569114033127797"
	for _, tc := range []struct {
		identity config.IdentityKind
		path     string
		token    string
	}{
		{config.IdentityApp, "/open-apis/contract/v1/projects/search", "app-token"},
		{config.IdentityUser, "/open-apis/contract/v1/mcp/projects/search", "user-token"},
	} {
		t.Run(string(tc.identity), func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
				t.Fatal(err)
			}
			stdout := &bytes.Buffer{}
			calls := 0
			app := cli.New(cli.Options{Stdout: stdout, Stderr: &bytes.Buffer{}, Store: store,
				HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					calls++
					if req.Method != http.MethodPost || req.URL.Path != tc.path {
						t.Fatalf("request = %s %s, want POST %s", req.Method, req.URL.Path, tc.path)
					}
					if req.Header.Get("Authorization") != "Bearer "+tc.token || req.URL.Query().Has("user_id") {
						t.Fatal("search must use the selected token without a delegated user")
					}
					var body map[string]any
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					want := map[string]any{"keyword": "采购项目", "page_index": float64(0), "page_size": float64(20)}
					if !reflect.DeepEqual(body, want) {
						t.Fatalf("body = %#v, want %#v", body, want)
					}
					return jsonResponse(`{"code":0,"data":{"content":[{"project_id":"` + projectID + `"}],"total_elements":1}}`), nil
				})}})
			args := []string{"contract", "project", "search", "--profile", "contract", "--as", string(tc.identity),
				"--keyword", "采购项目", "--page-index", "0", "--page-size", "20"}
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || !strings.Contains(stdout.String(), `"`+projectID+`"`) {
				t.Fatalf("calls = %d, output = %s", calls, stdout)
			}
		})
	}
}

func TestContractProjectSearchTable(t *testing.T) {
	t.Parallel()
	const response = `{"code":0,"data":{"page_index":1,"page_size":2,"total_elements":5,"content":[
		{"project_id":"9007199254740993","basic_info":{"name":"园区\t采购项目","start_date":"2026-01-15","end_date":"2026-10-30"},
		"department":{"display_name":"基建采购部"},"field_values":[{"field_key":"custom_tags","display_value":["采购","基建"]},{"field_key":"project_status","display_value":"进行中"}]},
		{"project_id":"9007199254740994","basic_info":{"name":"算力扩容项目"},"department":null,
		"field_values":[{"field_key":"custom_status","display_value":"其他状态"}]}
	]}}`
	for _, identity := range []config.IdentityKind{config.IdentityApp, config.IdentityUser} {
		t.Run(string(identity), func(t *testing.T) {
			stdout, err := runProjectSearchWithResponse(t, identity, response, "--output", "table")
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(stdout), "\n")
			for index, line := range lines {
				lines[index] = strings.Join(strings.Fields(line), " ")
			}
			want := []string{
				"项目名称 状态 所属部门 项目周期 项目 ID",
				"园区 采购项目 进行中 基建采购部 2026-01-15 至 2026-10-30 9007199254740993",
				"算力扩容项目 — — — 9007199254740994",
				"第 2 页，共 5 个项目",
			}
			if !reflect.DeepEqual(lines, want) {
				t.Fatalf("table lines = %q, want %q", lines, want)
			}
		})
	}
}

func TestContractProjectSearchTableBoundaries(t *testing.T) {
	t.Parallel()
	const empty = `{"code":0,"data":{"page_index":0,"page_size":10,"total_elements":0,"content":[]}}`
	const rejected = `{"code":123,"success":false,"msg":"permission denied","data":null}`
	for _, tc := range []struct {
		name     string
		identity config.IdentityKind
		response string
		args     []string
		want     string
		wantErr  string
	}{
		{"empty", config.IdentityUser, empty, []string{"--output", "table"}, "本页无项目\n第 1 页，共 0 个项目\n", ""},
		{"app business error", config.IdentityApp, rejected, []string{"--output", "table"}, "permission denied", ""},
		{"user business error", config.IdentityUser, rejected, []string{"--output", "table"}, "permission denied", "business code 123"},
		{"default JSON", config.IdentityApp, empty, nil, `"content": []`, ""},
		{"YAML", config.IdentityUser, empty, []string{"--output", "yaml"}, "total_elements: 0", ""},
		{"raw table request", config.IdentityUser, empty, []string{"--output", "table", "--raw"}, empty, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, err := runProjectSearchWithResponse(t, tc.identity, tc.response, tc.args...)
			if (tc.wantErr == "" && err != nil) || (tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr))) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
			if !strings.Contains(stdout, tc.want) {
				t.Fatalf("output = %q, want %q", stdout, tc.want)
			}
			if tc.name == "raw table request" && stdout != tc.response {
				t.Fatalf("raw output = %q, want %q", stdout, tc.response)
			}
		})
	}
}

func runProjectSearchWithResponse(t *testing.T, identity config.IdentityKind, response string, flags ...string) (string, error) {
	t.Helper()
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatal(err)
	}
	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{Stdout: stdout, Stderr: &bytes.Buffer{}, Store: store,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return jsonResponse(response), nil
		})}})
	args := append([]string{"contract", "project", "search", "--profile", "contract", "--as", string(identity)}, flags...)
	err := app.Run(context.Background(), args)
	return stdout.String(), err
}

func TestContractProjectSearchRejectsInvalidOptionsBeforeHTTP(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--user-id", "other"}, "--user-id"},
		{[]string{"--data", `{"tenant_id":"2"}`}, "does not accept"},
		{[]string{"--page-index", "-1"}, "--page-index"},
		{[]string{"--page-size", "201"}, "--page-size"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
				t.Fatal(err)
			}
			app := cli.New(cli.Options{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Store: store,
				HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					t.Fatal("invalid options must not send HTTP")
					return nil, nil
				})}})
			err := app.Run(context.Background(), append([]string{"contract", "project", "search"}, tc.args...))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
