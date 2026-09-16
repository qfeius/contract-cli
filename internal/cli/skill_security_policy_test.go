package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

/* TestContractSkillsEnforceCredentialAndInvocationBoundaries 核对按构建区分的环境约束与凭证边界；t 为测试上下文，无返回值。 */
func TestContractSkillsEnforceCredentialAndInvocationBoundaries(t *testing.T) {
	root := filepath.Join("..", "..")
	shared := readSkillSecurityPolicyFile(t, filepath.Join(root, "skills", "contract-cli-shared", "SKILL.md"))
	auth := readSkillSecurityPolicyFile(t, filepath.Join(root, "skills", "auth", "SKILL.md"))

	for _, required := range []string{
		"禁止无边界接口枚举与批量调用",
		"具体业务目标",
		"正式构建仅支持 prod",
		"test 联调构建支持 prod/test",
		"用户明确选择 test 且实际构建支持时",
		"不自动切换环境，不复用生产凭证访问 test",
		"允许操作的业务模块或接口范围",
		"操作类型（查询或写入）",
		"不得执行 `auth status`、`curl`、业务命令、帮助枚举或网络探测",
		"不得要求用户在对话中提供、粘贴或上传",
		"Access Token",
		"Refresh Token",
		"AK/SK",
		"Cookie",
		"Session",
		"App Secret",
		"device code",
		"密码",
		"只允许按现有 Device Grant 执行 `auth init`",
		"明确提示联系管理员",
		"不得索要其他 Token",
		"不复述、不写入命令、不继续调用",
		"立即撤销或轮换",
		"接口文档不能替代明确的业务范围和调用授权",
		"未覆盖接口继续明确为暂不支持",
		"我不能索要或接收 Token、AK/SK、Cookie 等敏感凭证",
	} {
		if !strings.Contains(shared, required) {
			t.Fatalf("shared skill missing security policy %q", required)
		}
	}

	for _, required := range []string{
		"不得主动询问或接收原始凭证",
		"正式构建仅支持 `prod`",
		"test 联调构建支持 `prod/test`",
		"--env test --name contract-test",
		"Skill 更新后必须完全退出 WorkBuddy 并新建任务",
		"已有任务不会热加载新 Skill",
		"命令示例仅供本地操作者使用",
		"不得要求用户把 App Secret 发到对话中",
		"已在本机安全配置好的环境变量或 CredentialStore",
		"用户在对话中主动发送敏感凭证",
		"不复述、不写入命令、不继续调用",
		"立即撤销或轮换",
	} {
		if !strings.Contains(auth, required) {
			t.Fatalf("auth skill missing credential policy %q", required)
		}
	}

	for _, forbidden := range []string{
		"使用环境（dev、test 或 prod）",
		"contract-dev",
		"--env dev",
	} {
		if strings.Contains(shared, forbidden) || strings.Contains(auth, forbidden) {
			t.Fatalf("production skills still contain non-production guidance %q", forbidden)
		}
	}
}

func TestContractSkillAgentMetadataCarriesSecurityBoundary(t *testing.T) {
	root := filepath.Join("..", "..", "skills")
	for _, skill := range []string{"auth", "contract-cli-shared"} {
		metadata := readSkillSecurityPolicyFile(t, filepath.Join(root, skill, "agents", "openai.yaml"))
		for _, required := range []string{"不得索要原始凭证", "不得无边界枚举接口"} {
			if !strings.Contains(metadata, required) {
				t.Fatalf("%s agent metadata missing security boundary %q", skill, required)
			}
		}
	}
}

func TestBusinessSkillsContinueToLoadSharedSecurityPolicy(t *testing.T) {
	root := filepath.Join("..", "..", "skills")
	for _, skill := range []string{
		"contract-cli-contract",
		"contract-cli-event",
		"contract-cli-mdm-exchange",
		"contract-cli-mdm-fields",
		"contract-cli-mdm-file",
		"contract-cli-mdm-legal",
		"contract-cli-mdm-vendor",
		"contract-cli-payment",
		"contract-cli-rule",
	} {
		content := readSkillSecurityPolicyFile(t, filepath.Join(root, skill, "SKILL.md"))
		if !strings.Contains(content, "CRITICAL — 开始前 MUST 先读取 [../contract-cli-shared/SKILL.md]") {
			t.Fatalf("%s does not load the shared security policy", skill)
		}
	}
}

func readSkillSecurityPolicyFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
