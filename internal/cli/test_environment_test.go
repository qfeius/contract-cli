package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
)

/*
legacyNonProductionProfile 构造历史 test/blue 配置，确保旧端点与环境字段完整一致。
入参 t（*testing.T）为测试上下文，environment、openOrigin、accountOrigin（string）为旧环境和端点；返回 config.Profile 为待拒绝配置。
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
TestConfigAddRejectsTestAndBlueBeforeHTTPRequest 验证非生产预设在配置发现前被拒绝且不落盘。
入参 t（*testing.T）为测试上下文；返回值为空，失败时报告测试错误。
*/
func TestConfigAddRejectsTestAndBlueBeforeHTTPRequest(t *testing.T) {
	for _, environment := range []string{"test", "blue"} {
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
TestExistingTestAndBlueProfilesRejectedBeforeHTTPRequest 验证旧非生产配置在授权和矩阵查询前被阻断。
入参 t（*testing.T）为测试上下文；返回值为空，失败时报告测试错误。
*/
func TestExistingTestAndBlueProfilesRejectedBeforeHTTPRequest(t *testing.T) {
	for _, item := range []struct{ environment, openOrigin, accountOrigin string }{
		{"test", "https://test-open.qtech.cn", "https://test-myaccount.qtech.cn"},
		{"blue", "https://open-b.qfei.cn", "https://myaccount-b.qfei.cn"},
	} {
		t.Run(item.environment, func(t *testing.T) {
			profile := legacyNonProductionProfile(t, item.environment, item.openOrigin, item.accountOrigin)
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
				{"auth", "status", "--profile", profile.Name, "--as", "user"},
				{"rule", "table", "get", "--profile", profile.Name, "--as", "app", "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "table-1"},
			} {
				err := app.Run(context.Background(), args)
				if err == nil || err.Error() != productionProfileErrorMessage(profile.Name) || requests != 0 {
					t.Fatalf("command %v error=%v requests=%d", args, err, requests)
				}
			}
		})
	}
}

/*
TestKnownTestAndBlueHostsBlocked 验证传输层不会把旧环境请求交给底层 HTTP 客户端。
入参 t（*testing.T）为测试上下文；返回值为空，失败时报告测试错误。
*/
func TestKnownTestAndBlueHostsBlocked(t *testing.T) {
	requests := 0
	app := New(Options{Store: config.NewStore(t.TempDir()), HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return productionResponse(http.StatusOK, `{}`), nil
	})}})
	for _, origin := range []string{"https://test-open.qtech.cn", "https://test-myaccount.qtech.cn", "https://open-b.qfei.cn", "https://myaccount-b.qfei.cn"} {
		response, err := app.httpClient.Get(origin)
		if response != nil {
			response.Body.Close()
		}
		if err == nil || !strings.Contains(err.Error(), "blocks non-production host") {
			t.Fatalf("host %s error=%v", origin, err)
		}
	}
	if requests != 0 {
		t.Fatalf("non-production requests=%d", requests)
	}
}

/*
TestConfigHelpOnlyShowsProductionEnvironment 验证帮助只向用户展示可用的 prod 预设。
入参 t（*testing.T）为测试上下文；返回值为空，失败时报告测试错误。
*/
func TestConfigHelpOnlyShowsProductionEnvironment(t *testing.T) {
	output := &bytes.Buffer{}
	app := New(Options{Store: config.NewStore(t.TempDir()), Stdout: output})
	if err := app.Run(context.Background(), []string{"config", "add", "--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "--env <prod>") || strings.Contains(output.String(), "--env test") || strings.Contains(output.String(), "--env blue") {
		t.Fatalf("non-production environment shown in help: %s", output.String())
	}
}
