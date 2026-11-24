package skillsync

import (
	"path/filepath"
	"testing"
)

func canonicalSkillTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
