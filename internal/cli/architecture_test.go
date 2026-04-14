package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIProductionCodeAvoidsDirectHTTPCalls(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", file, err)
		}
		text := string(content)
		if strings.Contains(text, "http.NewRequest(") {
			t.Fatalf("cli production file %q should not issue direct HTTP requests", file)
		}
	}
}
