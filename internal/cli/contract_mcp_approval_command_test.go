package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestContractMCPApprovalCommandsUseUserEndpoints(t *testing.T) {
	t.Parallel()

	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	testCases := []struct {
		name         string
		args         []string
		wantMethod   string
		wantPath     string
		wantQuery    string
		wantBody     string
		responseBody string
	}{
		{
			name:         "process instance detail",
			args:         []string{"contract", "approval", "get", "process-1", "--profile", "contract", "--as", "user", "--notice-filter", "notice_filter", "--task-instance-filter", "task_instance_filter"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/mcp/process_instances/process-1",
			wantQuery:    "notice_filter=notice_filter&task_instance_filter=task_instance_filter&user_id_type=user_id",
			responseBody: `{"code":0,"data":{"process_instance_id":"process-1"}}`,
		},
		{
			name:         "comment list",
			args:         []string{"contract", "approval", "comment", "list", "process-1", "--profile", "contract", "--as", "user"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/mcp/process_instances/process-1/comments",
			wantQuery:    "user_id_type=user_id",
			responseBody: `{"code":0,"data":{"items":[]}}`,
		},
		{
			name:         "comment create",
			args:         []string{"contract", "approval", "comment", "create", "process-1", "--profile", "contract", "--as", "user", "--mention-id-type", "open_id", "--data", `{"content":"请确认","user_id":["ou_1"],"file_ids":["6911661136408477999"]}`},
			wantMethod:   http.MethodPost,
			wantPath:     "/open-apis/contract/v1/mcp/process_instances/process-1/comments",
			wantQuery:    "user_id_type=open_id",
			wantBody:     `{"content":"请确认","user_id":["ou_1"],"file_ids":["6911661136408477999"]}`,
			responseBody: `{"code":0,"data":{"comment_id":"334455667788990011"}}`,
		},
		{
			name:         "task list",
			args:         []string{"contract", "approval", "task", "list", "--profile", "contract", "--as", "user", "--query", "采购合同", "--task-type", "done", "--page-index", "2", "--page-size", "50"},
			wantMethod:   http.MethodPost,
			wantPath:     "/open-apis/contract/v1/mcp/tasks",
			wantQuery:    "user_id_type=user_id",
			wantBody:     `{"page_index":2,"page_size":50,"query":"采购合同","task_type_code":1}`,
			responseBody: `{"code":0,"data":{"items":[]}}`,
		},
		{
			name:         "task approve",
			args:         []string{"contract", "approval", "task", "approve", "task-1", "--profile", "contract", "--as", "user", "--comment", "同意", "--file-id", "6911661136408477999", "--file-id", "6911661136408478000"},
			wantMethod:   http.MethodPost,
			wantPath:     "/open-apis/contract/v1/mcp/tasks/task-1/approval",
			wantBody:     `{"action":"approve","comment":"同意","file_ids":["6911661136408477999","6911661136408478000"]}`,
			responseBody: `{"code":0,"data":{"action":"approve"}}`,
		},
		{
			name:         "task reject",
			args:         []string{"contract", "approval", "task", "reject", "task-2", "--profile", "contract", "--as", "user", "--comment", "条款风险未解决"},
			wantMethod:   http.MethodPost,
			wantPath:     "/open-apis/contract/v1/mcp/tasks/task-2/approval",
			wantBody:     `{"action":"reject","comment":"条款风险未解决"}`,
			responseBody: `{"code":0,"data":{"action":"reject"}}`,
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
				Store:  store,
				HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != tc.wantMethod {
						t.Fatalf("method = %s, want %s", req.Method, tc.wantMethod)
					}
					if req.URL.Path != tc.wantPath {
						t.Fatalf("path = %s, want %s", req.URL.Path, tc.wantPath)
					}
					if req.URL.RawQuery != tc.wantQuery {
						t.Fatalf("query = %q, want %q", req.URL.RawQuery, tc.wantQuery)
					}
					if req.Header.Get("Authorization") != "Bearer user-token" {
						t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
					}
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("ReadAll() error = %v", err)
					}
					if string(body) != tc.wantBody {
						t.Fatalf("body = %q, want %q", string(body), tc.wantBody)
					}
					return jsonResponse(tc.responseBody), nil
				})},
			})

			if err := app.Run(context.Background(), tc.args); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if !strings.Contains(stdout.String(), `"code": 0`) {
				t.Fatalf("unexpected output: %s", stdout.String())
			}
		})
	}
}

func TestContractMCPApprovalCommandsValidateIdentityAndInputsBeforeHTTP(t *testing.T) {
	t.Parallel()

	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	testCases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "comments are user only",
			args:    []string{"contract", "approval", "comment", "list", "process-1", "--profile", "contract", "--as", "app"},
			wantErr: "only supports --as user",
		},
		{
			name:    "task list rejects invalid type",
			args:    []string{"contract", "approval", "task", "list", "--profile", "contract", "--as", "user", "--task-type", "archived"},
			wantErr: "--task-type must be one of todo, done, notice",
		},
		{
			name:    "task list rejects page size above maximum",
			args:    []string{"contract", "approval", "task", "list", "--profile", "contract", "--as", "user", "--page-size", "101"},
			wantErr: "--page-size must be between 1 and 100",
		},
		{
			name:    "reject requires comment",
			args:    []string{"contract", "approval", "task", "reject", "task-1", "--profile", "contract", "--as", "user", "--comment", "   "},
			wantErr: "--comment is required for reject",
		},
		{
			name:    "approve rejects invalid file id",
			args:    []string{"contract", "approval", "task", "approve", "task-1", "--profile", "contract", "--as", "user", "--file-id", "0"},
			wantErr: "--file-id must be a positive integer",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requests := 0
			app := cli.New(cli.Options{
				Stdout: io.Discard,
				Stderr: io.Discard,
				Store:  store,
				HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests++
					return jsonResponse(`{"code":0}`), nil
				})},
			})

			err := app.Run(context.Background(), tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
			if requests != 0 {
				t.Fatalf("validation error sent %d HTTP requests", requests)
			}
		})
	}
}

func TestContractMCPApprovalReadRetriesButWritesRemainUncertain(t *testing.T) {
	t.Parallel()

	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	t.Run("task list is a retriable read", func(t *testing.T) {
		calls := 0
		app := cli.New(cli.Options{
			Stdout: io.Discard,
			Stderr: io.Discard,
			Store:  store,
			HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return nil, temporaryApprovalNetworkError{}
				}
				return jsonResponse(`{"code":0,"data":{"items":[]}}`), nil
			})},
		})
		if err := app.Run(context.Background(), []string{"contract", "approval", "task", "list", "--profile", "contract", "--as", "user"}); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if calls != 2 {
			t.Fatalf("calls = %d, want 2", calls)
		}
	})

	t.Run("comment create is not retried", func(t *testing.T) {
		calls := 0
		app := cli.New(cli.Options{
			Stdout: io.Discard,
			Stderr: io.Discard,
			Store:  store,
			HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				return nil, temporaryApprovalNetworkError{}
			})},
		})
		err := app.Run(context.Background(), []string{"contract", "approval", "comment", "create", "process-1", "--profile", "contract", "--as", "user", "--data", `{"content":"请确认"}`})
		if err == nil || !strings.Contains(err.Error(), "执行结果不确定，请先查询确认") {
			t.Fatalf("Run() error = %v", err)
		}
		if calls != 1 {
			t.Fatalf("calls = %d, want 1", calls)
		}
	})
}

func TestContractApprovalCommentIsRedactedWithoutChangingRequest(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"approve", "reject"} {
		for _, flags := range [][]string{{"--comment", "private approval detail"}, {"--comment=private approval detail"}} {
			t.Run(action+"/"+flags[0], func(t *testing.T) {
				t.Parallel()
				store := config.NewStore(t.TempDir())
				if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
					t.Fatal(err)
				}
				logs := &bytes.Buffer{}
				app := cli.New(cli.Options{
					Stdout: io.Discard, Stderr: logs, Store: store,
					HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						body, err := io.ReadAll(req.Body)
						if err != nil {
							t.Fatal(err)
						}
						if !strings.Contains(string(body), `"comment":"private approval detail"`) {
							t.Fatalf("request comment changed: %s", body)
						}
						return jsonResponse(`{"code":0,"success":true}`), nil
					})},
				})
				args := []string{"contract", "approval", "task", action, "112233", "--profile", "contract", "--as", "user"}
				args = append(append(args, flags...), "--debug")
				if err := app.Run(context.Background(), args); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(logs.String(), "private approval detail") || !strings.Contains(logs.String(), "[REDACTED]") {
					t.Fatalf("approval comment was not redacted: %s", logs.String())
				}
			})
		}
	}
}

type temporaryApprovalNetworkError struct{}

func (temporaryApprovalNetworkError) Error() string   { return "temporary network error" }
func (temporaryApprovalNetworkError) Timeout() bool   { return true }
func (temporaryApprovalNetworkError) Temporary() bool { return true }

func TestContractMCPApprovalPreservesPartialSuccessAndStringIDs(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantErr    bool
	}{
		{"partial process with string IDs", `{"code":0,"success":true,"data":{"process_instance":{"tenant_id":"9223372036854775807","complete":false,"limitations":["MIGRATED_ATTACHMENTS_NOT_VERIFIABLE"],"task_instance_list":[]}}}`, false},
		{"downstream unavailable", `{"code":110002,"success":false,"msg":"MCP_DOWNSTREAM_UNAVAILABLE"}`, true},
	} {
		for _, format := range []string{"json", "yaml", "raw"} {
			t.Run(tc.name+"/"+format, func(t *testing.T) {
				store := config.NewStore(t.TempDir())
				if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
					t.Fatal(err)
				}
				stdout := &bytes.Buffer{}
				requests := 0
				app := cli.New(cli.Options{Store: store, Stdout: stdout, Stderr: io.Discard, HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { requests++; return jsonResponse(tc.body), nil })}})
				args := []string{"contract", "approval", "get", "process-1", "--profile", "contract", "--as", "user"}
				if format == "raw" {
					args = append(args, "--raw")
				} else {
					args = append(args, "--output", format)
				}
				err := app.Run(context.Background(), args)
				if (err != nil) != tc.wantErr || requests != 1 {
					t.Fatalf("error=%v requests=%d", err, requests)
				}
				if !tc.wantErr {
					if !strings.Contains(stdout.String(), "9223372036854775807") || !strings.Contains(stdout.String(), "MIGRATED_ATTACHMENTS_NOT_VERIFIABLE") {
						t.Fatalf("lost ID/completeness fields: %s", stdout.String())
					}
					if format == "json" {
						var body map[string]any
						if err := json.Unmarshal(stdout.Bytes(), &body); err != nil {
							t.Fatal(err)
						}
						data := body["data"].(map[string]any)["process_instance"].(map[string]any)
						if data["complete"] != false {
							t.Fatal("lost partial flag")
						}
					}
				}
			})
		}
	}
}
