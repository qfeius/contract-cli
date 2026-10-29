package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
	updatecheck "cn.qfei/contract-cli/internal/update"
)

// Cancel only at a deterministic command boundary before the first stdout
// write. The default command logger still owns formatting and --debug policy.
type nonBusinessBoundaryLogWriter struct {
	output        *bytes.Buffer
	cancel        context.CancelFunc
	message       string
	messageSuffix string
	triggered     bool
}

func (writer *nonBusinessBoundaryLogWriter) Write(body []byte) (int, error) {
	count, err := writer.output.Write(body)
	var record struct {
		Message string `json:"msg"`
	}
	isJSON := json.Unmarshal(body, &record) == nil
	isMessage := isJSON && (record.Message == writer.message || (writer.messageSuffix != "" && strings.HasSuffix(record.Message, writer.messageSuffix)))
	if !isMessage {
		isMessage = bytes.Contains(body, []byte("msg="+strconv.Quote(writer.message)))
	}
	if isMessage && !writer.triggered {
		writer.triggered = true
		writer.cancel()
	}
	return count, err
}

func TestNonBusinessCancelledVersionStartedKeepsStdoutEmpty(t *testing.T) {
	for _, alias := range []string{"version", "--version", "-version", "-v"} {
		for _, format := range []string{"json", "default-text"} {
			t.Run(alias+"/"+format, func(t *testing.T) {
				fixture := newNonBusinessJSONFixture(t)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				diagnostic := &nonBusinessBoundaryLogWriter{output: fixture.stderr, cancel: cancel, message: "version command started"}
				fixture.options.Stderr = diagnostic
				args := nonBusinessBoundaryFormatArgs([]string{alias, "--debug"}, format)
				code := cli.New(fixture.options).Execute(ctx, args)
				if !diagnostic.triggered || !errors.Is(ctx.Err(), context.Canceled) {
					t.Fatal("version fixture did not cancel at its started boundary")
				}
				nonBusinessBoundaryAssertCancelled(t, code, fixture.stdout.Bytes(), fixture.stderr.Bytes(), format)
			})
		}
	}
}

func TestNonBusinessCancelledAuthCompletionReportsCommittedState(t *testing.T) {
	for _, test := range []struct {
		operation, message string
		args               []string
	}{
		{"auth logout", "auth logout completed", []string{"auth", "logout", "--as", "user"}},
		{"auth use", "default identity switched", []string{"auth", "use", "--as", "app"}},
	} {
		t.Run(test.operation, func(t *testing.T) {
			fixture := newNonBusinessJSONFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			diagnostic := &nonBusinessBoundaryLogWriter{output: fixture.stderr, cancel: cancel, message: test.message}
			fixture.options.Stderr = diagnostic
			args := append(append([]string{}, test.args...), "--profile", "contract", "--output", "json", "--debug")
			code := cli.New(fixture.options).Execute(ctx, args)
			if !diagnostic.triggered || !errors.Is(ctx.Err(), context.Canceled) {
				t.Fatal("auth fixture did not cancel after local state committed")
			}
			failure := nonBusinessBoundaryAssertCancelled(t, code, fixture.stdout.Bytes(), fixture.stderr.Bytes(), "json")
			details, _ := failure["details"].(map[string]any)
			if details["state"] != "committed" || details["operation"] != test.operation {
				t.Errorf("committed auth operation was hidden or misreported: %#v", details)
			}
			profile, err := fixture.store.GetProfile("contract")
			if err != nil {
				t.Fatal(err)
			}
			switch test.operation {
			case "auth logout":
				if profile.Identities.User.Token != nil {
					t.Error("completed logout was incorrectly rolled back")
				}
			case "auth use":
				if profile.DefaultIdentity != config.IdentityApp || profile.Identities.App.Token == nil || profile.Identities.App.Token.AccessToken != nonBusinessJSONAppToken {
					t.Error("completed identity change was incorrectly rolled back")
				}
			}
			if secret, found, err := fixture.options.Secrets.Get(config.AppSecretKey("contract")); err != nil || !found || secret != nonBusinessJSONSecret {
				t.Fatalf("committed auth boundary changed unrelated credentials: found=%t err=%v", found, err)
			}
		})
	}
}

func TestNonBusinessCancelledUpdateCompletionDoesNotSaveCache(t *testing.T) {
	for _, format := range []string{"json", "legacy-json", "default-text"} {
		for _, cached := range []bool{false, true} {
			t.Run(format+"/cached="+strconv.FormatBool(cached), func(t *testing.T) {
				fixture := newNonBusinessJSONFixture(t)
				cachePath := filepath.Join(filepath.Dir(fixture.store.Path()), "update-check.json")
				var before []byte
				if cached {
					cache := updatecheck.Cache{CheckedAt: time.Unix(1, 0).UTC(), Channel: "latest", CurrentVersion: "1.9.7", LatestVersion: "1.9.7", UpdateAvailable: false}
					if err := updatecheck.SaveCache(cachePath, cache); err != nil {
						t.Fatal(err)
					}
					var err error
					before, err = os.ReadFile(cachePath)
					if err != nil {
						t.Fatal(err)
					}
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				diagnostic := &nonBusinessBoundaryLogWriter{output: fixture.stderr, cancel: cancel, message: "update check completed"}
				fixture.options.Stderr = diagnostic
				fixture.options.UpdateCurrentVersion = "1.9.7"
				fixture.options.UpdateRegistryURL = "https://registry.example.test/package"
				calls := 0
				fixture.options.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					calls++
					if request.URL.String() != "https://registry.example.test/package" {
						t.Errorf("unexpected mock update endpoint: %s", request.URL)
					}
					return jsonResponse(`{"dist-tags":{"latest":"1.9.8"}}`), nil
				})}
				args := nonBusinessBoundaryFormatArgs([]string{"update", "check", "--debug"}, format)
				code := cli.New(fixture.options).Execute(ctx, args)
				if !diagnostic.triggered || calls != 1 || !errors.Is(ctx.Err(), context.Canceled) {
					t.Fatalf("update completion cancellation not reached: triggered=%t calls=%d", diagnostic.triggered, calls)
				}
				nonBusinessBoundaryAssertCancelled(t, code, fixture.stdout.Bytes(), fixture.stderr.Bytes(), format)
				after, err := os.ReadFile(cachePath)
				if cached {
					if err != nil || !bytes.Equal(before, after) {
						t.Errorf("cancelled update overwrote the prior cache: %v", err)
					}
				} else if !errors.Is(err, os.ErrNotExist) {
					t.Errorf("cancelled update created a cache: %v", err)
				}
			})
		}
	}
}

// Deliver a complete, valid response and cancel before the caller can perform
// local state changes. A successful body read must not imply a live context.
type nonBusinessBoundaryResponseBody struct {
	reader    *strings.Reader
	cancel    context.CancelFunc
	delivered bool
}

func (body *nonBusinessBoundaryResponseBody) Read(buffer []byte) (int, error) {
	count, err := body.reader.Read(buffer)
	if body.reader.Len() == 0 && !body.delivered {
		body.delivered = true
		body.cancel()
	}
	return count, err
}
func (*nonBusinessBoundaryResponseBody) Close() error { return nil }

func TestNonBusinessCancelledConfigDiscoveryPreservesHistoricalState(t *testing.T) {
	for _, format := range []string{"json", "default-text"} {
		t.Run(format, func(t *testing.T) {
			fixture := newNonBusinessJSONFixture(t)
			legacy, err := fixture.store.GetProfile("contract")
			if err != nil {
				t.Fatal(err)
			}
			legacy.Environment = "test"
			legacy.OpenPlatformBaseURL = "https://test-open.qtech.cn"
			legacy.Resource = legacy.OpenPlatformBaseURL
			legacy.AppTokenEndpoint = legacy.OpenPlatformBaseURL + "/open-apis/auth/v3/tenant_access_token/internal"
			if err := fixture.store.SaveProfile(legacy); err != nil {
				t.Fatal(err)
			}
			beforeProfile, beforeSecret := nonBusinessBoundaryStateSnapshot(t, fixture)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			payload := `{"issuer":"https://myaccount.qfei.cn/contract","authorization_endpoint":"https://myaccount.qfei.cn/oauth/authorize/contract","token_endpoint":"https://myaccount.qfei.cn/oauth/token/contract","registration_endpoint":"https://myaccount.qfei.cn/oauth/register/contract"}`
			body := &nonBusinessBoundaryResponseBody{reader: strings.NewReader(payload), cancel: cancel}
			calls := 0
			fixture.options.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if request.Method != http.MethodGet || request.URL.String() != "https://myaccount.qfei.cn/.well-known/oauth-authorization-server/contract" {
					t.Errorf("unexpected prod discovery request: %s %s", request.Method, request.URL)
				}
				response := jsonResponse(payload)
				response.Body = body
				return response, nil
			})}
			args := nonBusinessBoundaryFormatArgs([]string{"config", "add", "--env", "prod", "--name", "contract"}, format)
			code := cli.New(fixture.options).Execute(ctx, args)
			if !body.delivered || calls != 1 || !errors.Is(ctx.Err(), context.Canceled) {
				t.Fatalf("fixture did not deliver full discovery then cancel: delivered=%t calls=%d", body.delivered, calls)
			}
			nonBusinessBoundaryAssertCancelled(t, code, fixture.stdout.Bytes(), fixture.stderr.Bytes(), format)
			nonBusinessBoundaryAssertStateUnchanged(t, fixture, beforeProfile, beforeSecret)
		})
	}
}

func TestNonBusinessCancelledAppTokenResponsePreservesCredentials(t *testing.T) {
	const replacementSecret = "response-cancel-new-fixture-secret"
	const replacementToken = "response-cancel-new-fixture-token"
	for _, format := range []string{"json", "default-text"} {
		t.Run(format, func(t *testing.T) {
			fixture := newNonBusinessJSONFixture(t)
			beforeProfile, beforeSecret := nonBusinessBoundaryStateSnapshot(t, fixture)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			payload := fmt.Sprintf(`{"code":0,"expire":7200,"tenant_access_token":%q}`, replacementToken)
			body := &nonBusinessBoundaryResponseBody{reader: strings.NewReader(payload), cancel: cancel}
			calls := 0
			fixture.options.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if request.Method != http.MethodPost || request.URL.String() != "https://open.qfei.cn/open-apis/auth/v3/tenant_access_token/internal" {
					t.Errorf("unexpected prod token request: %s %s", request.Method, request.URL)
				}
				response := jsonResponse(payload)
				response.Body = body
				return response, nil
			})}
			args := nonBusinessBoundaryFormatArgs([]string{"auth", "login", "--as", "app", "--profile", "contract", "--app-id", "response-cancel-new-fixture-app", "--app-secret", replacementSecret}, format)
			code := cli.New(fixture.options).Execute(ctx, args)
			if !body.delivered || calls != 1 || !errors.Is(ctx.Err(), context.Canceled) {
				t.Fatalf("fixture did not deliver full token response then cancel: delivered=%t calls=%d", body.delivered, calls)
			}
			nonBusinessBoundaryAssertCancelled(t, code, fixture.stdout.Bytes(), fixture.stderr.Bytes(), format)
			nonBusinessBoundaryAssertStateUnchanged(t, fixture, beforeProfile, beforeSecret)
			for _, secret := range []string{replacementSecret, replacementToken} {
				if strings.Contains(fixture.stdout.String()+fixture.stderr.String(), secret) {
					t.Error("cancelled login leaked a fixture credential")
				}
			}
		})
	}
}

func TestNonBusinessCancelledSkillsListMetadataKeepsStdoutEmpty(t *testing.T) {
	fixture := newNonBusinessJSONFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := &nonBusinessBoundaryMetadataFS{FS: fstest.MapFS{"auth/SKILL.md": {Data: []byte("---\nname: auth\ndescription: cancellable fixture\n---\n")}}, cancel: cancel}
	fixture.options.SkillsFS = source
	code := cli.New(fixture.options).Execute(ctx, []string{"skills", "list", "--output", "json", "--debug"})
	if !source.delivered || !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("fixture did not cancel after metadata read")
	}
	nonBusinessBoundaryAssertCancelled(t, code, fixture.stdout.Bytes(), fixture.stderr.Bytes(), "json")
}

type nonBusinessBoundaryMetadataFS struct {
	fs.FS
	cancel    context.CancelFunc
	delivered bool
}

func (source *nonBusinessBoundaryMetadataFS) ReadFile(name string) ([]byte, error) {
	body, err := fs.ReadFile(source.FS, name)
	if err == nil && !source.delivered {
		source.delivered = true
		source.cancel()
	}
	return body, err
}

func nonBusinessBoundaryFormatArgs(args []string, format string) []string {
	formatted := append([]string{}, args...)
	switch format {
	case "json":
		return append(formatted, "--output", "json")
	case "legacy-json":
		return append(formatted, "--json")
	default:
		return formatted
	}
}

func nonBusinessBoundaryAssertCancelled(t *testing.T, code int, stdout, stderr []byte, format string) map[string]any {
	t.Helper()
	if code != 130 {
		t.Errorf("cancelled command exit=%d, want 130", code)
	}
	if len(stdout) != 0 {
		t.Errorf("cancelled command wrote stdout before its error: %s", stdout)
	}
	nonBusinessJSONNoSecrets(t, string(stdout)+string(stderr))
	if format == "default-text" {
		if !strings.Contains(string(stderr), "context canceled") {
			t.Errorf("cancelled text error missing: %s", stderr)
		}
		return nil
	}
	records := nonBusinessJSONRecords(t, stderr)
	last := records[len(records)-1]
	failure, _ := last["error"].(map[string]any)
	if last["ok"] != false || failure["type"] != "cancelled" || failure["cancelled"] != true {
		t.Errorf("last stderr JSON is not cancellation: %#v", last)
	}
	return failure
}

func nonBusinessBoundaryStateSnapshot(t *testing.T, fixture nonBusinessJSONFixture) ([]byte, []byte) {
	t.Helper()
	profile, err := os.ReadFile(fixture.store.Path())
	if err != nil {
		t.Fatal(err)
	}
	secret, err := os.ReadFile(fixture.options.Secrets.Path())
	if err != nil {
		t.Fatal(err)
	}
	return profile, secret
}

func nonBusinessBoundaryAssertStateUnchanged(t *testing.T, fixture nonBusinessJSONFixture, profile, secret []byte) {
	t.Helper()
	afterProfile, afterSecret := nonBusinessBoundaryStateSnapshot(t, fixture)
	if !bytes.Equal(profile, afterProfile) {
		t.Error("cancelled command changed the saved profile or token")
	}
	if !bytes.Equal(secret, afterSecret) {
		t.Error("cancelled command changed saved secrets")
	}
}

var _ io.ReadCloser = (*nonBusinessBoundaryResponseBody)(nil)

func TestNonBusinessCancelledAppCredentialCommitKeepsConsistentProfile(t *testing.T) {
	const replacementSecret = "boundary-committed-fixture-secret"
	const replacementToken = "boundary-committed-fixture-token"
	const replacementApp = "boundary-committed-fixture-app"
	for _, test := range []struct {
		name, message string
		exchangeFails bool
	}{
		{"credentials-saved", "app credentials saved", false},
		{"login-completed", "app auth login completed", false},
		{"exchange-failed", "app token exchange failed", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newNonBusinessJSONFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			diagnostic := &nonBusinessBoundaryLogWriter{output: fixture.stderr, cancel: cancel, message: test.message}
			fixture.options.Stderr = diagnostic
			calls := 0
			fixture.options.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if request.Method != http.MethodPost || request.URL.String() != "https://open.qfei.cn/open-apis/auth/v3/tenant_access_token/internal" {
					t.Errorf("unexpected mock token request: %s %s", request.Method, request.URL)
				}
				if test.exchangeFails {
					return jsonResponse(`{"code":999,"msg":"fixture rejected app"}`), nil
				}
				return jsonResponse(fmt.Sprintf(`{"code":0,"expire":7200,"tenant_access_token":%q}`, replacementToken)), nil
			})}
			code := cli.New(fixture.options).Execute(ctx, []string{"auth", "login", "--as", "app", "--profile", "contract", "--app-id", replacementApp, "--app-secret", replacementSecret, "--output", "json", "--debug"})
			if !diagnostic.triggered || !errors.Is(ctx.Err(), context.Canceled) || calls != 1 {
				t.Fatalf("credential commit boundary not reached: trigger=%t calls=%d", diagnostic.triggered, calls)
			}
			failure := nonBusinessBoundaryAssertCancelled(t, code, fixture.stdout.Bytes(), fixture.stderr.Bytes(), "json")
			details, _ := failure["details"].(map[string]any)
			if details["state"] != "committed" || details["operation"] != "auth login" {
				t.Errorf("app credential commit was hidden: %#v", details)
			}
			profile, err := fixture.store.GetProfile("contract")
			if err != nil {
				t.Fatal(err)
			}
			if profile.Identities.App.AppID != replacementApp || profile.Identities.App.SecretRef != config.AppSecretKey("contract") {
				t.Error("committed Secret no longer matches the saved App profile")
			}
			if test.exchangeFails {
				if profile.Identities.App.Token != nil {
					t.Error("failed exchange retained the previous App token beside the new Secret")
				}
			} else if profile.Identities.App.Token == nil || profile.Identities.App.Token.AccessToken != replacementToken {
				t.Error("completed exchange lost its matching App token")
			}
			if profile.DefaultIdentity != config.IdentityUser || profile.Identities.User.Token == nil || profile.Identities.User.Token.AccessToken != nonBusinessJSONUserToken {
				t.Error("cancelled App login changed the default identity or unrelated user authorization")
			}
			if secret, found, err := fixture.options.Secrets.Get(config.AppSecretKey("contract")); err != nil || !found || secret != replacementSecret {
				t.Fatalf("committed profile lost its matching Secret: found=%t error=%v", found, err)
			}
			for _, secret := range []string{replacementSecret, replacementToken} {
				if strings.Contains(fixture.stdout.String()+fixture.stderr.String(), secret) {
					t.Error("credential commit cancellation leaked a fixture secret")
				}
			}
		})
	}
}

func TestNonBusinessCancelledConfigWriteLogsPreservePreviousState(t *testing.T) {
	for _, phase := range []string{"before-save", "before-clear"} {
		t.Run(phase, func(t *testing.T) {
			fixture := newNonBusinessBoundaryLegacyConfigFixture(t)
			beforeProfile, beforeSecret := nonBusinessBoundaryStateSnapshot(t, fixture)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			diagnostic := &nonBusinessBoundaryLogWriter{output: fixture.stderr, cancel: cancel, message: "save profile"}
			if phase == "before-clear" {
				diagnostic.message = ""
				diagnostic.messageSuffix = "profile authentication state"
			}
			fixture.options.Stderr = diagnostic
			code := cli.New(fixture.options).Execute(ctx, []string{"config", "add", "--env", "prod", "--name", "contract", "--output", "json", "--debug"})
			if !diagnostic.triggered || !errors.Is(ctx.Err(), context.Canceled) {
				t.Fatal("configuration write boundary did not trigger cancellation")
			}
			failure := nonBusinessBoundaryAssertCancelled(t, code, fixture.stdout.Bytes(), fixture.stderr.Bytes(), "json")
			if details, _ := failure["details"].(map[string]any); details["state"] == "committed" {
				t.Errorf("cancellation before state writes was reported committed: %#v", details)
			}
			nonBusinessBoundaryAssertStateUnchanged(t, fixture, beforeProfile, beforeSecret)
		})
	}
}

func TestNonBusinessCancelledConfigResetReportsPartialCommittedState(t *testing.T) {
	fixture := newNonBusinessBoundaryLegacyConfigFixture(t)
	beforeProfile, _ := nonBusinessBoundaryStateSnapshot(t, fixture)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	secretClearedAtDelete := false
	credentials := &nonBusinessBoundaryCredentialStore{present: true, afterDelete: func() {
		_, found, err := fixture.options.Secrets.Get(config.AppSecretKey("contract"))
		if err != nil {
			t.Errorf("read temporary Secret at reset boundary: %v", err)
		}
		secretClearedAtDelete = !found
		cancel()
	}}
	fixture.options.CredentialStore = credentials
	code := cli.New(fixture.options).Execute(ctx, []string{"config", "add", "--env", "prod", "--name", "contract", "--output", "json", "--debug"})
	if !credentials.deleted || credentials.present || !secretClearedAtDelete || !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("fixture did not cancel after successful temporary credential reset")
	}
	failure := nonBusinessBoundaryAssertCancelled(t, code, fixture.stdout.Bytes(), fixture.stderr.Bytes(), "json")
	details, _ := failure["details"].(map[string]any)
	result, _ := details["result"].(map[string]any)
	if details["state"] != "committed" || details["operation"] != "config add" || result["app_secret_cleared"] != true || result["profile_saved"] != false {
		t.Errorf("partial reset state was hidden: %#v", details)
	}
	afterProfile, err := os.ReadFile(fixture.store.Path())
	if err != nil || !bytes.Equal(beforeProfile, afterProfile) {
		t.Errorf("cancelled reset saved a new production profile: %v", err)
	}
	if _, found, err := fixture.options.Secrets.Get(config.AppSecretKey("contract")); err != nil || found {
		t.Fatalf("completed temporary Secret reset was not retained: found=%t error=%v", found, err)
	}
}

// This reset fixture remains entirely local; no platform Keyring is selected.
type nonBusinessBoundaryCredentialStore struct {
	present, deleted bool
	afterDelete      func()
}

func (store *nonBusinessBoundaryCredentialStore) Load(string) (credential.DeviceCredential, error) {
	if !store.present {
		return credential.DeviceCredential{}, credential.ErrCredentialNotFound
	}
	return credential.DeviceCredential{Token: &config.Token{AccessToken: "boundary-device-fixture-token"}}, nil
}
func (store *nonBusinessBoundaryCredentialStore) Save(_ string, _ credential.DeviceCredential) error {
	store.present = true
	return nil
}
func (store *nonBusinessBoundaryCredentialStore) Delete(string) error {
	store.present = false
	store.deleted = true
	if store.afterDelete != nil {
		store.afterDelete()
	}
	return nil
}

func newNonBusinessBoundaryLegacyConfigFixture(t *testing.T) nonBusinessJSONFixture {
	t.Helper()
	fixture := newNonBusinessJSONFixture(t)
	legacy, err := fixture.store.GetProfile("contract")
	if err != nil {
		t.Fatal(err)
	}
	legacy.Environment = "test"
	legacy.OpenPlatformBaseURL = "https://test-open.qtech.cn"
	legacy.Resource = legacy.OpenPlatformBaseURL
	legacy.AppTokenEndpoint = legacy.OpenPlatformBaseURL + "/open-apis/auth/v3/tenant_access_token/internal"
	if err := fixture.store.SaveProfile(legacy); err != nil {
		t.Fatal(err)
	}
	fixture.options.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.String() != "https://myaccount.qfei.cn/.well-known/oauth-authorization-server/contract" {
			t.Errorf("unexpected prod metadata request: %s %s", request.Method, request.URL)
		}
		return jsonResponse(`{"issuer":"https://myaccount.qfei.cn/contract","authorization_endpoint":"https://myaccount.qfei.cn/oauth/authorize/contract","token_endpoint":"https://myaccount.qfei.cn/oauth/token/contract","registration_endpoint":"https://myaccount.qfei.cn/oauth/register/contract"}`), nil
	})}
	return fixture
}
