package cli_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image/png"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
)

func TestAuthDeviceInitReusesActivePendingTransaction(t *testing.T) {
	stdout := &bytes.Buffer{}
	credentials := newLockedDeviceCredentialStore()
	requests := 0
	app := newDeviceAuthStateTestApp(t, stdout, credentials, func(_ *http.Request) (*http.Response, error) {
		requests++
		return jsonResponse(`{"device_code":"device-a","user_code":"user-a","verification_uri":"https://myaccount.qfei.cn/device","verification_uri_complete":"https://myaccount.qfei.cn/device?user_code=user-a","expires_in":600}`), nil
	})

	if err := app.Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	first := decodeJSONObject(t, stdout.Bytes())
	stdout.Reset()

	if err := app.Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	second := decodeJSONObject(t, stdout.Bytes())
	if requests != 1 {
		t.Fatalf("device authorization requests = %d, want 1", requests)
	}
	if first["verification_uri_complete"] != second["verification_uri_complete"] || second["status"] != "pending" {
		t.Fatalf("unexpected reused output: first=%v second=%v", first, second)
	}
	if first["qr_code_data_uri"] != second["qr_code_data_uri"] {
		t.Fatalf("reused QR data URI changed: first=%v second=%v", first["qr_code_data_uri"], second["qr_code_data_uri"])
	}
	for _, output := range []map[string]any{first, second} {
		if got := output["expires_at_display"]; got != "2026-04-20 16:10:00" {
			t.Fatalf("expires_at_display = %v, want human-readable Beijing time", got)
		}
		if got := output["expires_at"]; got != "2026-04-20T16:10:00+08:00" {
			t.Fatalf("expires_at = %v, want backward-compatible RFC3339 value", got)
		}
	}
	assertMatchingQRCodeOutputs(t, first)
	assertMatchingQRCodeOutputs(t, second)
	stored, err := credentials.Load("contract")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Pending == nil || stored.Pending.DeviceCode != "device-a" || stored.Pending.VerificationURIComplete != first["verification_uri_complete"] {
		t.Fatalf("unexpected pending transaction: %+v", stored.Pending)
	}
	if stored.Pending.EffectiveStatus() != credential.PendingStatusPending {
		t.Fatalf("pending status = %q", stored.Pending.EffectiveStatus())
	}
}

func TestAuthDeviceInitReturnsMatchingQRCodeDataURI(t *testing.T) {
	stdout := &bytes.Buffer{}
	credentials := newLockedDeviceCredentialStore()
	app := newDeviceAuthStateTestApp(t, stdout, credentials, func(_ *http.Request) (*http.Response, error) {
		return jsonResponse(`{"device_code":"device-a","user_code":"user-a","verification_uri":"https://myaccount.qfei.cn/device","verification_uri_complete":"https://myaccount.qfei.cn/device?user_code=user-a","expires_in":600}`), nil
	})

	if err := app.Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	output := decodeJSONObject(t, stdout.Bytes())
	assertMatchingQRCodeOutputs(t, output)
	if stored, err := credentials.Load("contract"); err != nil {
		t.Fatal(err)
	} else if raw, err := json.Marshal(stored); err != nil {
		t.Fatal(err)
	} else if bytes.Contains(raw, []byte("data:image/png;base64,")) {
		t.Fatal("QR data URI must not be persisted in CredentialStore")
	}
}

func TestAuthDeviceInitRetriesOnceWhenTCPConnectionWasNotEstablished(t *testing.T) {
	stdout := &bytes.Buffer{}
	credentials := newLockedDeviceCredentialStore()
	requests := 0
	app := newDeviceAuthStateTestApp(t, stdout, credentials, func(_ *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return nil, &net.OpError{Op: "dial", Net: "tcp", Err: deviceAuthTimeoutError{}}
		}
		return jsonResponse(`{"device_code":"device-a","user_code":"user-a","verification_uri":"https://myaccount.qfei.cn/device","verification_uri_complete":"https://myaccount.qfei.cn/device?user_code=user-a","expires_in":600}`), nil
	})

	if err := app.Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("device authorization requests = %d, want 2", requests)
	}
	if got := decodeJSONObject(t, stdout.Bytes())["status"]; got != "pending" {
		t.Fatalf("status = %v, want pending", got)
	}
}

func TestAuthDeviceInitStopsAfterOneTCPDialRetry(t *testing.T) {
	requests := 0
	app := newDeviceAuthStateTestApp(t, &bytes.Buffer{}, newLockedDeviceCredentialStore(), func(_ *http.Request) (*http.Response, error) {
		requests++
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: deviceAuthTimeoutError{}}
	})

	if err := app.Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json"}); err == nil {
		t.Fatal("auth init error = nil, want failure")
	}
	if requests != 2 {
		t.Fatalf("device authorization requests = %d, want exactly 2", requests)
	}
}

func TestAuthDeviceInitDoesNotRetryAfterConnectionWasEstablished(t *testing.T) {
	tests := []struct {
		name      string
		transport roundTripFunc
	}{
		{
			name: "redirect target dial failed after original request was written",
			transport: func(request *http.Request) (*http.Response, error) {
				trace := httptrace.ContextClientTrace(request.Context())
				if trace != nil && trace.WroteRequest != nil {
					trace.WroteRequest(httptrace.WroteRequestInfo{})
				}
				return nil, &net.OpError{Op: "dial", Net: "tcp", Err: deviceAuthTimeoutError{}}
			},
		},
		{
			name: "response read timeout",
			transport: func(_ *http.Request) (*http.Response, error) {
				return nil, &net.OpError{Op: "read", Net: "tcp", Err: deviceAuthTimeoutError{}}
			},
		},
		{
			name: "http 503",
			transport: func(_ *http.Request) (*http.Response, error) {
				response := jsonResponse(`{"error":"temporarily_unavailable"}`)
				response.StatusCode = http.StatusServiceUnavailable
				return response, nil
			},
		},
		{
			name: "canceled request",
			transport: func(_ *http.Request) (*http.Response, error) {
				return nil, context.Canceled
			},
		},
		{
			name: "dial stopped by context deadline",
			transport: func(_ *http.Request) (*http.Response, error) {
				return nil, &net.OpError{Op: "dial", Net: "tcp", Err: context.DeadlineExceeded}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			app := newDeviceAuthStateTestApp(t, &bytes.Buffer{}, newLockedDeviceCredentialStore(), func(request *http.Request) (*http.Response, error) {
				requests++
				return tt.transport(request)
			})

			if err := app.Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json"}); err == nil {
				t.Fatal("auth init error = nil, want failure")
			}
			if requests != 1 {
				t.Fatalf("device authorization requests = %d, want 1", requests)
			}
		})
	}
}

func TestAuthDeviceInitUsesTaskScopedQRCodePathInWorkBuddy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	outputs := make([]map[string]any, 0, 2)
	for _, taskID := range []string{"task-a", "task-b"} {
		stdout := &bytes.Buffer{}
		credentials := newLockedDeviceCredentialStore()
		app := newDeviceAuthStateTestAppWithLookupEnv(t, stdout, credentials, func(_ *http.Request) (*http.Response, error) {
			return jsonResponse(`{"device_code":"device-a","user_code":"user-a","verification_uri":"https://myaccount.qfei.cn/device","verification_uri_complete":"https://myaccount.qfei.cn/device?user_code=user-a","expires_in":600}`), nil
		}, func(name string) (string, bool) {
			if name == "CODEBUDDY_SESSION_ID" {
				return taskID, true
			}
			return "", false
		})

		if err := app.Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json"}); err != nil {
			t.Fatal(err)
		}
		outputs = append(outputs, decodeJSONObject(t, stdout.Bytes()))
	}

	if outputs[0]["qr_code_path"] == outputs[1]["qr_code_path"] {
		t.Fatalf("WorkBuddy tasks share QR path: %v", outputs[0]["qr_code_path"])
	}
	digest := sha256.Sum256([]byte("contract\x00task-a"))
	if got, want := filepath.Base(outputs[0]["qr_code_path"].(string)), "device-auth-"+hex.EncodeToString(digest[:8])+".png"; got != want {
		t.Fatalf("WorkBuddy QR file = %q, want backward-compatible %q", got, want)
	}
}

func TestAuthDeviceInitPersistsPendingBeforeQRCodeWrite(t *testing.T) {
	stdout := &bytes.Buffer{}
	credentials := newLockedDeviceCredentialStore()
	requests := 0
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".contract-cli"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".contract-cli", "artifacts"), []byte("not-a-directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := newDeviceAuthStateTestAppWithWorkspace(t, stdout, credentials, func(_ *http.Request) (*http.Response, error) {
		requests++
		return jsonResponse(`{"device_code":"device-a","user_code":"user-a","verification_uri":"https://myaccount.qfei.cn/device","verification_uri_complete":"https://myaccount.qfei.cn/device?user_code=user-a","expires_in":600}`), nil
	}, workspace)

	err := app.Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json"})
	if err == nil || !strings.Contains(err.Error(), "create qr directory") {
		t.Fatalf("error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("device authorization requests = %d, want 1", requests)
	}
	stored, loadErr := credentials.Load("contract")
	if loadErr != nil {
		t.Fatalf("load pending after QR failure: %v", loadErr)
	}
	if stored.Pending == nil || stored.Pending.DeviceCode != "device-a" || stored.Pending.VerificationURIComplete == "" {
		t.Fatalf("pending after QR failure = %+v", stored.Pending)
	}
}

func TestAuthDeviceInitRequiresExplicitRestartForUncertainTransaction(t *testing.T) {
	stdout := &bytes.Buffer{}
	credentials := newLockedDeviceCredentialStore()
	credentials.values["contract"] = credential.DeviceCredential{DeviceProfile: deviceSnapshotFixture(deviceAuthStateProfile()), Pending: &credential.PendingTransaction{
		Status: credential.PendingStatusUncertain, DeviceCode: "device-old",
		VerificationURIComplete: "https://myaccount.qfei.cn/device?user_code=old",
		TokenEndpoint:           "https://myaccount.qfei.cn/token", ClientID: "client-a", ExpiresAt: fixedCLINow().Add(10 * time.Minute),
	}}
	requests := 0
	app := newDeviceAuthStateTestApp(t, stdout, credentials, func(_ *http.Request) (*http.Response, error) {
		requests++
		return jsonResponse(`{"device_code":"device-new","user_code":"user-new","verification_uri":"https://myaccount.qfei.cn/device","verification_uri_complete":"https://myaccount.qfei.cn/device?user_code=user-new","expires_in":600}`), nil
	})

	if err := app.Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	output := decodeJSONObject(t, stdout.Bytes())
	if got := output["status"]; got != "uncertain" {
		t.Fatalf("status = %v, want uncertain", got)
	}
	if _, ok := output["qr_code_data_uri"]; ok {
		t.Fatalf("uncertain output exposed QR data URI: %v", output)
	}
	if _, ok := output["expires_at_display"]; ok {
		t.Fatalf("uncertain output exposed expiry display: %v", output)
	}
	if requests != 0 {
		t.Fatalf("requests before explicit restart = %d, want 0", requests)
	}

	stdout.Reset()
	if err := app.Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json", "--restart"}); err != nil {
		t.Fatal(err)
	}
	if got := decodeJSONObject(t, stdout.Bytes())["status"]; got != "pending" {
		t.Fatalf("restart status = %v, want pending", got)
	}
	if requests != 1 {
		t.Fatalf("restart requests = %d, want 1", requests)
	}
	stored, err := credentials.Load("contract")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Pending == nil || stored.Pending.DeviceCode != "device-new" || stored.Pending.EffectiveStatus() != credential.PendingStatusPending {
		t.Fatalf("unexpected restarted pending: %+v", stored.Pending)
	}
}

func TestAuthDeviceInitRestartWithoutTransactionFails(t *testing.T) {
	tests := []struct {
		name       string
		credential *credential.DeviceCredential
	}{
		{name: "credential missing"},
		{name: "token exists without pending", credential: &credential.DeviceCredential{DeviceProfile: deviceSnapshotFixture(deviceAuthStateProfile()), Token: &config.Token{AccessToken: "access-a"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credentials := newLockedDeviceCredentialStore()
			if tt.credential != nil {
				credentials.values["contract"] = *tt.credential
			}
			requests := 0
			app := newDeviceAuthStateTestApp(t, &bytes.Buffer{}, credentials, func(_ *http.Request) (*http.Response, error) {
				requests++
				return nil, errors.New("unexpected request")
			})

			err := app.Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json", "--restart"})
			if err == nil || !strings.Contains(err.Error(), "no device authorization to restart") {
				t.Fatalf("error = %v", err)
			}
			if requests != 0 {
				t.Fatalf("requests = %d, want 0", requests)
			}
		})
	}
}

func TestAuthDeviceCompleteMarksUncertainAndDoesNotRetryAfterTransportFailure(t *testing.T) {
	stdout := &bytes.Buffer{}
	credentials := newLockedDeviceCredentialStore()
	credentials.values["contract"] = pendingCredential(credential.PendingStatusPending)
	requests := 0
	app := newDeviceAuthStateTestApp(t, stdout, credentials, func(_ *http.Request) (*http.Response, error) {
		requests++
		stored, err := credentials.Load("contract")
		if err != nil {
			t.Fatal(err)
		}
		if stored.Pending == nil || stored.Pending.EffectiveStatus() != credential.PendingStatusChecking {
			t.Fatalf("status before token request = %+v, want checking", stored.Pending)
		}
		return nil, deviceAuthTimeoutError{}
	})

	if err := app.Run(context.Background(), []string{"auth", "complete", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	if got := decodeJSONObject(t, stdout.Bytes())["status"]; got != "uncertain" {
		t.Fatalf("status = %v, want uncertain", got)
	}
	stored, err := credentials.Load("contract")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Pending == nil || stored.Pending.EffectiveStatus() != credential.PendingStatusUncertain {
		t.Fatalf("stored pending = %+v, want uncertain", stored.Pending)
	}

	stdout.Reset()
	if err := app.Run(context.Background(), []string{"auth", "complete", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	if got := decodeJSONObject(t, stdout.Bytes())["status"]; got != "uncertain" {
		t.Fatalf("repeat status = %v, want uncertain", got)
	}
	if requests != 1 {
		t.Fatalf("token endpoint requests = %d, want 1", requests)
	}
}

func TestAuthDeviceCompleteDoesNotRetryWhenTCPConnectionWasNotEstablished(t *testing.T) {
	stdout := &bytes.Buffer{}
	credentials := newLockedDeviceCredentialStore()
	credentials.values["contract"] = pendingCredential(credential.PendingStatusPending)
	requests := 0
	app := newDeviceAuthStateTestApp(t, stdout, credentials, func(_ *http.Request) (*http.Response, error) {
		requests++
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: deviceAuthTimeoutError{}}
	})

	if err := app.Run(context.Background(), []string{"auth", "complete", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	if got := decodeJSONObject(t, stdout.Bytes())["status"]; got != "uncertain" {
		t.Fatalf("status = %v, want uncertain", got)
	}
	if requests != 1 {
		t.Fatalf("token endpoint requests = %d, want 1", requests)
	}
}

func TestAuthDeviceCompleteRestoresPendingAfterAuthorizationPending(t *testing.T) {
	stdout := &bytes.Buffer{}
	credentials := newLockedDeviceCredentialStore()
	credentials.values["contract"] = pendingCredential(credential.PendingStatusPending)
	requests := 0
	app := newDeviceAuthStateTestApp(t, stdout, credentials, func(_ *http.Request) (*http.Response, error) {
		requests++
		response := jsonResponse(`{"error":"authorization_pending"}`)
		response.StatusCode = http.StatusBadRequest
		return response, nil
	})

	for i := 0; i < 2; i++ {
		stdout.Reset()
		if err := app.Run(context.Background(), []string{"auth", "complete", "--profile", "contract", "--output", "json"}); err != nil {
			t.Fatal(err)
		}
		if got := decodeJSONObject(t, stdout.Bytes())["status"]; got != "pending" {
			t.Fatalf("attempt %d status = %v, want pending", i+1, got)
		}
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want one per explicit command", requests)
	}
	stored, err := credentials.Load("contract")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Pending == nil || stored.Pending.EffectiveStatus() != credential.PendingStatusPending {
		t.Fatalf("stored pending = %+v", stored.Pending)
	}
}

func TestAuthDeviceCompleteTerminalStatesRequireRestart(t *testing.T) {
	tests := []struct {
		name       string
		grantError string
		wantStatus string
		stored     credential.PendingStatus
	}{
		{name: "denied", grantError: "access_denied", wantStatus: "denied", stored: credential.PendingStatusDenied},
		{name: "expired", grantError: "expired_token", wantStatus: "expired", stored: credential.PendingStatusExpired},
		{name: "invalid grant", grantError: "invalid_grant", wantStatus: "restart_required", stored: credential.PendingStatusInvalidGrant},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			credentials := newLockedDeviceCredentialStore()
			credentials.values["contract"] = pendingCredential(credential.PendingStatusPending)
			requests := 0
			app := newDeviceAuthStateTestApp(t, stdout, credentials, func(_ *http.Request) (*http.Response, error) {
				requests++
				response := jsonResponse(`{"error":"` + tt.grantError + `"}`)
				response.StatusCode = http.StatusBadRequest
				return response, nil
			})

			if err := app.Run(context.Background(), []string{"auth", "complete", "--profile", "contract", "--output", "json"}); err != nil {
				t.Fatal(err)
			}
			if got := decodeJSONObject(t, stdout.Bytes())["status"]; got != tt.wantStatus {
				t.Fatalf("complete status = %v, want %s", got, tt.wantStatus)
			}
			stored, err := credentials.Load("contract")
			if err != nil {
				t.Fatal(err)
			}
			if stored.Pending == nil || stored.Pending.EffectiveStatus() != tt.stored {
				t.Fatalf("stored pending = %+v, want %s", stored.Pending, tt.stored)
			}

			stdout.Reset()
			if err := app.Run(context.Background(), []string{"auth", "init", "--profile", "contract", "--output", "json"}); err != nil {
				t.Fatal(err)
			}
			initOutput := decodeJSONObject(t, stdout.Bytes())
			wantInitStatus := tt.wantStatus
			if got := initOutput["status"]; got != wantInitStatus {
				t.Fatalf("init status = %v, want %s", got, wantInitStatus)
			}
			if _, ok := initOutput["qr_code_data_uri"]; ok {
				t.Fatalf("terminal init output exposed QR data URI: %v", initOutput)
			}
			if requests != 1 {
				t.Fatalf("requests after blocked init = %d, want 1", requests)
			}
		})
	}
}

func assertMatchingQRCodeOutputs(t *testing.T, output map[string]any) {
	t.Helper()
	const prefix = "data:image/png;base64,"
	dataURI, ok := output["qr_code_data_uri"].(string)
	if !ok || !strings.HasPrefix(dataURI, prefix) {
		t.Fatalf("qr_code_data_uri = %v, want PNG data URI", output["qr_code_data_uri"])
	}
	pngBytes, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(dataURI, prefix))
	if err != nil {
		t.Fatalf("decode QR data URI: %v", err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("decode QR PNG: %v", err)
	}
	if config.Width != 320 || config.Height != 320 {
		t.Fatalf("QR dimensions = %dx%d, want 320x320", config.Width, config.Height)
	}
	path, ok := output["qr_code_path"].(string)
	if !ok || strings.TrimSpace(path) == "" {
		t.Fatalf("qr_code_path = %v", output["qr_code_path"])
	}
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read QR file: %v", err)
	}
	if !bytes.Equal(pngBytes, fileBytes) {
		t.Fatal("QR data URI and qr_code_path contain different PNG bytes")
	}
}

func TestAuthDeviceCompleteAllowsOnlyOneConcurrentExchange(t *testing.T) {
	credentials := newLockedDeviceCredentialStore()
	credentials.values["contract"] = pendingCredential(credential.PendingStatusPending)
	workspace := t.TempDir()
	entered := make(chan struct{})
	release := make(chan struct{})
	requests := 0
	var requestsMu sync.Mutex
	transport := func(_ *http.Request) (*http.Response, error) {
		requestsMu.Lock()
		requests++
		requestsMu.Unlock()
		close(entered)
		<-release
		response := jsonResponse(`{"error":"authorization_pending"}`)
		response.StatusCode = http.StatusBadRequest
		return response, nil
	}
	firstOut := &bytes.Buffer{}
	secondOut := &bytes.Buffer{}
	first := newDeviceAuthStateTestAppWithWorkspace(t, firstOut, credentials, transport, workspace)
	second := newDeviceAuthStateTestAppWithWorkspace(t, secondOut, credentials, transport, workspace)

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- first.Run(context.Background(), []string{"auth", "complete", "--profile", "contract", "--output", "json"})
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("first token request did not start")
	}
	if err := second.Run(context.Background(), []string{"auth", "complete", "--profile", "contract", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	if got := decodeJSONObject(t, secondOut.Bytes())["status"]; got != "busy" {
		t.Fatalf("second status = %v, want busy", got)
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	requestsMu.Lock()
	defer requestsMu.Unlock()
	if requests != 1 {
		t.Fatalf("token endpoint requests = %d, want 1", requests)
	}
}

func TestAuthDeviceStatusReportsFailClosedPendingStates(t *testing.T) {
	tests := []struct {
		status credential.PendingStatus
		want   string
	}{
		{status: credential.PendingStatusPending, want: "Authorization: pending"},
		{status: credential.PendingStatusChecking, want: "Authorization: uncertain"},
		{status: credential.PendingStatusUncertain, want: "Authorization: uncertain"},
		{status: credential.PendingStatusDenied, want: "Authorization: denied"},
		{status: credential.PendingStatusExpired, want: "Authorization: expired"},
		{status: credential.PendingStatusInvalidGrant, want: "Authorization: restart_required"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			stdout := &bytes.Buffer{}
			credentials := newLockedDeviceCredentialStore()
			credentials.values["contract"] = pendingCredential(tt.status)
			app := newDeviceAuthStateTestApp(t, stdout, credentials, func(_ *http.Request) (*http.Response, error) {
				return nil, errors.New("unexpected request")
			})

			if err := app.Run(context.Background(), []string{"auth", "status", "--profile", "contract", "--as", "user"}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stdout.String(), tt.want) {
				t.Fatalf("status output = %s, want %q", stdout.String(), tt.want)
			}
		})
	}
}

func TestAuthDeviceStatusReportsExpiredWhenPendingDeadlinePassed(t *testing.T) {
	stdout := &bytes.Buffer{}
	credentials := newLockedDeviceCredentialStore()
	value := pendingCredential(credential.PendingStatusPending)
	value.Pending.ExpiresAt = fixedCLINow().Add(-time.Second)
	credentials.values["contract"] = value
	app := newDeviceAuthStateTestApp(t, stdout, credentials, func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("unexpected request")
	})

	if err := app.Run(context.Background(), []string{"auth", "status", "--profile", "contract", "--as", "user"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Authorization: expired") {
		t.Fatalf("status output = %s", stdout.String())
	}
}

func newDeviceAuthStateTestApp(
	t *testing.T,
	stdout *bytes.Buffer,
	credentials credential.Store,
	transport roundTripFunc,
) *cli.App {
	t.Helper()
	return newDeviceAuthStateTestAppWithWorkspace(t, stdout, credentials, transport, t.TempDir())
}

func newDeviceAuthStateTestAppWithWorkspace(
	t *testing.T,
	stdout *bytes.Buffer,
	credentials credential.Store,
	transport roundTripFunc,
	workspace string,
) *cli.App {
	t.Helper()
	return newDeviceAuthStateTestAppWithLookupEnv(t, stdout, credentials, transport, func(name string) (string, bool) {
		if name == "SKILL_SESSION_WORKSPACE" {
			return workspace, true
		}
		return "", false
	})
}

func newDeviceAuthStateTestAppWithLookupEnv(
	t *testing.T,
	stdout *bytes.Buffer,
	credentials credential.Store,
	transport roundTripFunc,
	lookupEnv func(string) (string, bool),
) *cli.App {
	t.Helper()
	store := config.NewStore(t.TempDir())
	profile := deviceAuthStateProfile()
	profile.Identities.User.AuthMode = credentialTestAuthMode(credentials)
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatal(err)
	}
	return cli.New(cli.Options{
		Stdout: stdout, Store: store, CredentialStore: credentials, Now: fixedCLINow,
		LookupEnv:  lookupEnv,
		HTTPClient: &http.Client{Transport: transport},
	})
}

func deviceAuthStateProfile() config.Profile {
	return productionProfileFixture(config.Profile{
		Name: "contract", Environment: "prod", Resource: "https://open.qfei.cn", OpenPlatformBaseURL: "https://open.qfei.cn",
		BusinessType: "contract", ClientName: "contract-cli", Identities: config.Identities{User: config.UserIdentity{
			AuthMode: config.UserAuthModeDevice, DeviceAuthorizationEndpoint: "https://myaccount.qfei.cn/device",
			TokenEndpoint: "https://myaccount.qfei.cn/token", DeviceClientID: "client-a", DeviceScope: "contract:full",
		}},
	})
}

func credentialTestAuthMode(credentials credential.Store) string {
	if _, err := credentials.Load("contract"); err == nil {
		return config.UserAuthModeDevice
	}
	return config.UserAuthModeAuthorizationCode
}

func pendingCredential(status credential.PendingStatus) credential.DeviceCredential {
	return credential.DeviceCredential{DeviceProfile: deviceSnapshotFixture(deviceAuthStateProfile()), Pending: &credential.PendingTransaction{
		Status: status, DeviceCode: "device-a", VerificationURIComplete: "https://myaccount.qfei.cn/device?user_code=user-a",
		TokenEndpoint: "https://myaccount.qfei.cn/token", ClientID: "client-a", ExpiresAt: fixedCLINow().Add(10 * time.Minute),
	}}
}

type lockedDeviceCredentialStore struct {
	mu     sync.Mutex
	values map[string]credential.DeviceCredential
}

func newLockedDeviceCredentialStore() *lockedDeviceCredentialStore {
	return &lockedDeviceCredentialStore{values: map[string]credential.DeviceCredential{}}
}

func (s *lockedDeviceCredentialStore) Load(profileName string) (credential.DeviceCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[profileName]
	if !ok {
		return credential.DeviceCredential{}, credential.ErrCredentialNotFound
	}
	return cloneDeviceCredential(value), nil
}

func (s *lockedDeviceCredentialStore) Save(profileName string, value credential.DeviceCredential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[profileName] = cloneDeviceCredential(value)
	return nil
}

func (s *lockedDeviceCredentialStore) Delete(profileName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, profileName)
	return nil
}

func cloneDeviceCredential(value credential.DeviceCredential) credential.DeviceCredential {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	var cloned credential.DeviceCredential
	if err := json.Unmarshal(raw, &cloned); err != nil {
		panic(err)
	}
	return cloned
}

type deviceAuthTimeoutError struct{}

func (deviceAuthTimeoutError) Error() string   { return "device token endpoint timed out" }
func (deviceAuthTimeoutError) Timeout() bool   { return true }
func (deviceAuthTimeoutError) Temporary() bool { return true }
