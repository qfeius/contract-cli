package cli_test

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

func TestTaskCenterContractTextIdentitySurvivesCLI(t *testing.T) {
	app, out := alignmentApp(t, func(r *http.Request) string {
		if r.Method != "GET" || r.URL.Path != "/open-apis/contract/v1/mcp/contracts/1148617361553621359" {
			t.Fatal(r.Method, r.URL.Path)
		}
		return `{"code":0,"data":{"contract":{"contract_files":{"contract_text":{"file_id":"1180469259373904207","text_id":"1180469259390681423","file_name":"合同.docx"}}}}}`
	})
	if err := app.Run(context.Background(), []string{"contract", "get", "1148617361553621359", "--as", "user", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Data struct {
			Contract struct {
				Files struct {
					Text map[string]string `json:"contract_text"`
				} `json:"contract_files"`
			} `json:"contract"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.Contract.Files.Text["text_id"] != "1180469259390681423" || response.Data.Contract.Files.Text["file_id"] != "1180469259373904207" {
		t.Fatal(response)
	}
}

func TestReviewTaskCenterSkillUploadExample(t *testing.T) {
	content := reviewSkill(t, "contract-cli-review")
	line := regexp.MustCompile(`(?m)^contract-cli review file upload .*"sourceFileType":2.*$`).FindString(content)
	if line == "" {
		t.Fatal("missing executable task-center reference upload example")
	}
	line = strings.NewReplacer("<profile>", "contract", "<contract-id>", "1148617361553621359", "<file-id>", "1180469259373904207", "<text-id>", "1180469259390681423", "<filename.docx>", "合同.docx").Replace(line)
	args := strings.Fields(line)[1:]
	for i := range args {
		args[i] = strings.Trim(args[i], "'")
	}
	app, out := alignmentApp(t, nil)
	if err := app.Run(context.Background(), append(args, "--dry-run")); err != nil {
		t.Fatal(err)
	}
	var preview map[string]string
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview["businessFileId"] != "1180469259373904207" || preview["businessFileVersion"] != "1180469259390681423" || preview["sourceLocator"] != `{"provider":"clm","sourceFileType":2}` {
		t.Fatal(preview)
	}
	// The same example must reject a missing text_id, not substitute file_id.
	for i := range args {
		if args[i] == "--business-file-version" {
			args[i+1] = ""
		}
	}
	app, _ = alignmentApp(t, nil)
	if err := app.Run(context.Background(), append(args, "--dry-run")); err == nil || !strings.Contains(err.Error(), "business-file-version") {
		t.Fatalf("missing text_id was not rejected: %v", err)
	}
}

func TestReviewTaskCenterSkillExtractsUploadedFileSubjects(t *testing.T) {
	content := reviewSkill(t, "contract-cli-review")
	section := strings.Split(strings.Split(content, "## 合同 ID → 任务中心引用上传")[1], "## 选择合同分类")[0]
	line := regexp.MustCompile(`(?m)^contract-cli review subject extract .*$`).FindString(section)
	if line == "" {
		t.Fatal("task-center workflow must extract subjects from the uploaded review file before choosing a position")
	}
	const fileHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	line = strings.NewReplacer("<profile>", "contract", "<business-id>", "1148617361553621359", "<file-id>", "1175703921365090648", "<file-hash>", fileHash).Replace(line)
	calls := 0
	app, out := alignmentApp(t, func(r *http.Request) string {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/open-apis/contract/v3/review/subjects" {
			t.Fatalf("expected subject extraction only, got %s %s", r.Method, r.URL.Path)
		}
		body := alignmentBody(t, r)
		if body["businessId"] != "1148617361553621359" || body["fileId"] != "1175703921365090648" || body["fileHash"] != fileHash {
			t.Fatalf("uploaded review file identity not preserved: %#v", body)
		}
		return `{"code":200,"data":{"counterparts":[{"name":"正文采购方","role":"甲方"},{"name":"正文供应方","role":"乙方"}]}}`
	})
	if err := app.Run(context.Background(), strings.Fields(line)[1:]); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Data struct {
			Counterparts []struct{ Name, Role string } `json:"counterparts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	parties := response.Data.Counterparts
	if calls != 1 || len(parties) != 2 || parties[0].Name != "正文采购方" || parties[0].Role != "甲方" || parties[1].Name != "正文供应方" || parties[1].Role != "乙方" {
		t.Fatalf("file subject name/role pairs changed: calls=%d response=%+v", calls, response)
	}
}
