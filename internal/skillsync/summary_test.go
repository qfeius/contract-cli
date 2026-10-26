package skillsync

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRegisteredLocationsReadOnlyAndFailure(t *testing.T) {
	s := fixture(t)
	locations, err := s.RegisteredLocations()
	if err != nil || len(locations) != 0 {
		t.Fatalf("empty snapshot: %v %v", locations, err)
	}
	if exists(filepath.Join(s.StateDir, "skill-locations.json")) {
		t.Fatal("snapshot created state")
	}
	target := filepath.Join(t.TempDir(), "skills")
	if _, err := s.Install(target, s.Names, true, true); err != nil {
		t.Fatal(err)
	}
	locations, err = s.RegisteredLocations()
	if err != nil || len(locations) != 1 || locations[0].Target != target {
		t.Fatalf("snapshot: %v %v", locations, err)
	}
	put(t, filepath.Join(s.StateDir, "skill-locations.json"), "broken")
	if _, err := s.RegisteredLocations(); err == nil {
		t.Fatal("broken snapshot reported empty")
	}
}

func TestTransactionFailureReportsActualRestoration(t *testing.T) {
	s := fixture(t)
	target := filepath.Join(t.TempDir(), "skills")
	if _, err := s.Install(target, s.Names, true, true); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(target, "auth", "SKILL.md"), "original modified content")
	injected := errors.New("replacement blocked")
	s.Rename = func(from, to string) error {
		if from == filepath.Join(transactionDir(target), "stage", "auth") {
			return injected
		}
		return os.Rename(from, to)
	}
	_, err := s.Install(target, s.Names, true, true)
	if !errors.Is(err, injected) || RecoveryStatus(err) != "restored" {
		t.Fatalf("restoration not reported: %v", err)
	}
	if read(t, filepath.Join(target, "auth", "SKILL.md")) != "original modified content" {
		t.Fatal("not restored")
	}
	results, err := s.Sync(nil)
	if err != nil || len(results) != 1 || results[0].Recovery != "restored" {
		t.Fatalf("sync recovery missing: %v %v", results, err)
	}
}

func TestRecoveryStatusNeverClaimsUnknownErrorRestored(t *testing.T) {
	if RecoveryStatus(errors.New("permission denied before transaction")) != "unconfirmed" {
		t.Fatal("unknown recovery claimed restored")
	}
	err := &TransactionFailure{Cause: errors.New("write failed"), RollbackErr: errors.New("restore failed")}
	if RecoveryStatus(err) != "failed" || !errors.Is(err, err.RollbackErr) {
		t.Fatal("failed rollback hidden")
	}
}

func TestInstallResultReportsCanonicalTarget(t *testing.T) {
	s := fixture(t)
	target := filepath.Join(t.TempDir(), "skills")
	result, err := s.Install(target+string(filepath.Separator)+".", s.Names, true, true)
	if err != nil || result.Target != target {
		t.Fatalf("actual target missing: %+v %v", result, err)
	}
}

func TestActualRollbackFailureRetainsRecoveryMaterials(t *testing.T) {
	s := fixture(t)
	target := filepath.Join(t.TempDir(), "skills")
	if _, err := s.Install(target, s.Names, true, true); err != nil {
		t.Fatal(err)
	}
	s.Rename = func(from, to string) error {
		if from == filepath.Join(transactionDir(target), "stage", "auth") {
			if err := os.Rename(target, target+"-moved"); err != nil {
				return err
			}
			if err := os.WriteFile(target, []byte("block restoration in isolated fixture"), 0600); err != nil {
				return err
			}
			return errors.New("injected install interruption")
		}
		return os.Rename(from, to)
	}
	_, err := s.Install(target, s.Names, true, true)
	if err == nil || RecoveryStatus(err) != "failed" {
		t.Fatalf("actual rollback failure not identified: %v", err)
	}
	if !exists(filepath.Join(transactionDir(target), "backup", "auth", "SKILL.md")) {
		t.Fatal("recovery material removed after failed rollback")
	}
}
