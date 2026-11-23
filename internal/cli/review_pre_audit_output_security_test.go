package cli_test

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
)

func TestPreAuditOutputRequiresOriginalIdentity(t *testing.T) {
	for _, mode := range []string{"execute", "--dry-run", "--print-input"} {
		for _, identity := range []string{"same", "tenant", "user", "logout", "profile"} {
			t.Run(mode+"/"+identity, func(t *testing.T) {
				calls := 0
				h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) { calls++; return preAuditUploadResponse(t, r) })
				runID := h.prepare()
				h.rewriteRecord(runID, func(r map[string]any) {
					r["subjects"] = map[string]any{"private": "PRIVATE_SUBJECT"}
					r["final"] = map[string]any{"private": "PRIVATE_REVIEW"}
				})
				before, err := os.ReadFile(h.recordPath(runID))
				if err != nil {
					t.Fatal(err)
				}
				profile, _ := h.store.GetProfile("contract")
				switch identity {
				case "tenant":
					profile.Identities.User.Token.AccessToken = preAuditToken("896684654038353012", "1002718663263913046")
				case "user":
					profile.Identities.User.Token.AccessToken = preAuditToken("896684654038353011", "1002718663263913047")
				case "logout":
					profile.Identities.User.Token = nil
					profile.Identities.User.AuthMode = config.UserAuthModeAuthorizationCode
				case "profile":
					profile.Name = "another"
				}
				if err := h.store.UpsertProfile(profile, true); err != nil {
					t.Fatal(err)
				}
				args := []string{"create", "--run-id", runID, "--category-id", "2001", "--output", "json"}
				if mode != "execute" {
					args = append(args, mode)
				}
				if identity == "same" && mode == "execute" {
					return
				}
				result, runErr := h.run(args...)
				if identity == "same" {
					if runErr != nil || result["subjects"] == nil {
						t.Fatalf("authorized preview failed: %v %v", result, runErr)
					}
				} else if runErr == nil || result["subjects"] != nil || result["final"] != nil || result["sourceFileId"] != nil || bytes.Contains(h.out.Bytes(), []byte("PRIVATE_")) {
					t.Fatalf("identity rejection exposed recovery data: %v %v", result, runErr)
				}
				after, err := os.ReadFile(h.recordPath(runID))
				if err != nil || !bytes.Equal(before, after) || calls != 1 {
					t.Fatalf("preview/rejection changed record or sent HTTP: %v calls=%d", err, calls)
				}
			})
		}
	}
}

func TestPreAuditOutputSuppressedOnServerAuthorizationFailure(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/open-apis/contract/v1/pre_audits" {
				response := jsonResponse(`{"code":401,"msg":"authorization rejected"}`)
				response.StatusCode = status
				return response, nil
			}
			return preAuditUploadResponse(t, r)
		})
		runID := h.prepare()
		h.rewriteRecord(runID, func(r map[string]any) { r["subjects"] = map[string]any{"private": "PRIVATE_SUBJECT"} })
		result, err := h.run("create", "--run-id", runID, "--category-id", "2001", "--output", "json")
		if err == nil || result["subjects"] != nil || result["sourceFileId"] != nil || bytes.Contains(h.out.Bytes(), []byte("PRIVATE_")) {
			t.Fatalf("HTTP %d leaked recovery output: %v %v", status, result, err)
		}
	}
}

func TestPreAuditAuthorizationFailureSurvivesStateSaveFailure(t *testing.T) {
	for _, phase := range []string{"rejected start", "candidate start", "poll"} {
		for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
			for _, failSave := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/status-%d/save-failure-%v", phase, status, failSave), func(t *testing.T) {
					var h *preAuditHarness
					var runID string
					starts, rejections := 0, 0
					h = newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
						switch r.URL.Path {
						case "/open-apis/contract/v1/pre_audits/" + preAuditID:
							return jsonResponse(preAuditCreatedData()), nil
						case "/open-apis/contract/v3/review/startReview":
							starts++
							if phase == "poll" {
								return jsonResponse(`{"code":200,"data":{"taskId":"` + preAuditTaskID + `"}}`), nil
							}
						case "/open-apis/contract/v3/review/status":
							if phase != "poll" {
								t.Fatal("rejected start must not poll")
							}
						default:
							return preAuditPreparationRequest(t, r)
						}
						rejections++
						if failSave {
							path := h.recordPath(runID)
							if err := os.Rename(path, path+".backup"); err != nil {
								t.Fatal(err)
							}
							if err := os.Mkdir(path, 0700); err != nil {
								t.Fatal(err)
							}
						}
						body := `{"detail":"Not authenticated"}`
						if phase == "candidate start" {
							body = fmt.Sprintf(`{"code":%d,"msg":"authorization rejected","data":{"taskId":"%s"}}`, status, preAuditTaskID)
						}
						response := jsonResponse(body)
						response.StatusCode = status
						return response, nil
					})
					runID = h.create()
					h.rewriteRecord(runID, func(record map[string]any) {
						record["subjects"] = map[string]any{"private": "PRIVATE_AUTH_RECOVERY"}
					})
					result, err := h.run("start", "--run-id", runID, "--data", `{"config":`+alignmentConfig+`}`, "--output", "json")
					if err == nil || starts != 1 || rejections != 1 {
						t.Fatalf("expected one rejected request without replay: starts=%d rejections=%d err=%v", starts, rejections, err)
					}
					var statusErr *openplatform.HTTPStatusError
					if !errors.As(err, &statusErr) || statusErr.StatusCode != status {
						t.Errorf("original HTTP %d lost from error chain: %v", status, err)
					}
					if failSave && !strings.Contains(err.Error(), "保存失败") {
						t.Errorf("state save failure lost from error: %v", err)
					}
					if result["subjects"] != nil || result["sourceFileId"] != nil || bytes.Contains(h.out.Bytes(), []byte("PRIVATE_AUTH_RECOVERY")) || h.out.Len() != 0 {
						t.Errorf("HTTP %d exposed recovery output after rejection", status)
					}
				})
			}
		}
	}
}
