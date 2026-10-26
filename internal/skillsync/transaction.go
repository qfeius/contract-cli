package skillsync

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type entry struct {
	Name    string `json:"name"`
	Existed bool   `json:"existed"`
	Install bool   `json:"install"`
}
type journal struct {
	Target  string  `json:"target"`
	ID      string  `json:"id"`
	Entries []entry `json:"entries"`
}

// TransactionFailure retains both the replacement and rollback errors, without
// claiming that an unobserved recovery succeeded.
type TransactionFailure struct {
	Cause       error
	RollbackErr error
}

func (e *TransactionFailure) Error() string {
	if e.RollbackErr != nil {
		return fmt.Sprintf("%v; rollback failed: %v", e.Cause, e.RollbackErr)
	}
	return fmt.Sprintf("%v; original entries restored", e.Cause)
}

func (e *TransactionFailure) Unwrap() []error {
	if e.RollbackErr == nil {
		return []error{e.Cause}
	}
	return []error{e.Cause, e.RollbackErr}
}

func RecoveryStatus(err error) string {
	var failure *TransactionFailure
	if errors.As(err, &failure) {
		if failure.RollbackErr != nil {
			return "failed"
		}
		return "restored"
	}
	return "unconfirmed"
}

func randomID() (string, error) {
	b := make([]byte, 16)
	_, e := rand.Read(b)
	return hex.EncodeToString(b), e
}
func transactionDir(target string) string {
	return filepath.Join(filepath.Dir(target), ".contract-cli-sync-"+targetKey(target))
}
func exists(name string) bool { _, e := os.Lstat(name); return e == nil }

func RewriteVersion(content []byte, version string) []byte {
	if strings.TrimSpace(version) == "" {
		return content
	}
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content
	}
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			lines = append(lines[:i], append([]string{"version: " + version}, lines[i:]...)...)
			return []byte(strings.Join(lines, "\n"))
		}
		key, _, ok := strings.Cut(lines[i], ":")
		if ok && strings.TrimRight(key, " \t") == "version" {
			lines[i] = "version: " + version
			return []byte(strings.Join(lines, "\n"))
		}
	}
	return content
}

func (s *Service) prepare(stage string, names []string) error {
	for _, name := range names {
		if _, e := fs.ReadFile(s.Source, path.Join(name, "SKILL.md")); e != nil {
			return fmt.Errorf("missing official Skill %s: %w", name, e)
		}
		if e := fs.WalkDir(s.Source, name, func(current string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.Type()&os.ModeSymlink != 0 {
				return errors.New("bundled Skill symlink rejected")
			}
			dest := filepath.Join(stage, filepath.FromSlash(current))
			if d.IsDir() {
				return os.MkdirAll(dest, 0755)
			}
			if !d.Type().IsRegular() {
				return errors.New("non-regular bundled Skill file")
			}
			b, e := fs.ReadFile(s.Source, current)
			if e != nil {
				return e
			}
			if path.Base(current) == "SKILL.md" {
				b = RewriteVersion(b, s.Version)
			}
			if e = os.WriteFile(dest, b, 0644); e != nil {
				return e
			}
			actual, e := os.ReadFile(dest)
			if e != nil {
				return e
			}
			if !bytes.Equal(b, actual) {
				return errors.New("staged Skill checksum mismatch")
			}
			return nil
		}); e != nil {
			return e
		}
	}
	return nil
}

func (s *Service) rollback(j journal, dir string) error {
	// A backup proves the original was moved. A missing staged entry proves a new entry was installed.
	var result error
	for i := len(j.Entries) - 1; i >= 0; i-- {
		item := j.Entries[i]
		dest := filepath.Join(j.Target, item.Name)
		backup := filepath.Join(dir, "backup", item.Name)
		staged := filepath.Join(dir, "stage", item.Name)
		if exists(backup) {
			if e := os.RemoveAll(dest); e != nil {
				result = errors.Join(result, e)
				continue
			}
			if e := os.Rename(backup, dest); e != nil {
				result = errors.Join(result, e)
			}
		} else if !item.Existed && item.Install && !exists(staged) {
			result = errors.Join(result, os.RemoveAll(dest))
		}
	}
	if result == nil {
		result = os.RemoveAll(dir)
	}
	return result
}

func (s *Service) recover(target string, r Registry) error {
	dir := transactionDir(target)
	if !exists(dir) {
		return nil
	}
	// No journal means preparation was interrupted before touching any live entry.
	b, e := os.ReadFile(filepath.Join(dir, "journal.json"))
	if os.IsNotExist(e) {
		return os.RemoveAll(dir)
	}
	if e != nil {
		return e
	}
	var j journal
	if e = json.Unmarshal(b, &j); e != nil {
		return e
	}
	if j.Target != target || j.ID == "" {
		return errors.New("invalid transaction journal")
	}
	for _, item := range j.Entries {
		if item.Name != markerName && !validName(item.Name) {
			return errors.New("unsafe transaction journal entry")
		}
	}
	for _, l := range r.Locations {
		if l.Target == target && l.Transaction == j.ID {
			return os.RemoveAll(dir)
		}
	}
	s.log().Warn("recovering interrupted Skill transaction", "location", targetKey(target))
	if rollbackErr := s.rollback(j, dir); rollbackErr != nil {
		return &TransactionFailure{Cause: errors.New("interrupted Skill transaction recovery failed"), RollbackErr: rollbackErr}
	}
	return nil
}

func (s *Service) replace(target string, names, remove []string, location *Location, commit func(string) error) error {
	dir := transactionDir(target)
	if e := os.Mkdir(dir, 0700); e != nil {
		return e
	}
	id, e := randomID()
	if e != nil {
		_ = os.RemoveAll(dir)
		return e
	}
	stage := filepath.Join(dir, "stage")
	backup := filepath.Join(dir, "backup")
	if e = os.MkdirAll(stage, 0700); e == nil {
		e = os.MkdirAll(backup, 0700)
	}
	if e == nil {
		e = s.prepare(stage, names)
	}
	if e != nil {
		_ = os.RemoveAll(dir)
		return e
	}
	j := journal{Target: target, ID: id}
	for _, name := range names {
		j.Entries = append(j.Entries, entry{Name: name, Existed: exists(filepath.Join(target, name)), Install: true})
	}
	for _, name := range remove {
		j.Entries = append(j.Entries, entry{Name: name, Existed: exists(filepath.Join(target, name))})
	}
	if location != nil {
		if e = atomicJSON(filepath.Join(stage, markerName), marker{ID: location.ID, Source: sourceName}); e != nil {
			_ = os.RemoveAll(dir)
			return e
		}
		j.Entries = append(j.Entries, entry{Name: markerName, Existed: exists(filepath.Join(target, markerName)), Install: true})
	}
	// Skill leaf links are moved as links, never modified through their destination.
	// A management marker must remain a real file.
	for _, item := range j.Entries {
		info, statErr := os.Lstat(filepath.Join(target, item.Name))
		if statErr != nil && !os.IsNotExist(statErr) {
			_ = os.RemoveAll(dir)
			return statErr
		}
		if statErr == nil && item.Name == markerName && info.Mode()&os.ModeSymlink != 0 {
			_ = os.RemoveAll(dir)
			return errors.New("managed Skill symlink rejected")
		}
	}
	if e = atomicJSON(filepath.Join(dir, "journal.json"), j); e != nil {
		_ = os.RemoveAll(dir)
		return e
	}
	rename := s.Rename
	if rename == nil {
		rename = os.Rename
	}
	for _, item := range j.Entries {
		dest := filepath.Join(target, item.Name)
		if item.Existed {
			e = rename(dest, filepath.Join(backup, item.Name))
		}
		if e == nil && item.Install {
			e = rename(filepath.Join(stage, item.Name), dest)
		}
		if e != nil {
			break
		}
	}
	if e == nil {
		e = s.verify(target, names)
	}
	if e == nil && commit != nil {
		e = commit(id)
	}
	if e != nil {
		s.log().Error("Skill transaction failed; rolling back", "location", targetKey(target))
		rollbackErr := s.rollback(j, dir)
		return &TransactionFailure{Cause: e, RollbackErr: rollbackErr}
	}
	if cleanupErr := os.RemoveAll(dir); cleanupErr != nil {
		s.log().Warn("Skill committed; cleanup deferred", "location", targetKey(target))
	}
	return nil
}

func (s *Service) verify(target string, names []string) error {
	for _, name := range names {
		if e := fs.WalkDir(s.Source, name, func(current string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			want, e := fs.ReadFile(s.Source, current)
			if e != nil {
				return e
			}
			if path.Base(current) == "SKILL.md" {
				want = RewriteVersion(want, s.Version)
			}
			actual, e := os.ReadFile(filepath.Join(target, filepath.FromSlash(current)))
			if e != nil {
				return e
			}
			if !bytes.Equal(want, actual) {
				return errors.New("installed Skill checksum mismatch")
			}
			return nil
		}); e != nil {
			return e
		}
	}
	return nil
}
