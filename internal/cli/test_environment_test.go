package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
	"cn.qfei/contract-cli/internal/openplatform"
)

/*
legacyNonProductionProfile 构造完整的自定义环境配置，不通过环境标签推断地址。
入参 t（*testing.T）为测试上下文，environment、openOrigin、accountOrigin（string）为环境和端点；返回自洽配置。
*/
func legacyNonProductionProfile(t *testing.T, environment, openOrigin, accountOrigin string) config.Profile {
	t.Helper()
	data, err := json.Marshal(validProductionProfile())
	if err != nil {
		t.Fatal(err)
	}
	// 将完整的生产端点集映射为旧环境，避免只改环境名导致测试误判。
	content := strings.ReplaceAll(string(data), productionOpenPlatformOrigin, openOrigin)
	content = strings.ReplaceAll(content, productionAccountOrigin, accountOrigin)
	var profile config.Profile
	if err := json.Unmarshal([]byte(content), &profile); err != nil {
		t.Fatal(err)
	}
	profile.Name = "contract-" + environment
	profile.Environment = environment
	profile.DefaultIdentity = config.IdentityApp
	profile.Identities.App = config.AppIdentity{AppID: "fixture-app", Token: &config.Token{AccessToken: "fixture-token", Expiry: time.Now().Add(time.Hour)}}
	return profile
}

/*
TestConfigAddRejectsNonProductionBeforeHTTPRequest 验证非生产预设在配置发现前被拒绝且不落盘。
入参 t（*testing.T）为测试上下文；返回值为空，失败时报告测试错误。
*/
func TestConfigAddRejectsNonProductionBeforeHTTPRequest(t *testing.T) {
	for _, environment := range []string{"dev", "blue"} {
		t.Run(environment, func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			requests := 0
			app := New(Options{Store: store, Stdout: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(*http.Request) (*http.Response, error) {
				requests++
				return productionResponse(http.StatusOK, `{}`), nil
			})}})
			profileName := "contract-" + environment
			err := app.Run(context.Background(), []string{"config", "add", "--env", environment, "--name", profileName})
			if err == nil || !strings.Contains(err.Error(), "supported environments: prod") || requests != 0 {
				t.Fatalf("config error=%v requests=%d", err, requests)
			}
			_, found, lookupErr := store.LookupProfile(profileName)
			if lookupErr != nil || found {
				t.Fatalf("rejected profile was saved: found=%v error=%v", found, lookupErr)
			}
		})
	}
}

/*
TestMismatchedConfiguredProfilesRejectedBeforeHTTPRequest 验证与声明授权地址混用的端点在授权和查询前被阻断。
入参 t（*testing.T）为测试上下文；返回值为空，失败时报告测试错误。
*/
func TestMismatchedConfiguredProfilesRejectedBeforeHTTPRequest(t *testing.T) {
	for _, item := range []struct{ environment, openOrigin, accountOrigin, mismatch string }{
		{"dev", "https://dev-open.qtech.cn", "https://dev-myaccount.qtech.cn", "account"},
		{"blue", "https://open-b.qfei.cn", "https://myaccount-b.qfei.cn", "account"},
		{"dev", "https://dev-open.qtech.cn", "https://dev-myaccount.qtech.cn", "resource"},
		{"blue", "https://open-b.qfei.cn", "https://myaccount-b.qfei.cn", "resource"},
	} {
		t.Run(item.environment+"/"+item.mismatch, func(t *testing.T) {
			profile := legacyNonProductionProfile(t, item.environment, item.openOrigin, item.accountOrigin)
			if item.mismatch == "resource" {
				profile.Resource = productionOpenPlatformOrigin
			} else {
				profile.Identities.User.TokenEndpoint = "https://myaccount.qfei.cn/api/public/oauth/token/contract"
			}
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(profile, true); err != nil {
				t.Fatal(err)
			}
			requests := 0
			app := New(Options{
				Store: store, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
				CredentialStore: &deviceMemoryCredentialStore{values: map[string]credential.DeviceCredential{}},
				HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(*http.Request) (*http.Response, error) {
					requests++
					return productionResponse(http.StatusOK, `{}`), nil
				})},
			})
			for _, args := range [][]string{
				{"auth", "login", "--profile", profile.Name, "--as", "user", "--no-open-browser"},
				{"auth", "login", "--profile", profile.Name, "--as", "app", "--app-id", "fixture-app", "--app-secret", "fixture-secret"},
				{"auth", "init", "--profile", profile.Name},
				{"auth", "complete", "--profile", profile.Name},
				{"auth", "status", "--profile", profile.Name, "--as", "user"},
				{"auth", "status", "--profile", profile.Name, "--as", "app"},
				{"auth", "logout", "--profile", profile.Name, "--as", "user"},
				{"auth", "logout", "--profile", profile.Name, "--as", "app"},
				{"auth", "use", "--profile", profile.Name, "--as", "app"},
				{"contract", "get", "fixture-contract", "--profile", profile.Name, "--as", "user"},
				{"contract", "get", "fixture-contract", "--profile", profile.Name, "--as", "app"},
				{"mdm", "fields", "list", "--biz-line", "vendor", "--profile", profile.Name, "--as", "app"},
				{"payment", "get", "fixture-payment", "--contract", "fixture-contract", "--profile", profile.Name, "--as", "app"},
				{"rule", "table", "get", "--profile", profile.Name, "--as", "app", "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "table-1"},
			} {
				err := app.Run(context.Background(), args)
				actual := err
				if isBusinessCommand(args[0]) {
					var traced *openplatform.TraceError
					if !errors.As(err, &traced) || len(traced.TraceID) != 32 {
						t.Fatalf("business command missing trace: %v", err)
					}
					actual = traced.Cause
				}
				if actual == nil || actual.Error() != productionProfileErrorMessage(profile.Name) || requests != 0 {
					t.Fatalf("command %v error=%v requests=%d", args, err, requests)
				}
			}
		})
	}
}

/*
TestConfiguredHTTPSHostsDoNotDependOnStaticEnvironmentBlacklist 验证 HTTPS 目标不被环境域名黑名单代替配置校验。
入参 t（*testing.T）为测试上下文；返回值为空，失败时报告测试错误。
*/
func TestConfiguredHTTPSHostsDoNotDependOnStaticEnvironmentBlacklist(t *testing.T) {
	requests := 0
	app := New(Options{Store: config.NewStore(t.TempDir()), HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return productionResponse(http.StatusOK, `{}`), nil
	})}})
	for _, origin := range []string{"https://dev-open.qtech.cn", "https://dev-myaccount.qtech.cn", "https://open-b.qfei.cn", "https://myaccount-b.qfei.cn"} {
		response, err := app.httpClient.Get(origin)
		if response != nil {
			_ = response.Body.Close()
		}
		if err != nil {
			t.Fatalf("host %s error=%v", origin, err)
		}
	}
	if requests != 4 {
		t.Fatalf("configured HTTPS requests=%d, want 4", requests)
	}
}

/*
TestConfigHelpShowsSupportedEnvironments 验证帮助展示 prod/test 预设，默认 prod。
入参 t（*testing.T）为测试上下文；返回值为空，失败时报告测试错误。
*/
func TestConfigHelpShowsSupportedEnvironments(t *testing.T) {
	output := &bytes.Buffer{}
	app := New(Options{Store: config.NewStore(t.TempDir()), Stdout: output})
	if err := app.Run(context.Background(), []string{"config", "add", "--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "--env <prod|test>") ||
		!strings.Contains(output.String(), "支持 prod/test，默认 prod") ||
		!strings.Contains(output.String(), "--env test --name contract-test") ||
		strings.Contains(output.String(), "--env dev") ||
		strings.Contains(output.String(), "--env blue") {
		t.Fatalf("unexpected environment help: %s", output.String())
	}
}

/*
TestCrossTargetDeviceStateRejected 验证另一目标的快照或待授权端点不能与当前声明地址混用。
入参 t（*testing.T）为测试上下文；无返回值，失败时报告测试错误。
*/
func TestCrossTargetDeviceStateRejected(t *testing.T) {
	blueProfile := legacyNonProductionProfile(t, "blue", "https://open-b.qfei.cn", "https://myaccount-b.qfei.cn")
	blueProfile.Name = "contract"
	prodProfile := validProductionProfile()
	tests := []struct {
		name   string
		stored credential.DeviceCredential
	}{
		{name: "snapshot", stored: credential.DeviceCredential{DeviceProfile: snapshotDeviceProfile(blueProfile)}},
		{name: "pending token endpoint", stored: credential.DeviceCredential{
			DeviceProfile: snapshotDeviceProfile(prodProfile),
			Pending: &credential.PendingTransaction{
				Status: credential.PendingStatusPending, DeviceCode: "fixture-device",
				TokenEndpoint: "https://myaccount-b.qfei.cn/api/public/oauth/token/contract",
				ClientID:      "zscli_892efdadc11a3f53", ExpiresAt: time.Now().Add(time.Minute),
			},
		}},
		{name: "pending verification uri", stored: credential.DeviceCredential{
			DeviceProfile: snapshotDeviceProfile(prodProfile),
			Pending: &credential.PendingTransaction{
				Status: credential.PendingStatusPending, DeviceCode: "fixture-device",
				TokenEndpoint:           "https://myaccount.qfei.cn/api/public/oauth/token/contract",
				VerificationURIComplete: "https://myaccount-b.qfei.cn/device?user_code=fixture",
				ClientID:                "zscli_892efdadc11a3f53", ExpiresAt: time.Now().Add(time.Minute),
			},
		}},
	}
	for _, test := range tests {
		for _, withProfile := range []bool{false, true} {
			// 无现存 profile 时，自洽的完整快照允许恢复，由单独用例覆盖。
			if test.name == "snapshot" && !withProfile {
				continue
			}
			mode := "recovery"
			if withProfile {
				mode = "existing prod profile"
			}
			t.Run(test.name+"/"+mode, func(t *testing.T) {
				store := config.NewStore(t.TempDir())
				if withProfile {
					if err := store.UpsertProfile(prodProfile, true); err != nil {
						t.Fatal(err)
					}
				}
				// 两种配置来源共享相同的历史凭据，校验失败不得写回快照或发送业务请求。
				workspace := t.TempDir()
				requests := 0
				app := New(Options{
					Store: store, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
					CredentialStore: &deviceMemoryCredentialStore{values: map[string]credential.DeviceCredential{"contract": test.stored}},
					LookupEnv: func(name string) (string, bool) {
						return workspace, name == "SKILL_SESSION_WORKSPACE"
					},
					HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(*http.Request) (*http.Response, error) {
						requests++
						return productionResponse(http.StatusOK, "{}"), nil
					})},
				})
				for _, args := range [][]string{
					{"auth", "init", "--profile", "contract"},
					{"auth", "complete", "--profile", "contract"},
					{"auth", "status", "--profile", "contract", "--as", "user"},
					{"auth", "logout", "--profile", "contract", "--as", "user"},
					{"contract", "template", "list", "--profile", "contract", "--as", "user"},
				} {
					err := app.Run(context.Background(), args)
					actual := err
					var traced *openplatform.TraceError
					if errors.As(err, &traced) {
						actual = traced.Cause
					}
					if actual == nil || actual.Error() != productionProfileErrorMessage("contract") || requests != 0 {
						t.Fatalf("command %v error=%v requests=%d", args, err, requests)
					}
				}
				if _, found, err := store.LookupProfile("contract"); err != nil || found != withProfile {
					t.Fatalf("rejected snapshot changed profile presence: found=%v error=%v", found, err)
				}
			})
		}
	}
}

/*
TestProductionRedirectToBlueBlocked 验证生产请求跳转到蓝环境时不会向目标域名发送请求。
入参 t（*testing.T）为测试上下文；无返回值，失败时报告测试错误。
*/
func TestProductionRedirectToBlueBlocked(t *testing.T) {
	for _, origin := range []string{"https://open-b.qfei.cn", "https://myaccount-b.qfei.cn"} {
		t.Run(origin, func(t *testing.T) {
			requests := 0
			app := New(Options{Store: config.NewStore(t.TempDir()), HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(*http.Request) (*http.Response, error) {
				requests++
				response := productionResponse(http.StatusTemporaryRedirect, "")
				response.Header.Set("Location", origin+"/finish")
				return response, nil
			})}})
			response, err := app.httpClient.Get(productionOpenPlatformOrigin + "/start")
			if response != nil {
				_ = response.Body.Close()
			}
			if err == nil || !strings.Contains(err.Error(), "different origin") || requests != 1 {
				t.Fatalf("redirect error=%v requests=%d", err, requests)
			}
		})
	}
}
