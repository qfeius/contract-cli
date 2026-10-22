package skillsync

import (
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Exact, whole-directory fingerprints from verified official 1.9.3 / 1.9.4
// installations. No version-line normalization: edited content is unverified.
//
//go:embed history_fingerprints.json
var historicalFingerprintsJSON []byte

func treeFingerprint(root fs.FS) (string, error) {
	h := sha256.New()
	err := fs.WalkDir(root, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("historical Skill contains a symbolic link")
		}
		if name == "." {
			return nil
		}
		if d.IsDir() {
			_, _ = fmt.Fprintf(h, "d:%s\x00", name)
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("historical Skill contains a non-regular file")
		}
		b, err := fs.ReadFile(root, name)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(h, "f:%s\x00%x\x00", name, sha256.Sum256(b))
		return nil
	})
	return fmt.Sprintf("%x", h.Sum(nil)), err
}

// verifyHistory checks every occupied incoming official name before the whole
// location can be adopted. An unrelated name is outside the replacement scope.
func (s *Service) verifyHistory(target string) (found bool, err error) {
	return s.verifyHistoricalNames(target, s.Names)
}

func (s *Service) verifyHistoricalNames(target string, names []string) (found bool, err error) {
	fingerprints := s.historyFingerprints
	if fingerprints == nil {
		if err = json.Unmarshal(historicalFingerprintsJSON, &fingerprints); err != nil {
			return false, err
		}
	}
	for _, name := range names {
		dir := filepath.Join(target, name)
		info, e := os.Lstat(dir)
		if os.IsNotExist(e) {
			continue
		}
		found = true
		if e != nil {
			return found, e
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return found, fmt.Errorf("unverified historical Skill %s: not a regular directory", name)
		}
		digest, e := treeFingerprint(os.DirFS(dir))
		if e != nil {
			return found, e
		}
		matched := false
		for _, known := range fingerprints[name] {
			if digest == known {
				matched = true
				break
			}
		}
		if !matched {
			return found, fmt.Errorf("unverified historical Skill %s: contents do not match a verified official release; confirm the actual client Skill directory and explicitly install the complete bundle with --target and --force", name)
		}
	}
	return found, nil
}
