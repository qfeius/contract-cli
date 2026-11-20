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
