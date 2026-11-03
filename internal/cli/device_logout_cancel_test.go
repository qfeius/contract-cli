package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
)

const (
	deviceLogoutCancelAccess  = "device-logout-fixture-private-access"
	deviceLogoutCancelRefresh = "device-logout-fixture-private-refresh"
)

func TestDeviceLogoutCancellationBoundaries(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"started-log", "revoke-log", "before-delete", "after-revoke", "after-delete"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			started, triggered := false, false
			revokeCalls := 0
			refresh := ""
			if phase == "revoke-log" || phase == "after-revoke" {
				refresh = deviceLogoutCancelRefresh
			}
			fixture := newDeviceLogoutCancelFixture(t, refresh, func(message string) {
				if message == "Device logout started" {
					started = true
				}
				if phase == "started-log" && message == "Device logout started" || phase == "revoke-log" && message == "device authorization revoke started" {
					triggered = true
					cancel()
				}
			}, func(request *http.Request) (*http.Response, error) {
				revokeCalls++
				if request.URL.String() != "https://myaccount.qfei.cn/api/public/oauth/revoke/contract" {
					t.Error("logout sent a request to an unexpected endpoint")
				}
				if phase == "after-revoke" {
					triggered = true
					cancel()
				}
				return deviceTokenTestResponse(http.StatusNoContent, ""), nil
			})
			fixture.credentials.onLoad = func() {
				if phase == "before-delete" && started {
					triggered = true
					cancel()
				}
			}
			fixture.credentials.afterDelete = func() {
				if phase == "after-delete" {
					triggered = true
					cancel()
				}
			}
			profileBefore := deviceLogoutCancelRead(t, fixture.app.store.Path())
			credentialsBefore := deviceLogoutCancelRead(t, fixture.credentials.path)
			code := fixture.app.Execute(ctx, []string{"auth", "logout", "--profile", "contract", "--as", "user", "--output", "json"})
			if !triggered || code != 130 || fixture.stdout.Len() != 0 {
				t.Errorf("cancelled logout result: triggered=%t code=%d stdout=%s", triggered, code, fixture.stdout)
			}
			lines := bytes.Split(bytes.TrimSpace(fixture.stderr.Bytes()), []byte("\n"))
			for _, line := range lines {
				if !json.Valid(line) {
					t.Errorf("logout stderr mixed JSON and text: %q", line)
				}
			}
			var failure struct {
				OK    *bool `json:"ok"`
				Error struct {
					Type      string `json:"type"`
					Cancelled bool   `json:"cancelled"`
					Details   struct {
						State  string         `json:"state"`
						Result map[string]any `json:"result"`
					} `json:"details"`
				} `json:"error"`
			}
			if err := json.Unmarshal(lines[len(lines)-1], &failure); err != nil || failure.OK == nil || *failure.OK || failure.Error.Type != "cancelled" || !failure.Error.Cancelled {
				t.Errorf("logout lost its final cancellation JSON: %s (%v)", lines[len(lines)-1], err)
			}
			if phase == "after-revoke" || phase == "after-delete" {
				wantRevoked := phase == "after-revoke"
				if fixture.credentials.deletes != 1 {
					t.Errorf("confirmed logout must complete one local deletion: deletes=%d", fixture.credentials.deletes)
				}
				if _, err := os.Stat(fixture.credentials.path); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("completed logout retained the credential file: %v", err)
				}
				result := failure.Error.Details.Result
				if failure.Error.Details.State != "committed" || result["profile"] != "contract" || result["device_token_revoked"] != wantRevoked || result["device_credentials_cleared"] != true {
					t.Errorf("late cancellation hid safe committed logout steps: %s", lines[len(lines)-1])
				}
				if revokeCalls != map[bool]int{true: 1, false: 0}[wantRevoked] {
					t.Errorf("logout retried or unexpectedly revoked: calls=%d", revokeCalls)
				}
			} else {
				if fixture.credentials.deletes != 0 || revokeCalls != 0 || !bytes.Equal(deviceLogoutCancelRead(t, fixture.credentials.path), credentialsBefore) {
					t.Errorf("logout wrote after cancellation: deletes=%d requests=%d", fixture.credentials.deletes, revokeCalls)
				}
				if failure.Error.Details.State == "committed" {
					t.Error("write-before-cancel was incorrectly reported as committed")
				}
			}
			if !bytes.Equal(deviceLogoutCancelRead(t, fixture.app.store.Path()), profileBefore) {
				t.Error("cancelled logout continued saving the local profile")
			}
			if phase == "started-log" {
				if _, err := os.Stat(filepath.Join(fixture.workspace, ".contract-cli", "locks")); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("cancelled logout started acquiring a credential lock: %v", err)
				}
			}
			for _, secret := range []string{deviceLogoutCancelAccess, deviceLogoutCancelRefresh} {
				if strings.Contains(fixture.stdout.String()+fixture.stderr.String(), secret) {
					t.Error("logout output leaked a credential")
				}
			}
		})
	}
}

func TestDeviceLogoutConfirmedRevokeCancellationKeepsDeleteFailure(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	fixture := newDeviceLogoutCancelFixture(t, deviceLogoutCancelRefresh, nil, func(*http.Request) (*http.Response, error) {
		calls++
		cancel()
		return deviceTokenTestResponse(http.StatusNoContent, ""), nil
	})
	deleteFailure := errors.New("fixture secure-store delete failed")
	fixture.credentials.deleteErr = deleteFailure
	_, err := fixture.app.deviceAuthLogout(ctx, fixture.profile)
	if !errors.Is(err, deleteFailure) || !errors.Is(err, context.Canceled) || calls != 1 || fixture.credentials.deletes != 1 {
		t.Fatalf("confirmed revoke cleanup lost errors or retried: err=%v calls=%d deletes=%d", err, calls, fixture.credentials.deletes)
	}
	var detailed commandFailureDetailer
	if !errors.As(err, &detailed) {
		t.Fatalf("confirmed revocation was hidden by the local delete failure: %v", err)
	}
	body, marshalErr := json.Marshal(detailed.commandFailureDetails())
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	var details struct {
		State  string         `json:"state"`
		Result map[string]any `json:"result"`
	}
	if unmarshalErr := json.Unmarshal(body, &details); unmarshalErr != nil || details.State != "committed" || details.Result["device_token_revoked"] != true || details.Result["device_credentials_cleared"] != false {
		t.Fatalf("confirmed remote revoke and failed local cleanup were misreported: %s (%v)", body, unmarshalErr)
	}
	if _, statErr := os.Stat(fixture.credentials.path); statErr != nil {
		t.Fatalf("failed deletion unexpectedly removed the credential file: %v", statErr)
	}
}

type deviceLogoutCancelFixture struct {
	app            *App
	credentials    *deviceLogoutCancelStore
	profile        config.Profile
	stdout, stderr *bytes.Buffer
	workspace      string
}

func newDeviceLogoutCancelFixture(t *testing.T, refresh string, onLog func(string), transport deviceRoundTripFunc) deviceLogoutCancelFixture {
	t.Helper()
	state, workspace := t.TempDir(), t.TempDir()
	store := config.NewStore(state)
	profile := validProductionProfile()
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatal(err)
	}
	credentials := &deviceLogoutCancelStore{path: filepath.Join(state, "fixture-device-credential.json")}
	if err := credentials.Save(profile.Name, credential.DeviceCredential{DeviceProfile: snapshotDeviceProfile(profile), Token: &config.Token{AccessToken: deviceLogoutCancelAccess, RefreshToken: refresh}}); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(deviceLogoutCancelLogWriter{output: stderr, onLog: onLog}, nil))
	app := New(Options{
		Store: store, Secrets: config.NewSecretsStore(state), CredentialStore: credentials,
		Stdout: stdout, Stderr: stderr, Logger: logger,
		LookupEnv: func(key string) (string, bool) {
			switch key {
			case "SKILL_SESSION_WORKSPACE":
				return workspace, true
			case "CONTRACT_CLI_NO_UPDATE_CHECK":
				return "1", true
			default:
				return "", false
			}
		},
		HTTPClient: &http.Client{Transport: transport},
	})
	return deviceLogoutCancelFixture{app: app, credentials: credentials, profile: profile, stdout: stdout, stderr: stderr, workspace: workspace}
}

type deviceLogoutCancelStore struct {
	path        string
	deletes     int
	deleteErr   error
	onLoad      func()
	afterDelete func()
}

func (store *deviceLogoutCancelStore) Load(string) (credential.DeviceCredential, error) {
	body, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return credential.DeviceCredential{}, credential.ErrCredentialNotFound
	}
	if err != nil {
		return credential.DeviceCredential{}, err
	}
	var value credential.DeviceCredential
	if err := json.Unmarshal(body, &value); err != nil {
		return credential.DeviceCredential{}, err
	}
	if store.onLoad != nil {
		store.onLoad()
	}
	return value, nil
}

func (store *deviceLogoutCancelStore) Save(_ string, value credential.DeviceCredential) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(store.path, body, 0600)
}

func (store *deviceLogoutCancelStore) Delete(string) error {
	store.deletes++
	if store.deleteErr != nil {
		return store.deleteErr
	}
	if err := os.Remove(store.path); err != nil {
		return err
	}
	if store.afterDelete != nil {
		store.afterDelete()
	}
	return nil
}

type deviceLogoutCancelLogWriter struct {
	output *bytes.Buffer
	onLog  func(string)
}

func (writer deviceLogoutCancelLogWriter) Write(body []byte) (int, error) {
	count, err := writer.output.Write(body)
	var record struct {
		Message string `json:"msg"`
	}
	if writer.onLog != nil && json.Unmarshal(body, &record) == nil {
		writer.onLog(record.Message)
	}
	return count, err
}

func deviceLogoutCancelRead(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
