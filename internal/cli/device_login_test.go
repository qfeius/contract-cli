package cli_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
)

func TestDeviceLoginStartsAndWaitsWithoutCallback(t *testing.T) {
	stdout := &bytes.Buffer{}
	credentials := newLockedDeviceCredentialStore()
	requests := 0
	app := newDeviceAuthStateTestApp(t, stdout, credentials, func(request *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return jsonResponse(`{"device_code":"device-private","user_code":"user-a","verification_uri_complete":"https://myaccount.qfei.cn/device?user_code=user-a","expires_in":600,"interval":1}`), nil
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:device_code" || request.Form.Get("device_code") != "device-private" {
			t.Fatal("not the existing device token exchange")
		}
		if !strings.Contains(stdout.String(), "\nhttps://myaccount.qfei.cn/device?user_code=user-a\n") {
			t.Fatal("authorization URL was not emitted before polling")
		}
		return jsonResponse(`{"access_token":"access-private","refresh_token":"refresh-private","token_type":"Bearer","expires_in":3600}`), nil
	})
	if err := app.Run(context.Background(), []string{"auth", "device-login", "--profile", "contract", "--timeout", "10s"}); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || !strings.Contains(stdout.String(), "Authorization: authorized") {
		t.Fatalf("requests=%d output=%s", requests, stdout)
	}
	for _, secret := range []string{"device-private", "access-private", "refresh-private"} {
		if strings.Contains(stdout.String(), secret) {
			t.Fatal("credential leaked")
		}
	}
	stored, err := credentials.Load("contract")
	if err != nil || stored.Pending != nil || stored.Token == nil {
		t.Fatal("authorization was not persisted")
	}
}

func TestDeviceLoginReusesExistingTokenWithoutURLOrNetwork(t *testing.T) {
	stdout := &bytes.Buffer{}
	credentials := newLockedDeviceCredentialStore()
	credentials.values["contract"] = credential.DeviceCredential{
		DeviceProfile: deviceSnapshotFixture(deviceAuthStateProfile()),
		Token:         &config.Token{AccessToken: "private", Expiry: fixedCLINow().Add(time.Hour)},
	}
	app := newDeviceAuthStateTestApp(t, stdout, credentials, func(_ *http.Request) (*http.Response, error) {
		t.Fatal("authorized credentials should not trigger a request")
		return nil, errors.New("unexpected request")
	})
	if err := app.Run(context.Background(), []string{"auth", "device-login"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), "https://") || !strings.Contains(stdout.String(), "Authorization: authorized") {
		t.Fatalf("unexpected output: %s", stdout)
	}
}

func TestDeviceLoginTimeoutDoesNotPollOrReplacePending(t *testing.T) {
	stdout := &bytes.Buffer{}
	credentials := newLockedDeviceCredentialStore()
	credentials.values["contract"] = pendingCredential(credential.PendingStatusPending)
	app := newDeviceAuthStateTestApp(t, stdout, credentials, func(_ *http.Request) (*http.Response, error) {
		t.Fatal("timeout before interval must not poll")
		return nil, errors.New("unexpected request")
	})
	err := app.Run(context.Background(), []string{"auth", "device-login", "--timeout", "20ms"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected timeout, got %v", err)
	}
	stored, err := credentials.Load("contract")
	if err != nil || stored.Pending.EffectiveStatus() != credential.PendingStatusPending {
		t.Fatal("timeout changed the pending authorization")
	}
}

func TestDeviceLoginTerminalStatesDoNotPoll(t *testing.T) {
	for _, status := range []credential.PendingStatus{credential.PendingStatusDenied, credential.PendingStatusExpired, credential.PendingStatusUncertain, credential.PendingStatusChecking, credential.PendingStatusInvalidGrant} {
		t.Run(string(status), func(t *testing.T) {
			credentials := newLockedDeviceCredentialStore()
			credentials.values["contract"] = pendingCredential(status)
			app := newDeviceAuthStateTestApp(t, &bytes.Buffer{}, credentials, func(_ *http.Request) (*http.Response, error) {
				t.Fatal("terminal state must not poll")
				return nil, errors.New("unexpected request")
			})
			if err := app.Run(context.Background(), []string{"auth", "device-login"}); err == nil {
				t.Fatal("terminal state must fail")
			}
		})
	}
}

func TestDeviceLoginRejectsInvalidArgumentsBeforeRequests(t *testing.T) {
	for _, args := range [][]string{{"--timeout", "0s"}, {"--timeout", "-1s"}, {"--timeout", "6m"}, {"--output", "json"}, {"unexpected"}} {
		app := newDeviceAuthStateTestApp(t, &bytes.Buffer{}, newLockedDeviceCredentialStore(), func(_ *http.Request) (*http.Response, error) {
			t.Fatal("invalid arguments sent a request")
			return nil, errors.New("unexpected request")
		})
		if err := app.Run(context.Background(), append([]string{"auth", "device-login"}, args...)); err == nil {
			t.Fatalf("arguments accepted: %v", args)
		}
	}
}

func TestDeviceLoginDoesNotRetryFailedTokenExchange(t *testing.T) {
	for _, code := range []string{"access_denied", "expired_token", "invalid_grant", "server_error"} {
		t.Run(code, func(t *testing.T) {
			credentials := newLockedDeviceCredentialStore()
			stdout := &bytes.Buffer{}
			calls := 0
			app := newDeviceAuthStateTestApp(t, stdout, credentials, func(_ *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return jsonResponse(`{"device_code":"private","user_code":"user-a","verification_uri_complete":"https://myaccount.qfei.cn/device?user_code=user-a","expires_in":60,"interval":1}`), nil
				}
				response := jsonResponse(`{"error":"` + code + `"}`)
				response.StatusCode = http.StatusBadRequest
				return response, nil
			})
			err := app.Run(context.Background(), []string{"auth", "device-login", "--timeout", "3s"})
			if err == nil || calls != 2 || strings.Contains(stdout.String(), "Authorization: authorized") {
				t.Fatalf("calls=%d error=%v output=%s", calls, err, stdout)
			}
		})
	}
}

func TestDeviceLoginStopsIfPendingTransactionIsReplaced(t *testing.T) {
	credentials := &replacePendingDeviceStore{lockedDeviceCredentialStore: newLockedDeviceCredentialStore()}
	stdout := &bytes.Buffer{}
	calls := 0
	app := newDeviceAuthStateTestApp(t, stdout, credentials, func(_ *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return jsonResponse(`{"device_code":"old-private","user_code":"user-a","verification_uri_complete":"https://myaccount.qfei.cn/device?user_code=user-a","expires_in":60,"interval":1}`), nil
		}
		credentials.replace = true
		response := jsonResponse(`{"error":"authorization_pending"}`)
		response.StatusCode = http.StatusBadRequest
		return response, nil
	})
	err := app.Run(context.Background(), []string{"auth", "device-login", "--timeout", "4s"})
	if err == nil || !strings.Contains(err.Error(), "changed while waiting") || calls != 2 {
		t.Fatalf("calls=%d error=%v", calls, err)
	}
}

type replacePendingDeviceStore struct {
	*lockedDeviceCredentialStore
	replace bool
}

func (s *replacePendingDeviceStore) Save(profile string, value credential.DeviceCredential) error {
	if s.replace && value.Pending != nil && value.Pending.Status == credential.PendingStatusPending {
		value.Pending.VerificationURIComplete = "https://myaccount.qfei.cn/device?user_code=replaced"
	}
	return s.lockedDeviceCredentialStore.Save(profile, value)
}
