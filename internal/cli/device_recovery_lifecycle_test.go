package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
	"cn.qfei/contract-cli/internal/invocation"
)

func TestAgentDeviceRecoveryLifecycle(t *testing.T) {
	for _, marker := range []string{"SKILL_SESSION_WORKSPACE", "SESSION_ID", "CODEBUDDY_SESSION_ID"} {
		for _, recreate := range []bool{false, true} {
			for _, operation := range []string{"business", "status", "logout"} {
				name := marker + "/empty-mode/" + operation
				if recreate {
					name = marker + "/recreated/" + operation
				}
				t.Run(name, func(t *testing.T) {
					t.Setenv("HOME", t.TempDir())
					t.Setenv("LOCALAPPDATA", t.TempDir())
					revocations, business := 0, 0
					app, credentials, _ := newDeviceTokenTestApp(t, &config.Token{
						AccessToken: "retained-access", RefreshToken: "retained-refresh", Expiry: deviceTokenTestNow().Add(time.Hour),
					}, func(r *http.Request) (*http.Response, error) {
						switch r.URL.Path {
						case "/.well-known/oauth-authorization-server/contract":
							return jsonResponse(`{"authorization_endpoint":"https://myaccount.qfei.cn/api/public/oauth/authorize/contract","token_endpoint":"https://myaccount.qfei.cn/api/public/oauth/token/contract","device_authorization_endpoint":"https://myaccount.qfei.cn/api/public/oauth/device-authorization/contract","revocation_endpoint":"https://myaccount.qfei.cn/api/public/oauth/revoke/contract","registration_endpoint":"https://myaccount.qfei.cn/api/public/oauth/register/contract"}`), nil
						case "/api/public/oauth/revoke/contract":
							revocations++
							if err := r.ParseForm(); err != nil || r.Form.Get("token") != "retained-refresh" {
								t.Fatalf("wrong revocation request: %v", err)
							}
						case "/open-apis/contract/v1/mcp/templates":
							business++
							if r.Header.Get("Authorization") != "Bearer retained-access" {
								t.Fatal("retained credential not used")
							}
						default:
							t.Fatalf("unexpected request: %s", r.URL.Path)
						}
						return jsonResponse(`{"code":0,"data":[]}`), nil
					})
					workspace := t.TempDir()
					app.lookupEnv = func(key string) (string, bool) {
						if key == marker {
							return workspace, true
						}
						return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK"
					}
					app.runtimeEvidence = &invocation.Result{}
					app.secrets = config.NewSecretsStore(t.TempDir())
					stdout := &bytes.Buffer{}
					app.stdout, app.stderr = stdout, &bytes.Buffer{}
					profile := mustDeviceProfile(t, app.store)
					profile.ProtectedResourceMetadataURL = ""
					profile.Identities.User.AuthMode = ""
					stored := credentials.values["contract"]
					stored.DeviceProfile = snapshotDeviceProfile(profile)
					credentials.values["contract"] = stored
					credentials.values["independent"] = stored
					if err := app.store.SaveProfile(profile); err != nil {
						t.Fatal(err)
					}
					if recreate {
						if err := os.Remove(app.store.Path()); err != nil {
							t.Fatal(err)
						}
						if err := app.Run(context.Background(), []string{"config", "add", "--env", "prod"}); err != nil {
							t.Fatal(err)
						}
					}
					stdout.Reset()
					args := []string{"auth", operation, "--profile", "contract"}
					if operation == "business" {
						args = []string{"contract", "template", "list", "--as", "user", "--profile", "contract", "--output", "json"}
					}
					if err := app.Run(context.Background(), args); err != nil {
						t.Fatal(err)
					}
					switch operation {
					case "business":
						if business != 1 {
							t.Fatal("business request not sent")
						}
					case "status":
						if !strings.Contains(stdout.String(), "Authorization: authorized") {
							t.Fatalf("incorrect status: %s", stdout)
						}
					case "logout":
						if _, err := credentials.Load("contract"); !errors.Is(err, credential.ErrCredentialNotFound) || revocations != 1 {
							t.Fatalf("logout skipped retained credential: revocations=%d error=%v", revocations, err)
						}
					}
					if _, err := credentials.Load("independent"); err != nil {
						t.Fatal("independent credential changed")
					}
				})
			}
		}
	}
}

func TestAgentDeviceRecoveryDoesNotReplaceLegacyOrMismatchedIdentity(t *testing.T) {
	for _, scenario := range []string{"legacy token", "legacy mode", "missing credential", "mismatched snapshot", "pending", "expired"} {
		t.Run(scenario, func(t *testing.T) {
			app, credentials, _ := newDeviceTokenTestApp(t, &config.Token{AccessToken: "retained-access"}, func(*http.Request) (*http.Response, error) {
				t.Fatal("identity recovery must not authorize or send business requests")
				return nil, nil
			})
			profile := mustDeviceProfile(t, app.store)
			profile.Identities.User.AuthMode = ""
			stored := credentials.values["contract"]
			wantMode, wantStatus := "", ""
			switch scenario {
			case "legacy token":
				profile.Identities.User.Token = &config.Token{AccessToken: "legacy-access"}
			case "legacy mode":
				profile.Identities.User.AuthMode = config.UserAuthModeAuthorizationCode
				wantMode = config.UserAuthModeAuthorizationCode
			case "missing credential":
				delete(credentials.values, "contract")
			case "mismatched snapshot":
				stored.DeviceProfile.Resource = "https://other.example.test"
			case "pending":
				stored.Token = nil
				stored.Pending = &credential.PendingTransaction{Status: credential.PendingStatusPending,
					TokenEndpoint: profile.Identities.User.TokenEndpoint, ClientID: profile.Identities.User.DeviceClientID,
					VerificationURIComplete: "https://myaccount.qfei.cn/device?code=fixture", ExpiresAt: deviceTokenTestNow().Add(time.Hour)}
				wantMode, wantStatus = config.UserAuthModeDevice, "pending"
			case "expired":
				stored.Token.Expiry = deviceTokenTestNow().Add(-time.Hour)
				wantMode, wantStatus = config.UserAuthModeDevice, "expired"
			}
			if scenario != "missing credential" {
				credentials.values["contract"] = stored
			}
			if err := app.store.SaveProfile(profile); err != nil {
				t.Fatal(err)
			}
			loaded, err := app.loadDeviceAwareProfile("contract")
			if scenario == "mismatched snapshot" {
				if err == nil {
					t.Fatal("mismatched credential restored")
				}
				return
			}
			if err != nil || loaded.Identities.User.AuthMode != wantMode {
				t.Fatalf("unexpected recovery: mode=%s error=%v", loaded.Identities.User.AuthMode, err)
			}
			if scenario == "legacy token" && (loaded.Identities.User.Token == nil || loaded.Identities.User.Token.AccessToken != "legacy-access") {
				t.Fatal("legacy token replaced")
			}
			if wantStatus != "" {
				status, err := app.deviceAuthStatus(loaded)
				if err != nil || status.Authorization != wantStatus {
					t.Fatalf("incorrect restored status: %+v %v", status, err)
				}
			}
		})
	}
}
