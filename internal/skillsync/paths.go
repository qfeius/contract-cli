package skillsync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func canonical(target string) (string, error) {
	if strings.TrimSpace(target) == "" {
		return "", fmt.Errorf("empty Skill target")
	}
	abs, e := filepath.Abs(target)
	if e != nil {
		return "", e
	}
	// Resolve existing ancestors as well, so missing child directories cannot hide aliases.
	parent := abs
	var suffix []string
	for {
		_, e := os.Lstat(parent)
		if e == nil {
			break
		}
		if !os.IsNotExist(e) {
			return "", e
		}
		next := filepath.Dir(parent)
		if next == parent {
			return "", e
		}
		suffix = append(suffix, filepath.Base(parent))
		parent = next
	}
	real, e := filepath.EvalSymlinks(parent)
	if e != nil {
		return "", e
	}
	for i := len(suffix) - 1; i >= 0; i-- {
		real = filepath.Join(real, suffix[i])
	}
	return filepath.Clean(real), nil
}

func containsPath(parent, child string) bool {
	rel, e := filepath.Rel(parent, child)
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (s *Service) target(target string) (string, error) {
	p, e := canonical(target)
	if e != nil {
		return "", e
	}
	if IsClientManagedSkillTarget(runtime.GOOS, target) || IsClientManagedSkillTarget(runtime.GOOS, p) {
		return "", fmt.Errorf("client-managed system Skill directory; verify the Skill directory that the current client actually supports installing into and reads")
	}
	state, e := canonical(s.StateDir)
	if e != nil {
		return "", e
	}
	home, _ := os.UserHomeDir()
	// A client may launch the CLI with its user Skill directory as the working
	// directory (notably Doubao's .user_skills). The target was explicit or
	// already confirmed by the caller, so the working-directory check would
	// reject a valid user target. Keep the root/home/config containment guards.
	if filepath.Dir(p) == p || p == home || containsPath(p, state) || containsPath(state, p) {
		return "", fmt.Errorf("unsafe Skill target")
	}
	return p, nil
}

func targetKey(target string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(target)))
	return hex.EncodeToString(sum[:])[:24]
}
