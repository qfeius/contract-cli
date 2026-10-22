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

func TestReviewUpload50MiB(t *testing.T) {
	const limit = 50 << 20
	path := filepath.Join(t.TempDir(), "contract.pdf")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	for _, size := range []int64{(2 << 20) + 1, limit, limit + 1} {
		if err := file.Truncate(size); err != nil {
			t.Fatal(err)
		}
		spec, _ := json.Marshal(map[string]any{"source": map[string]any{"type": "file", "path": path, "name": "contract.pdf"}, "config": json.RawMessage(alignmentConfig)})
		for _, args := range [][]string{{"review", "file", "upload", "--file", path, "--name", "contract.pdf", "--dry-run"}, {"review", "run", "--data", string(spec), "--dry-run"}} {
			app, _ := alignmentApp(t, nil)
			err := app.Run(context.Background(), args)
			if size <= limit && err != nil {
				t.Errorf("%s size %d should pass: %v", args[1], size, err)
			}
			if size > limit && (err == nil || !strings.Contains(err.Error(), "50 MiB")) {
				t.Errorf("oversize should fail at 50MiB: %v", err)
			}
		}
	}
	for _, size := range []int{limit, limit + 1} {
		t.Run("stdin "+string(rune(size-limit+'0')), func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
				t.Fatal(err)
			}
			calls := 0
			app := cli.New(cli.Options{Store: store, Stdin: bytes.NewReader(make([]byte, size)), Stdout: io.Discard, Stderr: io.Discard, HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				reader, err := r.MultipartReader()
				if err != nil {
					t.Fatal(err)
				}
				part, err := reader.NextPart()
				if err != nil {
					t.Fatal(err)
				}
				n, err := io.Copy(io.Discard, part)
				if err != nil || n != limit {
					t.Fatalf("wrong upload length %d: %v", n, err)
				}
				return jsonResponse(`{"code":200,"data":{}}`), nil
			})}})
			err := app.Run(context.Background(), []string{"review", "file", "upload", "--stdin", "--name", "contract.docx"})
			if size == limit && (err != nil || calls != 1) {
				t.Fatalf("50MiB stdin rejected: %v calls=%d", err, calls)
			}
			if size > limit && (err == nil || calls != 0 || !strings.Contains(err.Error(), "50 MiB")) {
				t.Fatalf("oversized stdin not rejected: %v calls=%d", err, calls)
			}
		})
	}
	for _, ext := range []string{"doc", "docx", "pdf", "txt"} {
		app, _ := alignmentApp(t, nil)
		err := app.Run(context.Background(), []string{"review", "file", "upload", "--stdin", "--name", "contract." + ext, "--dry-run"})
		if (err == nil) != (ext != "txt") {
			t.Fatalf("extension %s: %v", ext, err)
		}
	}
	app, _ := alignmentApp(t, nil)
	err = app.Run(context.Background(), []string{"review", "task", "start", "--data", alignmentStart() + strings.Repeat(" ", 2<<20), "--dry-run"})
	if err == nil || !strings.Contains(err.Error(), "JSON 输入不能超过 2 MiB") {
		t.Fatalf("JSON limit changed: %v", err)
	}
	app, out := alignmentApp(t, nil)
	if err := app.Run(context.Background(), []string{"review", "file", "upload", "--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "50 MiB") || strings.Contains(out.String(), "2 MiB") {
		t.Fatal("upload help limit stale")
	}
}
