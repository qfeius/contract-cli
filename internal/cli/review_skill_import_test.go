package cli_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// Validate the distributed instructions, not a second copy of the examples.
func TestReviewImportedSkillDocuments(t *testing.T) {
	for _, name := range []string{"contract-cli-review", "contract-cli-review-rule"} {
		t.Run(name, func(t *testing.T) {
			body := reviewSkill(t, name)
			if !strings.HasPrefix(body, "---\n") {
				t.Fatal("skill discovery requires YAML frontmatter")
			}
			for _, artifact := range []string{`\&#x20;`, `\\\<`, `\\\[`, `\\\_`} {
				if strings.Contains(body, artifact) {
					t.Errorf("export artifact makes distributed examples unusable: %q", artifact)
				}
			}
		})
	}
}

func TestReviewChecklistEditExamplePreservesCategory(t *testing.T) {
	examples := skillJSONExamples(t, reviewSkill(t, "contract-cli-review-rule"), 3)
	preview := skillDryRun(t, []string{"review", "checklist", "update", "--id", "checklist-fixture"}, examples[0])
	var body map[string]json.RawMessage
	if err := json.Unmarshal([]byte(preview), &body); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"contractCategory", "reviewRuleIds"} {
		var values []string
		if err := json.Unmarshal(body[field], &values); err != nil || len(values) != 1 {
			t.Fatalf("edit example lost existing %s: %s (%v)", field, preview, err)
		}
	}
}
