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

// 本回归验证真实官方 Skill 的增量安装，包含审查、协商群及审批业务能力。
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
	if count != 28 {
		t.Fatalf("integrated distributable Skill count = %d, want 28", count)
	}
	previous := fstest.MapFS{}
	err = fs.WalkDir(contractskills.FS, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path == "contract-cli-review" || path == "contract-cli-review-rule" || path == "contract-cooperation-chat-group" ||
				path == "contract-approval-efficiency" || path == "contract-cli-approval-actions" || path == "contract-expiry-query" ||
				path == "contract-cooperation-chat" || path == "contract-sharev1-2" || path == "contract-review-records" ||
				path == "contract-submit" || path == "contract-process-person-replace" {
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
	for _, name := range []string{"contract-cli-review", "contract-cli-review-rule", "contract-cooperation-chat-group", "contract-approval-efficiency", "contract-cli-approval-actions", "contract-expiry-query", "contract-cooperation-chat", "contract-sharev1-2", "contract-review-records", "contract-submit", "contract-process-person-replace"} {
		body, err := os.ReadFile(filepath.Join(target, name, "SKILL.md"))
		if err != nil || !strings.Contains(string(body), "name: "+name) {
			t.Fatalf("new official Skill %s missing or invalid: %v", name, err)
		}
	}
	for _, path := range []string{
		"contract-cooperation-chat/references/contract-cli.md",
		"contract-sharev1-2/references/api.md",
		"contract-sharev1-2/references/business-rules.md",
		"contract-review-records/references/query.md",
		"contract-submit/references/cooperation-preform.md",
		"contract-submit/references/cooperation-submit.md",
		"contract-submit/references/discovery.md",
		"contract-submit/references/field-save.md",
		"contract-submit/references/form.md",
		"contract-submit/references/ordinary-file-preparation.md",
		"contract-submit/references/runtime.md",
		"contract-submit/references/submit.md",
		"contract-process-person-replace/references/api.md",
		"contract-process-person-replace/references/interactions.md",
		"contract-process-person-replace/references/node-rules.md",
		"contract-process-person-replace/references/runtime.md",
	} {
		want, readErr := fs.ReadFile(contractskills.FS, path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		got, readErr := os.ReadFile(filepath.Join(target, filepath.FromSlash(path)))
		if readErr != nil || !bytes.Equal(got, want) {
			t.Fatalf("installed Skill reference %s differs from bundle: %v", path, readErr)
		}
	}
	registry := readSkillSyncRegistry(t, filepath.Join(state, "skill-locations.json"))
	if len(registry.Locations) != 1 || len(registry.Locations[0].Managed) != count || registry.Locations[0].Status != "success" {
		t.Fatalf("registry did not synchronize all %d Skills: %+v; output=%s", count, registry, output.String())
	}
	content, err := os.ReadFile(thirdParty)
	if err != nil || string(content) != "keep unrelated skill" {
		t.Fatalf("third-party Skill changed: %v", err)
	}
}
