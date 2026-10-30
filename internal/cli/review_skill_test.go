package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	contractskills "cn.qfei/contract-cli/skills"
)

func reviewSkill(t *testing.T, name string) string {
	t.Helper()
	body, err := fs.ReadFile(contractskills.FS, name+"/SKILL.md")
	if err != nil {
		t.Fatalf("bundled review skill unavailable: %s: %v", name, err)
	}
	return string(body)
}

func TestReviewSkillDistributionAndRouting(t *testing.T) {
	out := new(bytes.Buffer)
	app := cli.New(cli.Options{Stdout: out, Stderr: new(bytes.Buffer), Store: config.NewStore(t.TempDir()), SkillsFS: contractskills.FS})
	if err := app.Run(context.Background(), []string{"skills", "list"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"contract-cli-review", "contract-cli-review-rule", "contract-cli-rule"} {
		if !strings.Contains(out.String(), name+"\t") {
			t.Fatalf("published catalog missing %s", name)
		}
	}
	if strings.Contains(out.String(), "contract-cli-review-config") {
		t.Fatal("superseded skill name must not be published")
	}
	target := filepath.Join(t.TempDir(), "skills")
	if err := app.Run(context.Background(), []string{"skills", "install", "--target", target}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}
	if count != 19 {
		t.Fatalf("want 19 installed skill directories, got %d", count)
	}
	if _, err := os.Stat(filepath.Join(target, ".contract-cli-managed.json")); err != nil {
		t.Fatalf("complete installation must retain its management marker: %v", err)
	}
	for _, name := range []string{"contract-cli-shared", "contract-cli-contract"} {
		content := readTextFile(t, filepath.Join(target, name, "SKILL.md"))
		for _, dest := range []string{"contract-cli-review", "contract-cli-review-rule"} {
			if !strings.Contains(content, "../"+dest+"/SKILL.md") {
				t.Fatalf("%s has no route to %s", name, dest)
			}
			if _, err := os.Stat(filepath.Join(target, dest, "SKILL.md")); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Packaged references must resolve after installation. Historical source-code
	// citations intentionally point outside the skill package; check those in source.
	links := regexp.MustCompile(`\]\(([^)#]+)(?:#[^)]*)?\)`)
	// Fenced output templates contain illustrative links, not package dependencies.
	fencedExamples := regexp.MustCompile("(?ms)^[ \\t]*```[^\\n]*\\n.*?^[ \\t]*```[ \\t]*$")
	inlineCode := regexp.MustCompile("`[^`\\n]*`")
	for _, name := range []string{"contract-cli-shared", "contract-cli-contract", "contract-cli-review", "contract-cli-review-rule"} {
		content := readTextFile(t, filepath.Join(target, name, "SKILL.md"))
		content = fencedExamples.ReplaceAllString(content, "")
		content = inlineCode.ReplaceAllString(content, "")
		for _, link := range links.FindAllStringSubmatch(content, -1) {
			if strings.Contains(link[1], "://") {
				continue
			}
			if strings.HasPrefix(link[1], "../../internal/") && (name == "contract-cli-shared" || name == "contract-cli-contract") {
				if _, err := os.Stat(filepath.Join("..", "..", "skills", name, link[1])); err != nil {
					t.Errorf("broken historical source citation %s: %v", link[1], err)
				}
				continue
			}
			if _, err := os.Stat(filepath.Join(target, name, link[1])); err != nil {
				t.Errorf("broken installed reference %s: %v", link[1], err)
			}
		}
	}
}

func skillJSONExamples(t *testing.T, content string, want int) []string {
	t.Helper()
	matches := regexp.MustCompile("(?s)```json\\n(.*?)\\n```").FindAllStringSubmatch(content, -1)
	if len(matches) != want {
		t.Fatalf("want %d executable JSON examples, got %d", want, len(matches))
	}
	var result []string
	for _, m := range matches {
		if !json.Valid([]byte(m[1])) {
			t.Fatalf("invalid skill JSON example: %s", m[1])
		}
		result = append(result, m[1])
	}
	return result
}

func skillDryRun(t *testing.T, args []string, body string) string {
	t.Helper()
	app, out := alignmentApp(t, nil) // Any remote request fails the test.
	if body != "" {
		if len(args) >= 2 && args[0] == "review" && args[1] == "run" {
			var input map[string]json.RawMessage
			if err := json.Unmarshal([]byte(body), &input); err != nil {
				t.Fatal(err)
			}
			var source map[string]string
			if err := json.Unmarshal(input["source"], &source); err != nil || source["type"] != "file" {
				t.Fatalf("published run example must use a local CLM file: %s", input["source"])
			}
			// Supply the documented local attachment, keeping its business input intact.
			source["path"] = filepath.Join(t.TempDir(), source["name"])
			if err := os.WriteFile(source["path"], []byte("contract fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			input["source"], _ = json.Marshal(source)
			prepared, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			body = string(prepared)
		}
		file := filepath.Join(t.TempDir(), "input.json")
		if err := os.WriteFile(file, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		args = append(args, "--input-file", file)
	}
	if err := app.Run(context.Background(), append(args, "--as", "user", "--dry-run")); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestReviewSkillExamplesMatchCLI(t *testing.T) {
	content := reviewSkill(t, "contract-cli-review")
	examples := skillJSONExamples(t, content, 2)
	start := skillDryRun(t, []string{"review", "task", "start"}, examples[0])
	if !strings.Contains(start, "clm_100_openApi_cli") {
		t.Fatalf("start preview lacks Contract usage attribution: %s", start)
	}
	if !strings.Contains(start, `"fileId": "1175703921365090648"`) {
		t.Fatalf("documented int64 file ID lost precision or wire string type: %s", start)
	}
	skillDryRun(t, []string{"review", "run"}, examples[1])
	skillDryRun(t, []string{"review", "subject", "extract", "--business-id", "biz-fixture", "--file-id", "1175703921365090648"}, "")
	// The documentation's selected command paths must exist in the public help tree.
	for _, cmd := range []string{"review file upload", "review subject extract", "review task start", "review task status", "review task info", "review task result", "review run", "review checklist list"} {
		if !strings.Contains(content, "contract-cli "+cmd) {
			t.Fatalf("workflow lacks command %s", cmd)
		}
		app, _ := alignmentApp(t, nil)
		if err := app.Run(context.Background(), append(strings.Fields(cmd), "--help")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReviewSkillBuiltinRulePackageExamples(t *testing.T) {
	examples := skillJSONExamples(t, reviewSkill(t, "contract-cli-review"), 2)
	for i, command := range [][]string{{"review", "task", "start"}, {"review", "run"}} {
		t.Run(strings.Join(command, "-"), func(t *testing.T) {
			var input map[string]json.RawMessage
			if err := json.Unmarshal([]byte(examples[i]), &input); err != nil {
				t.Fatal(err)
			}
			var cfg map[string]json.RawMessage
			if err := json.Unmarshal(input["config"], &cfg); err != nil {
				t.Fatal(err)
			}
			check := func(body string, wantBuiltin, wantCustom bool) {
				t.Helper()
				output := skillDryRun(t, command, body)
				if i == 1 {
					var preview struct {
						Stage string `json:"workflowStage"`
					}
					if err := json.Unmarshal([]byte(output), &preview); err != nil || preview.Stage != "awaiting_category" {
						t.Fatalf("run must stop before category/config selection regardless of configured rules: %s", output)
					}
					return
				}
				var preview struct {
					Config struct {
						Builtin bool     `json:"matchContractTypeRulePackage"`
						IDs     []string `json:"selectedCheckListIds"`
					} `json:"config"`
				}
				if err := json.Unmarshal([]byte(output), &preview); err != nil {
					t.Fatal(err)
				}
				if preview.Config.Builtin != wantBuiltin {
					t.Fatalf("published example: want builtin=%t, got %t", wantBuiltin, preview.Config.Builtin)
				}
				if wantCustom {
					if len(preview.Config.IDs) != 1 || preview.Config.IDs[0] != "CHECKLIST_ID_FROM_QUERY" {
						t.Fatalf("custom checklist identity changed: %v", preview.Config.IDs)
					}
				} else if len(preview.Config.IDs) != 0 {
					t.Fatalf("builtin selection must not fabricate a checklist ID: %v", preview.Config.IDs)
				}
			}
			encode := func() string {
				t.Helper()
				configJSON, err := json.Marshal(cfg)
				if err != nil {
					t.Fatal(err)
				}
				input["config"] = configJSON
				body, err := json.Marshal(input)
				if err != nil {
					t.Fatal(err)
				}
				return string(body)
			}
			check(examples[i], true, true)
			customIDs := cfg["selectedCheckListIds"]
			delete(cfg, "selectedCheckListIds")
			check(encode(), true, false)
			cfg["selectedCheckListIds"] = customIDs
			delete(cfg, "matchContractTypeRulePackage")
			check(encode(), false, true)
		})
	}
}

func TestReviewRuleSkillExamplesMatchCLI(t *testing.T) {
	content := reviewSkill(t, "contract-cli-review-rule")
	examples := skillJSONExamples(t, content, 3)
	for i, cmd := range [][]string{{"review", "checklist", "create"}, {"review", "rule", "group", "create"}, {"review", "rule", "create", "--group-id", "group-fixture"}} {
		skillDryRun(t, cmd, examples[i])
	}
	skillDryRun(t, []string{"review", "rule", "update", "--group-id", "group-fixture", "--rule-id", "rule-fixture"}, examples[2])
	for _, cmd := range [][]string{{"review", "checklist", "delete", "--id", "checklist-fixture"}, {"review", "rule", "delete", "--group-id", "group-fixture", "--rule-id", "rule-fixture"}, {"review", "rule", "group", "delete", "--id", "group-fixture"}} {
		skillDryRun(t, cmd, "") // Preview is not a deletion or approval.
	}
}

func TestReviewSkillChecklistExampleFiltersSelectedCategory(t *testing.T) {
	// Execute the published example, not a separately reconstructed command.
	const categoryID = "1175703921365090648"
	content := reviewSkill(t, "contract-cli-review")
	var example string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "contract-cli review checklist list ") {
			example = line
			break
		}
	}
	if example == "" {
		t.Fatal("review skill has no executable checklist query example")
	}
	called := false
	app, _ := alignmentApp(t, func(r *http.Request) string {
		called = true
		q := r.URL.Query()
		if r.Method != http.MethodGet {
			t.Fatalf("checklist selection must be read-only, got %s", r.Method)
		}
		categories := q["contractCategories"]
		if len(categories) != 1 || categories[0] != categoryID {
			t.Errorf("query must use the selected category without precision loss: %v", categories)
		}
		if q.Get("enabled") != "true" {
			t.Errorf("available checklist example must request enabled=true, got %q", q.Get("enabled"))
		}
		if q.Get("pageIndex") != "1" || q.Get("pageSize") != "100" {
			t.Errorf("documented checklist pagination not forwarded: %v", q)
		}
		return `{"code":200,"data":{"content":[],"hasNext":false,"totalPages":0}}`
	})
	args := strings.Fields(example)[1:]
	// The fixture's current profile supplies an isolated, authorized user context.
	var resolved []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--profile" && i+1 < len(args) && args[i+1] == "<profile>" {
			i++
			continue
		}
		resolved = append(resolved, strings.ReplaceAll(args[i], "<category-id>", categoryID))
	}
	if err := app.Run(context.Background(), resolved); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("documented example never reached the checklist query")
	}
}
