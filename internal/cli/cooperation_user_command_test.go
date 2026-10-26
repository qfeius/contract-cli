package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	contractskills "cn.qfei/contract-cli/skills"
)

func TestUserCooperationCommands(t *testing.T) {
	cases := []struct {
		name                string
		args                []string
		method, path, extra string
	}{
		{"search", []string{"search", "--data", `{"page_size":20,"page_token":"opaque+/=","search_tab_enum_code":1,"cooperation_display_tab_code":1,"create_user_id":"9007199254740993"}`, "--as", "user"}, "POST", "/search", ""},
		{"detail", []string{"get", "--cooperation-id", "1178244026340475224"}, "GET", "/1178244026340475224", ""},
		{"history default", []string{"history-files", "list", "--cooperation-id", "1178244026340475224"}, "GET", "/1178244026340475224/history_files", "true"},
		{"history false", []string{"history-files", "list", "--cooperation-id", "1178244026340475224", "--include-current-version=false"}, "GET", "/1178244026340475224/history_files", "false"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			app, out := alignmentApp(t, func(r *http.Request) string {
				calls++
				if r.Method != tc.method || r.URL.Path != "/open-apis/contract/v1/mcp/cooperations"+tc.path {
					t.Fatalf("unexpected request %s %s", r.Method, r.URL)
				}
				q := r.URL.Query()
				if q.Get("user_id_type") != "user_id" || q.Get("department_id_type") != "open_department_id" {
					t.Fatal(q)
				}
				if tc.extra != "" && q.Get("include_current_version") != tc.extra {
					t.Fatal(q)
				}
				if tc.method == "POST" {
					body := alignmentBody(t, r)
					if body["create_user_id"] != "9007199254740993" || body["page_token"] != "opaque+/=" || body["page_size"].(interface{ String() string }).String() != "20" {
						t.Fatal(body)
					}
				}
				return `{"code":0,"data":{"cooperation_id":1178244026340475224,"page_token":"next+/="}}`
			})
			if err := app.Run(context.Background(), append([]string{"contract", "cooperation"}, tc.args...)); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || !strings.Contains(out.String(), "1178244026340475224") {
				t.Fatalf("calls=%d output=%s", calls, out)
			}
		})
	}
}

func TestUserCooperationSearchFileDefaultsAndDefaultIdentity(t *testing.T) {
	for _, identity := range []config.IdentityKind{config.IdentityUser, config.IdentityApp} {
		t.Run(string(identity), func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(identity), true); err != nil {
				t.Fatal(err)
			}
			input := filepath.Join(t.TempDir(), "search.json")
			if err := os.WriteFile(input, []byte(`{}`), 0600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			app := cli.New(cli.Options{Store: store, Stdout: &bytes.Buffer{}, Stderr: io.Discard, HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if identity == config.IdentityUser {
					if r.URL.Path != "/open-apis/contract/v1/mcp/cooperations/search" || r.Header.Get("Authorization") != "Bearer user-token" {
						t.Fatal(r.URL)
					}
					body := alignmentBody(t, r)
					if body["page_size"].(interface{ String() string }).String() != "10" || body["order"] != "DESC" {
						t.Fatal(body)
					}
				} else if r.URL.Path != "/open-apis/contract/v1/cooperation/search" {
					t.Fatal(r.URL)
				}
				return jsonResponse(`{"code":0,"data":{}}`), nil
			})}})
			if err := app.Run(context.Background(), []string{"contract", "cooperation", "search", "--input-file", input}); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatal(calls)
			}
		})
	}
}

func TestUserCooperationInvalidInputsBeforeHTTP(t *testing.T) {
	cases := [][]string{
		{"get", "--cooperation-id", "0"}, {"get", "--cooperation-id", "../123"}, {"get", "--cooperation-id", "9223372036854775808"},
		{"get", "--cooperation-id", "123", "--as", "app"}, {"get", "--cooperation-id", "123", "--data", "{}"},
		{"history-files", "list", "--cooperation-id", "123", "--as", "app"}, {"history-files", "list", "--cooperation-id", "123", "--include-current-version=wat"},
		{"history-files", "list"}, {"search", "--as", "user", "--data", "[]"},
	}
	for _, body := range []string{`{"page_size":21}`, `{"page_size":0}`, `{"page_size":1.5}`, `{"search_tab_enum_code":2}`, `{"cooperation_display_tab_code":6}`, `{"order":"sideways"}`, `{"create_user_id":9007199254740993}`, `{"create_user_ids":["123"]}`, `{"user_id":"another-user"}`, `{"search_cooperation_by_create_time":["2026-02-30","2026-03-01"]}`, `{"search_cooperation_by_last_confirm_time":["2026-10-02","2026-10-01"]}`, `{"search_cooperation_by_latest_version_upload_time":["2026-10-01"]}`, `{"department_collection":[1]}`} {
		cases = append(cases, []string{"search", "--as", "user", "--data", body})
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			app, _ := alignmentApp(t, nil)
			if err := app.Run(context.Background(), append([]string{"contract", "cooperation"}, args...)); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
}

func TestUserCooperationHistoryFalsePreservesReturnedFinalVersion(t *testing.T) {
	want := `{"code":0,"data":{"history_files":[{"cooperation_file_id":"9007199254740993","version":"V1.0","latest_version":true,"history":[{"cooperation_file_id":"9007199254740993","version":"V1.0","latest_version":true}]}]}}`
	calls := 0
	app, out := alignmentApp(t, func(request *http.Request) string {
		calls++
		if request.URL.Query().Get("include_current_version") != "false" {
			t.Fatal(request.URL)
		}
		return want
	})
	args := []string{"contract", "cooperation", "history-files", "list", "--cooperation-id", "123", "--include-current-version=false"}
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	var got, expected struct {
		Data any `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !reflect.DeepEqual(got.Data, expected.Data) {
		t.Fatalf("calls=%d got=%+v want=%+v", calls, got.Data, expected.Data)
	}
}

func TestUserCooperationPermissionFailureDoesNotRetry(t *testing.T) {
	for _, body := range []string{`{"code":403,"msg":"无协商权限"}`, `{"code":"403","msg":"无协商权限"}`, `{"code":401,"message":"请重新授权"}`} {
		t.Run(body, func(t *testing.T) {
			calls := 0
			app, out := alignmentApp(t, func(*http.Request) string { calls++; return body })
			err := app.Run(context.Background(), []string{"contract", "cooperation", "get", "--cooperation-id", "123"})
			if err == nil || calls != 1 || !json.Valid(out.Bytes()) {
				t.Fatalf("err=%v calls=%d output=%s", err, calls, out)
			}
			if !strings.Contains(out.String(), "权限") && !strings.Contains(out.String(), "授权") {
				t.Fatal("upstream reason lost")
			}
		})
	}
}

func TestUserCooperationHTTPForbiddenDoesNotRetry(t *testing.T) {
	calls := 0
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
		t.Fatal(err)
	}
	app := cli.New(cli.Options{Store: store, Stdout: &bytes.Buffer{}, Stderr: io.Discard, HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		r := jsonResponse(`{"code":403,"msg":"forbidden"}`)
		r.StatusCode = 403
		return r, nil
	})}})
	if err := app.Run(context.Background(), []string{"contract", "cooperation", "history-files", "list", "--cooperation-id", "123"}); err == nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestCooperationSkillInstalledWithReferences(t *testing.T) {
	target := t.TempDir()
	app := cli.New(cli.Options{Store: config.NewStore(t.TempDir()), Stdout: &bytes.Buffer{}, Stderr: io.Discard, SkillsFS: contractskills.FS})
	if err := app.Run(context.Background(), []string{"skills", "install", "--target", target}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(target, "contract-cooperation")
	for _, name := range []string{"SKILL.md", "agents/openai.yaml", "references/search-parameters.md"} {
		if body, err := os.ReadFile(filepath.Join(root, name)); err != nil || len(body) == 0 {
			t.Fatalf("resource %s missing: %v", name, err)
		}
	}
	assertSearchSkillLinksResolve(t, root)
}

func TestUserCooperationSearchPreservesMCPFilters(t *testing.T) {
	input := `{"page_size":20,"page_token":"next+/=","keyword":"采购","search_tab_enum_code":1,"cooperation_display_tab_code":2,"cooperation_query_status_codes":[2,4],"contract_category_ids":["9007199254740993"],"search_cooperation_by_create_time":["2026-01-01","2026-10-21"],"search_cooperation_by_last_confirm_time":["2026-01-02","2026-10-21"],"search_cooperation_by_latest_version_upload_time":["2026-01-03","2026-10-21"],"search_cooperation_by_cooperation_complete_time":["2026-01-04","2026-10-21"],"create_user_id":"user-1","department_collection":["dept-1"],"confirmed_role_codes":["legal"],"order":"ASC"}`
	app, _ := alignmentApp(t, func(r *http.Request) string {
		got, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var wantObject, gotObject map[string]any
		if err := json.Unmarshal([]byte(input), &wantObject); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(got, &gotObject); err != nil {
			t.Fatal(err)
		}
		wantJSON, _ := json.Marshal(wantObject)
		gotJSON, _ := json.Marshal(gotObject)
		if !bytes.Equal(wantJSON, gotJSON) {
			t.Fatalf("filters changed: %s", got)
		}
		return `{"code":0,"data":{}}`
	})
	if err := app.Run(context.Background(), []string{"contract", "cooperation", "search", "--as", "user", "--data", input}); err != nil {
		t.Fatal(err)
	}
}

func TestUserCooperationSearchRejectsCallerOverrides(t *testing.T) {
	for _, flag := range []string{"--user-id", "--user-id-type", "--department-id-type"} {
		t.Run(flag, func(t *testing.T) {
			app, _ := alignmentApp(t, nil)
			err := app.Run(context.Background(), []string{"contract", "cooperation", "search", "--as", "user", "--data", "{}", flag, "override"})
			if err == nil {
				t.Fatal("caller override accepted")
			}
		})
	}
}

func TestCooperationHelpAndReviewCompatibility(t *testing.T) {
	for _, command := range [][]string{{"contract", "cooperation", "get"}, {"contract", "cooperation", "history-files", "list"}, {"contract", "cooperation", "search"}} {
		t.Run(strings.Join(command, " "), func(t *testing.T) {
			app, out := alignmentApp(t, nil)
			if err := app.Run(context.Background(), append(command, "--help")); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), "user") {
				t.Fatal(out.String())
			}
		})
	}
	calls := 0
	app, _ := alignmentApp(t, func(r *http.Request) string {
		calls++
		if r.URL.Query().Get("department_id_type") != "open_department_id" {
			t.Fatal(r.URL)
		}
		return `{"code":0,"data":{}}`
	})
	if err := app.Run(context.Background(), []string{"review", "cooperation", "get", "--cooperation-id", "123", "--as", "user", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}
