package cli_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/cli"
)

func (h *preAuditHarness) rewriteRecord(runID string, mutate func(map[string]any)) {
	h.t.Helper()
	body, err := os.ReadFile(h.recordPath(runID))
	if err != nil {
		h.t.Fatal(err)
	}
	var record map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	if err := decoder.Decode(&record); err != nil {
		h.t.Fatal(err)
	}
	mutate(record)
	body, _ = json.Marshal(record)
	if err := os.WriteFile(h.recordPath(runID), body, 0600); err != nil {
		h.t.Fatal(err)
	}
}

func TestPreAuditRecoveryRejectsCorruptRecordsBeforeRequests(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"version", func(r map[string]any) { r["schemaVersion"] = 2 }},
		{"unknown stage", func(r map[string]any) { r["workflowStage"] = "fake" }},
		{"missing source", func(r map[string]any) { delete(r, "sourceFileId") }},
		{"bad source", func(r map[string]any) { r["sourceFileId"] = "../other" }},
		{"bad request", func(r map[string]any) { r["workflowStage"] = "create_pending"; r["requestId"] = "bad" }},
		{"missing original object", func(r map[string]any) { r["workflowStage"] = "running"; r["taskId"] = preAuditTaskID }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
				calls++
				return preAuditUploadResponse(t, r)
			})
			runID := h.prepare()
			h.rewriteRecord(runID, tc.mutate)
			if _, err := h.run("create", "--run-id", runID, "--category-id", "2001"); err == nil || calls != 1 {
				t.Fatal("corrupt recovery must stop before network", err, calls)
			}
		})
	}
}

func TestPreAuditRecoveryRejectsIdentityAndOriginChanges(t *testing.T) {
	for _, kind := range []string{"tenant", "user", "token", "origin", "profile"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) { calls++; return preAuditUploadResponse(t, r) })
			runID := h.prepare()
			profile, _ := h.store.GetProfile("contract")
			switch kind {
			case "tenant":
				profile.Identities.User.Token.AccessToken = preAuditToken("896684654038353012", "1002718663263913046")
			case "user":
				profile.Identities.User.Token.AccessToken = preAuditToken("896684654038353011", "1002718663263913047")
			case "token":
				profile.Identities.User.Token.AccessToken = "opaque-token"
			case "origin":
				profile.OpenPlatformBaseURL = "https://other.example.cn"
			case "profile":
				profile.Name = "another"
			}
			if err := h.store.UpsertProfile(profile, true); err != nil {
				t.Fatal(err)
			}
			if _, err := h.run("create", "--run-id", runID, "--category-id", "2001"); err == nil || calls != 1 {
				t.Fatal("identity/origin change must prevent all business requests", err, calls)
			}
		})
	}
}

func TestPreAuditRecoveryRejectsSymlinksAndMissingRecords(t *testing.T) {
	for _, kind := range []string{"directory", "record", "lock", "missing", "invalid-id"} {
		t.Run(kind, func(t *testing.T) {
			h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) { return preAuditUploadResponse(t, r) })
			runID := h.prepare()
			path := h.recordPath(runID)
			switch kind {
			case "directory":
				path = filepath.Dir(path)
			case "lock":
				path = strings.TrimSuffix(path, ".json") + ".lock"
			case "invalid-id":
				runID = "../../anything"
			}
			if kind == "missing" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else if kind != "invalid-id" {
				backup := path + "-backup"
				if err := os.Rename(path, backup); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(backup, path); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			if _, err := h.run("resume", "--run-id", runID); err == nil {
				t.Fatal("unsafe/missing recovery path accepted")
			}
		})
	}
}

func TestPreAuditSaveFailureStopsBeforeStart(t *testing.T) {
	var h *preAuditHarness
	var runID string
	starts := 0
	h = newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/open-apis/contract/v1/pre_audits/"+preAuditID {
			path := h.recordPath(runID)
			if err := os.Rename(path, path+".backup"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			return jsonResponse(preAuditCreatedData()), nil
		}
		if strings.HasSuffix(r.URL.Path, "/startReview") {
			starts++
		}
		return preAuditPreparationRequest(t, r)
	})
	runID = h.create()
	if _, err := h.run("start", "--run-id", runID, "--data", `{"config":`+alignmentConfig+`}`); err == nil || starts != 0 {
		t.Fatal("write-ahead save failure reached startReview", err, starts)
	}
}

func TestPreAuditConcurrentRunIsLocked(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/open-apis/contract/v1/pre_audits" {
			close(entered)
			<-release
			return jsonResponse(preAuditCreatedData()), nil
		}
		return preAuditPreparationRequest(t, r)
	})
	runID := h.prepare()
	done := make(chan error, 1)
	go func() { _, err := h.run("create", "--run-id", runID, "--category-id", "2001"); done <- err }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("first process did not reach create")
	}
	second := cli.New(cli.Options{Store: h.store, Stdout: io.Discard, Stderr: io.Discard})
	err := second.Run(context.Background(), []string{"review", "pre-audit", "create", "--run-id", runID, "--category-id", "2001"})
	close(release)
	if err == nil || !strings.Contains(err.Error(), "正在执行") {
		t.Fatal("second process must fail before any network write", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestPreAuditKnownTaskResumesPollingWithoutNewWrites(t *testing.T) {
	resume := false
	starts := 0
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/open-apis/contract/v1/pre_audits/" + preAuditID:
			return jsonResponse(preAuditCreatedData()), nil
		case "/open-apis/contract/v3/review/startReview":
			starts++
			return jsonResponse(`{"code":200,"data":{"taskId":"` + preAuditTaskID + `"}}`), nil
		case "/open-apis/contract/v3/review/status":
			status := "running"
			if resume {
				status = "success"
			}
			return jsonResponse(`{"code":200,"data":{"status":"` + status + `"}}`), nil
		case "/open-apis/contract/v3/review/info":
			return jsonResponse(`{"code":200,"data":{"ruleResult":[]}}`), nil
		default:
			if resume {
				t.Fatal("known-task resume must only read existing objects", r.URL)
			}
			return preAuditPreparationRequest(t, r)
		}
	})
	runID := h.create()
	result, err := h.run("start", "--run-id", runID, "--data", `{"config":`+alignmentConfig+`}`, "--interval", "1s", "--deadline", "80ms")
	if err == nil || result["workflowStage"] != "running" || result["taskId"] != preAuditTaskID {
		t.Fatal("deadline must retain the accepted task", result, err)
	}
	resume = true
	if result, err = h.run("resume", "--run-id", runID, "--interval", "1ms"); err != nil || result["workflowStage"] != "completed" || starts != 1 {
		t.Fatal(result, err, starts)
	}
}

func TestPreAuditStartInputCannotReplaceBoundContext(t *testing.T) {
	h := newPreAuditHarness(t, nil)
	for _, field := range []string{`"businessId":"other",`, `"fileId":123,`, `"usageReportContext":{"biz1":"OTHER"},`, `"clientRequestId":"new",`, `"aiFunction":100,`} {
		_, err := h.run("start", "--run-id", strings.Repeat("a", 32), "--data", `{`+field+`"config":`+alignmentConfig+`}`)
		if err == nil || !strings.Contains(err.Error(), "unknown field") {
			t.Fatal("caller replaced run binding", err)
		}
	}
}

func TestPreAuditRejectsInvalidPagesAndChangedCurrentFile(t *testing.T) {
	for _, invalid := range []string{"", "javascript:alert(1)", "https://user:pass@tenant.example.cn/pre-audit/detail?preAuditId=" + preAuditID + "&from=cli", preAuditURL + "#fragment", preAuditURL + "&preAuditId=" + preAuditID, preAuditURL + "&from=cli", strings.Replace(preAuditURL, preAuditID, "100", 1), strings.Replace(preAuditURL, "from=cli", "from=home", 1)} {
		t.Run(invalid, func(t *testing.T) {
			details := 0
			h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case "/open-apis/contract/v1/pre_audits/" + preAuditID:
					details++
					data := preAuditCreatedData()
					if details == 2 {
						data = strings.Replace(data, preAuditURL, invalid, 1)
					}
					return jsonResponse(data), nil
				case "/open-apis/contract/v3/review/startReview":
					return jsonResponse(`{"code":200,"data":{"taskId":"` + preAuditTaskID + `"}}`), nil
				case "/open-apis/contract/v3/review/status":
					return jsonResponse(`{"code":200,"data":{"status":"success"}}`), nil
				case "/open-apis/contract/v3/review/info":
					return jsonResponse(`{"code":200,"data":{"ruleResult":[]}}`), nil
				default:
					return preAuditPreparationRequest(t, r)
				}
			})
			runID := h.create()
			result, err := h.run("start", "--run-id", runID, "--data", `{"config":`+alignmentConfig+`}`)
			final, _ := result["final"].(map[string]any)
			if err == nil || result["workflowStage"] != "review_succeeded" || final["reviewDetailUrl"] != nil || final["ruleResult"] == nil {
				t.Fatal("invalid page must preserve success without delivering a URL", result, err)
			}
		})
	}
	t.Run("current file changed", func(t *testing.T) {
		h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/open-apis/contract/v1/pre_audits/"+preAuditID {
				return jsonResponse(strings.Replace(preAuditCreatedData(), preAuditLatestID, "100", 1)), nil
			}
			return preAuditPreparationRequest(t, r)
		})
		runID := h.create()
		if _, err := h.run("start", "--run-id", runID, "--data", `{"config":`+alignmentConfig+`}`); err == nil {
			t.Fatal("must not start review for a changed CLM current file")
		}
	})
}

func TestPreAuditUpstreamFailuresStopAtTheirBoundary(t *testing.T) {
	for _, kind := range []string{"upload denied", "create denied", "create missing code", "reference missing hash", "reference unknown"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
				calls++
				switch r.URL.Path {
				case "/open-apis/contract/v1/files/upload":
					if kind == "upload denied" {
						return jsonResponse(`{"code":110004,"data":{}}`), nil
					}
					return preAuditUploadResponse(t, r)
				case "/open-apis/contract/v1/pre_audits":
					if kind == "create denied" {
						return jsonResponse(`{"code":110004,"data":{}}`), nil
					}
					if kind == "create missing code" {
						return jsonResponse(strings.Replace(preAuditCreatedData(), `"code":0,`, "", 1)), nil
					}
					return jsonResponse(preAuditCreatedData()), nil
				case "/open-apis/contract/v3/review/file/upload":
					if strings.HasPrefix(kind, "create") {
						t.Fatal("invalid CLM response must stop before AI reference")
					}
					if kind == "reference unknown" {
						return nil, io.ErrUnexpectedEOF
					}
					return jsonResponse(strings.Replace(preAuditReferenceData(true), `"fileHash":"`+strings.Repeat("a", 64)+`",`, "", 1)), nil
				default:
					t.Fatal("preparation failure must not start review", r.URL)
					return nil, nil
				}
			})
			result, err := h.run("prepare", "--file", h.file, "--name", "合同.pdf")
			if kind == "upload denied" {
				if err == nil || calls != 1 {
					t.Fatal(err, calls)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			runID := result["runId"].(string)
			result, err = h.run("create", "--run-id", runID, "--category-id", "2001")
			if err == nil || result["workflowStage"] == "awaiting_config" {
				t.Fatal("failed preparation reported success", result, err)
			}
			before := calls
			if strings.HasPrefix(kind, "reference") {
				if _, err := h.run("resume", "--run-id", runID); err == nil || calls != before {
					t.Fatal("resume must not repeat unknown/unready reference writes", err, calls)
				}
			}
		})
	}
}
