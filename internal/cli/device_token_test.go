package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
	"cn.qfei/contract-cli/internal/openplatform"
)

func TestDeviceTokenRefreshesBeforeBusinessRequestWhenNearExpiry(t *testing.T) {
	app, credentials, calls := newDeviceTokenTestApp(t, &config.Token{
		AccessToken: "old-access", RefreshToken: "refresh-one", Expiry: deviceTokenTestNow().Add(4 * time.Minute),
	}, func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/public/oauth/token/contract":
			return deviceTokenTestResponse(http.StatusOK, `{"access_token":"new-access","refresh_token":"refresh-two","token_type":"Bearer","expires_in":3600}`), nil
		case "/open-apis/contract/v1/mcp/templates":
			if got := request.Header.Get("Authorization"); got != "Bearer new-access" {
				t.Fatalf("Authorization = %q", got)
			}
			return deviceTokenTestResponse(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
			return nil, nil
		}
	})

	client, requestContext, err := app.openPlatformClientAndContext("contract", "user", "/open-apis/contract/v1/mcp/templates", openplatform.IdentityPolicyUserOnly)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Do(context.Background(), requestContext, openplatform.Request{
		Method: http.MethodGet, Path: "/open-apis/contract/v1/mcp/templates", OperationKind: openplatform.OperationRead,
	})
	if err != nil {
		t.Fatal(err)
	}
	if *calls != 2 {
		t.Fatalf("HTTP calls = %d, want 2", *calls)
	}
	stored, err := credentials.Load("contract")
	if err != nil || stored.Token == nil || stored.Token.AccessToken != "new-access" || stored.Token.RefreshToken != "refresh-two" {
		t.Fatalf("stored = %+v, err=%v", stored, err)
	}
}

func TestDeviceTokenRefreshesOnceOnExplicitGatewayTokenExpired(t *testing.T) {
	businessCalls := 0
	app, _, calls := newDeviceTokenTestApp(t, &config.Token{
		AccessToken: "old-access", RefreshToken: "refresh-one", Expiry: deviceTokenTestNow().Add(time.Hour),
	}, func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/public/oauth/token/contract":
			return deviceTokenTestResponse(http.StatusOK, `{"access_token":"new-access","refresh_token":"refresh-two","token_type":"Bearer","expires_in":3600}`), nil
		case "/open-apis/contract/v1/mcp/templates":
			businessCalls++
			if businessCalls == 1 {
				response := deviceTokenTestResponse(http.StatusUnauthorized, `{"data":{"error_type":"token_expired"}}`)
				response.Header.Set("X-Qfei-Open-Platform-Auth-Error", "token_expired")
				return response, nil
			}
			if got := request.Header.Get("Authorization"); got != "Bearer new-access" {
				t.Fatalf("Authorization = %q", got)
			}
			return deviceTokenTestResponse(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
			return nil, nil
		}
	})

	client, requestContext, err := app.openPlatformClientAndContext("contract", "user", "/open-apis/contract/v1/mcp/templates", openplatform.IdentityPolicyUserOnly)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Do(context.Background(), requestContext, openplatform.Request{
		Method: http.MethodGet, Path: "/open-apis/contract/v1/mcp/templates", OperationKind: openplatform.OperationRead,
	})
	if err != nil {
		t.Fatal(err)
	}
	if *calls != 3 || businessCalls != 2 {
		t.Fatalf("HTTP calls=%d businessCalls=%d", *calls, businessCalls)
	}
}

func TestDeviceTokenInvalidGrantClearsTokenAndPreservesPendingAuthorization(t *testing.T) {
	pending := &credential.PendingTransaction{
		DeviceCode: "pending-device", TokenEndpoint: "https://myaccount.qfei.cn/api/public/oauth/token/contract",
		ClientID: "zscli_892efdadc11a3f53", ExpiresAt: deviceTokenTestNow().Add(10 * time.Minute),
	}
	app, credentials, _ := newDeviceTokenTestApp(t, &config.Token{
		AccessToken: "old-access", RefreshToken: "invalid-refresh", Expiry: deviceTokenTestNow().Add(time.Hour),
	}, func(request *http.Request) (*http.Response, error) {
		return deviceTokenTestResponse(http.StatusBadRequest, `{"error":"invalid_grant","error_description":"refresh token revoked"}`), nil
	})
	stored := credentials.values["contract"]
	stored.Pending = pending
	credentials.values["contract"] = stored

	_, err := app.refreshDeviceToken(context.Background(), mustDeviceProfile(t, app.store), "old-access", true)

	if !errors.Is(err, ErrDeviceReauthorizationRequired) {
		t.Fatalf("refresh error = %v", err)
	}
	if !strings.Contains(err.Error(), "user confirmation is required") {
		t.Fatalf("refresh error must require explicit user confirmation: %v", err)
	}
	stored = credentials.values["contract"]
	if stored.Token != nil || stored.Pending != pending {
		t.Fatalf("stored credential = %+v", stored)
	}
}

func TestDeviceTokenTemporaryRefreshFailureKeepsExistingToken(t *testing.T) {
	app, credentials, _ := newDeviceTokenTestApp(t, &config.Token{
		AccessToken: "old-access", RefreshToken: "refresh-one", Expiry: deviceTokenTestNow().Add(time.Hour),
	}, func(request *http.Request) (*http.Response, error) {
		return deviceTokenTestResponse(http.StatusServiceUnavailable, `{"error":"temporarily_unavailable"}`), nil
	})

	_, err := app.refreshDeviceToken(context.Background(), mustDeviceProfile(t, app.store), "old-access", true)

	if err == nil {
		t.Fatal("expected refresh error")
	}
	stored := credentials.values["contract"]
	if stored.Token == nil || stored.Token.AccessToken != "old-access" || stored.Token.RefreshToken != "refresh-one" {
		t.Fatalf("stored credential = %+v", stored)
	}
}

func TestDeviceTokenSuccessfulRotationReportsCredentialSaveFailureWithoutRetry(t *testing.T) {
	app, credentials, calls := newDeviceTokenTestApp(t, &config.Token{
		AccessToken: "old-access", RefreshToken: "refresh-one", Expiry: deviceTokenTestNow().Add(time.Hour),
	}, func(request *http.Request) (*http.Response, error) {
		return deviceTokenTestResponse(http.StatusOK, `{"access_token":"new-access","refresh_token":"refresh-two","token_type":"Bearer","expires_in":3600}`), nil
	})
	credentials.saveErr = errors.New("secure storage unavailable")

	_, err := app.refreshDeviceToken(context.Background(), mustDeviceProfile(t, app.store), "old-access", true)

	if err == nil || !strings.Contains(err.Error(), "authorization state may be invalid") || *calls != 1 {
		t.Fatalf("err=%v calls=%d", err, *calls)
	}
}

func TestDeviceTokenRefreshFailsWhenAnotherProcessOwnsLock(t *testing.T) {
	app, _, _ := newDeviceTokenTestApp(t, &config.Token{
		AccessToken: "old-access", RefreshToken: "refresh-one", Expiry: deviceTokenTestNow().Add(4 * time.Minute),
	}, func(request *http.Request) (*http.Response, error) {
		t.Fatal("HTTP must not be called while refresh lock is held")
		return nil, nil
	})
	lock, err := app.deviceRefreshLock("contract")
	if err != nil {
		t.Fatal(err)
	}
	locked, err := lock.TryLock()
	if err != nil || !locked {
		t.Fatalf("TryLock() = %v, %v", locked, err)
	}
	defer func() { _ = lock.Unlock() }()

	_, err = app.refreshDeviceToken(context.Background(), mustDeviceProfile(t, app.store), "old-access", false)
	if err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("refresh error = %v", err)
	}
}

func TestDeviceAuthorizationAndRefreshShareCredentialOperationLock(t *testing.T) {
	app, _, _ := newDeviceTokenTestApp(t, &config.Token{
		AccessToken: "old-access", RefreshToken: "refresh-one", Expiry: deviceTokenTestNow().Add(time.Hour),
	}, func(request *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request to %s", request.URL)
		return nil, nil
	})
	authorizationLock, err := app.deviceAuthorizationLock("contract")
	if err != nil {
		t.Fatal(err)
	}
	locked, err := authorizationLock.TryLock()
	if err != nil || !locked {
		t.Fatalf("authorization TryLock() = %v, %v", locked, err)
	}
	defer func() { _ = authorizationLock.Unlock() }()

	refreshLock, err := app.deviceRefreshLock("contract")
	if err != nil {
		t.Fatal(err)
	}
	locked, err = refreshLock.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if locked {
		_ = refreshLock.Unlock()
		t.Fatal("refresh lock unexpectedly acquired while authorization owns the shared credential lock")
	}
}

func TestWorkBuddyAuthorizationAndRefreshShareCredentialOperationLock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	app, _, _ := newDeviceTokenTestApp(t, &config.Token{
		AccessToken: "old-access", RefreshToken: "refresh-one", Expiry: deviceTokenTestNow().Add(time.Hour),
	}, func(request *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request to %s", request.URL)
		return nil, nil
	})
	app.lookupEnv = func(name string) (string, bool) {
		if name == "CODEBUDDY_SESSION_ID" {
			return "task-a", true
		}
		return "", false
	}

	authorizationLock, err := app.deviceAuthorizationLock("contract")
	if err != nil {
		t.Fatal(err)
	}
	locked, err := authorizationLock.TryLock()
	if err != nil || !locked {
		t.Fatalf("authorization TryLock() = %v, %v", locked, err)
	}
	defer func() { _ = authorizationLock.Unlock() }()

	refreshLock, err := app.deviceRefreshLock("contract")
	if err != nil {
		t.Fatal(err)
	}
	locked, err = refreshLock.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if locked {
		_ = refreshLock.Unlock()
		t.Fatal("WorkBuddy refresh lock unexpectedly acquired while authorization owns the shared credential lock")
	}

	digest := sha256.Sum256([]byte("task-a:contract"))
	if got, want := filepath.Base(authorizationLock.Path()), hex.EncodeToString(digest[:])+".lock"; got != want {
		t.Fatalf("WorkBuddy lock file = %q, want backward-compatible %q", got, want)
	}
}

func TestWorkBuddyTasksUseIsolatedCredentialOperationLocks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	newApp := func(taskID string) *App {
		app, _, _ := newDeviceTokenTestApp(t, &config.Token{
			AccessToken: "old-access", RefreshToken: "refresh-one", Expiry: deviceTokenTestNow().Add(time.Hour),
		}, func(request *http.Request) (*http.Response, error) {
			t.Fatalf("unexpected HTTP request to %s", request.URL)
			return nil, nil
		})
		app.lookupEnv = func(name string) (string, bool) {
			if name == "CODEBUDDY_SESSION_ID" {
				return taskID, true
			}
			return "", false
		}
		return app
	}

	lockA, err := newApp("task-a").deviceAuthorizationLock("contract")
	if err != nil {
		t.Fatal(err)
	}
	locked, err := lockA.TryLock()
	if err != nil || !locked {
		t.Fatalf("task-a TryLock() = %v, %v", locked, err)
	}
	defer func() { _ = lockA.Unlock() }()

	lockB, err := newApp("task-b").deviceAuthorizationLock("contract")
	if err != nil {
		t.Fatal(err)
	}
	locked, err = lockB.TryLock()
	if err != nil || !locked {
		t.Fatalf("task-b TryLock() = %v, %v; WorkBuddy tasks must use isolated locks", locked, err)
	}
	defer func() { _ = lockB.Unlock() }()
}

func TestDoubaoWorkTaskUsesSessionIsolatedLockAndQRCodePaths(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	app, _, _ := newDeviceTokenTestApp(t, &config.Token{
		AccessToken: "old-access", RefreshToken: "refresh-one", Expiry: deviceTokenTestNow().Add(time.Hour),
	}, func(request *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request to %s", request.URL)
		return nil, nil
	})
	app.lookupEnv = func(name string) (string, bool) {
		if name == "SESSION_ID" {
			return "doubao-task-a", true
		}
		return "", false
	}

	lockA, err := app.deviceAuthorizationLock("contract")
	if err != nil {
		t.Fatal(err)
	}
	locked, err := lockA.TryLock()
	if err != nil || !locked {
		t.Fatalf("task-a TryLock() = %v, %v", locked, err)
	}
	defer func() { _ = lockA.Unlock() }()
	qrA, err := app.writeAuthorizationQRCode("contract", "https://auth.example/device?user_code=a")
	if err != nil {
		t.Fatal(err)
	}

	app.lookupEnv = func(name string) (string, bool) {
		if name == "SESSION_ID" {
			return "doubao-task-b", true
		}
		return "", false
	}
	lockB, err := app.deviceAuthorizationLock("contract")
	if err != nil {
		t.Fatal(err)
	}
	locked, err = lockB.TryLock()
	if err != nil || !locked {
		t.Fatalf("task-b TryLock() = %v, %v; Doubao tasks must use isolated locks", locked, err)
	}
	defer func() { _ = lockB.Unlock() }()
	qrB, err := app.writeAuthorizationQRCode("contract", "https://auth.example/device?user_code=b")
	if err != nil {
		t.Fatal(err)
	}
	if qrA.Path == qrB.Path {
		t.Fatalf("Doubao work tasks share QR path %q", qrA.Path)
	}
	for _, path := range []string{qrA.Path, qrB.Path} {
		if filepath.Dir(path) != workspace {
			t.Fatalf("QR path %q is not in the visible task root %q", path, workspace)
		}
		if !strings.HasPrefix(filepath.Base(path), "contract-cli-device-auth-") {
			t.Fatalf("QR file %q does not use the visible artifact prefix", path)
		}
	}
}

type deviceMemoryCredentialStore struct {
	values  map[string]credential.DeviceCredential
	saveErr error
}

func (s *deviceMemoryCredentialStore) Load(profileName string) (credential.DeviceCredential, error) {
	value, ok := s.values[profileName]
	if !ok {
		return credential.DeviceCredential{}, credential.ErrCredentialNotFound
	}
	return value, nil
}

func (s *deviceMemoryCredentialStore) Save(profileName string, value credential.DeviceCredential) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.values[profileName] = value
	return nil
}

func (s *deviceMemoryCredentialStore) Delete(profileName string) error {
	delete(s.values, profileName)
	return nil
}

type deviceRoundTripFunc func(*http.Request) (*http.Response, error)

func (f deviceRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func newDeviceTokenTestApp(t *testing.T, token *config.Token, handler deviceRoundTripFunc) (*App, *deviceMemoryCredentialStore, *int) {
	t.Helper()
	store := config.NewStore(t.TempDir())
	profile := validProductionProfile()
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatal(err)
	}
	credentials := &deviceMemoryCredentialStore{values: map[string]credential.DeviceCredential{
		"contract": {Token: token, DeviceProfile: snapshotDeviceProfile(profile)},
	}}
	calls := 0
	workspace := t.TempDir()
	client := &http.Client{Transport: deviceRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return handler(request)
	})}
	app := New(Options{
		Store: store, CredentialStore: credentials, HTTPClient: client, Now: deviceTokenTestNow,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		LookupEnv: func(name string) (string, bool) {
			if name == "SKILL_SESSION_WORKSPACE" {
				return workspace, true
			}
			return "", false
		},
	})
	return app, credentials, &calls
}

func mustDeviceProfile(t *testing.T, store *config.Store) config.Profile {
	t.Helper()
	profile, err := store.GetProfile("contract")
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func deviceTokenTestNow() time.Time {
	return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
}

func deviceTokenTestResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

var _ credential.Store = (*deviceMemoryCredentialStore)(nil)
