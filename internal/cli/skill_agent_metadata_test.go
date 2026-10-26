package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicSkillAgentMetadata(t *testing.T) {
	root := filepath.Join("..", "..", "skills")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, err := os.Stat(filepath.Join(root, name, "SKILL.md")); os.IsNotExist(err) {
			continue
		} else if err != nil {
			t.Fatal(err)
		}
		metadata, err := os.ReadFile(filepath.Join(root, name, "agents", "openai.yaml"))
		if err != nil {
			t.Errorf("%s must provide agents/openai.yaml: %v", name, err)
			continue
		}
		for _, fragment := range []string{
			"interface:",
			"display_name:",
			"short_description:",
			"default_prompt:",
			"policy:",
			"allow_implicit_invocation:",
		} {
			if !strings.Contains(string(metadata), fragment) {
				t.Errorf("%s agents/openai.yaml missing %q", name, fragment)
			}
		}
	}
}

func TestReviewSkillAgentMetadata(t *testing.T) {
	root := filepath.Join("..", "..", "skills")
	cases := map[string][]string{
		"contract-cli-review": {
			"$contract-cli-review",
			"$contract-cli-review-rule",
			"allow_implicit_invocation: true",
		},
		"contract-cli-review-rule": {
			"$contract-cli-review-rule",
			"$contract-cli-review",
			"allow_implicit_invocation: true",
		},
	}
	for name, fragments := range cases {
		metadata, err := os.ReadFile(filepath.Join(root, name, "agents", "openai.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		for _, fragment := range fragments {
			if !strings.Contains(string(metadata), fragment) {
				t.Errorf("%s agents/openai.yaml missing route %q", name, fragment)
			}
		}
	}
}
