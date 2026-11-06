package cli_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestReviewReferenceSkillAndHelp(t *testing.T) {
	content := reviewSkill(t, "contract-cli-review")
	match := regexp.MustCompile(`--source-locator '([^']+)'`).FindStringSubmatch(content)
	if len(match) != 2 {
		t.Fatal("Skill lacks executable reference upload locator")
	}
	locator := strings.NewReplacer("COOPERATION_ID", "1178244026340475224", "COOPERATION_FILE_ID", "1178244026390806872").Replace(match[1])
	args := referenceArgs()
	args[len(args)-1] = locator
	app, out := alignmentApp(t, nil)
	if err := app.Run(context.Background(), append(args, "--dry-run")); err != nil {
		t.Fatal(err)
	}
	var preview map[string]string
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	var source map[string]any
	if err := json.Unmarshal([]byte(preview["sourceLocator"]), &source); err != nil {
		t.Fatal(err)
	}
	if source["cooperationFileId"] != preview["businessFileId"] || source["sourceFileType"] != float64(1) {
		t.Fatal("Skill reference mapping disagrees with upload", source)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"review", "cooperation", "get", "--help"}, "--cooperation-id"},
		{[]string{"review", "file", "upload", "--help"}, "--source-locator"},
		{[]string{"review", "file", "upload", "--help"}, "--business-file-version"},
	} {
		app, out := alignmentApp(t, nil)
		if err := app.Run(context.Background(), tc.args); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), tc.want) {
			t.Fatalf("public help missing %s", tc.want)
		}
	}
}

func referenceArgs() []string {
	return []string{"review", "file", "upload", "--business-id", "1178244026676019544", "--business-file-id", "1178244026390806872", "--name", "合同.docx", "--business-file-version", "V1.0", "--source-locator", `{"provider":"clm","sourceFileType":1,"cooperationId":"1178244026340475224","cooperationFileId":"1178244026390806872"}`}
}

func TestReviewReferenceUpload(t *testing.T) {
	called := 0
	app, _ := alignmentApp(t, func(r *http.Request) string {
		called++
		if r.Method != "POST" || r.URL.Path != "/open-apis/contract/v3/review/file/upload" {
			t.Fatal(r.URL)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = r.MultipartForm.RemoveAll() }()
		if len(r.MultipartForm.File) != 0 {
			t.Fatal("reference must not upload bytes")
		}
		for k, want := range map[string]string{"businessId": "1178244026676019544", "businessFileId": "1178244026390806872", "businessFileVersion": "V1.0", "name": "合同.docx", "channelType": "cli", "sourceContentMd5": `"738f104f3ee333437d7d261c2ea9f2a9"`} {
			if r.FormValue(k) != want {
				t.Errorf("%s = %q, want %q", k, r.FormValue(k), want)
			}
		}
		for _, k := range []string{"cooperationId", "cooperationFileId", "appType", "file"} {
			if _, ok := r.MultipartForm.Value[k]; ok {
				t.Errorf("unexpected top level %s", k)
			}
		}
		var locator map[string]any
		if err := json.Unmarshal([]byte(r.FormValue("sourceLocator")), &locator); err != nil {
			t.Fatal(err)
		}
		if locator["sourceFileType"] != float64(1) || locator["cooperationFileId"] != "1178244026390806872" {
			t.Fatal(locator)
		}
		return `{"code":200,"data":{"fileId":"1006610","businessId":"1178244026676019544"}}`
	})
	err := app.Run(context.Background(), append(referenceArgs(), "--channel-type", "cli", "--source-content-md5", `"738f104f3ee333437d7d261c2ea9f2a9"`))
	if err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatal(called)
	}
}

func TestReviewReferenceValidation(t *testing.T) {
	for _, tc := range []struct{ name, flag, value, want string }{
		{"missing business", "--business-id", "", "business-id"},
		{"missing file", "--business-file-id", " ", "business-file-id"},
		{"missing version", "--business-file-version", "", "business-file-version"},
		{"long version", "--business-file-version", strings.Repeat("版", 65), "64"},
		{"array", "--source-locator", "[]", "source-locator"},
		{"null", "--source-locator", "null", "source-locator"},
		{"missing type", "--source-locator", `{}`, "sourceFileType"},
		{"boolean type", "--source-locator", `{"sourceFileType":true}`, "sourceFileType"},
		{"invalid type", "--source-locator", `{"sourceFileType":3}`, "sourceFileType"},
		{"numeric id", "--source-locator", `{"sourceFileType":1,"cooperationFileId":1178244026390806872}`, "cooperationFileId"},
		{"unknown", "--source-locator", `{"sourceFileType":1,"untrusted":1}`, "untrusted"},
		{"large", "--source-locator", `{"sourceFileType":1,"provider":"` + strings.Repeat("中", 1400) + `"}`, "4096"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := referenceArgs()
			for i := range args {
				if args[i] == tc.flag {
					args[i+1] = tc.value
					break
				}
			}
			app, _ := alignmentApp(t, nil)
			err := app.Run(context.Background(), append(args, "--dry-run"))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %s, got %v", tc.want, err)
			}
		})
	}
	for _, flags := range [][]string{{"--stdin"}, {"--file", "contract.docx"}, {"--source-revision-id", strings.Repeat("a", 129)}} {
		app, _ := alignmentApp(t, nil)
		if err := app.Run(context.Background(), append(append(referenceArgs(), flags...), "--dry-run")); err == nil {
			t.Fatal("invalid combination accepted")
		}
	}
	for _, sourceType := range []string{"0", "1", "2"} {
		args := referenceArgs()
		args[len(args)-1] = `{"sourceFileType":` + sourceType + `}`
		app, out := alignmentApp(t, nil)
		if err := app.Run(context.Background(), append(args, "--dry-run")); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "1178244026390806872") {
			t.Fatal("lost reference ID")
		}
	}
}

func TestReviewStreamUploadBusinessFieldsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "合同.pdf")
	if err := os.WriteFile(path, []byte("contract"), 0600); err != nil {
		t.Fatal(err)
	}
	called := false
	app, _ := alignmentApp(t, func(*http.Request) string {
		called = true
		return `{"code":200,"data":{}}`
	})
	err := app.Run(context.Background(), []string{"review", "file", "upload", "--file", path, "--name", "合同.pdf", "--business-id", "existing", "--business-file-id", "123"})
	if err == nil || !strings.Contains(err.Error(), "review pre-audit prepare") || called {
		t.Fatalf("business fields must not enable binary upload: err=%v called=%t", err, called)
	}
}

func TestReviewCooperationGet(t *testing.T) {
	called := 0
	app, out := alignmentApp(t, func(r *http.Request) string {
		called++
		if r.Method != "GET" || r.URL.Path != "/open-apis/contract/v1/mcp/cooperations/1178244026340475224" {
			t.Fatal(r.URL)
		}
		return `{"code":0,"data":{"cooperation_id":"1178244026340475224","current_round_files":[{"cooperation_file_id":"1178244026390806872","file_id":"999","version":"V1.0","latest":true,"cooperation_file_type_code":0}]}}`
	})
	if err := app.Run(context.Background(), []string{"review", "cooperation", "get", "--cooperation-id", "1178244026340475224"}); err != nil {
		t.Fatal(err)
	}
	if called != 1 || !strings.Contains(out.String(), "1178244026390806872") {
		t.Fatal(out.String())
	}
	for _, flags := range [][]string{{"--cooperation-id", "../123"}, {"--cooperation-id", "0"}, {"--cooperation-id", "123", "--as", "app"}, {"--cooperation-id", "123", "--data", "{}"}} {
		app, _ := alignmentApp(t, nil)
		if err := app.Run(context.Background(), append([]string{"review", "cooperation", "get"}, flags...)); err == nil {
			t.Fatal("bad query accepted")
		}
	}
}
