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
			const original = `{"old":true}`
			if err := os.WriteFile(path, []byte(original), 0644); err != nil {
				t.Fatal(err)
			}
			// Keep an independent name for the old file across the replacement.
			oldPath := filepath.Join(dir, "old-link")
			if err := os.Link(path, oldPath); err != nil {
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
			if old, err := os.ReadFile(oldPath); err != nil || string(old) != original {
				t.Fatalf("original file was modified in place: %v", err)
			}
			if saved, err := os.ReadFile(path); err != nil || string(saved) == original {
				t.Fatalf("new state did not replace the original: %v", err)
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
