package contractskills

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func TestReviewSkillExcludesTemporaryCommandPreview(t *testing.T) {
	body, err := FS.ReadFile("contract-cli-review/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"临时测试：执行前命令预览", "test-command-preview.md", "先向用户打印本次命令、对应接口及参数"} {
		if strings.Contains(string(body), marker) {
			t.Errorf("distributed review Skill still enables temporary preview: %s", marker)
		}
	}
	if _, err := FS.ReadFile("contract-cli-review/references/test-command-preview.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("temporary preview reference must not be distributed, got %v", err)
	}
}
