package skillsync

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func fixture(t *testing.T) *Service {
	t.Helper()
	return &Service{StateDir: t.TempDir(), Source: fstest.MapFS{"auth/SKILL.md": {Data: []byte("---\nname: auth\nversion: old\n---\nnew")}}, Names: []string{"auth"}, Version: "2.0.0"}
}

func read(t *testing.T, name string) string {
	t.Helper()
	b, e := os.ReadFile(name)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func put(t *testing.T, name, value string) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(name), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(name, []byte(value), 0600); e != nil {
		t.Fatal(e)
	}
}

func TestInstallSyncPreservesThirdPartyAndRemovesLocalChanges(t *testing.T) {
	s := fixture(t)
	target := filepath.Join(t.TempDir(), "skills")
	if _, e := s.Install(target, s.Names, true, true); e != nil {
		t.Fatal(e)
	}
	put(t, filepath.Join(target, "auth", "local.txt"), "custom")
	put(t, filepath.Join(target, "third-party", "SKILL.md"), "third-party")
	put(t, filepath.Join(target, "auth", "SKILL.md"), "custom")
	results, e := s.Sync(nil)
	if e != nil {
		t.Fatal(e)
	}
	if len(results) != 1 || results[0].Status != "success" {
		t.Fatalf("results=%#v", results)
	}
	if !strings.Contains(read(t, filepath.Join(target, "auth", "SKILL.md")), "version: 2.0.0") {
		t.Fatal("version missing")
	}
	if _, e := os.Stat(filepath.Join(target, "auth", "local.txt")); !os.IsNotExist(e) {
		t.Fatal("local file retained")
	}
	if read(t, filepath.Join(target, "third-party", "SKILL.md")) != "third-party" {
		t.Fatal("third party changed")
	}
}

func TestSyncDeletedRootDoesNotRecreateAndPartialDeletionRepairs(t *testing.T) {
	s := fixture(t)
	a := filepath.Join(t.TempDir(), "a")
	b := filepath.Join(t.TempDir(), "b")
	for _, p := range []string{a, b} {
		if _, e := s.Install(p, s.Names, true, true); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.RemoveAll(a); e != nil {
		t.Fatal(e)
	}
	if e := os.RemoveAll(filepath.Join(b, "auth")); e != nil {
		t.Fatal(e)
	}
	r, e := s.Sync(nil)
	if e != nil {
		t.Fatal(e)
	}
	if len(r) != 2 {
		t.Fatal(r)
	}
	if _, e := os.Stat(a); !os.IsNotExist(e) {
		t.Fatal("deleted root recreated")
	}
	if !strings.Contains(read(t, filepath.Join(b, "auth", "SKILL.md")), "new") {
		t.Fatal("not repaired")
	}
}

func TestRenameFailureRollsBackAndOtherTargetContinues(t *testing.T) {
	s := fixture(t)
	a := filepath.Join(t.TempDir(), "a")
	b := filepath.Join(t.TempDir(), "b")
	for _, p := range []string{a, b} {
		if _, e := s.Install(p, s.Names, true, true); e != nil {
			t.Fatal(e)
		}
		put(t, filepath.Join(p, "auth", "SKILL.md"), "old")
	}
	s.Rename = func(from, to string) error {
		if strings.Contains(from, "stage") && to == filepath.Join(a, "auth") {
			return errors.New("injected replacement failure")
		}
		return os.Rename(from, to)
	}
	r, e := s.Sync(nil)
	if e != nil {
		t.Fatal(e)
	}
	if r[0].Status != "failed" || r[1].Status != "success" {
		t.Fatal(r)
	}
	if read(t, filepath.Join(a, "auth", "SKILL.md")) != "old" {
		t.Fatal("old version lost")
	}
	if len(s.Pending()) != 1 {
		t.Fatal("failure not persisted")
	}
	s.Rename = nil
	if _, e := s.Install(a, s.Names, true, true); e != nil {
		t.Fatal(e)
	}
	if len(s.Pending()) != 0 {
		t.Fatal("repair did not clear failure")
	}
}

func TestSkipAndSelectedInstallDoNotConfirm(t *testing.T) {
	for _, selected := range []bool{false, true} {
		s := fixture(t)
		target := filepath.Join(t.TempDir(), "skills")
		if !selected {
			put(t, filepath.Join(target, "auth", "SKILL.md"), "custom")
		}
		if _, e := s.Install(target, s.Names, selected, !selected); e != nil {
			t.Fatal(e)
		}
		r, e := s.Sync(nil)
		if e != nil || len(r) != 0 {
			t.Fatalf("unconfirmed location synced: %v %v", r, e)
		}
	}
}

func TestOfficialRemovalUsesPreviousList(t *testing.T) {
	s := fixture(t)
	target := filepath.Join(t.TempDir(), "skills")
	if _, e := s.Install(target, s.Names, true, true); e != nil {
		t.Fatal(e)
	}
	put(t, filepath.Join(target, "contract-cli-unrelated", "SKILL.md"), "keep")
	s.Names = []string{"new"}
	s.Source = fstest.MapFS{"new/SKILL.md": {Data: []byte("---\nname: new\n---\nnew")}}
	if _, e := s.Sync(nil); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(target, "auth")); !os.IsNotExist(e) {
		t.Fatal("retired official retained")
	}
	if read(t, filepath.Join(target, "contract-cli-unrelated", "SKILL.md")) != "keep" {
		t.Fatal("prefix deletion")
	}
}

func TestHistoryOnlyUsesExplicitCandidatesAndNeverCreatesAbsentPaths(t *testing.T) {
	s := fixture(t)
	known := filepath.Join(t.TempDir(), "skills")
	absent := filepath.Join(t.TempDir(), "absent")
	put(t, filepath.Join(known, "auth", "SKILL.md"), "verified historical content")
	trustHistoryFixture(t, s, known)
	r, e := s.Sync([]string{known, absent})
	if e != nil {
		t.Fatal(e)
	}
	if len(r) != 1 || r[0].Status != "success" {
		t.Fatal(r)
	}
	if _, e := os.Stat(absent); !os.IsNotExist(e) {
		t.Fatal("absent history created")
	}
}

func TestInterruptedReplacementRecoversBeforeRetry(t *testing.T) {
	s := fixture(t)
	target := filepath.Join(t.TempDir(), "skills")
	if _, e := s.Install(target, s.Names, true, true); e != nil {
		t.Fatal(e)
	}
	put(t, filepath.Join(target, "auth", "SKILL.md"), "old before crash")
	s.Rename = func(from, to string) error {
		e := os.Rename(from, to)
		if e == nil && strings.Contains(from, "stage") && to == filepath.Join(target, "auth") {
			panic("simulated process interruption")
		}
		return e
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("interruption not injected")
			}
		}()
		_, _ = s.Sync(nil)
	}()
	s.Rename = nil
	r, e := s.Sync(nil)
	if e != nil || r[0].Status != "success" {
		t.Fatalf("recovery failed: %v %v", r, e)
	}
	if _, e := os.Stat(transactionDir(target)); !os.IsNotExist(e) {
		t.Fatal("transaction not cleaned")
	}
}

func TestHistoricalFailureCanRetryWithoutMarker(t *testing.T) {
	s := fixture(t)
	target := filepath.Join(t.TempDir(), "skills")
	put(t, filepath.Join(target, "auth", "SKILL.md"), "historical")
	trustHistoryFixture(t, s, target)
	s.Rename = func(from, to string) error {
		if strings.Contains(from, "stage") {
			return errors.New("failure")
		}
		return os.Rename(from, to)
	}
	r, e := s.Sync([]string{target})
	if e != nil || r[0].Status != "failed" {
		t.Fatal(r, e)
	}
	s.Rename = nil
	r, e = s.Sync([]string{target})
	if e != nil || r[0].Status != "success" {
		t.Fatalf("historical retry: %v %v", r, e)
	}
}

func TestMarkerRemovalInvalidatesRecreatedRoot(t *testing.T) {
	s := fixture(t)
	target := filepath.Join(t.TempDir(), "skills")
	if _, e := s.Install(target, s.Names, true, true); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(filepath.Join(target, markerName)); e != nil {
		t.Fatal(e)
	}
	put(t, filepath.Join(target, "auth", "SKILL.md"), "unmanaged replacement")
	r, e := s.Sync([]string{target})
	if e != nil || r[0].Status != "invalid" {
		t.Fatal(r, e)
	}
	if read(t, filepath.Join(target, "auth", "SKILL.md")) != "unmanaged replacement" {
		t.Fatal("recreated root overwritten")
	}
}

func TestUnsafeTargetsAndMissingBundleDoNotOverwrite(t *testing.T) {
	s := fixture(t)
	for _, target := range []string{s.StateDir, filepath.Dir(s.StateDir), filepath.VolumeName(s.StateDir) + string(filepath.Separator)} {
		if _, e := s.Install(target, s.Names, true, true); e == nil {
			t.Errorf("accepted %s", target)
		}
	}
	target := filepath.Join(t.TempDir(), "skills")
	put(t, filepath.Join(target, "auth", "SKILL.md"), "old")
	s.Source = fstest.MapFS{}
	if _, e := s.Install(target, s.Names, true, true); e == nil {
		t.Fatal("missing bundle accepted")
	}
	if read(t, filepath.Join(target, "auth", "SKILL.md")) != "old" {
		t.Fatal("old overwritten")
	}
}

func TestUserSkillTargetMayBeCurrentWorkingDirectory(t *testing.T) {
	s := fixture(t)
	target := filepath.Join(t.TempDir(), ".doubao", "agent_mode", "workspace", ".user_skills")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(target)

	if _, err := s.Install(target, s.Names, true, true); err != nil {
		t.Fatalf("user Skill target was rejected when it is the working directory: %v", err)
	}
	if got := read(t, filepath.Join(target, "auth", "SKILL.md")); !strings.Contains(got, "version: 2.0.0") {
		t.Fatalf("user Skill target was not installed: %q", got)
	}
}

func TestInstallAdditionalLocationPreservesExistingRegistration(t *testing.T) {
	state := t.TempDir()
	oldTarget := filepath.Join(t.TempDir(), "old-skills")
	newTarget := filepath.Join(t.TempDir(), "new-skills")
	s := fixture(t)
	s.StateDir = state
	if _, err := s.Install(oldTarget, s.Names, true, true); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(oldTarget, "client-marker.txt"), "keep old files")
	_, err := s.Install(newTarget, s.Names, true, true)
	if err != nil {
		t.Fatal(err)
	}
	locations, err := s.RegisteredLocations()
	if err != nil {
		t.Fatal(err)
	}
	if len(locations) != 2 {
		t.Fatalf("locations after replacement = %+v", locations)
	}
	seen := map[string]bool{}
	for _, location := range locations {
		seen[location.Target] = true
	}
	if !seen[oldTarget] || !seen[newTarget] {
		t.Fatalf("missing location: %+v", locations)
	}
	if read(t, filepath.Join(oldTarget, "client-marker.txt")) != "keep old files" {
		t.Fatal("old target files were deleted")
	}
}

func TestConcurrentLockRejectsModification(t *testing.T) {
	s := fixture(t)
	release, e := s.lock()
	if e != nil {
		t.Fatal(e)
	}
	defer release()
	if _, e := s.Sync(nil); e == nil {
		t.Fatal("concurrent synchronization accepted")
	}
}

func TestInterruptedFirstHistoryMoveRetainsRecoveryRecord(t *testing.T) {
	s := fixture(t)
	target := filepath.Join(t.TempDir(), "skills")
	put(t, filepath.Join(target, "auth", "SKILL.md"), "historical old")
	trustHistoryFixture(t, s, target)
	s.Rename = func(from, to string) error {
		e := os.Rename(from, to)
		if e == nil && strings.Contains(to, "backup") {
			panic("interrupted after backup")
		}
		return e
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("no interruption")
			}
		}()
		_, _ = s.Sync([]string{target})
	}()
	s.Rename = nil
	r, e := s.Sync([]string{target})
	if e != nil || len(r) != 1 || r[0].Status != "success" {
		t.Fatalf("first history recovery lost: %v %v", r, e)
	}
}

func TestInterruptedSyncVisibleOnSubsequentCommand(t *testing.T) {
	s := fixture(t)
	target := filepath.Join(t.TempDir(), "skills")
	if _, e := s.Install(target, s.Names, true, true); e != nil {
		t.Fatal(e)
	}
	s.Rename = func(from, to string) error {
		e := os.Rename(from, to)
		if e == nil && strings.Contains(to, "backup") {
			panic("interrupted")
		}
		return e
	}
	func() { defer func() { _ = recover() }(); _, _ = s.Sync(nil) }()
	if len(s.Pending()) != 1 {
		t.Fatal("interrupted sync not reported")
	}
}

func TestHistoricalSymlinkEntryIsNotAdopted(t *testing.T) {
	s := fixture(t)
	target := filepath.Join(t.TempDir(), "skills")
	source := t.TempDir()
	put(t, filepath.Join(source, "SKILL.md"), "shared old")
	if e := os.MkdirAll(target, 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(source, filepath.Join(target, "auth")); e != nil {
		t.Skipf("symlink privileges unavailable: %v", e)
	}
	r, e := s.Sync([]string{target})
	if e != nil || len(r) != 1 || r[0].Status != "unverified" {
		t.Fatal(r, e)
	}
	if read(t, filepath.Join(source, "SKILL.md")) != "shared old" {
		t.Fatal("linked source overwritten")
	}
	info, e := os.Lstat(filepath.Join(target, "auth"))
	if e != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("unverified link changed")
	}
}
