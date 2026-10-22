package cli_test

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestMatrixHelpDoesNotClaimAPICallIsDisabled(t *testing.T) {
	t.Parallel()
	for _, command := range []string{
		"rule group get", "rule table get", "rule table create", "rule table update", "rule table delete",
		"rule table column add", "rule table column update", "rule table column update-condition",
		"rule table column update-result", "rule table column delete",
		"approval-matrix group get", "approval-matrix table get", "approval-matrix table create",
		"approval-matrix table update", "approval-matrix table delete", "approval-matrix column add",
		"approval-matrix column update", "approval-matrix column update-condition",
		"approval-matrix column update-result", "approval-matrix column delete",
	} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			app := cli.New(cli.Options{Stdout: &output, Stderr: &bytes.Buffer{}, Store: config.NewStore(t.TempDir())})
			if err := app.Run(context.Background(), append([]string{"help"}, strings.Fields(command)...)); err != nil {
				t.Fatalf("help %s: %v", command, err)
			}
			if strings.Contains(output.String(), "api call 保持关闭") {
				t.Fatalf("help %s incorrectly claims api call is disabled", command)
			}
			assertContainsAll(t, command, output.String(), []string{"别名映射到对应 rule 命令", "矩阵操作使用上述结构化命令"})
		})
	}
}

func TestAPICallGuidanceDescribesAvailableWebTransport(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"skills/contract-cli-shared/SKILL.md",
		"skills/contract-cli-contract-search/SKILL.md",
		"skills/contract-cli-rule/SKILL.md",
		"skills/contract-cli-mdm-fields/references/commands.md",
		"skills/contract-cli-mdm-fields/references/schema-fields-guide.md",
		"skills/contract-cli-mdm-fields/references/schema-biz-lines.md",
		"docs/cli-command-design.md",
		"docs/cli-test-plan.md",
		"docs/app-command-development-guide.md",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			content := readTextFile(t, filepath.Join("..", "..", filepath.FromSlash(path)))
			assertContainsAll(t, path, content, []string{"api call --channel web"})
			assertNoDisabledAPICallGuidance(t, content)
		})
	}
}

func assertNoDisabledAPICallGuidance(t *testing.T, content string) {
	t.Helper()
	for _, stale := range []string{
		"`api call` 当前不对外开放", "`api call` 仍未开放", "`api call` 当前暂未开放",
		"`contract-cli api call` 当前暂未开放", "api call 暂未开放使用", "api call 保持关闭",
		"项目 api call 关闭约定", "预留能力，当前暂未开放使用", "当前不对外开放；公开命令",
		"api call 暂未开放拦截", "`api call` 暂未开放拦截", "help 隐藏",
		"不要退回 `api call`", "结构化命令和 `api call` 都支持",
	} {
		if strings.Contains(content, stale) {
			t.Errorf("guidance still contains disabled API call description %q", stale)
		}
	}
}

func TestSharedAPICallGuidanceKeepsVerifiedRecipeBoundary(t *testing.T) {
	t.Parallel()
	content := readTextFile(t, filepath.Join("..", "..", "skills", "contract-cli-shared", "SKILL.md"))
	assertContainsAll(t, "shared API call guidance", content, []string{
		"api call --channel web", "/open-web/web/**", "仅支持 user", "已验证的接口配方",
		"不发布独立 `SKILL.md`", "Authorization", "Cookie", "可信身份头",
		"不接受 `/open-apis/**`", "未覆盖接口继续明确为暂不支持",
	})
}
