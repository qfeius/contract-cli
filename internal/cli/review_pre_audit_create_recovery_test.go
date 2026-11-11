package cli_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func preAuditCreateResponse(status int, body string) (*http.Response, error) {
	response := jsonResponse(body)
	response.StatusCode = status
	return response, nil
}

func preAuditStoredRecord(t *testing.T, h *preAuditHarness, runID string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(h.recordPath(runID))
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(body, &record); err != nil {
		t.Fatal(err)
	}
	return record
}

func TestPreAuditCreateKnownRejectionAllowsCategoryReselection(t *testing.T) {
	type rejectionCase struct {
		name   string
		status int
		body   string
	}
	var cases []rejectionCase
	for _, code := range []int{110000, 110004, 110301, 110413, 110903, 117008} {
		cases = append(cases, rejectionCase{fmt.Sprintf("business-%d", code), 200, fmt.Sprintf(`{"code":%d,"data":{}}`, code)})
	}
	for _, status := range []int{400, 401, 403, 404, 405, 413, 415, 422, 429} {
		cases = append(cases, rejectionCase{fmt.Sprintf("http-%d", status), status, `{"detail":"rejected"}`})
	}
	for _, tc := range cases {
		for _, category := range []string{"2001", "2002"} {
			t.Run(tc.name+"-category-"+category, func(t *testing.T) {
				uploads, creates, references := 0, 0, 0
				h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
					switch r.URL.Path {
					case "/open-apis/contract/v1/files/upload":
						uploads++
						return preAuditUploadResponse(t, r)
					case "/open-apis/contract/v1/pre_audits":
						creates++
						body := alignmentBody(t, r)
						if creates == 1 {
							return preAuditCreateResponse(tc.status, tc.body)
						}
						if body["source_file_id"] != preAuditSourceID || body["category_id"] != category || body["request_id"] != nil {
							t.Fatal("reselection must reuse the uploaded file without an idempotency key", body)
						}
						return jsonResponse(strings.ReplaceAll(preAuditCreatedData(), `"2001"`, `"`+category+`"`)), nil
					case "/open-apis/contract/v3/review/file/upload":
						references++
						return jsonResponse(preAuditReferenceData(true)), nil
					default:
						t.Fatal("unexpected request", r.URL.Path)
						return nil, nil
					}
				})
				runID := h.prepare()
				result, err := h.run("create", "--run-id", runID, "--category-id", "2001")
				if err == nil || result["workflowStage"] != "awaiting_category" || result["sourceFileId"] != preAuditSourceID || result["categoryId"] != nil || result["preAuditId"] != nil || references != 0 {
					t.Fatal("known rejection must reopen category selection and preserve the file", result, err)
				}
				record := preAuditStoredRecord(t, h, runID)
				if record["workflowStage"] != "awaiting_category" || record["categoryId"] != nil || record["requestId"] != nil {
					t.Fatal("failed category and creation key must be durably cleared", record)
				}
				if _, err := h.run("resume", "--run-id", runID); err == nil || creates != 1 {
					t.Fatal("resume must wait for the user's category selection", err, creates)
				}
				if _, err := h.run("create", "--run-id", runID, "--category-id", category, "--dry-run"); err != nil || creates != 1 {
					t.Fatal("preview must permit reselection without creating", err, creates)
				}
				result, err = h.run("create", "--run-id", runID, "--category-id", category)
				if err != nil || result["workflowStage"] != "awaiting_config" || result["categoryId"] != category || uploads != 1 || creates != 2 || references != 1 {
					t.Fatal("explicit reselection did not continue the same run", result, err, uploads, creates, references)
				}
				if _, err := h.run("create", "--run-id", runID, "--category-id", "2003"); err == nil || creates != 2 {
					t.Fatal("a successfully created preaudit must keep its category", err, creates)
				}
			})
		}
	}
}

func TestPreAuditCreateUncertainOutcomeStopsWithoutRecovery(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		lost   bool
	}{
		{"response lost", 0, "", true},
		{"request timeout", 408, `{"code":110301}`, false},
		{"conflict", 409, `{"code":409}`, false},
		{"server error", 500, `{"code":110004}`, false},
		{"generic error", 200, `{"code":110002}`, false},
		{"unknown business error", 200, `{"code":999999}`, false},
		{"missing code", 200, `{"data":{}}`, false},
		{"invalid response", 200, "invalid json", false},
		{"missing identity", 200, `{"code":0,"data":{}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			creates, references := 0, 0
			h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case "/open-apis/contract/v1/files/upload":
					return preAuditUploadResponse(t, r)
				case "/open-apis/contract/v1/pre_audits":
					creates++
					if tc.lost {
						return nil, io.ErrUnexpectedEOF
					}
					return preAuditCreateResponse(tc.status, tc.body)
				case "/open-apis/contract/v3/review/file/upload":
					references++
					return jsonResponse(preAuditReferenceData(true)), nil
				default:
					t.Fatal(r.URL.Path)
					return nil, nil
				}
			})
			runID := h.prepare()
			result, err := h.run("create", "--run-id", runID, "--category-id", "2001")
			if err == nil || result["workflowStage"] != "create_pending" || references != 0 {
				t.Fatal("unknown outcome must preserve the run and stop", result, err)
			}
			record := preAuditStoredRecord(t, h, runID)
			if record["requestId"] != nil || record["categoryId"] != "2001" {
				t.Fatal("new run must retain category without an idempotency key", record)
			}
			for _, args := range [][]string{
				{"create", "--run-id", runID, "--category-id", "2001"},
				{"create", "--run-id", runID, "--category-id", "2002"},
				{"resume", "--run-id", runID},
			} {
				if _, err := h.run(args...); err == nil || creates != 1 || references != 0 {
					t.Fatal("unknown creation must stop before another POST", args, err, creates, references)
				}
			}
		})
	}
}

func TestPreAuditLegacyUnknownCreateDoesNotReplay(t *testing.T) {
	creates := 0
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/open-apis/contract/v1/pre_audits" {
			creates++
			return nil, io.ErrUnexpectedEOF
		}
		return preAuditPreparationRequest(t, r)
	})
	runID := h.prepare()
	h.rewriteRecord(runID, func(record map[string]any) {
		record["workflowStage"] = "create_pending"
		record["categoryId"] = "2001"
		record["requestId"] = "00112233445566778899aabbccddeeff"
	})
	for _, args := range [][]string{
		{"resume", "--run-id", runID},
		{"create", "--run-id", runID, "--category-id", "2001"},
	} {
		if _, err := h.run(args...); err == nil || creates != 0 {
			t.Fatal("legacy creation key must never enable replay", err, creates)
		}
	}
}

func TestPreAuditCreateRejectedStateSaveFailureKeepsOriginalBinding(t *testing.T) {
	var h *preAuditHarness
	var runID string
	creates := 0
	h = newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/open-apis/contract/v1/pre_audits" {
			return preAuditPreparationRequest(t, r)
		}
		creates++
		path := h.recordPath(runID)
		if err := os.Rename(path, path+".backup"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		return jsonResponse(`{"code":110301,"data":{}}`), nil
	})
	runID = h.prepare()
	result, err := h.run("create", "--run-id", runID, "--category-id", "2001")
	if err == nil || !strings.Contains(err.Error(), "保存失败") || result["workflowStage"] != "create_pending" || result["categoryId"] != "2001" || creates != 1 {
		t.Fatal("failed persistence must keep the original creation binding and stop", result, err, creates)
	}
	path := h.recordPath(runID)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".backup", path); err != nil {
		t.Fatal(err)
	}
	if record := preAuditStoredRecord(t, h, runID); record["requestId"] != nil || record["categoryId"] != "2001" {
		t.Fatal("failed replacement lost the original record", record)
	}
	if _, err := h.run("create", "--run-id", runID, "--category-id", "2002"); err == nil || creates != 1 {
		t.Fatal("failed state persistence must not allow a new creation", err, creates)
	}
}
