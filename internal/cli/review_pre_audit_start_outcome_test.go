package cli_test

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPreAuditAmbiguousStartsNeverAllowAnotherPOST(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"timeout", 408, `{"code":408}`},
		{"unknown business", 200, `{"code":999999,"data":{}}`},
		{"CLM code is not AI rejection proof", 200, `{"code":110301,"data":{}}`},
		{"error with task", 400, `{"code":400,"data":{"taskId":"` + preAuditTaskID + `"}}`},
		{"business error with task", 200, `{"code":999999,"data":{"taskId":"` + preAuditTaskID + `"}}`},
		{"missing code with task", 200, `{"data":{"taskId":"` + preAuditTaskID + `"}}`},
		{"server failure", 500, `{"code":500}`},
		{"invalid response", 200, `invalid`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			starts := 0
			h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/open-apis/contract/v3/review/startReview" {
					starts++
					return preAuditCreateResponse(tc.status, tc.body)
				}
				if r.URL.Path == "/open-apis/contract/v1/pre_audits/"+preAuditID {
					return jsonResponse(preAuditCreatedData()), nil
				}
				return preAuditPreparationRequest(t, r)
			})
			runID := h.create()
			result, err := h.run("start", "--run-id", runID, "--data", `{"config":`+alignmentConfig+`}`)
			if err == nil || result["workflowStage"] != "start_unknown" {
				t.Fatalf("ambiguous start released: %v %v", result, err)
			}
			if strings.Contains(tc.name, "with task") {
				record := preAuditStoredRecord(t, h, runID)
				start, _ := record["start"].(map[string]any)
				if record["taskId"] != nil || start["candidateTaskId"] != preAuditTaskID {
					t.Fatalf("candidate evidence lost or treated as success: %v", record)
				}
			}
			for _, command := range []string{"start", "resume"} {
				args := []string{command, "--run-id", runID}
				if command == "start" {
					args = append(args, "--data", `{"config":`+alignmentConfig+`}`)
				}
				if _, err := h.run(args...); err == nil || starts != 1 {
					t.Fatalf("unknown start replayed: err=%v starts=%d", err, starts)
				}
			}
		})
	}
}

func TestPreAuditPrepareHonorsUploadDeadline(t *testing.T) {
	for _, duration := range []string{"2m", "1s", ""} {
		t.Run("deadline-"+duration, func(t *testing.T) {
			h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
				deadline, ok := r.Context().Deadline()
				remaining := time.Until(deadline)
				if !ok || (duration != "1s" && remaining < time.Minute) || (duration == "1s" && remaining > time.Second) {
					t.Fatalf("wrong deadline: %v %v", ok, remaining)
				}
				return preAuditUploadResponse(t, r)
			})
			args := []string{"prepare", "--file", h.file, "--name", "合同.pdf"}
			if duration != "" {
				args = append(args, "--deadline", duration)
			}
			if _, err := h.run(args...); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPreAuditLegacyStartRejectionCannotReplay(t *testing.T) {
	calls := 0
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) { calls++; return preAuditPreparationRequest(t, r) })
	runID := h.create()
	h.rewriteRecord(runID, func(record map[string]any) { record["workflowStage"] = "start_rejected" })
	before := calls
	if _, err := h.run("start", "--run-id", runID, "--data", `{"config":`+alignmentConfig+`}`); err == nil || calls != before {
		t.Fatalf("legacy rejection replayed: %v %d", err, calls)
	}
}

func TestPreAuditConfirmedRejectionAllowsExplicitStart(t *testing.T) {
	starts := 0
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/open-apis/contract/v3/review/startReview":
			starts++
			if starts == 1 {
				return preAuditCreateResponse(422, `{"code":422,"msg":"参数错误","data":["config不能为空"]}`)
			}
			return jsonResponse(`{"code":200,"data":{"taskId":"` + preAuditTaskID + `"}}`), nil
		case "/open-apis/contract/v1/pre_audits/" + preAuditID:
			return jsonResponse(preAuditCreatedData()), nil
		case "/open-apis/contract/v3/review/status":
			return jsonResponse(`{"code":200,"data":{"status":"running"}}`), nil
		default:
			return preAuditPreparationRequest(t, r)
		}
	})
	runID := h.create()
	result, err := h.run("start", "--run-id", runID, "--data", `{"config":`+alignmentConfig+`}`)
	if err == nil || result["workflowStage"] != "start_rejected_confirmed" || starts != 1 {
		t.Fatalf("rejection not confirmed: %v %v %d", result, err, starts)
	}
	result, err = h.run("start", "--run-id", runID, "--data", `{"config":`+alignmentConfig+`}`, "--deadline", "500ms", "--interval", "1s")
	if err == nil || starts != 2 || result["taskId"] != preAuditTaskID || result["workflowStage"] != "running" {
		t.Fatalf("explicit start failed: %v %v %d", result, err, starts)
	}
}

func TestPreAuditUploadTimeoutKeepsUnknownWithoutReplay(t *testing.T) {
	calls := 0
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if _, err := preAuditUploadResponse(t, r); err != nil {
			t.Fatal(err)
		}
		return nil, context.DeadlineExceeded
	})
	result, err := h.run("prepare", "--file", h.file, "--name", "合同.pdf", "--deadline", "2m")
	if err == nil || calls != 1 || result["workflowStage"] != "upload_unknown" {
		t.Fatalf("sent upload lost unknown state: %v %v calls=%d", result, err, calls)
	}
	runID, _ := result["runId"].(string)
	if _, err := h.run("resume", "--run-id", runID); err == nil || calls != 1 {
		t.Fatalf("unknown upload replayed: %v calls=%d", err, calls)
	}
}

func TestPreAuditPrepareCannotExtendCallerDeadline(t *testing.T) {
	calls := 0
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		calls++
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) > time.Minute {
			t.Fatal("prepare extended caller deadline")
		}
		return preAuditUploadResponse(t, r)
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	err := h.app.Run(ctx, []string{"review", "pre-audit", "prepare", "--file", h.file, "--name", "合同.pdf", "--deadline", "2m"})
	if err != nil || calls != 1 {
		t.Fatalf("prepare failed: %v calls=%d", err, calls)
	}
}

func TestPreAuditInvalidDeadlineDoesNotUploadOrCreateRecord(t *testing.T) {
	for _, deadline := range []string{"invalid", "0s", "-1s"} {
		t.Run(deadline, func(t *testing.T) {
			h := newPreAuditHarness(t, nil)
			if _, err := h.run("prepare", "--file", h.file, "--name", "合同.pdf", "--deadline", deadline); err == nil {
				t.Fatal("invalid deadline accepted")
			}
			files, err := filepath.Glob(h.recordPath("*"))
			if err != nil || len(files) != 0 {
				t.Fatalf("invalid deadline left records: %v %v", files, err)
			}
		})
	}
}
