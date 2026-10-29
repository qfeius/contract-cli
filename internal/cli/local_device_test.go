package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
	"cn.qfei/contract-cli/internal/invocation"
)

func localDeviceApp(t *testing.T, task string, token *config.Token, handler deviceRoundTripFunc) (*App, *deviceMemoryCredentialStore) {
	t.Helper()
	app, store, _ := newDeviceTokenTestApp(t, token, handler)
	localHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	app.localUserHome = func() (string, error) { return localHome, nil }
	app.lookupEnv = func(name string) (string, bool) { return task, name == "SESSION_ID" }
	app.inspectEnvironment = func(context.Context, int) invocation.Result {
		return invocation.Result{Platform: "darwin", AgentSourceType: "doubaoWork", EvidenceType: "macos_code_signature", Confidence: "high", RuleID: "client.doubao_work.signed-bundle"}
	}
	profile := mustDeviceProfile(t, app.store)
	profile.Identities.User.DeviceAuthorizationEndpoint = "https://myaccount.qfei.cn/api/public/oauth/device-authorization/contract"
	profile.Identities.User.RevocationEndpoint = "https://myaccount.qfei.cn/api/public/oauth/revoke/contract"
	profile.Identities.User.DeviceScope = "contract:full contract-review:full"
	if err := app.store.SaveProfile(profile); err != nil {
		t.Fatal(err)
	}
	return app, store
}

type localTestKeyring struct {
	mu     sync.Mutex
	values map[string]string
}

func (k *localTestKeyring) Get(service, user string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	value, ok := k.values[service+":"+user]
	if !ok {
		return "", credential.ErrCredentialNotFound
	}
	return value, nil
}

func (k *localTestKeyring) Set(service, user, value string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.values[service+":"+user] = value
	return nil
}

func (k *localTestKeyring) Delete(service, user string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.values, service+":"+user)
	return nil
}

func TestLocalDeviceRealStoreReusesAuthorizationAfterTaskAndProfileDirectoryChange(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	keyring := &localTestKeyring{values: map[string]string{}}
	requests := 0
	transport := func(request *http.Request) (*http.Response, error) {
		requests++
		switch request.URL.Path {
		case "/api/public/oauth/device-authorization/contract":
			return deviceTokenTestResponse(http.StatusOK, `{"device_code":"test-device","user_code":"test-code","verification_uri":"https://myaccount.qfei.cn/device","verification_uri_complete":"https://myaccount.qfei.cn/device?user_code=test-code","expires_in":600}`), nil
		case "/api/public/oauth/token/contract":
			return deviceTokenTestResponse(http.StatusOK, `{"access_token":"test-access","refresh_token":"test-refresh","expires_in":3600}`), nil
		case "/api/public/oauth/revoke/contract":
			return deviceTokenTestResponse(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected request: %s", request.URL.Path)
			return nil, nil
		}
	}
	first, _ := localDeviceApp(t, "task-a", nil, transport)
	first.credentialStore, first.credentialKeyring = nil, keyring
	var stdout bytes.Buffer
	first.stdout = &stdout
	for _, command := range []string{"init", "complete"} {
		stdout.Reset()
		if err := first.Run(context.Background(), []string{"auth", command, "--profile", "contract"}); err != nil {
			t.Fatal(err)
		}
	}
	second, _ := localDeviceApp(t, "task-b", nil, transport)
	second.credentialStore, second.credentialKeyring = nil, keyring
	second.store = config.NewStore(t.TempDir())
	second.stdout = &stdout
	stdout.Reset()
	if err := second.Run(context.Background(), []string{"auth", "status", "--profile", "contract"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Authorization: authorized") || !strings.Contains(stdout.String(), "Credential Scope: user") {
		t.Fatalf("new task status: %s", stdout.String())
	}
	stdout.Reset()
	if err := second.Run(context.Background(), []string{"auth", "init", "--profile", "contract"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"status":"authorized"`) || requests != 2 {
		t.Fatalf("new task reauthorized: requests=%d output=%s", requests, stdout.String())
	}
	if err := second.Run(context.Background(), []string{"auth", "logout", "--profile", "contract"}); err != nil {
		t.Fatal(err)
	}
	view, err := first.deviceAuthStatus(mustDeviceProfile(t, first.store))
	if err != nil || view.Authorization != "unauthorized" || requests != 3 {
		t.Fatalf("logout not shared: view=%+v err=%v requests=%d", view, err, requests)
	}
}

func TestLocalDeviceConcurrentRefreshRotatesOnlyOnceAcrossTasks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	keyring := &localTestKeyring{values: map[string]string{}}
	entered, finish := make(chan struct{}), make(chan struct{})
	first, _ := localDeviceApp(t, "task-a", nil, func(*http.Request) (*http.Response, error) {
		close(entered)
		<-finish
		return deviceTokenTestResponse(http.StatusOK, `{"access_token":"rotated","refresh_token":"rotated-refresh","expires_in":3600}`), nil
	})
	first.credentialStore, first.credentialKeyring = nil, keyring
	second, _ := localDeviceApp(t, "task-b", nil, func(*http.Request) (*http.Response, error) {
		t.Error("second task rotated the shared refresh token")
		return nil, errors.New("unexpected request")
	})
	second.credentialStore, second.credentialKeyring = nil, keyring
	store, err := first.deviceCredentials()
	if err != nil {
		t.Fatal(err)
	}
	profile := mustDeviceProfile(t, first.store)
	if err := store.Save("contract", credential.DeviceCredential{
		Token:         &config.Token{AccessToken: "old", RefreshToken: "refresh", Expiry: deviceTokenTestNow().Add(-time.Minute)},
		DeviceProfile: snapshotDeviceProfile(profile),
	}); err != nil {
		t.Fatal(err)
	}
	completed := make(chan error, 1)
	go func() {
		_, err := first.refreshDeviceToken(context.Background(), profile, "old", false)
		completed <- err
	}()
	<-entered
	_, refreshErr := second.refreshDeviceToken(context.Background(), profile, "old", false)
	close(finish)
	if err := <-completed; err != nil {
		t.Fatal(err)
	}
	if refreshErr == nil || !strings.Contains(refreshErr.Error(), "already in progress") {
		t.Fatalf("parallel refresh not blocked: %v", refreshErr)
	}
	access, err := second.refreshDeviceToken(context.Background(), profile, "old", false)
	if err != nil || access != "rotated" {
		t.Fatalf("did not reload rotated token: %q %v", access, err)
	}
}

func TestLocalDeviceTasksShareRefreshAndLogoutLock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	handler := func(*http.Request) (*http.Response, error) {
		t.Fatal("unexpected remote request while lock held")
		return nil, nil
	}
	first, shared := localDeviceApp(t, "task-a", &config.Token{AccessToken: "access", RefreshToken: "refresh"}, handler)
	second, _ := localDeviceApp(t, "task-b", nil, handler)
	second.credentialStore = shared
	lock, err := first.deviceAuthorizationLock("contract")
	if err != nil {
		t.Fatal(err)
	}
	locked, err := lock.TryLock()
	if err != nil || !locked {
		t.Fatalf("lock: %v %v", locked, err)
	}
	t.Cleanup(func() { _ = lock.Unlock() })
	other, err := second.deviceRefreshLock("contract")
	if err != nil {
		t.Fatal(err)
	}
	if other.Path() != lock.Path() {
		t.Fatalf("local tasks have different locks: %s / %s", lock.Path(), other.Path())
	}
	if _, err := second.deviceAuthLogout(context.Background(), mustDeviceProfile(t, second.store)); err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("logout must respect refresh/authorization lock: %v", err)
	}
	if _, err := shared.Load("contract"); err != nil {
		t.Fatal("busy logout deleted credentials")
	}
}

func TestLocalDeviceInitReusesTokenAndRefreshesWhenNeeded(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(map[bool]string{false: "valid", true: "expired_with_refresh"}[expired], func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Chdir(t.TempDir())
			expiry := deviceTokenTestNow().Add(time.Hour)
			if expired {
				expiry = deviceTokenTestNow().Add(-time.Minute)
			}
			requests := 0
			app, _ := localDeviceApp(t, "new-task", &config.Token{AccessToken: "old-access", RefreshToken: "refresh", Expiry: expiry}, func(request *http.Request) (*http.Response, error) {
				requests++
				if request.URL.Path != "/api/public/oauth/token/contract" {
					t.Fatalf("reauthorized instead of reusing: %s", request.URL.Path)
				}
				return deviceTokenTestResponse(http.StatusOK, `{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`), nil
			})
			var stdout bytes.Buffer
			app.stdout = &stdout
			if err := app.Run(context.Background(), []string{"auth", "init", "--profile", "contract"}); err != nil {
				t.Fatal(err)
			}
			var output map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
				t.Fatal(err)
			}
			if output["status"] != "authorized" || output["credential_scope"] != "user" {
				t.Fatalf("reuse output = %v", output)
			}
			if _, ok := output["verification_uri_complete"]; ok {
				t.Fatal("reused auth exposes a new link")
			}
			want := 0
			if expired {
				want = 1
			}
			if requests != want {
				t.Fatalf("requests = %d, want %d", requests, want)
			}
		})
	}
}

func TestLocalDeviceLogoutIsVisibleToOtherTasks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	requests := 0
	first, shared := localDeviceApp(t, "task-a", &config.Token{AccessToken: "access", RefreshToken: "refresh"}, func(request *http.Request) (*http.Response, error) {
		requests++
		if !strings.HasSuffix(request.URL.Path, "/revoke/contract") {
			t.Fatalf("unexpected request: %s", request.URL.Path)
		}
		return deviceTokenTestResponse(http.StatusOK, `{}`), nil
	})
	second, _ := localDeviceApp(t, "task-b", nil, nil)
	second.credentialStore = shared
	if _, err := first.deviceAuthLogout(context.Background(), mustDeviceProfile(t, first.store)); err != nil {
		t.Fatal(err)
	}
	if _, err := shared.Load("contract"); !errors.Is(err, credential.ErrCredentialNotFound) {
		t.Fatalf("credential remains: %v", err)
	}
	view, err := second.deviceAuthStatus(mustDeviceProfile(t, second.store))
	if err != nil || view.Authorization != "unauthorized" || requests != 1 {
		t.Fatalf("logout status = %+v, err=%v, requests=%d", view, err, requests)
	}
}

func TestLocalDeviceExistingEmptyProfileRestoresSharedIdentity(t *testing.T) {
	for _, operation := range []string{"business", "logout"} {
		t.Run(operation, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			app, shared := localDeviceApp(t, "new-task", &config.Token{AccessToken: "shared-access", RefreshToken: "shared-refresh"}, func(*http.Request) (*http.Response, error) { return deviceTokenTestResponse(http.StatusOK, `{}`), nil })
			profile := mustDeviceProfile(t, app.store)
			shared.values["contract"] = credential.DeviceCredential{Token: shared.values["contract"].Token, DeviceProfile: snapshotDeviceProfile(profile)}
			profile.Identities.User.AuthMode = ""
			if err := app.store.SaveProfile(profile); err != nil {
				t.Fatal(err)
			}
			if operation == "logout" {
				if err := app.Run(context.Background(), []string{"auth", "logout", "--profile", "contract"}); err != nil {
					t.Fatal(err)
				}
				if _, err := shared.Load("contract"); !errors.Is(err, credential.ErrCredentialNotFound) {
					t.Fatalf("logout left shared credentials: %v", err)
				}
				return
			}
			loaded, err := app.loadDeviceAwareProfile("contract")
			if err != nil || loaded.Identities.User.AuthMode != config.UserAuthModeDevice {
				t.Fatalf("shared identity not restored: %+v, %v", loaded.Identities.User, err)
			}
		})
	}
}

func TestLocalDeviceLockIgnoresTemporaryHomeAndCache(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())
	app, _ := localDeviceApp(t, "task-a", nil, nil)
	first, err := app.deviceAuthorizationLock("contract")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())
	second, err := app.deviceRefreshLock("contract")
	if err != nil {
		t.Fatal(err)
	}
	if first.Path() != second.Path() {
		t.Fatalf("same OS user gets different locks when task changes HOME: %q / %q", first.Path(), second.Path())
	}
}

func TestLocalDeviceRestorePreservesLegacyLoginConfiguration(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	app, shared := localDeviceApp(t, "new-task", &config.Token{AccessToken: "shared"}, nil)
	profile := mustDeviceProfile(t, app.store)
	shared.values["contract"] = credential.DeviceCredential{Token: shared.values["contract"].Token, DeviceProfile: snapshotDeviceProfile(profile)}
	profile.Identities.User.AuthMode = ""
	profile.Identities.User.AuthorizationEndpoint = "https://myaccount.qfei.cn/authorize"
	profile.Identities.User.RegistrationEndpoint = "https://myaccount.qfei.cn/register"
	profile.Identities.User.RedirectURL = "http://127.0.0.1:8000/callback"
	profile.Identities.User.ClientID = "legacy-client"
	if err := app.store.SaveProfile(profile); err != nil {
		t.Fatal(err)
	}
	loaded, err := app.loadDeviceAwareProfile("contract")
	if err != nil {
		t.Fatal(err)
	}
	want, got := profile.Identities.User, loaded.Identities.User
	if got.AuthorizationEndpoint != want.AuthorizationEndpoint || got.RegistrationEndpoint != want.RegistrationEndpoint || got.RedirectURL != want.RedirectURL || got.ClientID != want.ClientID {
		t.Fatalf("restoring Device profile erased legacy login configuration: %+v", got)
	}
}
