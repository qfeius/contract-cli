package cli_test

import (
	"bytes"
	"net/http"
	"os"
	"testing"

	"cn.qfei/contract-cli/internal/config"
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
