package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillsInstallHelpExplainsExplicitTargetAndCodexDefault(t *testing.T) {
	for _, helpFlag := range []string{"--help", "-h"} {
		t.Run(helpFlag, func(t *testing.T) {
			a, out, stderr, state := summaryApp(t)
			err := a.Run(context.Background(), []string{"skills", "install", helpFlag})
			if err != nil {
				t.Errorf("help must succeed without installation: %v", err)
			}
			output := out.String() + stderr.String()
			requireSummary(t, output, "当前客户端实际读取的用户 Skill 目录", "省略 --target 时默认使用 Codex", "CODEX_HOME/skills", "~/.codex/skills", "不会自动识别当前客户端")
			for _, unwanted := range []string{"Skill 安装：失败", "Skill 安装：成功", "已登记的 Skill 目录：", "Skill registration snapshot started", "Skill installation started"} {
				if strings.Contains(output, unwanted) {
					t.Errorf("help performed/reported installation: %q in %s", unwanted, output)
				}
			}
			if _, err := os.Stat(filepath.Join(state, "skill-locations.json")); !os.IsNotExist(err) {
				t.Errorf("help created registry: %v", err)
			}
		})
	}
}

func TestSkillsHelpRetainsFirstInstallConfirmationAndMatchingSource(t *testing.T) {
	a, out, _, _ := summaryApp(t)
	if err := a.Run(context.Background(), []string{"skills", "--help"}); err != nil {
		t.Fatal(err)
	}
	requireSummary(t, out.String(), "当前 CLI 内置内容", "未找到可自动同步的位置", "提示用户确认真实目录", "经过核实的推荐目录", "确认前不自动安装所有候选", "历史通用安装入口仍可使用")
	if strings.Contains(out.String(), "推荐跨平台安装方式：npx skills add") {
		t.Fatal("default-branch source still recommended as matching bundled version")
	}
}
