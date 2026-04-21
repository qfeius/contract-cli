package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandReferenceDocumentCoversCurrentSupportedCommands(t *testing.T) {
	t.Parallel()

	docPath := filepath.Join("..", "..", "docs", "cli-command-reference.md")
	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", docPath, err)
	}

	text := string(content)
	requiredFragments := []string{
		"# contract-cli 命令文档",
		"contract-cli config add",
		"contract-cli auth login",
		"contract-cli auth status",
		"contract-cli auth logout",
		"contract-cli auth use",
		"contract-cli version",
		"contract-cli skills list",
		"contract-cli skills install",
		"npx skills add qfeius/contract-cli -y -g",
		"contract-cli update check",
		"contract-cli api call",
		"contract-cli contract search",
		"contract-cli contract get",
		"contract-cli contract sync-user-groups",
		"contract-cli contract text",
		"contract-cli contract create",
		"contract-cli contract upload-file",
		"contract-cli contract category list",
		"contract-cli contract template list",
		"contract-cli contract template get",
		"contract-cli contract template instantiate",
		"contract-cli contract enum list",
		"contract-cli mdm vendor list",
		"contract-cli mdm vendor get",
		"contract-cli mdm legal list",
		"contract-cli mdm legal get",
		"contract-cli mdm fields list",
		"`contract get`、`contract search`、`contract create`、`contract sync-user-groups`、`contract text`、`contract category list`、`contract template list`、`contract template get`、`contract template instantiate`、`mdm vendor list`、`mdm vendor get`、`mdm legal list`、`mdm legal get`、`mdm fields list` 是当前仅有的十四个同时支持 `user` 与 `bot` 的结构化业务命令",
		"`contract upload-file` 当前仅支持 `--as bot`",
		"`--user-id-type`",
		"`--user-id`",
		"传了就拼接到 query string",
		"`auth login --as bot`",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(text, fragment) {
			t.Fatalf("command reference missing %q", fragment)
		}
	}
}
