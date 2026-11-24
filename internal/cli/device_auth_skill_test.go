package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeviceAuthSkillsEnforceWorkBuddyTurnBoundaries(t *testing.T) {
	root := filepath.Join("..", "..")
	auth := readDeviceAuthSkillFile(t, filepath.Join(root, "skills", "auth", "SKILL.md"))
	shared := readDeviceAuthSkillFile(t, filepath.Join(root, "skills", "contract-cli-shared", "SKILL.md"))
	readme := readDeviceAuthSkillFile(t, filepath.Join(root, "README.md"))

	for _, required := range []string{
		"present_files(files: [\"<qr_code_path>\"])",
		"WorkBuddy 主路径只调用一次 `present_files`",
		"禁止读取、复制或重新编码 `qr_code_data_uri`",
		"二维码图片附件/产物卡片",
		"`expires_at_display`",
		"继续查询前，需要先完成智书合同授权。",
		"1. 点击蓝色链接「[打开授权页面](<verification_uri_complete>)」，或扫描本消息中的二维码图片附件",
		"2. 在 **<expires_at_display>** 前完成：手机号验证 + 企业确认授权",
		"3. 授权全部完成后，回复消息：**已授权**，我将立刻为你执行合同查询",
		"不得向用户展示 `expires_at` 的 RFC3339 原值",
		"不得使用反引号或代码样式展示时间",
		"只能调用一次授权展示工具",
		"禁止执行 `auth complete`、再次执行 `auth init`、业务命令、轮询或网络重试",
		"收到新的用户消息",
		"auth init --profile <profile> --output json --restart",
		"最终回复必须同时包含",
		"[打开授权页面](<verification_uri_complete>)",
		"WorkBuddy 主路径",
		"豆包普通工作任务只展示可点击的完整 HTTPS 授权链接和 `expires_at_display`",
		"禁止读取、复制、修改或交付 `qr_code_path`",
		"禁止调用代码执行、图片处理或图片交付工具处理二维码",
		"返回授权链接和过期时间后立即结束当前轮次",
		"不得只返回授权链接或只返回二维码",
		"二维码附件展示失败，请直接使用授权链接",
		"附件展示失败后禁止重试 `present_files`",
		"不得声称二维码已经展示",
		"Refresh Token 返回 `invalid_grant` 时也必须先询问用户",
		"先执行 `auth status --profile <profile> --as user`",
		"根据真实状态选择复用现有会话、普通 `auth init` 或带 `--restart` 的 `auth init`",
		"auth status` 可使用 `--output json`",
		"豆包 AgentKit / Skills Sandbox 运行在云端 Skill 环境",
		"豆包普通工作任务在隔离范围内使用 `SESSION_ID`",
		"必须从任务初始工作目录执行",
		"不能抵御同一沙箱内具有文件和进程访问能力的 Shell",
		"macOS Keychain",
		"必须提供 `SKILL_SESSION_WORKSPACE`",
		"CLI 确认当前进程来自本地桌面客户端",
		"一次授权同时包含合同与智审平台访问范围",
	} {
		if !strings.Contains(auth, required) {
			t.Fatalf("auth skill missing required WorkBuddy rule %q", required)
		}
	}
	if strings.Contains(auth, "当前 user 身份未授权") {
		t.Fatal("auth skill still contains the old technical authorization copy")
	}
	for _, forbidden := range []string{
		"`show_widget`",
		"show_widget(",
		"read_me(modules: \"diagram\")",
		"data:image/png;base64",
		"width:220px",
		"height:220px",
	} {
		if strings.Contains(auth, forbidden) || strings.Contains(shared, forbidden) {
			t.Fatalf("WorkBuddy skill still relies on model-copied inline QR data %q", forbidden)
		}
	}
	for _, forbidden := range []string{"DOUBAO_SESSION_ID", "DOUBAO_TASK_ID", "豆包本地 Skill 必须提供", "豆包 Skill 只暴露固定子命令和结构化参数", "普通 Shell 无法读取"} {
		if strings.Contains(auth, forbidden) {
			t.Fatalf("auth skill still documents removed Doubao local runtime contract %q", forbidden)
		}
	}
	for _, ambiguous := range []string{"扫描上方二维码", "扫描下方二维码"} {
		if strings.Contains(auth, ambiguous) {
			t.Fatalf("auth skill still uses layout-dependent QR copy %q", ambiguous)
		}
	}
	for _, removed := range []string{
		"豆包普通工作任务必须将 `qr_code_path` 对应的 PNG 作为图片附件或图片产物交付",
		"实际出现图片缩略图或图片产物卡片",
		"未实际出现图片时不得声称二维码已经展示",
	} {
		if strings.Contains(auth, removed) {
			t.Fatalf("auth skill still requires unsupported Doubao work-task QR delivery %q", removed)
		}
	}
	for _, required := range []string{
		"WorkBuddy 使用 `present_files` 交付 `qr_code_path` 对应的原始 PNG 附件",
		"AgentKit 继续使用 `qr_code_path`",
		"豆包普通工作任务只展示可点击授权链接和过期时间",
		"不处理 `qr_code_path` 或 `qr_code_data_uri`",
		"Skill / 模型层不得重试任何 OAuth 命令",
		"CLI 内部仅对 `auth init` 的 TCP `dial` 失败自动重试一次",
		"请求已发送后的超时、HTTP 5xx、响应中断或解析失败不重试",
		"`auth complete`、Token 刷新和撤销始终不自动重试",
	} {
		if !strings.Contains(shared, required) {
			t.Fatalf("shared skill missing required OAuth retry boundary %q", required)
		}
	}
	for _, removed := range []string{
		"豆包普通工作任务必须把 `qr_code_path` 作为图片附件或图片产物交付",
		"未实际出现图片时不得声称已经展示二维码",
	} {
		if strings.Contains(shared, removed) {
			t.Fatalf("shared skill still requires unsupported Doubao work-task QR delivery %q", removed)
		}
	}
	for _, required := range []string{
		"豆包普通工作任务只展示 `verification_uri_complete` 和 `expires_at_display`，不展示二维码",
		"WorkBuddy 使用 `qr_code_path` 交付原始 PNG 附件，AgentKit 使用 `qr_code_path`",
	} {
		if !strings.Contains(readme, required) {
			t.Fatalf("README missing Device Grant presentation contract %q", required)
		}
	}
	for _, required := range []string{
		"CLI 内部的单次安全重试不算第二次 `auth init` 命令",
		"禁止额外执行 `curl`、`auth status` 或其他探测命令",
		"明确询问用户是否重新发起授权",
	} {
		if !strings.Contains(auth, required) {
			t.Fatalf("auth skill missing required init failure boundary %q", required)
		}
	}
}

func TestDeviceAuthSkillsDistinguishReusableCredentialsFromPendingAuthorization(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, path := range []string{
		filepath.Join(root, "skills", "auth", "SKILL.md"),
		filepath.Join(root, "skills", "contract-cli-shared", "SKILL.md"),
		filepath.Join(root, "skills", "contract-cli-contract", "SKILL.md"),
		filepath.Join(root, "README.md"),
	} {
		content := readDeviceAuthSkillFile(t, path)
		for _, required := range []string{
			"auth status --profile contract --as user",
			"status=authorized",
			"status=pending",
		} {
			if !strings.Contains(content, required) {
				t.Errorf("%s missing reusable authorization boundary %q", path, required)
			}
		}
	}
	auth := readDeviceAuthSkillFile(t, filepath.Join(root, "skills", "auth", "SKILL.md"))
	for _, required := range []string{
		"当前系统用户 + profile",
		"跨任务复用",
		"不能仅凭 `SESSION_ID` 或操作系统类型判断为本地",
		"云端、沙箱或识别不明确",
		"不自动迁移旧任务凭证",
		"同一 profile 的所有本地任务",
		"不得展示不存在的授权链接或二维码",
	} {
		if !strings.Contains(auth, required) {
			t.Errorf("auth skill missing credential scope boundary %q", required)
		}
	}
	for _, forbidden := range []string{
		"只在同一任务内复用，新建任务必须重新授权",
		"WorkBuddy 运行在客户本机，必须提供 `CODEBUDDY_SESSION_ID`",
	} {
		if strings.Contains(auth, forbidden) {
			t.Errorf("auth skill still requires task isolation for all desktop sessions: %q", forbidden)
		}
	}
}

func readDeviceAuthSkillFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
