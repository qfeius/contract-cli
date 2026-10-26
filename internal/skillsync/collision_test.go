package skillsync

import (
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSyncNewNameCollisionPreservesWholeLocationAndContinues(t *testing.T) {
	s := fixture(t)
	a, b := filepath.Join(t.TempDir(), "a"), filepath.Join(t.TempDir(), "b")
	for _, target := range []string{a, b} {
		if _, err := s.Install(target, s.Names, true, true); err != nil {
			t.Fatal(err)
		}
	}
	put(t, filepath.Join(a, "auth", "SKILL.md"), "user modified official")
	put(t, filepath.Join(a, "contract-cli-new", "SKILL.md"), "third party")
	s.Source.(fstest.MapFS)["contract-cli-new/SKILL.md"] = &fstest.MapFile{Data: []byte("---\nname: contract-cli-new\n---\nnew official")}
	s.Names = append(s.Names, "contract-cli-new")
	s.Version = "3.0.0"
	results, err := s.Sync(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Status != "failed" || results[1].Status != "success" {
		t.Fatalf("results=%+v", results)
	}
	if !strings.Contains(results[0].Error, "contract-cli-new") {
		t.Fatal("missing conflict name")
	}
	if read(t, filepath.Join(a, "auth", "SKILL.md")) != "user modified official" || read(t, filepath.Join(a, "contract-cli-new", "SKILL.md")) != "third party" {
		t.Fatal("conflicting location changed")
	}
	r, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	if r.Locations[0].Version != "2.0.0" || len(r.Locations[0].Managed) != 1 {
		t.Fatal("conflict registered as upgraded")
	}
	if !strings.Contains(read(t, filepath.Join(b, "contract-cli-new", "SKILL.md")), "3.0.0") {
		t.Fatal("other location did not upgrade")
	}
	// Explicit full force installation remains the user's confirmed override.
	if _, err := s.Install(a, s.Names, true, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, filepath.Join(a, "contract-cli-new", "SKILL.md")), "3.0.0") {
		t.Fatal("explicit install did not resolve conflict")
	}
}

func TestSyncNewNameVerifiedOfficialMayBeAdopted(t *testing.T) {
	s := fixture(t)
	target := filepath.Join(t.TempDir(), "skills")
	if _, err := s.Install(target, s.Names, true, true); err != nil {
		t.Fatal(err)
	}
	old := fstest.MapFS{"SKILL.md": &fstest.MapFile{Data: []byte("verified old official")}}
	digest, err := treeFingerprint(old)
	if err != nil {
		t.Fatal(err)
	}
	s.historyFingerprints = map[string][]string{"contract-cli-new": {digest}}
	put(t, filepath.Join(target, "contract-cli-new", "SKILL.md"), "verified old official")
	s.Source.(fstest.MapFS)["contract-cli-new/SKILL.md"] = &fstest.MapFile{Data: []byte("new official")}
	s.Names = append(s.Names, "contract-cli-new")
	results, err := s.Sync(nil)
	if err != nil || len(results) != 1 || results[0].Status != "success" {
		t.Fatalf("%+v %v", results, err)
	}
}
