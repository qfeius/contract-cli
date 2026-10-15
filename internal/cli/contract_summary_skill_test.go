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

func TestZhishuContractSummarySkillMatchesCurrentCLIContract(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "skills", "zhishu-contract-summary")
	skill := readTextFile(t, filepath.Join(root, "SKILL.md"))
	for _, required := range []string{
		"CRITICAL — 开始前 MUST 先读取 [../contract-cli-shared/SKILL.md]",
		"当前 CLI 仅支持 `prod`",
		"contract-cli contract summary query --profile contract --as user --contract-number",
		"contract-cli contract summary query --profile contract --as user --contract-name",
		"contract-cli contract summary generate --profile contract --as user --file",
		"查询接口可能在没有可用摘要时触发生成",
		"不要自动轮询或重复调用",
		"doc`、`docx` 或 `pdf",
		"200MB",
		"当前 CLI 不提供发送到关联协商群的命令",
	} {
		if !strings.Contains(skill, required) {
			t.Fatalf("SKILL.md missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"默认 **test 环境**",
		"不依赖作者电脑、合同 CLI",
		"--as app",
		"每 5 秒一次",
	} {
		if strings.Contains(skill, forbidden) {
			t.Fatalf("SKILL.md keeps incompatible guidance %q", forbidden)
		}
	}

	metadata := readTextFile(t, filepath.Join(root, "agents", "openai.yaml"))
	for _, required := range []string{"智书合同智能摘要", "$zhishu-contract-summary", "合同编号或完整名称", "本地合同文件"} {
		if !strings.Contains(metadata, required) {
			t.Fatalf("openai.yaml missing %q", required)
		}
	}
	if strings.Contains(metadata, "发送至关联协商群") {
		t.Fatal("openai.yaml advertises unsupported cooperation-group sending")
	}

	contractRef := readTextFile(t, filepath.Join(root, "references", "connector-contract.md"))
	for _, required := range []string{
		"POST /open-apis/contract/v1/summary/query",
		"contractNumber",
		"contractName",
		"POST /open-apis/contract/v1/summary/generate",
		"multipart/form-data",
		"仅包含一个 `file` 字段",
	} {
		if !strings.Contains(contractRef, required) {
			t.Fatalf("connector-contract.md missing %q", required)
		}
	}

	responseRef := readTextFile(t, filepath.Join(root, "references", "summary-and-card.md"))
	for _, required := range []string{
		"`0` 未生成",
		"`1` 生成中",
		"`2` 已生成",
		"`3` 生成失败",
		"overview",
		"summary",
		"structuredSummary",
		"cacheData",
		"generated",
	} {
		if !strings.Contains(responseRef, required) {
			t.Fatalf("summary-and-card.md missing %q", required)
		}
	}
}

func TestZhishuContractSummarySkillIsBundled(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
		Store:  config.NewStore(t.TempDir()),
	})
	if err := app.Run(context.Background(), []string{"skills", "list"}); err != nil {
		t.Fatalf("skills list error = %v", err)
	}
	if !strings.Contains(stdout.String(), "zhishu-contract-summary") {
		t.Fatalf("skills list missing zhishu-contract-summary: %s", stdout.String())
	}
}
