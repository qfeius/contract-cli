package cli

import (
	"context"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
)

func TestConfigRecreationOnlyResetsMismatchedDeviceCredentials(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "valid production", true: "legacy environment"}[legacy], func(t *testing.T) {
			app, credentials, _ := newDeviceTokenTestApp(t, &config.Token{AccessToken: "keep-valid", Expiry: deviceTokenTestNow().Add(time.Hour)}, func(*http.Request) (*http.Response, error) {
				return jsonResponse(`{"issuer":"https://myaccount.qfei.cn/contract","authorization_endpoint":"https://myaccount.qfei.cn/api/public/oauth/authorize/contract","token_endpoint":"https://myaccount.qfei.cn/api/public/oauth/token/contract","device_authorization_endpoint":"https://myaccount.qfei.cn/api/public/oauth/device-authorization/contract","revocation_endpoint":"https://myaccount.qfei.cn/api/public/oauth/revoke/contract","registration_endpoint":"https://myaccount.qfei.cn/api/public/oauth/register/contract"}`), nil
			})
			app.stdout = io.Discard
			credentials.values["contract"].DeviceProfile.ProtectedResourceMetadataURL = ""
			if legacy {
				stored := credentials.values["contract"]
				stored.DeviceProfile.Environment = "test"
				stored.DeviceProfile.OpenPlatformBaseURL = "https://test-open.qtech.cn"
				credentials.values["contract"] = stored
			}
			if err := os.Remove(app.store.Path()); err != nil {
				t.Fatal(err)
			}
			if err := app.Run(context.Background(), []string{"config", "add", "--env", "prod"}); err != nil {
				t.Fatal(err)
			}
			stored, err := credentials.Load("contract")
			if legacy {
				if err != credential.ErrCredentialNotFound {
					t.Fatalf("legacy credential retained: %v", err)
				}
			} else if err != nil || stored.Token == nil || stored.Token.AccessToken != "keep-valid" {
				t.Fatal("valid production login was cleared")
			}
		})
	}
}

func TestDeviceProfileRecoveryUsesDefaultNameWhenConfigMissing(t *testing.T) {
	app, _, _ := newDeviceTokenTestApp(t, nil, func(*http.Request) (*http.Response, error) {
		t.Fatal("recovery must not call network")
		return nil, nil
	})
	if err := os.Remove(app.store.Path()); err != nil {
		t.Fatal(err)
	}
	profile, err := app.loadDeviceAwareProfile("")
	if err != nil || profile.Name != "contract" {
		t.Fatalf("default profile recovery failed: %q %v", profile.Name, err)
	}
}

func TestAuthenticationResetRespectsExistingDeviceOperation(t *testing.T) {
	app, credentials, _ := newDeviceTokenTestApp(t, &config.Token{AccessToken: "keep-valid"}, func(*http.Request) (*http.Response, error) { t.Fatal("unexpected network"); return nil, nil })
	if err := app.secrets.Set(config.AppSecretKey("contract"), "keep-secret"); err != nil {
		t.Fatal(err)
	}
	release, acquired, err := app.tryDeviceAuthorizationOperation("contract")
	if err != nil || !acquired {
		t.Fatalf("lock fixture failed: %v", err)
	}
	defer release()
	if err := app.clearProfileAuthenticationState(context.Background(), validProductionProfile()); err == nil {
		t.Fatal("reset ignored in-flight refresh/login")
	}
	if _, err := credentials.Load("contract"); err != nil {
		t.Fatal("credential deleted while busy")
	}
	if _, found, err := app.secrets.Get(config.AppSecretKey("contract")); err != nil || !found {
		t.Fatal("secret cleared before credential operation could start")
	}
}

func TestAuthenticationResetKeepsMatchingCredentialAtResetBoundary(t *testing.T) {
	app, credentials, _ := newDeviceTokenTestApp(t, &config.Token{AccessToken: "fresh-production-login"}, func(*http.Request) (*http.Response, error) { t.Fatal("unexpected network"); return nil, nil })
	if err := app.clearProfileAuthenticationState(context.Background(), validProductionProfile()); err != nil {
		t.Fatal(err)
	}
	stored, err := credentials.Load("contract")
	if err != nil || stored.Token == nil || stored.Token.AccessToken != "fresh-production-login" {
		t.Fatal("reset deleted a currently matching login")
	}
}
