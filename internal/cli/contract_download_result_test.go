package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestDownloadStructuredResultReportsActualSavedFile(t *testing.T) {
	for _, format := range []string{"json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			store := config.NewStore(dir)
			if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
				t.Fatal(err)
			}
			stdout := &bytes.Buffer{}
			app := cli.New(cli.Options{Store: store, Stdout: stdout, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host == "files.example.test" {
					return &http.Response{StatusCode: 200, Header: make(http.Header), ContentLength: 3, Body: io.NopCloser(strings.NewReader("abc"))}, nil
				}
				return jsonResponse(`{"code":0,"data":{"file_id":"123","file_name":"原始合同.pdf","file_size":999,"download_url":"https://files.example.test/file?secret=hidden"}}`), nil
			})}})
			path := filepath.Join(dir, "saved.pdf")
			if err := app.Run(context.Background(), []string{"contract", "download-file", "123", "--contract", "456", "--as", "user", "--profile", "contract", "--output-file", path, "--output", format}); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(stdout.String(), "hidden") || strings.Contains(stdout.String(), "download_url") {
				t.Fatal("leaked signed URL")
			}
			if format == "json" {
				var result struct {
					FileID     string `json:"file_id"`
					FileName   string `json:"file_name"`
					OutputPath string `json:"output_path"`
					FileSize   int64  `json:"file_size"`
				}
				if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result.FileID != "123" || result.FileName != "原始合同.pdf" || result.OutputPath != path || result.FileSize != 3 {
					t.Fatalf("result = %+v", result)
				}
			} else if !strings.Contains(stdout.String(), "file_size: 3") || !strings.Contains(stdout.String(), "output_path:") {
				t.Fatalf("bad YAML: %s", stdout)
			}
			if b, err := os.ReadFile(path); err != nil || string(b) != "abc" {
				t.Fatalf("saved file %q, %v", b, err)
			}
		})
	}
}

func TestDownloadRejectsInvalidOutputBeforeRequest(t *testing.T) {
	for _, flags := range [][]string{{"--output", "invalid"}, {"--raw", "--output", "json"}} {
		app := cli.New(cli.Options{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("unexpected request"); return nil, nil })}})
		err := app.Run(context.Background(), append([]string{"contract", "download-file", "123"}, flags...))
		if err == nil || (!strings.Contains(err.Error(), "output") && !strings.Contains(err.Error(), "raw")) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestAppDownloadStructuredResult(t *testing.T) {
	dir := t.TempDir()
	store := config.NewStore(dir)
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatal(err)
	}
	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{Store: store, Stdout: stdout, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), ContentLength: 4, Body: io.NopCloser(strings.NewReader("file"))}, nil
	})}})
	path := filepath.Join(dir, "app.txt")
	if err := app.Run(context.Background(), []string{"contract", "download-file", "123", "--as", "app", "--profile", "contract", "--output-file", path, "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["file_name"] != "app.txt" || result["file_size"] != float64(4) || result["output_path"] != path {
		t.Fatalf("result=%v", result)
	}
}
