package cli

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
)

func TestProfileEndpointsRequireProductionAddresses(t *testing.T) {
	profile := validProductionProfile()
	if err := validateProductionProfile(profile); err != nil {
		t.Fatalf("valid production profile: %v", err)
	}
	for _, change := range []func(*config.Profile){
		func(p *config.Profile) { p.Environment = "test" },
		func(p *config.Profile) { p.Resource = "https://configured-api.example" },
		func(p *config.Profile) { p.AppTokenEndpoint = "https://configured-api.example/token" },
		func(p *config.Profile) { p.Identities.User.TokenEndpoint = "https://configured-account.example/token" },
		func(p *config.Profile) { *p = validTestProfile(); p.Environment = "prod" },
		func(p *config.Profile) { p.OpenPlatformBaseURL = "http://test-open.qtech.cn" },
	} {
		changed := profile
		change(&changed)
		if err := validateProductionProfile(changed); err == nil {
			t.Fatal("accepted mismatched or insecure profile")
		}
	}
}

func TestConfigProductionAddressesAndDeviceQuery(t *testing.T) {
	store := config.NewStore(t.TempDir())
	workspace := t.TempDir()
	credentials := &deviceMemoryCredentialStore{values: map[string]credential.DeviceCredential{}}
	output := &bytes.Buffer{}
	requests := 0
	app := New(Options{
		Store: store, Secrets: config.NewSecretsStore(t.TempDir()), CredentialStore: credentials,
		Stdout: output, Stderr: &bytes.Buffer{},
		LookupEnv: func(name string) (string, bool) { return workspace, name == "SKILL_SESSION_WORKSPACE" },
		HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests++
			if req.URL.Host == "myaccount.qfei.cn" {
				switch req.URL.Path {
				case "/.well-known/oauth-authorization-server/contract":
					return productionResponse(http.StatusOK, productionAccountMetadata), nil
				case "/api/public/oauth/device-authorization/contract":
					return productionResponse(http.StatusOK, `{"device_code":"fixture-code","user_code":"fixture-user","verification_uri":"https://myaccount.qfei.cn/device","verification_uri_complete":"https://myaccount.qfei.cn/device?user_code=fixture-user","expires_in":600}`), nil
				case "/api/public/oauth/token/contract":
					return productionResponse(http.StatusOK, `{"access_token":"fixture-token","refresh_token":"fixture-refresh","token_type":"Bearer","expires_in":3600}`), nil
				}
			}
			if req.URL.Host != "open.qfei.cn" || req.Header.Get("Authorization") != "Bearer fixture-token" {
				t.Fatalf("request escaped configured target: %s", req.URL.Host)
			}
			return productionResponse(http.StatusOK, `{"code":0,"data":{"items":[]}}`), nil
		})},
	})
	commands := [][]string{
		{"config", "add", "--env", "prod", "--name", "contract", "--open-platform-base-url", "https://open.qfei.cn", "--authorization-server-metadata-url", "https://myaccount.qfei.cn/.well-known/oauth-authorization-server/contract"},
		{"auth", "init", "--profile", "contract"},
		{"auth", "complete", "--profile", "contract"},
		{"contract", "search", "--profile", "contract", "--data", `{"page_size":1}`},
		{"api", "call", "--profile", "contract", "--channel", "web", "GET", "/open-web/web/clm/contract/info"},
	}
	for _, args := range commands {
		output.Reset()
		if err := app.Run(context.Background(), args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if args[0] == "auth" && args[1] == "complete" && !strings.Contains(output.String(), `"status":"succeeded"`) {
			t.Fatalf("authorization did not complete: %s", output.String())
		}
		if strings.Contains(output.String(), "fixture-token") || strings.Contains(output.String(), "fixture-code") {
			t.Fatal("credential leaked to command output")
		}
	}
	if requests != 5 {
		t.Fatalf("requests=%d, want metadata, init, complete and two queries", requests)
	}
	profile, err := store.GetProfile("contract")
	if err != nil || profile.Environment != "prod" || profile.Resource != "https://open.qfei.cn" {
		t.Fatalf("configured target mismatch: %v", err)
	}
}

func TestDeviceSnapshotRejectsAddressChangeWithSameEnvironmentLabel(t *testing.T) {
	old := validProductionProfile()
	changed := validTestProfile()
	changed.Environment = "prod"
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(changed, true); err != nil {
		t.Fatal(err)
	}
	credentials := &deviceMemoryCredentialStore{values: map[string]credential.DeviceCredential{
		"contract": {DeviceProfile: snapshotDeviceProfile(old), Token: &config.Token{AccessToken: "fixture-token", Expiry: time.Now().Add(time.Hour)}},
	}}
	app := New(Options{Store: store, CredentialStore: credentials})
	if _, err := app.loadDeviceAwareProfile("contract"); err == nil {
		t.Fatal("accepted credential from previous target")
	}
}

func TestProfileHTTPRedirectsStayOnOriginalOrigin(t *testing.T) {
	for _, target := range []string{"https://test-open.qtech.cn/finish", "https://myaccount.qfei.cn/finish", "http://open.qfei.cn/finish"} {
		t.Run(target, func(t *testing.T) {
			requests := 0
			app := New(Options{Store: config.NewStore(t.TempDir()), HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(*http.Request) (*http.Response, error) {
				requests++
				response := productionResponse(http.StatusTemporaryRedirect, "")
				response.Header.Set("Location", target)
				return response, nil
			})}})
			response, err := app.httpClient.Get(productionOpenPlatformOrigin + "/start")
			if response != nil {
				_ = response.Body.Close()
			}
			if err == nil || requests != 1 {
				t.Fatalf("cross-origin redirect err=%v requests=%d", err, requests)
			}
		})
	}
}

const productionAccountMetadata = `{"issuer":"contract","authorization_endpoint":"https://myaccount.qfei.cn/api/public/oauth/authorize/contract","device_authorization_endpoint":"https://myaccount.qfei.cn/api/public/oauth/device-authorization/contract","token_endpoint":"https://myaccount.qfei.cn/api/public/oauth/token/contract","revocation_endpoint":"https://myaccount.qfei.cn/api/public/oauth/revoke/contract","registration_endpoint":"https://myaccount.qfei.cn/api/public/oauth/register/contract"}`

func TestConfigAddRejectsNonProductionTargetOverridesBeforeDiscovery(t *testing.T) {
	for name, overrides := range map[string][]string{
		"test API":          {"--open-platform-base-url", "https://test-open.qtech.cn"},
		"test account":      {"--authorization-server-metadata-url", "https://test-myaccount.qtech.cn/.well-known/oauth-authorization-server/contract"},
		"custom origins":    {"--open-platform-base-url", "https://configured-api.example", "--authorization-server-metadata-url", "https://configured-account.example/metadata"},
		"lookalike account": {"--authorization-server-metadata-url", "https://myaccount.qfei.cn.example/metadata"},
		"API port":          {"--open-platform-base-url", "https://open.qfei.cn:8443"},
		"account port":      {"--authorization-server-metadata-url", "https://myaccount.qfei.cn:8443/metadata"},
	} {
		t.Run(name, func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			requests := 0
			app := New(Options{
				Store: store, Secrets: config.NewSecretsStore(t.TempDir()),
				HTTPClient: &http.Client{Transport: productionRoundTripFunc(func(*http.Request) (*http.Response, error) {
					requests++
					return productionResponse(http.StatusOK, `{}`), nil
				})},
			})
			args := append([]string{"config", "add", "--env", "prod", "--name", "contract"}, overrides...)
			if err := app.Run(context.Background(), args); err == nil || err.Error() != productionProfileErrorMessage("contract") || requests != 0 {
				t.Fatalf("non-production override reached discovery: err=%v requests=%d", err, requests)
			}
			if _, found, err := store.LookupProfile("contract"); err != nil || found {
				t.Fatalf("rejected target was saved: found=%v err=%v", found, err)
			}
		})
	}
}
