package cli_test

import (
	"io/fs"
	"strings"
	"testing"

	contractskills "cn.qfei/contract-cli/skills"
)

func TestApprovalSkillsReleaseEnvironmentGuidance(t *testing.T) {
	for _, name := range []string{"contract-approval-efficiency", "contract-cli-approval-actions"} {
		t.Run(name, func(t *testing.T) {
			body, err := fs.ReadFile(contractskills.FS, name+"/SKILL.md")
			if err != nil {
				t.Fatal(err)
			}
			text := string(body)
			if !strings.Contains(text, "仅支持生产") || !strings.Contains(text, "`contract` profile") {
				t.Error("release Skill must select the production profile explicitly")
			}
			for _, forbidden := range []string{"contract-test", "contract-blue", "--env test", "--env blue"} {
				if strings.Contains(text, forbidden) {
					t.Errorf("release Skill still recommends a non-production configuration: %s", forbidden)
				}
			}
		})
	}
}
