package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
)

func preAuditTestJWT(user string) string {
	return "eyJhbGciOiJSUzI1NiJ9." + base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"common-organization-v2","userInfo":{"tenantId":896684654038353011,"userId":`+user+`}}`)) + ".test"
}

func TestPreAuditIdentityGuardCoversTokenPreparationAndRefresh(t *testing.T) {
	for _, mode := range []string{"same account", "changed during refresh", "changed before request", "dry run never refreshes"} {
		t.Run(mode, func(t *testing.T) {
			creates, references, refreshes := 0, 0, 0
			oldToken := preAuditTestJWT("1002718663263913046")
			newToken := oldToken + "-refreshed"
			if strings.HasPrefix(mode, "changed") {
				newToken = preAuditTestJWT("1002718663263913047")
			}
			app, credentials, _ := newDeviceTokenTestApp(t, &config.Token{AccessToken: oldToken, RefreshToken: "refresh-one", Expiry: deviceTokenTestNow().Add(time.Hour)}, func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case "/open-apis/contract/v1/files/upload":
					_, _ = io.Copy(io.Discard, r.Body)
					return deviceTokenTestResponse(200, `{"code":0,"data":{"file_id":"11"}}`), nil
				case "/api/public/oauth/token/contract":
					refreshes++
					return deviceTokenTestResponse(200, `{"access_token":"`+newToken+`","refresh_token":"refresh-two","token_type":"Bearer","expires_in":3600}`), nil
				case "/open-apis/contract/v1/pre_audits":
					creates++
					if creates == 1 && mode != "changed before request" && mode != "dry run never refreshes" {
						response := deviceTokenTestResponse(http.StatusUnauthorized, `{"data":{"error_type":"token_expired"}}`)
						response.Header.Set("X-Qfei-Open-Platform-Auth-Error", "token_expired")
						return response, nil
					}
					if r.Header.Get("Authorization") != "Bearer "+newToken {
						t.Fatal("request did not use the same-account refreshed token")
					}
					return deviceTokenTestResponse(200, `{"code":0,"data":{"pre_audit_id":"22","source_file_id":"11","latest_file_id":"11","category_id":"33","business_type_code":24}}`), nil
				case "/open-apis/contract/v3/review/file/upload":
					references++
					return deviceTokenTestResponse(200, `{"code":200,"data":{"fileId":"44","fileHash":"`+strings.Repeat("a", 64)+`","snapshotReady":true}}`), nil
				default:
					t.Fatal("unexpected request", r.URL.Path)
					return nil, nil
				}
			})
			out := new(bytes.Buffer)
			app.stdout = out
			path := filepath.Join(t.TempDir(), "contract.pdf")
			if err := os.WriteFile(path, []byte("contract"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := app.Run(context.Background(), []string{"review", "pre-audit", "prepare", "--file", path, "--name", "合同.pdf"}); err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if mode == "changed before request" || mode == "dry run never refreshes" {
				stored := credentials.values["contract"]
				stored.Token.Expiry = deviceTokenTestNow().Add(time.Minute)
				credentials.values["contract"] = stored
			}
			args := []string{"review", "pre-audit", "create", "--run-id", result["runId"].(string), "--category-id", "33"}
			if mode == "dry run never refreshes" {
				args = append(args, "--dry-run")
			}
			err := app.Run(context.Background(), args)
			switch mode {
			case "same account":
				if err != nil || creates != 2 || references != 1 || refreshes != 1 {
					t.Fatal(err, creates, references, refreshes)
				}
			case "changed during refresh":
				if err == nil || creates != 1 || references != 0 || refreshes != 1 {
					t.Fatal("changed account was retried", err, creates, references, refreshes)
				}
			case "changed before request":
				if err == nil || creates != 0 || references != 0 || refreshes != 1 {
					t.Fatal("changed prepared account reached a business write", err, creates, references, refreshes)
				}
			case "dry run never refreshes":
				if err != nil || creates != 0 || references != 0 || refreshes != 0 {
					t.Fatal("dry run performed I/O", err, creates, references, refreshes)
				}
			}
		})
	}
}
