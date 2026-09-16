package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
)

/*
enableTestEnvironmentBuild 临时启用联调开关，测试结束后恢复；调用者不得并行运行。
入参 t（*testing.T）为测试上下文；无返回值。
*/
func enableTestEnvironmentBuild(t *testing.T) {
	previous := testBuild
	testBuild = "true"
	t.Cleanup(func() { testBuild = previous })
}

/*
validTestEnvironmentProfile 将正式测试配置整体映射至 test，避免遗漏认证端点。
无入参；返回合法 test 配置（config.Profile）。
*/
func validTestEnvironmentProfile() config.Profile {
	data, _ := json.Marshal(validProductionProfile())
	text := strings.ReplaceAll(string(data), productionOpenPlatformOrigin, testOpenPlatformOrigin)
	text = strings.ReplaceAll(text, productionAccountOrigin, testAccountOrigin)
	var profile config.Profile
	_ = json.Unmarshal([]byte(text), &profile)
	profile.Environment = "test"
	profile.Name = "contract-test"
	return profile
}

/*
TestTestEnvironmentIsolation 验证预设、配置、快照、待授权状态和网络的环境边界。
入参 t（*testing.T）为测试上下文；无返回值。
*/
func TestTestEnvironmentIsolation(t *testing.T) {
	enableTestEnvironmentBuild(t)
	preset, err := resolveEnvironment("test")
	if err != nil || preset.Resource != testOpenPlatformOrigin {
		t.Fatalf("preset=%+v err=%v", preset, err)
	}
	if _, err := resolveEnvironment("dev"); err == nil {
		t.Fatal("removed dev environment still accepted")
	}
	for _, profile := range []config.Profile{validProductionProfile(), validTestEnvironmentProfile()} {
		if err := validateProductionProfile(profile); err != nil {
			t.Fatal(err)
		}
	}
	for _, mutate := range []func(*config.Profile){
		func(p *config.Profile) { p.Resource = productionOpenPlatformOrigin },
		func(p *config.Profile) { p.Identities.User.TokenEndpoint = productionAccountOrigin + "/token" },
		func(p *config.Profile) { p.AppTokenEndpoint = productionOpenPlatformOrigin + "/token" },
		func(p *config.Profile) { p.OpenPlatformBaseURL += "/unexpected" },
		func(p *config.Profile) { p.Environment = "qa" },
		func(p *config.Profile) { p.Environment = "dev" },
		func(p *config.Profile) { p.OpenPlatformBaseURL = "https://dev-open.qtech.cn" },
		func(p *config.Profile) { p.Identities.User.TokenEndpoint = "https://dev-myaccount.qtech.cn/token" },
	} {
		profile := validTestEnvironmentProfile()
		mutate(&profile)
		if validateProductionProfile(profile) == nil {
			t.Fatalf("accepted mixed profile: %+v", profile)
		}
	}
	profile := validTestEnvironmentProfile()
	stored := credential.DeviceCredential{DeviceProfile: snapshotDeviceProfile(profile), Pending: &credential.PendingTransaction{
		TokenEndpoint:           profile.Identities.User.TokenEndpoint,
		VerificationURIComplete: testAccountOrigin + "/device?user_code=test",
	}}
	if err := validateProductionDeviceCredential(profile.Name, stored, "test"); err != nil {
		t.Fatal(err)
	}
	if validateProductionDeviceCredential(profile.Name, stored, "prod") == nil {
		t.Fatal("accepted test credential in prod")
	}
	stored.Pending.TokenEndpoint = productionAccountOrigin + "/token"
	if validateProductionDeviceCredential(profile.Name, stored, "test") == nil {
		t.Fatal("accepted mixed pending token")
	}
	calls := 0
	app := New(Options{Store: config.NewStore(t.TempDir()), HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return productionResponse(200, `{}`), nil
	})}})
	for _, rawURL := range []string{testOpenPlatformOrigin + "/test", testAccountOrigin + "/test", productionOpenPlatformOrigin + "/test"} {
		resp, err := app.httpClient.Get(rawURL)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	for _, rawURL := range []string{"http://test-open.qtech.cn/test", "https://test-open.qtech.cn./test", "https://test-open.qtech.cn:444/test", "https://dev-open.qtech.cn/test", "https://dev-myaccount.qtech.cn/test"} {
		if _, err := app.httpClient.Get(rawURL); err == nil {
			t.Fatal("accepted invalid test URL")
		}
	}
	if calls != 3 {
		t.Fatalf("calls=%d", calls)
	}
	var output bytes.Buffer
	app.stdout = &output
	if err := app.Run(context.Background(), []string{"config", "add", "--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "--env <prod|test>") {
		t.Fatal(output.String())
	}
}

/*
TestProductionBuildRejectsTestEnvironment 验证正式构建拒绝 test 配置及网络地址。
入参 t（*testing.T）为测试上下文；无返回值。
*/
func TestProductionBuildRejectsTestEnvironment(t *testing.T) {
	if _, err := resolveEnvironment("test"); err == nil {
		t.Fatal("production build accepted test environment")
	}
	if validateProductionProfile(validTestEnvironmentProfile()) == nil {
		t.Fatal("production build accepted test profile")
	}
	app := New(Options{Store: config.NewStore(t.TempDir()), HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("test request escaped production guard")
		return nil, nil
	})}})
	for _, origin := range []string{testOpenPlatformOrigin, testAccountOrigin} {
		if _, err := app.httpClient.Get(origin); err == nil {
			t.Fatal("production build accepted test host")
		}
	}
}

/*
TestTestEnvironmentConfigAndApprovalMatrix 验证 test 发现、默认 prod、跨环境重置和审批矩阵请求使用 test token。
入参 t（*testing.T）为测试上下文；无返回值。
*/
func TestTestEnvironmentConfigAndApprovalMatrix(t *testing.T) {
	enableTestEnvironmentBuild(t)
	store := config.NewStore(t.TempDir())
	secrets := config.NewSecretsStore(t.TempDir())
	credentials := &deviceMemoryCredentialStore{values: map[string]credential.DeviceCredential{}}
	matrixCalls := 0
	app := New(Options{Store: store, Secrets: secrets, CredentialStore: credentials, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "test-open.qtech.cn" && req.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal" {
			return productionResponse(200, `{"code":0,"tenant_access_token":"test-token","expire":7200}`), nil
		}
		if strings.HasPrefix(req.URL.Path, "/.well-known/") {
			origin := "https://" + req.URL.Host
			return productionResponse(200, `{"issuer":"test","authorization_endpoint":"`+origin+`/api/public/oauth/authorize/contract","device_authorization_endpoint":"`+origin+`/api/public/oauth/device-authorization/contract","token_endpoint":"`+origin+`/api/public/oauth/token/contract","revocation_endpoint":"`+origin+`/api/public/oauth/revoke/contract","registration_endpoint":"`+origin+`/api/public/oauth/register/contract"}`), nil
		}
		if req.URL.Host != "test-open.qtech.cn" || req.URL.Path != "/open-apis/rule_engine/v1/products/contract/groups/approve_matrix/rule_tables/table-1" || req.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("unexpected matrix request: %s %s", req.Method, req.URL)
		}
		matrixCalls++
		return productionResponse(200, `{"code":0,"data":{}}`), nil
	})}})
	for _, args := range [][]string{
		{"config", "add", "--name", "contract"},
		{"config", "add", "--env", "test", "--name", "contract-test"},
	} {
		if err := app.Run(context.Background(), args); err != nil {
			t.Fatal(err)
		}
	}
	prod, _ := store.GetProfile("contract")
	if prod.Environment != "prod" {
		t.Fatal("default environment changed")
	}
	testProfile, _ := store.GetProfile("contract-test")
	if testProfile.Environment != "test" || testProfile.OpenPlatformBaseURL != testOpenPlatformOrigin {
		t.Fatalf("test=%+v", testProfile)
	}
	if err := app.Run(context.Background(), []string{"auth", "login", "--profile", "contract-test", "--as", "app", "--app-id", "test-app", "--app-secret", "test-fixture-secret"}); err != nil {
		t.Fatal(err)
	}
	// 同环境重复配置须保留认证；随后矩阵请求验证 token 仍可用。
	if err := app.Run(context.Background(), []string{"config", "add", "--env", "test", "--name", "contract-test"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background(), []string{"approval-matrix", "table", "get", "--profile", "contract-test", "--as", "app", "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "table-1"}); err != nil {
		t.Fatal(err)
	}
	if matrixCalls != 1 {
		t.Fatalf("matrixCalls=%d", matrixCalls)
	}
	credentials.values[testProfile.Name] = credential.DeviceCredential{DeviceProfile: snapshotDeviceProfile(testProfile)}
	if err := app.Run(context.Background(), []string{"config", "add", "--env", "prod", "--name", "contract-test"}); err != nil {
		t.Fatal(err)
	}
	switched, _ := store.GetProfile("contract-test")
	if switched.Identities.App.Token != nil || switched.Identities.App.AppID != "" {
		t.Fatal("test credentials retained after switching to prod")
	}
	if _, exists := credentials.values[testProfile.Name]; exists {
		t.Fatal("test Device credential retained after switching")
	}
	if _, exists, err := secrets.Get(config.AppSecretKey(testProfile.Name)); err != nil || exists {
		t.Fatalf("old app secret retained: exists=%v err=%v", exists, err)
	}
}

/*
TestTestEnvironmentDeviceAuthorization 验证 test Device 授权只接受同环境链接并正确保存凭据。
入参 t（*testing.T）为测试上下文；无返回值。
*/
func TestTestEnvironmentDeviceAuthorization(t *testing.T) {
	enableTestEnvironmentBuild(t)
	for _, origin := range []string{testAccountOrigin, productionAccountOrigin} {
		t.Run(origin, func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			profile := validTestEnvironmentProfile()
			if err := store.UpsertProfile(profile, true); err != nil {
				t.Fatal(err)
			}
			credentials := &deviceMemoryCredentialStore{values: map[string]credential.DeviceCredential{}}
			app := New(Options{Store: store, CredentialStore: credentials, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != profile.Identities.User.DeviceAuthorizationEndpoint {
					t.Fatalf("unexpected URL: %s", req.URL)
				}
				return productionResponse(200, `{"device_code":"test-device","user_code":"test-user","verification_uri":"`+origin+`/device","verification_uri_complete":"`+origin+`/device?user_code=test-user","expires_in":600}`), nil
			})}})
			workspace := t.TempDir()
			app.lookupEnv = func(name string) (string, bool) {
				if name == "SKILL_SESSION_WORKSPACE" {
					return workspace, true
				}
				return "", false
			}
			err := app.Run(context.Background(), []string{"auth", "init", "--profile", profile.Name, "--output", "json"})
			if origin == testAccountOrigin {
				if err != nil {
					t.Fatal(err)
				}
				stored, err := app.loadProductionDeviceCredential(profile, credentials)
				if err != nil || stored.DeviceProfile.Environment != "test" || stored.Pending == nil {
					t.Fatalf("test authorization not saved: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("accepted prod authorization link for test profile")
				}
				if len(credentials.values) != 0 {
					t.Fatal("saved mixed environment authorization")
				}
			}
		})
	}
}
