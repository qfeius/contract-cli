package cli_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 本地文件的大小与格式限制由 CLM 预审准备入口继续验证。
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
		for _, args := range [][]string{{"review", "pre-audit", "prepare", "--file", path, "--name", "contract.pdf", "--dry-run"}, {"review", "run", "--data", string(spec), "--dry-run"}} {
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
	for _, ext := range []string{"doc", "docx", "pdf", "txt"} {
		filePath := filepath.Join(t.TempDir(), "contract."+ext)
		if err := os.WriteFile(filePath, []byte("contract"), 0600); err != nil {
			t.Fatal(err)
		}
		app, _ := alignmentApp(t, nil)
		err := app.Run(context.Background(), []string{"review", "pre-audit", "prepare", "--file", filePath, "--name", "contract." + ext, "--dry-run"})
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
	if err := app.Run(context.Background(), []string{"review", "pre-audit", "prepare", "--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "50 MiB") || strings.Contains(out.String(), "2 MiB") {
		t.Fatal("pre-audit upload help limit stale")
	}
}
