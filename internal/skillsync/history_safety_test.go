package skillsync

import (
	"os"
	"path/filepath"
	"testing"
)

func trustHistoryFixture(t *testing.T, s *Service, target string) {
	t.Helper()
	s.historyFingerprints = map[string][]string{}
	for _, name := range s.Names {
		digest, err := treeFingerprint(os.DirFS(filepath.Join(target, name)))
		if err != nil {
			t.Fatal(err)
		}
		s.historyFingerprints[name] = []string{digest}
	}
}

func TestHistoryChecksEveryOccupiedOfficialName(t *testing.T) {
	for _, change := range []string{"same-name-third-party", "extra-file", "changed-reference"} {
		t.Run(change, func(t *testing.T) {
			s := fixture(t)
			s.Names = []string{"auth", "contract-cli-contract"}
			target := filepath.Join(t.TempDir(), "skills")
			put(t, filepath.Join(target, "auth", "SKILL.md"), "official auth")
			put(t, filepath.Join(target, "contract-cli-contract", "SKILL.md"), "official contract")
			put(t, filepath.Join(target, "contract-cli-contract", "references", "help.md"), "official reference")
			trustHistoryFixture(t, s, target)
			switch change {
			case "same-name-third-party":
				put(t, filepath.Join(target, "auth", "SKILL.md"), "third-party auth")
			case "extra-file":
				put(t, filepath.Join(target, "auth", "extra.txt"), "user file")
			case "changed-reference":
				put(t, filepath.Join(target, "contract-cli-contract", "references", "help.md"), "user reference")
			}
			r, e := s.Sync([]string{target})
			if e != nil || len(r) != 1 || r[0].Status != "unverified" {
				t.Fatalf("conflict adopted: %+v %v", r, e)
			}
			if read(t, filepath.Join(target, "contract-cli-contract", "SKILL.md")) != "official contract" {
				t.Fatal("partial overwrite")
			}
		})
	}
}

func TestUnverifiedHistoricalCandidateIsNotRegisteredOrOverwritten(t *testing.T) {
	s := fixture(t)
	target := filepath.Join(t.TempDir(), "skills")
	file := filepath.Join(target, "auth", "SKILL.md")
	put(t, file, "third-party auth")
	results, err := s.Sync([]string{target})
	if err != nil || len(results) != 1 || results[0].Status != "unverified" {
		t.Fatalf("results=%+v err=%v", results, err)
	}
	if read(t, file) != "third-party auth" {
		t.Fatal("unverified contents overwritten")
	}
	if _, err := os.Stat(filepath.Join(s.StateDir, "skill-locations.json")); !os.IsNotExist(err) {
		t.Fatal("unverified target registered")
	}
	if _, err := s.Install(target, s.Names, true, true); err != nil {
		t.Fatal(err)
	}
	put(t, file, "user modified confirmed skill")
	results, err = s.Sync(nil)
	if err != nil || len(results) != 1 || results[0].Status != "success" {
		t.Fatalf("explicit install did not enable sync: %+v %v", results, err)
	}
}

func TestLegacyHistoricalFlagAloneDoesNotAuthorizeOverwrite(t *testing.T) {
	s := fixture(t)
	target := filepath.Join(t.TempDir(), "skills")
	put(t, filepath.Join(target, "auth", "SKILL.md"), "unverified old migration")
	if err := s.save(Registry{Schema: 1, Locations: []Location{{Target: target, ID: "legacy", Managed: s.Names, Historical: true}}}); err != nil {
		t.Fatal(err)
	}
	r, e := s.Sync(nil)
	if e != nil || len(r) != 1 || r[0].Status != "unverified" {
		t.Fatalf("legacy bypass: %+v %v", r, e)
	}
	if read(t, filepath.Join(target, "auth", "SKILL.md")) != "unverified old migration" {
		t.Fatal("legacy overwritten")
	}
}
