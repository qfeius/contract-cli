package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPrivateConfigurationSaveReplacesRatherThanTruncates(t *testing.T) {
	for _, kind := range []string{"config", "secrets"} {
		t.Run(kind, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "state")
			if err := os.Mkdir(dir, 0755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, kind+".json")
			if err := os.WriteFile(path, []byte(`{"old":true}`), 0644); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			var saveErr error
			if kind == "config" {
				saveErr = NewStore(dir).Save(&File{Profiles: map[string]Profile{}})
			} else {
				saveErr = NewSecretsStore(dir).Set("fixture", "secret")
			}
			if saveErr != nil {
				t.Fatal(saveErr)
			}
			after, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if os.SameFile(before, after) {
				t.Fatal("private file was modified in place")
			}
			parent, err := os.Stat(dir)
			if err != nil {
				t.Fatal(err)
			}
			if runtime.GOOS != "windows" && (after.Mode().Perm() != 0600 || parent.Mode().Perm() != 0700) {
				t.Fatalf("insecure permissions: file=%o directory=%o", after.Mode().Perm(), parent.Mode().Perm())
			}
			files, err := filepath.Glob(filepath.Join(dir, ".private-*"))
			if err != nil || len(files) != 0 {
				t.Fatalf("temporary files leaked: %v %v", files, err)
			}
		})
	}
}

func TestPrivateSaveRejectsSymlinkWithoutChangingTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privileges vary on Windows")
	}
	dir := t.TempDir()
	external := filepath.Join(t.TempDir(), "external")
	const original = `{"profiles":{}}`
	if err := os.WriteFile(external, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(dir, "config.json")); err != nil {
		t.Fatal(err)
	}
	if err := NewStore(dir).Save(&File{Profiles: map[string]Profile{}}); err == nil {
		t.Fatal("symlink accepted")
	}
	body, err := os.ReadFile(external)
	if err != nil || string(body) != original {
		t.Fatalf("external file changed: %s %v", body, err)
	}
}
