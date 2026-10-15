package cli_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestContractSummaryGenerateDirectlyUploadsFileToGenerate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	uploadPath := filepath.Join(dir, "SMOKE-001.docx")
	if err := os.WriteFile(uploadPath, []byte("docx bytes"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	store := config.NewStore(dir)
	if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	requests := 0
	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
		Store:  store,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if requests != 1 {
					t.Fatalf("unexpected request %d: %s %s", requests, req.Method, req.URL.Path)
				}
				if req.Method != http.MethodPost || req.URL.Path != "/open-apis/contract/v1/summary/generate" {
					t.Fatalf("request = %s %s", req.Method, req.URL.Path)
				}
				if req.Header.Get("Authorization") != "Bearer user-token" {
					t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
				}
				if req.URL.Query().Get("user_id_type") != "user_id" {
					t.Fatalf("user_id_type = %q", req.URL.Query().Get("user_id_type"))
				}
				assertSummaryGenerateMultipart(t, req, "SMOKE-001.docx", "docx bytes")
				return jsonResponse(`{"code":0,"data":{"summary":"摘要内容"},"msg":"success"}`), nil
			}),
		},
	})

	err := app.Run(context.Background(), []string{
		"contract", "summary", "generate",
		"--profile", "contract",
		"--as", "user",
		"--file", uploadPath,
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
	if !strings.Contains(stdout.String(), `"summary": "摘要内容"`) {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestContractSummaryGenerateRejectsUnsupportedOutputBeforeHTTP(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	uploadPath := filepath.Join(dir, "SMOKE-001.docx")
	if err := os.WriteFile(uploadPath, []byte("docx bytes"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	store := config.NewStore(dir)
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
		"contract", "summary", "generate",
		"--profile", "contract",
		"--as", "user",
		"--file", uploadPath,
		"--output", "unsupported-format",
	})
	if err == nil || !strings.Contains(err.Error(), `unsupported output format "unsupported-format"`) {
		t.Fatalf("Run() error = %v, want unsupported output rejection", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestContractSummaryGenerateSupportsFileName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	uploadPath := filepath.Join(dir, "local.docx")
	if err := os.WriteFile(uploadPath, []byte("renamed bytes"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	store := config.NewStore(dir)
	if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Store:  store,
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				assertSummaryGenerateMultipart(t, req, "SMOKE-001.docx", "renamed bytes")
				return jsonResponse(`{"code":0,"data":{"summary":"摘要内容"},"msg":"success"}`), nil
			}),
		},
	})

	err := app.Run(context.Background(), []string{
		"contract", "summary", "generate",
		"--profile", "contract",
		"--as", "user",
		"--file", uploadPath,
		"--file-name", "SMOKE-001.docx",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestContractSummaryGenerateRequiresFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := config.NewStore(dir)
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
		"contract", "summary", "generate",
		"--profile", "contract",
		"--as", "user",
	})
	if err == nil || !strings.Contains(err.Error(), "--file is required") {
		t.Fatalf("Run() error = %v, want missing file", err)
	}
}

func TestContractSummaryGenerateRejectsJSONBody(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	uploadPath := filepath.Join(dir, "SMOKE-001.docx")
	if err := os.WriteFile(uploadPath, []byte("docx bytes"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	store := config.NewStore(dir)
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
		"contract", "summary", "generate",
		"--profile", "contract",
		"--as", "user",
		"--file", uploadPath,
		"--data", `{"file_id":"summary-file-1"}`,
	})
	if err == nil || !strings.Contains(err.Error(), "contract summary generate does not accept --input-file or --data; use --file for binary upload") {
		t.Fatalf("Run() error = %v, want JSON body rejection", err)
	}
}

func TestContractSummaryGenerateRejectsAppIdentityBeforeHTTP(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	uploadPath := filepath.Join(dir, "SMOKE-001.docx")
	if err := os.WriteFile(uploadPath, []byte("docx bytes"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	store := config.NewStore(dir)
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
		"contract", "summary", "generate",
		"--profile", "contract",
		"--as", "app",
		"--file", uploadPath,
	})
	if err == nil || !strings.Contains(err.Error(), "only supports --as user") {
		t.Fatalf("Run() error = %v, want user-only identity rejection", err)
	}
}

func TestContractSummaryGenerateRejectsUnsupportedFileTypeBeforeHTTP(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	uploadPath := filepath.Join(dir, "SMOKE-001.txt")
	if err := os.WriteFile(uploadPath, []byte("plain text"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	store := config.NewStore(dir)
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
		"contract", "summary", "generate",
		"--profile", "contract",
		"--as", "user",
		"--file", uploadPath,
	})
	if err == nil || !strings.Contains(err.Error(), "only supports doc, docx, or pdf") {
		t.Fatalf("Run() error = %v, want unsupported file type rejection", err)
	}
}

func assertSummaryGenerateMultipart(t *testing.T, req *http.Request, wantFileName, wantContent string) {
	t.Helper()

	if got := req.Header.Get("Content-Type"); !strings.HasPrefix(got, "multipart/form-data; boundary=") {
		t.Fatalf("content-type = %q", got)
	}
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("ParseMultipartForm() error = %v", err)
	}
	if len(req.MultipartForm.Value["file_name"]) != 0 || len(req.MultipartForm.Value["file_type"]) != 0 {
		t.Fatalf("unexpected metadata fields: %v", req.MultipartForm.Value)
	}
	files := req.MultipartForm.File["file"]
	if len(files) != 1 {
		t.Fatalf("file parts = %d", len(files))
	}
	if files[0].Filename != wantFileName {
		t.Fatalf("filename = %q, want %q", files[0].Filename, wantFileName)
	}
	uploaded, err := files[0].Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = uploaded.Close() }()
	content, err := io.ReadAll(uploaded)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(content) != wantContent {
		t.Fatalf("uploaded content = %q, want %q", string(content), wantContent)
	}
}
