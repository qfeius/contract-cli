package cli_test

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	contractskills "cn.qfei/contract-cli/skills"
)

// 本回归验证合并 1.9.5 后新增的两个真实官方 Skill，不使用新增技能夹具替代。
func TestSkillSyncUpgradeAddsBundledReviewSkills(t *testing.T) {
	entries, err := fs.ReadDir(contractskills.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "contract-cli-api-call" {
			count++
		}
	}
	if count != 17 {
		t.Fatalf("integrated distributable Skill count = %d, want 17", count)
	}
	previous := fstest.MapFS{}
	err = fs.WalkDir(contractskills.FS, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path == "contract-cli-review" || path == "contract-cli-review-rule" {
				return fs.SkipDir
			}
			return nil
		}
		content, readErr := fs.ReadFile(contractskills.FS, path)
		if readErr != nil {
			return readErr
		}
		previous[path] = &fstest.MapFile{Data: content}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	state, target, home := t.TempDir(), filepath.Join(t.TempDir(), "skills"), t.TempDir()
	makeApp := func(source fs.FS, output *bytes.Buffer) *cli.App {
		return cli.New(cli.Options{Store: config.NewStore(state), SkillsFS: source, Stdout: output, Stderr: &bytes.Buffer{},
			LookupEnv: func(string) (string, bool) { return "", false }, LocalUserHome: func() (string, error) { return home, nil },
		})
	}
	if err := makeApp(previous, &bytes.Buffer{}).Run(context.Background(), []string{"skills", "install", "--target", target, "--force"}); err != nil {
		t.Fatal(err)
	}
	thirdParty := filepath.Join(target, "third-party", "SKILL.md")
	writeSkillSyncFixture(t, thirdParty, "keep unrelated skill")
	var output bytes.Buffer
	if err := makeApp(contractskills.FS, &output).Run(context.Background(), []string{"__internal", "skill-sync-on-install"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"contract-cli-review", "contract-cli-review-rule"} {
		body, err := os.ReadFile(filepath.Join(target, name, "SKILL.md"))
		if err != nil || !strings.Contains(string(body), "name: "+name) {
			t.Fatalf("new official Skill %s missing or invalid: %v", name, err)
		}
	}
	registry := readSkillSyncRegistry(t, filepath.Join(state, "skill-locations.json"))
	if len(registry.Locations) != 1 || len(registry.Locations[0].Managed) != 17 || registry.Locations[0].Status != "success" {
		t.Fatalf("registry did not synchronize all 17 Skills: %+v; output=%s", registry, output.String())
	}
	content, err := os.ReadFile(thirdParty)
	if err != nil || string(content) != "keep unrelated skill" {
		t.Fatalf("third-party Skill changed: %v", err)
	}
}
