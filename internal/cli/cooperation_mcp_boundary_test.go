package cli_test

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	contractskills "cn.qfei/contract-cli/skills"
)

func TestCooperationSearchRejectsWebRoutingBeforeHTTP(t *testing.T) {
	app, _ := alignmentApp(t, nil)
	err := app.Run(context.Background(), []string{"contract", "cooperation", "search", "--channel", "web", "--as", "user", "--data", "{}"})
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("cooperation search must not route through page APIs: %v", err)
	}
}

func TestCooperationSearchRejectsPageOnlyFieldsBeforeHTTP(t *testing.T) {
	for _, body := range []string{`{"confirm_process_status_codes":[4]}`, `{"confirmable_status_codes":[1]}`, `{"create_employee_ids":["123","456"]}`, `{"create_department_ids":["123"]}`, `{"page_index":1}`} {
		app, _ := alignmentApp(t, nil)
		if err := app.Run(context.Background(), []string{"contract", "cooperation", "search", "--as", "user", "--data", body}); err == nil {
			t.Fatalf("undeclared MCP field accepted: %s", body)
		}
	}
}

func TestCooperationSkillDoesNotDependOnPageAPIs(t *testing.T) {
	err := fs.WalkDir(contractskills.FS, "contract-cooperation", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		content, err := fs.ReadFile(contractskills.FS, path)
		if err != nil {
			return err
		}
		for _, transport := range []string{"/open-web/", "--channel web", "/clm/api/"} {
			if strings.Contains(string(content), transport) {
				t.Errorf("cooperation Skill depends on page transport %q in %s", transport, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
