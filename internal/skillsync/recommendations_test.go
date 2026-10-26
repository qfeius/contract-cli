package skillsync

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestClientSkillHintsUseConfiguredCodexAndVerifiedDefaults(t *testing.T) {
	home, codex, local := t.TempDir(), t.TempDir(), t.TempDir()
	hints := ClientSkillHints("windows", home, codex, local)
	if len(hints) != 2 {
		t.Fatalf("want only Codex and WorkBuddy hints; do not guess Doubao paths: %+v", hints)
	}
	if hints[0].Target != filepath.Join(codex, "skills") || !hints[0].Configured {
		t.Fatalf("CODEX_HOME not honored: %+v", hints[0])
	}
	if hints[1].Target != filepath.Join(home, ".workbuddy", "skills") || hints[1].Configured {
		t.Fatalf("WorkBuddy default falsely claimed as configuration: %+v", hints[1])
	}
	for _, platform := range []string{"linux", "darwin"} {
		hints = ClientSkillHints(platform, home, codex, local)
		if len(hints) != 1 || hints[0].Client != "Codex" {
			t.Fatalf("unverified defaults leaked into %s: %+v", platform, hints)
		}
	}
}

func TestClientSkillHintsRejectRelativeOrUnknownRoots(t *testing.T) {
	if hints := ClientSkillHints("windows", "relative-home", "relative-codex", "relative-local"); len(hints) != 0 {
		t.Fatalf("relative roots yielded recommendations: %+v", hints)
	}
	hints := ClientSkillHints("linux", "", t.TempDir(), "")
	if len(hints) != 1 || !hints[0].Configured {
		t.Fatalf("explicit Codex override must work without home: %+v", hints)
	}
}

func TestRecommendationsAreReadOnlyAndExcludeAlreadyProcessedTargets(t *testing.T) {
	home, state := t.TempDir(), filepath.Join(t.TempDir(), "not-created-state")
	root := filepath.Join(home, ".codex")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	s := &Service{StateDir: state}
	hints := ClientSkillHints("linux", home, "", "")
	got := s.Recommendations(hints, nil)
	if len(got) != 1 || got[0].Target != filepath.Join(root, "skills") {
		t.Fatalf("existing client root not recommended: %+v", got)
	}
	for _, name := range []string{state, got[0].Target} {
		if _, err := os.Stat(name); !os.IsNotExist(err) {
			t.Fatalf("recommendation wrote %s: %v", name, err)
		}
	}
	for _, status := range []string{"success", "invalid", "failed"} {
		if got := s.Recommendations(hints, []Result{{Target: filepath.Join(root, "skills"), Status: status}}); len(got) != 0 {
			t.Fatalf("processed location offered as first install: %+v", got)
		}
	}
	if hints[0].Target != filepath.Join(root, "skills") {
		t.Fatal("input hint mutated")
	}
}

func TestRecommendationsDoNotInferAbsentDefaultClientsOrUnsafeTargets(t *testing.T) {
	home, state := t.TempDir(), t.TempDir()
	s := &Service{StateDir: state}
	if got := s.Recommendations(ClientSkillHints("linux", home, "", ""), nil); len(got) != 0 {
		t.Fatalf("absent client inferred: %+v", got)
	}
	unsafe := []SkillHint{{Client: "Codex", Root: home, Target: state, Configured: true}, {Client: "Codex", Root: home, Target: "relative", Configured: true}}
	if got := s.Recommendations(unsafe, nil); len(got) != 0 {
		t.Fatalf("unsafe target recommended: %+v", got)
	}
	// An explicit absolute configuration may propose a missing client root, but never creates it.
	configured := filepath.Join(home, "custom-codex")
	if got := s.Recommendations(ClientSkillHints("linux", home, configured, ""), nil); len(got) != 1 {
		t.Fatalf("configured first-install location omitted: %+v", got)
	}
}

func TestRecommendationsDeduplicateCanonicalTargets(t *testing.T) {
	home, state := t.TempDir(), t.TempDir()
	root := filepath.Join(home, ".codex")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	hint := ClientSkillHints("linux", home, "", "")[0]
	s := &Service{StateDir: state}
	if got := s.Recommendations([]SkillHint{hint, hint}, nil); len(got) != 1 {
		t.Fatalf("duplicate recommendations: %+v", got)
	}
}

func TestInstallRejectsDoubaoSystemSkillsBeforeAnyWrite(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows Doubao system directory verified on Windows only")
	}
	home := t.TempDir()
	target := filepath.Join(home, "Doubao", "User Data", "Default", ".doubao", "agent_mode", "workspace", ".skills")
	state := filepath.Join(home, "config")
	s := &Service{StateDir: state, Source: fixture(t).Source, Version: "test"}
	_, err := s.Install(target, []string{"auth"}, true, true)
	if err == nil || !strings.Contains(err.Error(), "client-managed system") {
		t.Fatalf("system target not rejected: %v", err)
	}
	if strings.Contains(err.Error(), ".user_skills") {
		t.Fatalf("error recommends an unverified fixed directory: %v", err)
	}
	for _, name := range []string{state, target} {
		if _, err := os.Stat(name); !os.IsNotExist(err) {
			t.Fatalf("unsafe install wrote %s: %v", name, err)
		}
	}
}

func TestSyncRejectsOldDoubaoSystemRegistrationWithoutRestoringSkills(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows Doubao system directory verified on Windows only")
	}
	home, state := t.TempDir(), t.TempDir()
	target := filepath.Join(home, ".doubao", "agent_mode", "workspace", ".skills")
	userTarget := filepath.Join(home, ".doubao", "agent_mode", "workspace", ".user_skills")
	put(t, filepath.Join(target, "preinstalled", "SKILL.md"), "system content")
	s := &Service{StateDir: state, Source: fixture(t).Source, Version: "old", Names: []string{"auth"}}
	if _, err := s.Install(userTarget, s.Names, true, true); err != nil {
		t.Fatal(err)
	}
	r, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	r.Locations = append(r.Locations, Location{Target: target, ID: "old-system-record", Version: "old", Managed: s.Names, Status: "success"})
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	s.Version = "new"
	results, err := s.Sync(nil)
	if err != nil || len(results) != 2 || results[0].Status != "success" || results[1].Status != "failed" || !strings.Contains(results[1].Error, "client-managed system") {
		t.Fatalf("wrong per-target results: %+v, %v", results, err)
	}
	if _, err := os.Stat(filepath.Join(target, "auth")); !os.IsNotExist(err) {
		t.Fatalf("system Skills restored: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(target, "preinstalled", "SKILL.md"))
	if err != nil || !bytes.Equal(content, []byte("system content")) {
		t.Fatalf("system content changed: %s, %v", content, err)
	}
}
