package skillsync

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestRewriteVersionPreservesNestedMetadata(t *testing.T) {
	cases := []struct {
		name, input, expected string
	}{
		{
			name:     "insert after nested metadata",
			input:    "---\nname: review\nmetadata:\n  version: old-metadata\n  owner: contracts\n---\nbody\n",
			expected: "---\nname: review\nmetadata:\n  version: old-metadata\n  owner: contracts\nversion: 1.9.9\n---\nbody\n",
		},
		{
			name:     "replace top-level version after metadata",
			input:    "---\nmetadata:\n  version: old-metadata\nversion: old-package\n---\nbody\n",
			expected: "---\nmetadata:\n  version: old-metadata\nversion: 1.9.9\n---\nbody\n",
		},
		{
			name:     "replace version before metadata",
			input:    "---\nversion: old-package\nmetadata:\n  version: old-metadata\n---\nbody\n",
			expected: "---\nversion: 1.9.9\nmetadata:\n  version: old-metadata\n---\nbody\n",
		},
		{
			name:     "keep indented description and body",
			input:    "---\ndescription: |\n  version: documentation\n---\nversion: body\n",
			expected: "---\ndescription: |\n  version: documentation\nversion: 1.9.9\n---\nversion: body\n",
		},
		{
			name:     "allow whitespace before top-level colon",
			input:    "---\nversion : old-package\n---\nbody\n",
			expected: "---\nversion: 1.9.9\n---\nbody\n",
		},
		{
			name:     "leave plain text unchanged",
			input:    "version: body\n",
			expected: "version: body\n",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			actual := string(RewriteVersion([]byte(test.input), "1.9.9"))
			if actual != test.expected {
				t.Fatalf("unexpected rewritten document:\n%s\nwant:\n%s", actual, test.expected)
			}
		})
	}
}

func TestInstallPreservesNestedSkillMetadata(t *testing.T) {
	content := "---\nname: contract-cli-review\nmetadata:\n  version: review-schema-v1\n---\nreview instructions\n"
	service := &Service{StateDir: t.TempDir(), Version: "1.9.9", Source: fstest.MapFS{
		"contract-cli-review/SKILL.md": {Data: []byte(content)},
	}}
	target := t.TempDir()
	if _, err := service.Install(target, []string{"contract-cli-review"}, false, true); err != nil {
		t.Fatal(err)
	}
	installed, err := os.ReadFile(filepath.Join(target, "contract-cli-review", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	expected := "---\nname: contract-cli-review\nmetadata:\n  version: review-schema-v1\nversion: 1.9.9\n---\nreview instructions\n"
	if string(installed) != expected {
		t.Fatalf("installed Skill metadata changed:\n%s", installed)
	}
}
