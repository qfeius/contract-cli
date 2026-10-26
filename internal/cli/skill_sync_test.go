package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/build"
	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/skillsync"
)

func TestCompleteInstallRegistersAndInternalSyncOverwrites(t *testing.T) {
	state := t.TempDir()
	target := filepath.Join(t.TempDir(), "skills")
	out := &bytes.Buffer{}
	a := cli.New(cli.Options{Store: config.NewStore(state), SkillsFS: testSkillsFS(), Stdout: out, Stderr: &bytes.Buffer{}, LookupEnv: func(string) (string, bool) { return "", false }, LocalUserHome: func() (string, error) { return t.TempDir(), nil }})
	if err := a.Run(context.Background(), []string{"skills", "install", "--target", target, "--force"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(state, "skill-locations.json")); err != nil {
		t.Fatalf("registration missing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "auth", "SKILL.md"), []byte("custom"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.Run(context.Background(), []string{"__internal", "skill-sync-on-install"}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(target, "auth", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "name: auth") {
		t.Fatalf("not overwritten: %s", content)
	}
}

func TestSelectedInstallDoesNotRegister(t *testing.T) {
	state := t.TempDir()
	a := cli.New(cli.Options{Store: config.NewStore(state), SkillsFS: testSkillsFS(), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err := a.Run(context.Background(), []string{"skills", "install", "--target", filepath.Join(t.TempDir(), "skills"), "--name", "auth"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(state, "skill-locations.json")); !os.IsNotExist(err) {
		t.Fatalf("selected install registered: %v", err)
	}
}

func TestInstallRejectsRemovedReplaceTargetWithoutChangingRegistration(t *testing.T) {
	state := t.TempDir()
	oldTarget := filepath.Join(t.TempDir(), "old-skills")
	newTarget := filepath.Join(t.TempDir(), "new-skills")
	out := &bytes.Buffer{}
	a := cli.New(cli.Options{Store: config.NewStore(state), SkillsFS: testSkillsFS(), Stdout: out, Stderr: &bytes.Buffer{}})
	if err := a.Run(context.Background(), []string{"skills", "install", "--target", oldTarget, "--force"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Run(context.Background(), []string{"skills", "install", "--target", newTarget, "--replace-target", oldTarget, "--force"}); err == nil {
		t.Fatal("removed migration flag must be rejected")
	}
	registry := readSkillSyncRegistry(t, filepath.Join(state, "skill-locations.json"))
	if len(registry.Locations) != 1 || registry.Locations[0].Target != oldTarget {
		t.Fatalf("registry after directory migration = %+v", registry.Locations)
	}
	if _, err := os.Stat(newTarget); !os.IsNotExist(err) {
		t.Fatalf("rejected flag created target: %v", err)
	}
}

func TestSkillsHelpDoesNotAdvertiseDirectoryMigration(t *testing.T) {
	out := &bytes.Buffer{}
	a := cli.New(cli.Options{Store: config.NewStore(t.TempDir()), Stdout: out, Stderr: &bytes.Buffer{}})
	if err := a.Run(context.Background(), []string{"skills", "--help"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "--replace-target") {
		t.Fatal("removed migration flag advertised")
	}
}

func TestSkillSyncRequiresConfirmationForModifiedHistoryAlongsideCodex(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("WorkBuddy historical path verified on Windows only")
	}
	for _, overrideCodex := range []bool{false, true} {
		t.Run(map[bool]string{false: "default_codex", true: "overridden_codex"}[overrideCodex], func(t *testing.T) {
			home, state := t.TempDir(), t.TempDir()
			codexHome := filepath.Join(home, ".codex")
			if overrideCodex {
				codexHome = filepath.Join(t.TempDir(), "codex")
			}
			codexTarget := filepath.Join(codexHome, "skills")
			buddyTarget := filepath.Join(home, ".workbuddy", "skills")
			for _, target := range []string{codexTarget, buddyTarget} {
				writeSkillSyncFixture(t, filepath.Join(target, "auth", "SKILL.md"), "modified historical official skill")
				writeSkillSyncFixture(t, filepath.Join(target, "auth", "local-extra.txt"), "remove")
				writeSkillSyncFixture(t, filepath.Join(target, "my-own-skill", "SKILL.md"), "third party must stay")
			}
			metadata := filepath.Join(buddyTarget, "_bm_skillid_migration.json")
			writeSkillSyncFixture(t, metadata, "client metadata must stay")
			a := cli.New(cli.Options{Store: config.NewStore(state), SkillsFS: testSkillsFS(), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
				LookupEnv:     func(key string) (string, bool) { return codexHome, overrideCodex && key == "CODEX_HOME" },
				LocalUserHome: func() (string, error) { return home, nil },
			})
			// No install command or registration before upgrading: this is historical migration.
			registryFile := filepath.Join(state, "skill-locations.json")
			if _, err := os.Stat(registryFile); !os.IsNotExist(err) {
				t.Fatalf("unexpected prior registration: %v", err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				if err := a.Run(context.Background(), []string{"__internal", "skill-sync-on-install"}); err != nil {
					t.Fatal(err)
				}
				if attempt == 0 {
					if _, err := os.Stat(registryFile); !os.IsNotExist(err) {
						t.Fatal("unverified history registered")
					}
					for _, target := range []string{codexTarget, buddyTarget} {
						if content, err := os.ReadFile(filepath.Join(target, "auth", "SKILL.md")); err != nil || string(content) != "modified historical official skill" {
							t.Fatal("unverified history changed")
						}
						if err := a.Run(context.Background(), []string{"skills", "install", "--target", target, "--force"}); err != nil {
							t.Fatal(err)
						}
					}
				}
				r := readSkillSyncRegistry(t, registryFile)
				if len(r.Locations) != 2 {
					t.Fatalf("want both clients once, got %+v", r.Locations)
				}
				for _, location := range r.Locations {
					if location.Target != codexTarget && location.Target != buddyTarget {
						t.Fatalf("unexpected target: %s", location.Target)
					}
					if location.Status != "success" || location.Historical || location.Version != build.Current().Version {
						t.Fatalf("migration incomplete: %+v", location)
					}
				}
			}
			for _, target := range []string{codexTarget, buddyTarget} {
				content, err := os.ReadFile(filepath.Join(target, "auth", "SKILL.md"))
				if err != nil || !strings.Contains(string(content), "name: auth") || !strings.Contains(string(content), "version: "+build.Current().Version) {
					t.Fatalf("historical skill not replaced at %s: %s, %v", target, content, err)
				}
				if _, err := os.Stat(filepath.Join(target, "auth", "local-extra.txt")); !os.IsNotExist(err) {
					t.Fatalf("old extra file retained: %v", err)
				}
				content, err = os.ReadFile(filepath.Join(target, "my-own-skill", "SKILL.md"))
				if err != nil || string(content) != "third party must stay" {
					t.Fatalf("third-party content changed: %s, %v", content, err)
				}
			}
			if content, err := os.ReadFile(metadata); err != nil || string(content) != "client metadata must stay" {
				t.Fatalf("client metadata changed: %s, %v", content, err)
			}
		})
	}
}

func TestSkillSyncDoesNotCreateOrAdoptUnrelatedWorkBuddyDirectory(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("WorkBuddy historical path verified on Windows only")
	}
	for _, unrelated := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "third_party_only"}[unrelated], func(t *testing.T) {
			home, state := t.TempDir(), t.TempDir()
			target := filepath.Join(home, ".workbuddy", "skills")
			if unrelated {
				writeSkillSyncFixture(t, filepath.Join(target, "my-own-skill", "SKILL.md"), "keep")
			}
			a := cli.New(cli.Options{Store: config.NewStore(state), SkillsFS: testSkillsFS(), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
				LookupEnv: func(string) (string, bool) { return "", false }, LocalUserHome: func() (string, error) { return home, nil },
			})
			if err := a.Run(context.Background(), []string{"__internal", "skill-sync-on-install"}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(state, "skill-locations.json")); !os.IsNotExist(err) {
				t.Fatalf("unrelated location adopted: %v", err)
			}
			if _, err := os.Stat(filepath.Join(target, "auth")); !os.IsNotExist(err) {
				t.Fatalf("unexpected official skill installation: %v", err)
			}
			if !unrelated {
				if _, err := os.Stat(target); !os.IsNotExist(err) {
					t.Fatalf("missing client root created: %v", err)
				}
			}
		})
	}
}

func TestSkillSyncHomeLookupFailureStillChecksCodexOverride(t *testing.T) {
	state, codexHome := t.TempDir(), t.TempDir()
	writeSkillSyncFixture(t, filepath.Join(codexHome, "skills", "auth", "SKILL.md"), "old")
	var stderr, out bytes.Buffer
	a := cli.New(cli.Options{Store: config.NewStore(state), SkillsFS: testSkillsFS(), Stdout: &out, Stderr: &stderr,
		LookupEnv:     func(key string) (string, bool) { return codexHome, key == "CODEX_HOME" },
		LocalUserHome: func() (string, error) { return "", errors.New("home lookup unavailable") },
	})
	if err := a.Run(context.Background(), []string{"__internal", "skill-sync-on-install"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(state, "skill-locations.json")); !os.IsNotExist(err) {
		t.Fatal("unverified Codex override registered")
	}
	requireSummary(t, out.String(), "历史 Skill 未自动接管", "需要确认历史位置", "--target", "--force")
	if !strings.Contains(stderr.String(), "historical WorkBuddy") {
		t.Fatalf("missing actionable history lookup warning: %s", stderr.String())
	}
}

func TestSkillSyncWorkBuddyHistoricalSampleCopy(t *testing.T) {
	sample := os.Getenv("CONTRACT_CLI_WORKBUDDY_HISTORY_SAMPLE")
	if runtime.GOOS != "windows" || sample == "" {
		t.Skip("requires a Windows historical Skill sample; source is read-only")
	}
	home, state := t.TempDir(), t.TempDir()
	target := filepath.Join(home, ".workbuddy", "skills")
	// Copy first; never supply the real sample as a synchronization target.
	if err := os.CopyFS(target, os.DirFS(sample)); err != nil {
		t.Fatal(err)
	}
	prior, err := os.ReadFile(filepath.Join(target, "auth", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(prior), "version:") {
		t.Fatal("sample has no versioned auth Skill")
	}
	writeSkillSyncFixture(t, filepath.Join(target, "auth", "SKILL.md"), "user edited historical Skill")
	writeSkillSyncFixture(t, filepath.Join(target, "auth", "local-extra.txt"), "remove")
	var out bytes.Buffer
	a := cli.New(cli.Options{Store: config.NewStore(state), Stdout: &out, Stderr: &bytes.Buffer{},
		LookupEnv: func(string) (string, bool) { return "", false }, LocalUserHome: func() (string, error) { return home, nil },
	})
	registryFile := filepath.Join(state, "skill-locations.json")
	if err := a.Run(context.Background(), []string{"__internal", "skill-sync-on-install"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(registryFile); !os.IsNotExist(err) {
		t.Fatal("modified historical sample registered without confirmation")
	}
	if err := a.Run(context.Background(), []string{"skills", "install", "--target", target, "--force"}); err != nil {
		t.Fatal(err)
	}
	r := readSkillSyncRegistry(t, registryFile)
	if len(r.Locations) != 1 || r.Locations[0].Target != target || r.Locations[0].Status != "success" {
		t.Fatalf("historical sample migration failed: %+v", r)
	}
	managed := map[string]bool{}
	for _, name := range r.Locations[0].Managed {
		managed[name] = true
		content, err := os.ReadFile(filepath.Join(target, name, "SKILL.md"))
		if err != nil || !strings.Contains(string(content), "version: "+build.Current().Version) {
			t.Fatalf("bundled Skill %s missing/wrong version: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "auth", "local-extra.txt")); !os.IsNotExist(err) {
		t.Fatalf("extra file retained: %v", err)
	}
	// All non-official sample files, including WorkBuddy root metadata, must survive byte-for-byte.
	err = filepath.WalkDir(sample, func(name string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(sample, name)
		if err != nil {
			return err
		}
		if managed[strings.Split(rel, string(filepath.Separator))[0]] {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		before, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		after, err := os.ReadFile(filepath.Join(target, rel))
		if err != nil {
			return err
		}
		if !bytes.Equal(before, after) {
			return errors.New("non-official sample file changed: " + rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Run(context.Background(), []string{"__internal", "skill-sync-on-install"}); err != nil {
		t.Fatal(err)
	}
	if len(readSkillSyncRegistry(t, registryFile).Locations) != 1 {
		t.Fatal("repeat sync added duplicate location")
	}
	// Deletion is a stop condition, not a reason to recreate a whole client directory.
	if err := os.Rename(target, filepath.Join(filepath.Dir(target), "skills-backup")); err != nil {
		t.Fatal(err)
	}
	if err := a.Run(context.Background(), []string{"__internal", "skill-sync-on-install"}); err == nil {
		t.Fatal("deleted root should report an invalid location")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("deleted root recreated: %v", err)
	}
	if r := readSkillSyncRegistry(t, registryFile); r.Locations[0].Status != "invalid" {
		t.Fatalf("want invalid: %+v", r)
	}
}

func writeSkillSyncFixture(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func readSkillSyncRegistry(t *testing.T, name string) skillsync.Registry {
	t.Helper()
	content, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var r skillsync.Registry
	if err := json.Unmarshal(content, &r); err != nil {
		t.Fatal(err)
	}
	return r
}
