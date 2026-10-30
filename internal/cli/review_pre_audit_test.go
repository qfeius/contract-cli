package cli_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

const preAuditID = "1195051801729040728"
const preAuditSourceID = "1195051801729040729"
const preAuditLatestID = "1195051801729040730"
const preAuditAIID = "1195051801729040731"
const preAuditTaskID = "1195051801729040732"
const preAuditURL = "https://tenant.example.cn/pre-audit/detail?preAuditId=" + preAuditID + "&from=cli&extra=keep%2Fme"

func preAuditToken(tenant, user string) string {
	claims := `{"iss":"common-organization-v2","userInfo":{"tenantId":` + tenant + `,"userId":` + user + `}}`
	return "eyJhbGciOiJSUzI1NiJ9." + base64.RawURLEncoding.EncodeToString([]byte(claims)) + ".test-signature"
}

type preAuditHarness struct {
	t     *testing.T
	app   *cli.App
	out   *bytes.Buffer
	store *config.Store
	file  string
}

func newPreAuditHarness(t *testing.T, transport func(*http.Request) (*http.Response, error)) *preAuditHarness {
	t.Helper()
	store := config.NewStore(t.TempDir())
	profile := uploadProfile(config.IdentityApp)
	profile.Identities.User.Token.AccessToken = preAuditToken("896684654038353011", "1002718663263913046")
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatal(err)
	}
	out := new(bytes.Buffer)
	app := cli.New(cli.Options{Store: store, Stdout: out, Stderr: io.Discard, HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if transport == nil {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer "+preAuditToken("896684654038353011", "1002718663263913046") {
			t.Fatal("preaudit must send the bound user token")
		}
		return transport(r)
	})}})
	file := filepath.Join(t.TempDir(), "contract.pdf")
	if err := os.WriteFile(file, []byte("contract bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	return &preAuditHarness{t: t, app: app, out: out, store: store, file: file}
}

func (h *preAuditHarness) run(args ...string) (map[string]any, error) {
	h.t.Helper()
	h.out.Reset()
	err := h.app.Run(context.Background(), append([]string{"review", "pre-audit"}, args...))
	var result map[string]any
	if h.out.Len() > 0 {
		decoder := json.NewDecoder(bytes.NewReader(h.out.Bytes()))
		decoder.UseNumber()
		if decodeErr := decoder.Decode(&result); decodeErr != nil {
			h.t.Fatalf("decode output: %v; %s", decodeErr, h.out.String())
		}
	}
	return result, err
}

func (h *preAuditHarness) recordPath(runID string) string {
	return filepath.Join(filepath.Dir(h.store.Path()), "preaudit-runs", runID+".json")
}

func (h *preAuditHarness) prepare() string {
	h.t.Helper()
	result, err := h.run("prepare", "--file", h.file, "--name", "合同.pdf", "--output", "json")
	if err != nil {
		h.t.Fatal(err)
	}
	runID, _ := result["runId"].(string)
	if !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(runID) {
		h.t.Fatalf("missing opaque runId: %v", result)
	}
	return runID
}

func preAuditUploadResponse(t *testing.T, r *http.Request) (*http.Response, error) {
	t.Helper()
	if r.Method != "POST" || r.URL.Path != "/open-apis/contract/v1/files/upload" {
		t.Fatalf("expected CLM upload only, got %s %s", r.Method, r.URL.Path)
	}
	assertUploadMultipart(t, r, "合同.pdf", "text", "contract bytes")
	return jsonResponse(`{"code":0,"data":{"file_id":"` + preAuditSourceID + `"}}`), nil
}

func TestPreAuditPrepareUploadsBeforeCategorySelection(t *testing.T) {
	calls := 0
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		calls++
		return preAuditUploadResponse(t, r)
	})
	runID := h.prepare()
	if calls != 1 || !strings.Contains(h.out.String(), `"workflowStage": "awaiting_category"`) || !strings.Contains(h.out.String(), preAuditSourceID) {
		t.Fatal("prepare must stop after CLM upload with source ID and category selection stage", h.out.String())
	}
	body, err := os.ReadFile(h.recordPath(runID))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"test-signature", "contract bytes", h.file, "https://tenant", "page_url"} {
		if bytes.Contains(body, []byte(secret)) {
			t.Fatalf("recovery record contains prohibited data %q", secret)
		}
	}
	for _, value := range []string{`"schemaVersion": 1`, `"tenantId": "896684654038353011"`, `"userId": "1002718663263913046"`} {
		if !bytes.Contains(body, []byte(value)) {
			t.Fatalf("record lacks exact bound identity/schema: %s", body)
		}
	}
	if runtime.GOOS != "windows" {
		for path, mode := range map[string]os.FileMode{h.recordPath(runID): 0600, filepath.Dir(h.recordPath(runID)): 0700} {
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != mode {
				t.Fatalf("insecure persistence: %s, %v, %v", path, info, err)
			}
		}
	}
}

func TestPreAuditPrepareValidationAndDryRun(t *testing.T) {
	h := newPreAuditHarness(t, nil)
	for _, flags := range [][]string{
		{"--file", h.file, "--name", "合同.exe"},
		{"--file", h.file},
		{"--file", filepath.Dir(h.file), "--name", "合同.pdf"},
		{"--file", h.file, "--name", "合同.pdf", "--as", "app"},
	} {
		if _, err := h.run(append([]string{"prepare"}, flags...)...); err == nil {
			t.Fatal("invalid prepare accepted", flags)
		}
	}
	if _, err := h.run("prepare", "--file", h.file, "--name", "合同.pdf", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(h.store.Path()), "preaudit-runs")); !os.IsNotExist(err) {
		t.Fatal("preview must not create recovery files", err)
	}
}

func TestPreAuditDirectFileRunPreparesAndStops(t *testing.T) {
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) { return preAuditUploadResponse(t, r) })
	body, _ := json.Marshal(map[string]any{"source": map[string]string{"type": "file", "path": h.file, "name": "合同.pdf"}, "wait": false})
	err := h.app.Run(context.Background(), []string{"review", "run", "--data", string(body)})
	if err == nil || !strings.Contains(h.out.String(), "awaiting_category") || !strings.Contains(h.out.String(), "runId") {
		t.Fatalf("direct local file must preserve preparation and require post-upload selection: %v; %s", err, h.out.String())
	}
}

func TestPreAuditDirectFilePreviewShowsSelectionStageWithoutWrites(t *testing.T) {
	h := newPreAuditHarness(t, nil)
	body, _ := json.Marshal(map[string]any{"source": map[string]string{"type": "file", "path": h.file, "name": "合同.pdf"}})
	if err := h.app.Run(context.Background(), []string{"review", "run", "--data", string(body), "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.out.String(), "awaiting_category") || strings.Contains(h.out.String(), "\"reviewStrength\": null") {
		t.Fatal("local preview must describe its actual selection stage", h.out.String())
	}
	if _, err := os.Stat(filepath.Dir(h.recordPath("unused"))); !os.IsNotExist(err) {
		t.Fatal("preview created a workflow record", err)
	}
}

func TestPreAuditDirectFileRunPreservesDeadline(t *testing.T) {
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) > time.Second {
			t.Fatal("local review run must honor its supplied deadline")
		}
		return preAuditUploadResponse(t, r)
	})
	body, _ := json.Marshal(map[string]any{"source": map[string]string{"type": "file", "path": h.file, "name": "合同.pdf"}})
	_ = h.app.Run(context.Background(), []string{"review", "run", "--data", string(body), "--deadline", "1s"})
}

func preAuditCreatedData() string {
	return `{"code":0,"data":{"pre_audit_id":"` + preAuditID + `","source_file_id":"` + preAuditSourceID + `","latest_file_id":"` + preAuditLatestID + `","category_id":"2001","business_type_code":24,"page_url":"` + preAuditURL + `"}}`
}

func preAuditReferenceData(ready bool) string {
	flag := "false"
	if ready {
		flag = "true"
	}
	return `{"code":200,"data":{"businessId":"` + preAuditID + `","fileId":"` + preAuditAIID + `","fileHash":"` + strings.Repeat("a", 64) + `","snapshotReady":` + flag + `}}`
}

func TestPreAuditCreateUsesTwoPublicAPIsAndCLMReference(t *testing.T) {
	calls := 0
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		calls++
		switch r.URL.Path {
		case "/open-apis/contract/v1/files/upload":
			return preAuditUploadResponse(t, r)
		case "/open-apis/contract/v1/pre_audits":
			body := alignmentBody(t, r)
			if r.Method != "POST" || body["source_file_id"] != preAuditSourceID || body["category_id"] != "2001" || body["pre_audit_name"] != "合同.pdf" || body["request_id"] != nil || len(body) != 3 {
				t.Fatal("incorrect public create contract", body)
			}
			return jsonResponse(preAuditCreatedData()), nil
		case "/open-apis/contract/v3/review/file/upload":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = r.MultipartForm.RemoveAll() }()
			if len(r.MultipartForm.File) != 0 {
				t.Fatal("preaudit reference must not upload bytes to AI")
			}
			for k, want := range map[string]string{"businessId": preAuditID, "businessFileId": preAuditLatestID, "businessFileVersion": "v1", "channelType": "cli", "name": "合同.pdf", "sourceLocator": `{"provider":"clm","sourceFileType":0}`} {
				if r.FormValue(k) != want {
					t.Fatalf("%s=%q, want %q", k, r.FormValue(k), want)
				}
			}
			return jsonResponse(preAuditReferenceData(true)), nil
		default:
			t.Fatalf("unexpected create request %s", r.URL)
			return nil, nil
		}
	})
	runID := h.prepare()
	result, err := h.run("create", "--run-id", runID, "--category-id", "2001")
	if err != nil {
		t.Fatal(err)
	}
	if result["workflowStage"] != "awaiting_config" || result["preAuditId"] != preAuditID || result["latestFileId"] != preAuditLatestID || result["fileId"] != preAuditAIID || strings.Contains(h.out.String(), "https://") {
		t.Fatal("create must retain separate file identities and defer the page URL", result)
	}
	record, _ := os.ReadFile(h.recordPath(runID))
	if bytes.Contains(record, []byte("requestId")) || bytes.Contains(record, []byte(preAuditURL)) {
		t.Fatal("create must not persist an idempotency key or navigation URL")
	}
	if _, err := h.run("create", "--run-id", runID, "--category-id", "2001"); err != nil || calls != 3 {
		t.Fatalf("repeated preparation must reuse the same preaudit/reference: %d %v", calls, err)
	}
	if _, err := h.run("create", "--run-id", runID, "--category-id", "2002"); err == nil || calls != 3 {
		t.Fatal("category change must not overwrite an existing create request")
	}
}

func TestPreAuditCreateResponseLossStopsWithoutRetryRecovery(t *testing.T) {
	creates, references := 0, 0
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/open-apis/contract/v1/files/upload":
			return preAuditUploadResponse(t, r)
		case "/open-apis/contract/v1/pre_audits":
			creates++
			return nil, errors.New("lost create response")
		case "/open-apis/contract/v3/review/file/upload":
			references++
			return jsonResponse(preAuditReferenceData(true)), nil
		default:
			t.Fatal(r.URL)
			return nil, nil
		}
	})
	runID := h.prepare()
	if _, err := h.run("create", "--run-id", runID, "--category-id", "2001"); err == nil || creates != 1 || references != 0 {
		t.Fatal("unknown creation must stop before AI reference", err, creates, references)
	}
	for _, args := range [][]string{
		{"create", "--run-id", runID, "--category-id", "2001"},
		{"create", "--run-id", runID, "--category-id", "2002"},
		{"resume", "--run-id", runID},
	} {
		result, err := h.run(args...)
		if err == nil || result["workflowStage"] != "create_pending" || creates != 1 || references != 0 {
			t.Fatal("unknown creation must not be retried or recovered", args, result, err, creates, references)
		}
	}
}

func TestPreAuditCreateRejectsUnreadySnapshot(t *testing.T) {
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/open-apis/contract/v1/files/upload":
			return preAuditUploadResponse(t, r)
		case "/open-apis/contract/v1/pre_audits":
			return jsonResponse(preAuditCreatedData()), nil
		case "/open-apis/contract/v3/review/file/upload":
			return jsonResponse(preAuditReferenceData(false)), nil
		default:
			t.Fatalf("unready snapshot reached next step: %s", r.URL)
			return nil, nil
		}
	})
	runID := h.prepare()
	result, err := h.run("create", "--run-id", runID, "--category-id", "2001")
	if err == nil || result["preAuditId"] != preAuditID || result["workflowStage"] == "awaiting_config" {
		t.Fatal("unready snapshot must preserve preaudit but block start", result, err)
	}
}

func preAuditPreparationRequest(t *testing.T, r *http.Request) (*http.Response, error) {
	t.Helper()
	switch r.URL.Path {
	case "/open-apis/contract/v1/files/upload":
		return preAuditUploadResponse(t, r)
	case "/open-apis/contract/v1/pre_audits":
		return jsonResponse(preAuditCreatedData()), nil
	case "/open-apis/contract/v3/review/file/upload":
		return jsonResponse(preAuditReferenceData(true)), nil
	default:
		t.Fatalf("unexpected preparation: %s %s", r.Method, r.URL.Path)
		return nil, nil
	}
}

func (h *preAuditHarness) create() string {
	h.t.Helper()
	runID := h.prepare()
	if _, err := h.run("create", "--run-id", runID, "--category-id", "2001"); err != nil {
		h.t.Fatal(err)
	}
	return runID
}

func TestPreAuditStartWaitsAndReturnsOnlyCLMPage(t *testing.T) {
	started, statusCalls, detailCalls := false, 0, 0
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/open-apis/contract/v1/pre_audits/" + preAuditID:
			detailCalls++
			if r.Method != "GET" {
				t.Fatal(r.Method)
			}
			return jsonResponse(preAuditCreatedData()), nil
		case "/open-apis/contract/v3/review/startReview":
			body := alignmentBody(t, r)
			usage, _ := body["usageReportContext"].(map[string]any)
			cfg, _ := body["config"].(map[string]any)
			if body["businessId"] != preAuditID || body["fileId"] != preAuditAIID || body["fileHash"] != strings.Repeat("a", 64) || len(body) != 5 || usage["biz1"] != "SMART_PRE_AUDIT" || usage["businessId"] != preAuditID || usage["reportBusinessCode"] != "clm_100_openApi_cli" || cfg["reviewStrength"] != json.Number("1") {
				t.Fatal("start must use bound preaudit context and existing config", body)
			}
			if detailCalls != 1 {
				t.Fatal("start must verify the original preaudit/current file")
			}
			started = true
			return jsonResponse(`{"code":200,"data":{"taskId":"` + preAuditTaskID + `"}}`), nil
		case "/open-apis/contract/v3/review/status":
			statusCalls++
			if !started || r.URL.Query().Get("taskId") != preAuditTaskID || r.URL.Query().Get("businessId") != preAuditID {
				t.Fatal("poll must bind the original task/business", r.URL)
			}
			if statusCalls == 1 {
				return jsonResponse(`{"code":200,"data":{"status":"running"}}`), nil
			}
			return jsonResponse(`{"code":200,"data":{"status":"success"}}`), nil
		case "/open-apis/contract/v3/review/info":
			return jsonResponse(`{"code":200,"data":{"status":"success","ruleResult":[{"id":"risk1","description":"风险明细，临时附件：https://files.example.cn/a?Signature=secret，后续说明仍需保留"}],"url":"https://ai.example.cn/intelligent-review/wrong","reportUrl":"https://files.example.cn/a?X-Amz-Signature=secret","attachments":[["HTTPS://files.example.cn/a?Signature=secret"]]}}`), nil
		default:
			return preAuditPreparationRequest(t, r)
		}
	})
	runID := h.create()
	result, err := h.run("start", "--run-id", runID, "--data", `{"config":`+alignmentConfig+`}`, "--interval", "1ms", "--deadline", "2s")
	if err != nil {
		t.Fatal(err)
	}
	final, _ := result["final"].(map[string]any)
	if statusCalls != 2 || detailCalls != 2 || result["workflowStage"] != "completed" || result["taskId"] != preAuditTaskID || final["reviewDetailUrl"] != preAuditURL || final["ruleResult"] == nil {
		t.Fatal("review must finish with exact backend URL and details", result, statusCalls, detailCalls)
	}
	if strings.Contains(h.out.String(), "intelligent-review") || strings.Contains(h.out.String(), "X-Amz-Signature") {
		t.Fatal("preaudit must not expose an AI URL fallback or signed download URL")
	}
	if !strings.Contains(h.out.String(), "风险明细") || !strings.Contains(h.out.String(), "后续说明仍需保留") {
		t.Fatal("redacting temporary links must preserve the surrounding risk explanation")
	}
	stored, _ := os.ReadFile(h.recordPath(runID))
	if bytes.Contains(stored, []byte("Signature")) || bytes.Contains(stored, []byte("test-signature")) {
		t.Fatal("recovery persistence retained secret URL/credentials")
	}
}

func TestPreAuditLinkFailureRetainsResultsAndResumeOnlyFetchesLink(t *testing.T) {
	getCount, startCount, pollCount := 0, 0, 0
	resume := false
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/open-apis/contract/v1/pre_audits/" + preAuditID:
			getCount++
			data := preAuditCreatedData()
			if getCount == 2 {
				data = strings.Replace(data, preAuditURL, "https://ai.example.cn/intelligent-review/wrong", 1)
			}
			return jsonResponse(data), nil
		case "/open-apis/contract/v3/review/startReview":
			startCount++
			return jsonResponse(`{"code":200,"data":{"taskId":"` + preAuditTaskID + `"}}`), nil
		case "/open-apis/contract/v3/review/status":
			pollCount++
			return jsonResponse(`{"code":200,"data":{"status":"success"}}`), nil
		case "/open-apis/contract/v3/review/info":
			return jsonResponse(`{"code":200,"data":{"ruleResult":[{"id":"risk1"}],"url":"https://ai.example.cn/fallback"}}`), nil
		default:
			if resume {
				t.Fatal("D5-F recovery must only fetch the original page", r.URL)
			}
			return preAuditPreparationRequest(t, r)
		}
	})
	runID := h.create()
	result, err := h.run("start", "--run-id", runID, "--data", `{"config":`+alignmentConfig+`}`)
	final, _ := result["final"].(map[string]any)
	if err == nil || result["workflowStage"] != "review_succeeded" || result["preAuditId"] != preAuditID || result["taskId"] != preAuditTaskID || final["ruleResult"] == nil || final["reviewDetailUrl"] != nil {
		t.Fatal("link error must preserve review success but fail the command", result, err)
	}
	resume = true
	result, err = h.run("resume", "--run-id", runID)
	final, _ = result["final"].(map[string]any)
	if err != nil || getCount != 3 || startCount != 1 || pollCount != 1 || final["reviewDetailUrl"] != preAuditURL {
		t.Fatal("resume should only re-fetch URL", result, err, getCount, startCount, pollCount)
	}
}

func TestPreAuditUnknownStartNeverResends(t *testing.T) {
	starts := 0
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/open-apis/contract/v1/pre_audits/" + preAuditID:
			return jsonResponse(preAuditCreatedData()), nil
		case "/open-apis/contract/v3/review/startReview":
			starts++
			return nil, errors.New("start response lost")
		default:
			return preAuditPreparationRequest(t, r)
		}
	})
	runID := h.create()
	result, err := h.run("start", "--run-id", runID, "--data", `{"config":`+alignmentConfig+`}`)
	if err == nil || result["workflowStage"] != "start_unknown" || result["preAuditId"] != preAuditID {
		t.Fatal("unknown start must be explicit and recoverable by retained IDs", result, err)
	}
	for _, args := range [][]string{{"resume", "--run-id", runID}, {"start", "--run-id", runID, "--data", `{"config":` + alignmentConfig + `}`}} {
		if _, err := h.run(args...); err == nil {
			t.Fatal("unknown start must require manual investigation, not another POST")
		}
	}
	if starts != 1 {
		t.Fatal("start was automatically resent", starts)
	}
}
