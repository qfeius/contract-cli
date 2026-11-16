package privatefile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFailedReplacementKeepsPreviousPrivateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := Write(path, []byte("original")); err != nil {
		t.Fatal(err)
	}
	if err := Replace(filepath.Join(dir, "missing"), path); err == nil {
		t.Fatal("missing replacement unexpectedly succeeded")
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != "original" {
		t.Fatalf("previous private file was lost: %s %v", body, err)
	}
	if err := Write(path, []byte("replacement")); err != nil {
		t.Fatal(err)
	}
	body, err = os.ReadFile(path)
	if err != nil || string(body) != "replacement" {
		t.Fatalf("complete replacement failed: %s %v", body, err)
	}
}

func TestPrivateWriteRefusesDirectoryAndCleansTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "state.json")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := Write(target, []byte("new state")); err == nil {
		t.Fatal("directory replaced by a private file")
	}
	files, err := filepath.Glob(filepath.Join(dir, ".private-*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary files leaked: %v %v", files, err)
	}
}
