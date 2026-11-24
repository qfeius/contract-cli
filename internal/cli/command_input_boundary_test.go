package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultCommandInputRejectsOversizedJSONBeforeParsing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversized.json")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate((32 << 20) + 1); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	body, err := readCommandInput(context.Background(), path)
	if err == nil || len(body) != 0 || !strings.Contains(err.Error(), "input exceeds") {
		t.Fatalf("unbounded input accepted: bytes=%d error=%v", len(body), err)
	}
}

func TestDefaultInjectedCommandReaderIsAlsoBounded(t *testing.T) {
	body, err := readCommandReader(context.Background(), strings.NewReader(strings.Repeat("x", (32<<20)+1)), 0)
	if err == nil || len(body) != 0 || !strings.Contains(err.Error(), "input exceeds") {
		t.Fatalf("unbounded reader accepted: bytes=%d error=%v", len(body), err)
	}
}
