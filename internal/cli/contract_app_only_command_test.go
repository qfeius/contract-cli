package cli_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestContractAppOnlyCommandsUseExpectedEndpoints(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := config.NewStore(dir)
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	testCases := []struct {
		name         string
		args         []string
		wantMethod   string
		wantPath     string
		wantBody     string
		responseBody string
	}{
		{
			name:         "submit",
			args:         []string{"contract", "submit", "contract-1", "--profile", "contract", "--data", `{"comment":"ok"}`, "--user-id", "ou_123"},
			wantMethod:   http.MethodPost,
			wantPath:     "/open-apis/contract/v1/contracts/contract-1/submit",
			wantBody:     `{"comment":"ok"}`,
			responseBody: `{"code":0,"data":{"submitted":true}}`,
		},
		{
			name:         "resubmit without body",
			args:         []string{"contract", "resubmit", "contract-1", "--profile", "contract"},
			wantMethod:   http.MethodPost,
			wantPath:     "/open-apis/contract/v1/contracts/contract-1/resubmit",
			responseBody: `{"code":0,"data":{"resubmitted":true}}`,
		},
		{
			name:         "patch",
			args:         []string{"contract", "patch", "contract-1", "--profile", "contract", "--data", `{"title":"demo"}`},
			wantMethod:   http.MethodPatch,
			wantPath:     "/open-apis/contract/v1/contracts/contract-1",
			wantBody:     `{"title":"demo"}`,
			responseBody: `{"code":0,"data":{"patched":true}}`,
		},
		{
			name:         "delete",
			args:         []string{"contract", "delete", "contract-1", "--profile", "contract"},
			wantMethod:   http.MethodDelete,
			wantPath:     "/open-apis/contract/v1/contracts/contract-1",
			responseBody: `{"code":0,"data":{"deleted":true}}`,
		},
		{
			name:         "print file",
			args:         []string{"contract", "print-file", "--profile", "contract", "--data", `{"contract_id":"contract-1"}`},
			wantMethod:   http.MethodPost,
			wantPath:     "/open-apis/contract/v1/files",
			wantBody:     `{"contract_id":"contract-1"}`,
			responseBody: `{"code":0,"data":{"file_id":"print-file-1"}}`,
		},
		{
			name:         "share get",
			args:         []string{"contract", "share", "get", "contract-1", "--profile", "contract"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/contracts/contract-1/share_records",
			responseBody: `{"code":0,"data":{"items":[{"share_id":"share-1"}]}}`,
		},
		{
			name:         "cooperation link get",
			args:         []string{"contract", "cooperation", "link", "get", "contract-1", "--profile", "contract"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/contracts/contract-1/cooperation_link",
			responseBody: `{"code":0,"data":{"link":"https://example.test/cooperate"}}`,
		},
		{
			name:         "cooperation record get",
			args:         []string{"contract", "cooperation", "record", "get", "contract-1", "--profile", "contract"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/contracts/contract-1/cooperation_record_info",
			responseBody: `{"code":0,"data":{"records":[{"action":"open"}]}}`,
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
				HTTPClient: &http.Client{
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						if req.Method != tc.wantMethod {
							t.Fatalf("method = %s, want %s", req.Method, tc.wantMethod)
						}
						if req.URL.Path != tc.wantPath {
							t.Fatalf("path = %s, want %s", req.URL.Path, tc.wantPath)
						}
						if req.Header.Get("Authorization") != "Bearer app-token" {
							t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
						}
						if got := req.URL.Query().Get("user_id_type"); got != "user_id" {
							t.Fatalf("user_id_type = %q, want user_id", got)
						}
						if tc.name == "submit" && req.URL.Query().Get("user_id") != "ou_123" {
							t.Fatalf("user_id = %q, want ou_123", req.URL.Query().Get("user_id"))
						}
						body, err := io.ReadAll(req.Body)
						if err != nil {
							t.Fatalf("ReadAll() error = %v", err)
						}
						if string(body) != tc.wantBody {
							t.Fatalf("body = %q, want %q", string(body), tc.wantBody)
						}
						return jsonResponse(tc.responseBody), nil
					}),
				},
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

func TestContractDownloadFileCommandWritesOutputFileAsApp(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outputPath := filepath.Join(dir, "contract.pdf")
	store := config.NewStore(dir)
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	dialogCalled := false
	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
		Store:  store,
		SaveFileDialog: func(context.Context, string) (string, error) {
			dialogCalled = true
			return "", errors.New("dialog should not be called")
		},
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("method = %s", req.Method)
				}
				if req.URL.Path != "/open-apis/contract/v1/files/file-123" {
					t.Fatalf("path = %s", req.URL.Path)
				}
				return &http.Response{
					StatusCode:    http.StatusOK,
					Header:        make(http.Header),
					Body:          io.NopCloser(strings.NewReader("download bytes")),
					ContentLength: int64(len("download bytes")),
				}, nil
			}),
		},
	})

	err := app.Run(context.Background(), []string{
		"contract", "download-file", "file-123",
		"--profile", "contract",
		"--output-file", outputPath,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if dialogCalled {
		t.Fatalf("save dialog should not be used with --output-file")
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(output) error = %v", err)
	}
	if string(content) != "download bytes" {
		t.Fatalf("downloaded content = %q", string(content))
	}
	if !strings.Contains(stdout.String(), "Downloaded file to "+outputPath) {
		t.Fatalf("missing download message: %s", stdout.String())
	}
}

func TestContractDownloadFileCommandUsesSaveDialogByDefault(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outputPath := filepath.Join(dir, "from-dialog.pdf")
	store := config.NewStore(dir)
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	dialogSuggestions := []string{}
	app := cli.New(cli.Options{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Store:  store,
		SaveFileDialog: func(ctx context.Context, suggestedName string) (string, error) {
			dialogSuggestions = append(dialogSuggestions, suggestedName)
			return outputPath, nil
		},
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode:    http.StatusOK,
					Header:        make(http.Header),
					Body:          io.NopCloser(strings.NewReader("dialog bytes")),
					ContentLength: int64(len("dialog bytes")),
				}, nil
			}),
		},
	})

	if err := app.Run(context.Background(), []string{
		"contract", "download-file", "file-123",
		"--profile", "contract",
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(dialogSuggestions) != 1 || dialogSuggestions[0] != "file-123" {
		t.Fatalf("dialog suggestions = %v", dialogSuggestions)
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(output) error = %v", err)
	}
	if string(content) != "dialog bytes" {
		t.Fatalf("downloaded content = %q", string(content))
	}
}

func TestContractApprovalCommandsUseExpectedEndpoints(t *testing.T) {
	t.Parallel()

	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	testCases := []struct {
		name         string
		args         []string
		wantMethod   string
		wantPath     string
		wantQuery    map[string]string
		wantBody     string
		responseBody string
	}{
		{
			name:         "approval start",
			args:         []string{"contract", "approval", "start", "process-1", "--profile", "contract", "--data", `{"task_instance_id":"task-1","command_type":"general"}`},
			wantMethod:   http.MethodPost,
			wantPath:     "/open-apis/contract/v1/process_instances/process-1/task_approval",
			wantQuery:    map[string]string{"user_id_type": "user_id"},
			wantBody:     `{"task_instance_id":"task-1","command_type":"general"}`,
			responseBody: `{"code":0,"data":{"process_instance":{"process_instance_id":"process-1"}}}`,
		},
		{
			name:         "approval get",
			args:         []string{"contract", "approval", "get", "process-1", "--profile", "contract", "--notice-filter", "notice_filter", "--task-instance-filter", "task_instance_filter"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/process_instances/process-1",
			wantQuery:    map[string]string{"user_id_type": "user_id", "notice_filter": "notice_filter", "task_instance_filter": "task_instance_filter"},
			responseBody: `{"code":0,"data":{"process_instance":{"process_instance_id":"process-1"}}}`,
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
				HTTPClient: &http.Client{
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						if req.Method != tc.wantMethod {
							t.Fatalf("method = %s, want %s", req.Method, tc.wantMethod)
						}
						if req.URL.Path != tc.wantPath {
							t.Fatalf("path = %s, want %s", req.URL.Path, tc.wantPath)
						}
						for key, want := range tc.wantQuery {
							if got := req.URL.Query().Get(key); got != want {
								t.Fatalf("query %s = %q, want %q", key, got, want)
							}
						}
						if req.Header.Get("Authorization") != "Bearer app-token" {
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
					}),
				},
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

func TestContractDownloadFileCommandRawWritesStdout(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := config.NewStore(dir)
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
		Store:  store,
		SaveFileDialog: func(context.Context, string) (string, error) {
			return "", errors.New("dialog should not be called for --raw")
		},
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode:    http.StatusOK,
					Header:        make(http.Header),
					Body:          io.NopCloser(strings.NewReader("raw bytes")),
					ContentLength: int64(len("raw bytes")),
				}, nil
			}),
		},
	})

	if err := app.Run(context.Background(), []string{
		"contract", "download-file", "file-123",
		"--profile", "contract",
		"--raw",
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.String() != "raw bytes" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestContractDownloadFileCommandValidationAndForce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	existingPath := filepath.Join(dir, "existing.pdf")
	if err := os.WriteFile(existingPath, []byte("old bytes"), 0o600); err != nil {
		t.Fatalf("WriteFile(existing) error = %v", err)
	}
	store := config.NewStore(dir)
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	requests := 0
	app := cli.New(cli.Options{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Store:  store,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				return &http.Response{
					StatusCode:    http.StatusOK,
					Header:        make(http.Header),
					Body:          io.NopCloser(strings.NewReader("new bytes")),
					ContentLength: int64(len("new bytes")),
				}, nil
			}),
		},
	})

	err := app.Run(context.Background(), []string{
		"contract", "download-file", "file-123",
		"--profile", "contract",
		"--output-file", existingPath,
	})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected existing-file error: %v", err)
	}
	if requests != 0 {
		t.Fatalf("existing file should fail before HTTP, got %d requests", requests)
	}

	if err := app.Run(context.Background(), []string{
		"contract", "download-file", "file-123",
		"--profile", "contract",
		"--output-file", existingPath,
		"--force",
	}); err != nil {
		t.Fatalf("Run(force) error = %v", err)
	}
	content, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("ReadFile(existing) error = %v", err)
	}
	if string(content) != "new bytes" {
		t.Fatalf("content after force = %q", string(content))
	}
}

func TestContractDownloadFileCommandDialogFailureDoesNotSendHTTP(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := config.NewStore(dir)
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	requests := 0
	app := cli.New(cli.Options{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Store:  store,
		SaveFileDialog: func(context.Context, string) (string, error) {
			return "", errors.New("no gui")
		},
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				return nil, nil
			}),
		},
	})

	err := app.Run(context.Background(), []string{
		"contract", "download-file", "file-123",
		"--profile", "contract",
	})
	if err == nil || !strings.Contains(err.Error(), "select save path") || !strings.Contains(err.Error(), "--output-file") {
		t.Fatalf("unexpected dialog error: %v", err)
	}
	if requests != 0 {
		t.Fatalf("dialog failure should fail before HTTP, got %d requests", requests)
	}
}

func TestContractAppOnlyCommandsRejectUserIdentityBeforeHTTP(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := config.NewStore(dir)
	if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	testCases := [][]string{
		{"contract", "submit", "contract-1", "--profile", "contract", "--as", "user"},
		{"contract", "resubmit", "contract-1", "--profile", "contract", "--as", "user"},
		{"contract", "patch", "contract-1", "--profile", "contract", "--as", "user", "--data", `{"title":"demo"}`},
		{"contract", "delete", "contract-1", "--profile", "contract", "--as", "user"},
		{"contract", "print-file", "--profile", "contract", "--as", "user", "--data", `{"contract_id":"contract-1"}`},
		{"contract", "share", "get", "contract-1", "--profile", "contract", "--as", "user"},
		{"contract", "cooperation", "link", "get", "contract-1", "--profile", "contract", "--as", "user"},
		{"contract", "cooperation", "record", "get", "contract-1", "--profile", "contract", "--as", "user"},
		{"contract", "approval", "start", "process-1", "--profile", "contract", "--as", "user", "--data", `{"task_instance_id":"task-1"}`},
	}

	for _, args := range testCases {
		args := args
		t.Run(strings.Join(args[:min(4, len(args))], " "), func(t *testing.T) {
			t.Parallel()

			requests := 0
			dialogs := 0
			app := cli.New(cli.Options{
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
				Store:  store,
				SaveFileDialog: func(context.Context, string) (string, error) {
					dialogs++
					return filepath.Join(dir, "dialog.pdf"), nil
				},
				HTTPClient: &http.Client{
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						requests++
						return jsonResponse(`{"code":0}`), nil
					}),
				},
			})

			err := app.Run(context.Background(), args)
			if err == nil || !strings.Contains(err.Error(), "only supports --as app") {
				t.Fatalf("unexpected user error: %v", err)
			}
			if requests != 0 {
				t.Fatalf("user rejection should not send HTTP, got %d requests", requests)
			}
			if dialogs != 0 {
				t.Fatalf("user rejection should not open save dialog, got %d dialogs", dialogs)
			}
		})
	}
}

func TestContractAppOnlyCommandValidationErrors(t *testing.T) {
	t.Parallel()

	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	testCases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "submit missing id",
			args:    []string{"contract", "submit", "--profile", "contract"},
			wantErr: "usage: contract-cli contract submit <contract-id> [flags]",
		},
		{
			name:    "patch missing body",
			args:    []string{"contract", "patch", "contract-1", "--profile", "contract"},
			wantErr: "--input-file or --data is required",
		},
		{
			name:    "print missing body",
			args:    []string{"contract", "print-file", "--profile", "contract"},
			wantErr: "--input-file or --data is required",
		},
		{
			name:    "share missing get",
			args:    []string{"contract", "share"},
			wantErr: "missing contract share subcommand",
		},
		{
			name:    "cooperation missing resource",
			args:    []string{"contract", "cooperation"},
			wantErr: "missing contract cooperation resource",
		},
		{
			name:    "approval missing subcommand",
			args:    []string{"contract", "approval"},
			wantErr: "missing contract approval subcommand",
		},
		{
			name:    "approval start missing body",
			args:    []string{"contract", "approval", "start", "process-1", "--profile", "contract"},
			wantErr: "--input-file or --data is required",
		},
		{
			name:    "approval get missing id",
			args:    []string{"contract", "approval", "get", "--profile", "contract"},
			wantErr: "usage: contract-cli contract approval get <process-instance-id> [flags]",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := cli.New(cli.Options{
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
				Store:  store,
				HTTPClient: &http.Client{
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						t.Fatalf("validation error should not send HTTP")
						return nil, nil
					}),
				},
			})

			err := app.Run(context.Background(), tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("unexpected error: %v, want %q", err, tc.wantErr)
			}
		})
	}
}
