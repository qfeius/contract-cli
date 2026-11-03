package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/build"
	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/invocation"
)

const (
	nonBusinessJSONUserToken = "json-regression-private-user-token"
	nonBusinessJSONAppToken  = "json-regression-private-app-token"
	nonBusinessJSONSecret    = "json-regression-private-app-secret"
)

func TestNonBusinessExplicitJSONVersionAliases(t *testing.T) {
	t.Parallel()
	for _, alias := range []string{"version", "--version", "-version", "-v"} {
		t.Run(alias, func(t *testing.T) {
			t.Parallel()
			fixture := newNonBusinessJSONFixture(t)
			code := cli.New(fixture.options).Execute(context.Background(), []string{alias, "--output=json"})
			if code != 0 || fixture.stderr.Len() != 0 {
				t.Fatalf("version JSON failed: code=%d stderr=%s", code, fixture.stderr)
			}
			data := nonBusinessJSONSuccess(t, fixture.stdout.Bytes())
			current := build.Current()
			for key, want := range map[string]string{
				"name": current.Name, "version": current.Version, "commit": current.Commit,
				"built_at": current.Date, "feature_baseline": build.FeatureBaseline(),
			} {
				if data[key] != want {
					t.Errorf("version %s = %#v, want %q", key, data[key], want)
				}
			}
		})
	}
}

func TestNonBusinessExplicitJSONConfigAdd(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, profile   string
		explicit, reset bool
	}{
		{"default-prod", "json-new", false, false},
		{"explicit-prod", "json-new", true, false},
		{"production-reconfiguration-preserves-auth", "contract", true, false},
		{"production-resets-historical-auth", "contract", true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newNonBusinessJSONFixture(t)
			openOrigin, accountOrigin := "https://open.qfei.cn", "https://myaccount.qfei.cn"
			if test.reset {
				legacy, err := fixture.store.GetProfile(test.profile)
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
			}
			calls := 0
			fixture.options.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if request.URL.String() != accountOrigin+"/.well-known/oauth-authorization-server/contract" {
					t.Errorf("unexpected discovery endpoint: %s", request.URL)
				}
				return jsonResponse(fmt.Sprintf(`{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q,"registration_endpoint":%q}`,
					accountOrigin+"/contract", accountOrigin+"/oauth/authorize/contract", accountOrigin+"/oauth/token/contract", accountOrigin+"/oauth/register/contract")), nil
			})}
			args := []string{"config", "add", "--name", test.profile, "--output", "json"}
			if test.explicit {
				args = append(args, "--env", "prod")
			}
			if code := cli.New(fixture.options).Execute(context.Background(), args); code != 0 || calls != 1 || fixture.stderr.Len() != 0 {
				t.Fatalf("config JSON failed: code=%d calls=%d stderr=%s", code, calls, fixture.stderr)
			}
			data := nonBusinessJSONSuccess(t, fixture.stdout.Bytes())
			if data["profile"] != test.profile || data["environment"] != "prod" || data["default_identity"] != "user" || data["resource"] != openOrigin || data["authentication_reset"] != test.reset {
				t.Fatalf("config JSON lost safe configuration details: %#v", data)
			}
			profile, err := fixture.store.GetProfile(test.profile)
			if err != nil || profile.Environment != "prod" || profile.OpenPlatformBaseURL != openOrigin {
				t.Fatalf("JSON option changed saved environment: environment=%q error=%v", profile.Environment, err)
			}
			if test.reset {
				if profile.Identities.User.Token != nil || profile.Identities.App.Token != nil {
					t.Fatal("JSON configuration reset preserved old environment tokens")
				}
				if _, found, err := fixture.options.Secrets.Get(config.AppSecretKey(test.profile)); err != nil || found {
					t.Fatalf("JSON configuration reset preserved old App Secret: found=%t error=%v", found, err)
				}
			} else if test.profile == "contract" {
				if profile.Identities.User.Token == nil || profile.Identities.User.Token.AccessToken != nonBusinessJSONUserToken || profile.Identities.App.Token == nil || profile.Identities.App.Token.AccessToken != nonBusinessJSONAppToken {
					t.Fatal("valid production reconfiguration cleared existing authorization")
				}
				if secret, found, err := fixture.options.Secrets.Get(config.AppSecretKey(test.profile)); err != nil || !found || secret != nonBusinessJSONSecret {
					t.Fatalf("valid production reconfiguration changed the App Secret: found=%t error=%v", found, err)
				}
			}
		})
	}
}

func TestNonBusinessExplicitJSONRejectsUnsupportedPresets(t *testing.T) {
	t.Parallel()
	for _, environment := range []string{"dev", "blue"} {
		for _, profile := range []string{"contract", "blocked"} {
			t.Run(environment+"/"+profile, func(t *testing.T) {
				t.Parallel()
				fixture := newNonBusinessJSONFixture(t)
				before, err := os.ReadFile(fixture.store.Path())
				if err != nil {
					t.Fatal(err)
				}
				calls := 0
				fixture.options.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					calls++
					return nil, errors.New("unsupported environment reached mock HTTP")
				})}
				code := cli.New(fixture.options).Execute(context.Background(), []string{
					"config", "add", "--env", environment, "--name", profile, "--output=json", "--debug",
				})
				if code != 1 || calls != 0 || fixture.stdout.Len() != 0 {
					t.Fatalf("non-production JSON config dispatched or printed success: code=%d calls=%d stdout=%s", code, calls, fixture.stdout)
				}
				records := nonBusinessJSONRecords(t, fixture.stderr.Bytes())
				last := records[len(records)-1]
				failure, _ := last["error"].(map[string]any)
				message, _ := failure["message"].(string)
				if last["ok"] != false || !strings.Contains(message, "unsupported environment") || !strings.Contains(message, "supported environments: prod") {
					t.Fatalf("non-production rejection lost its JSON error: %#v", last)
				}
				after, err := os.ReadFile(fixture.store.Path())
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("rejected non-production config changed saved profiles: %v", err)
				}
				if _, found, err := fixture.store.LookupProfile("blocked"); err != nil || found {
					t.Fatalf("rejected non-production config created a profile: found=%t error=%v", found, err)
				}
				if secret, found, err := fixture.options.Secrets.Get(config.AppSecretKey("contract")); err != nil || !found || secret != nonBusinessJSONSecret {
					t.Fatalf("rejected non-production config changed production credentials: found=%t error=%v", found, err)
				}
				nonBusinessJSONNoSecrets(t, fixture.stdout.String()+fixture.stderr.String())
			})
		}
	}
}

func TestNonBusinessExplicitJSONAuthResults(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, identity, defaultIdentity string
		args                            []string
	}{
		{"login-app", "app", "app", []string{"login", "--as", "app", "--app-id", "json-fixture-app", "--app-secret", nonBusinessJSONSecret}},
		{"status-user", "user", "user", []string{"status", "--as", "user"}},
		{"status-app", "app", "user", []string{"status", "--as", "app"}},
		{"logout-user", "user", "user", []string{"logout", "--as", "user"}},
		{"logout-app", "app", "user", []string{"logout", "--as", "app"}},
		{"use-app", "app", "app", []string{"use", "--as", "app"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newNonBusinessJSONFixture(t)
			calls := 0
			fixture.options.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if test.name != "login-app" || request.URL.String() != "https://open.qfei.cn/open-apis/auth/v3/tenant_access_token/internal" {
					t.Errorf("unexpected auth HTTP request: %s", request.URL)
				}
				return jsonResponse(`{"code":0,"expire":7200,"tenant_access_token":"` + nonBusinessJSONAppToken + `"}`), nil
			})}
			args := append([]string{"auth"}, test.args...)
			args = append(args, "--profile", "contract", "--output=json", "--debug")
			if code := cli.New(fixture.options).Execute(context.Background(), args); code != 0 {
				t.Fatalf("auth JSON failed: code=%d stderr=%s", code, fixture.stderr)
			}
			data := nonBusinessJSONSuccess(t, fixture.stdout.Bytes())
			if data["profile"] != "contract" || data["environment"] != "prod" || data["identity"] != test.identity || data["default_identity"] != test.defaultIdentity {
				t.Fatalf("auth JSON lost profile/identity details: %#v", data)
			}
			if strings.HasPrefix(test.name, "status-") {
				fields, ok := data["fields"].([]any)
				if data["authorization"] != "authorized" || !ok || len(fields) == 0 {
					t.Fatalf("status lost authorization or fields: %#v", data)
				}
				for _, value := range fields {
					field, ok := value.(map[string]any)
					if !ok || field["label"] == "" || field["value"] == nil {
						t.Fatalf("status field is not label/value JSON: %#v", value)
					}
				}
			} else if message, ok := data["message"].(string); !ok || message == "" {
				t.Fatalf("auth JSON message missing: %#v", data)
			}
			if test.name == "login-app" && (calls != 1 || data["authorization"] != "authorized") {
				t.Fatalf("login did not return authorized after one token exchange: calls=%d data=%#v", calls, data)
			}
			nonBusinessJSONRecords(t, fixture.stderr.Bytes())
			nonBusinessJSONNoSecrets(t, fixture.stdout.String()+fixture.stderr.String())
			profile, err := fixture.store.GetProfile("contract")
			if err != nil || string(profile.DefaultIdentity) != test.defaultIdentity {
				t.Fatalf("JSON option changed saved default identity: identity=%q error=%v", profile.DefaultIdentity, err)
			}
			if test.name == "logout-user" && profile.Identities.User.Token != nil || test.name == "logout-app" && profile.Identities.App.Token != nil {
				t.Fatal("JSON logout returned success without clearing the selected token")
			}
		})
	}
}

func TestNonBusinessExplicitJSONErrorsKeepStdoutEmpty(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"config", "add", "--env", "dev"},
		{"auth", "login", "--as", "invalid"},
		{"auth", "status", "--as", "invalid"},
		{"auth", "logout", "--as", "invalid"},
		{"auth", "use", "--as", "invalid"},
	} {
		t.Run(strings.Join(args, "/"), func(t *testing.T) {
			t.Parallel()
			fixture := newNonBusinessJSONFixture(t)
			args := append(append([]string{}, args...), "--output", "json", "--debug")
			if code := cli.New(fixture.options).Execute(context.Background(), args); code != 1 || fixture.stdout.Len() != 0 {
				t.Fatalf("failure wrote stdout or changed exit status: code=%d stdout=%s", code, fixture.stdout)
			}
			records := nonBusinessJSONRecords(t, fixture.stderr.Bytes())
			last := records[len(records)-1]
			if last["ok"] != false || last["error"] == nil {
				t.Fatalf("last stderr record is not the command error: %#v", last)
			}
			failure, _ := last["error"].(map[string]any)
			message, _ := failure["message"].(string)
			wantCause := "unsupported identity"
			if args[0] == "config" {
				wantCause = "unsupported environment"
			}
			if !strings.Contains(message, wantCause) {
				t.Fatalf("JSON flag hid the actual validation failure: message=%q want=%q", message, wantCause)
			}
			nonBusinessJSONNoSecrets(t, fixture.stderr.String())
		})
	}
}

func TestNonBusinessExplicitJSONAuthorizationGuidanceAndCancellation(t *testing.T) {
	t.Parallel()
	for _, noOpenBrowser := range []bool{true, false} {
		t.Run(fmt.Sprintf("no-open-browser=%t", noOpenBrowser), func(t *testing.T) {
			t.Parallel()
			fixture := newNonBusinessJSONFixture(t)
			profile, err := fixture.store.GetProfile("contract")
			if err != nil {
				t.Fatal(err)
			}
			profile.Identities.User.RegistrationEndpoint = "https://myaccount.qfei.cn/oauth/register/contract"
			profile.Identities.User.AuthorizationEndpoint = "https://myaccount.qfei.cn/oauth/authorize/contract"
			profile.Identities.User.TokenEndpoint = "https://myaccount.qfei.cn/oauth/token/contract"
			// A real local callback listener uses an ephemeral port and is closed by cancellation.
			profile.Identities.User.RedirectURL = "http://127.0.0.1:0/callback"
			profile.CredentialBinding = config.CredentialTargetBinding(profile)
			if err := fixture.store.SaveProfile(profile); err != nil {
				t.Fatal(err)
			}
			before, err := fixture.store.GetProfile("contract")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stderr := &nonBusinessJSONAuthorizationWriter{cancel: cancel, cancelOnGuidance: noOpenBrowser}
			fixture.options.Stderr = stderr
			fixture.options.OpenBrowser = func(string) error {
				if noOpenBrowser {
					t.Error("--no-open-browser opened the browser")
				}
				cancel()
				return errors.New("fixture browser unavailable")
			}
			calls := 0
			fixture.options.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if request.URL.Path != "/oauth/register/contract" {
					t.Errorf("cancelled login reached token exchange: %s", request.URL)
				}
				return jsonResponse(`{"client_id":"json-regression-client"}`), nil
			})}
			args := []string{"auth", "login", "--profile", "contract", "--as", "user", "--output", "json", "--timeout", "1s"}
			if noOpenBrowser {
				args = append(args, "--no-open-browser")
			}
			if code := cli.New(fixture.options).Execute(ctx, args); code != 130 || fixture.stdout.Len() != 0 || calls != 1 {
				t.Fatalf("OAuth cancellation failed: code=%d calls=%d stdout=%s stderr=%s", code, calls, fixture.stdout, stderr)
			}
			records := nonBusinessJSONRecords(t, stderr.Bytes())
			if len(records) != 2 || records[0]["event"] != "authorization_required" {
				t.Fatalf("want one authorization event and final error: %#v", records)
			}
			authorizationURL, _ := records[0]["authorization_url"].(string)
			parsed, err := url.Parse(authorizationURL)
			if err != nil || parsed.Host != "myaccount.qfei.cn" || parsed.Path != "/oauth/authorize/contract" || parsed.Query().Get("state") == "" {
				t.Fatalf("authorization event lost the usable URL: %#v (%v)", records[0], err)
			}
			failure, _ := records[1]["error"].(map[string]any)
			if records[1]["ok"] != false || failure["type"] != "cancelled" || failure["cancelled"] != true {
				t.Fatalf("cancelled error must be the final JSON record: %#v", records[1])
			}
			nonBusinessJSONNoSecrets(t, fixture.stdout.String()+stderr.String())
			after, err := fixture.store.GetProfile("contract")
			if err != nil || after.Identities.User.Token == nil || after.Identities.User.Token.AccessToken != before.Identities.User.Token.AccessToken || after.Identities.User.ClientID != before.Identities.User.ClientID {
				t.Fatalf("cancelled user login changed persisted authorization: error=%v", err)
			}
		})
	}
}

type nonBusinessJSONFixture struct {
	store          *config.Store
	stdout, stderr *bytes.Buffer
	options        cli.Options
}

func newNonBusinessJSONFixture(t *testing.T) nonBusinessJSONFixture {
	t.Helper()
	directory := t.TempDir()
	store, secrets := config.NewStore(directory), config.NewSecretsStore(directory)
	profile := uploadProfile(config.IdentityUser)
	profile.Identities.User.AuthMode = config.UserAuthModeAuthorizationCode
	profile.Identities.User.AuthorizationEndpoint = "https://myaccount.qfei.cn/oauth/authorize/contract"
	profile.Identities.User.RegistrationEndpoint = "https://myaccount.qfei.cn/oauth/register/contract"
	profile.Identities.User.TokenEndpoint = "https://myaccount.qfei.cn/oauth/token/contract"
	profile.Identities.User.RedirectURL = "http://127.0.0.1:8000/callback"
	profile.Identities.User.Token.AccessToken = nonBusinessJSONUserToken
	profile.Identities.App.AppID = "json-fixture-app"
	profile.Identities.App.SecretRef = config.AppSecretKey(profile.Name)
	profile.Identities.App.Token.AccessToken = nonBusinessJSONAppToken
	profile.AppTokenEndpoint = "https://open.qfei.cn/open-apis/auth/v3/tenant_access_token/internal"
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatal(err)
	}
	if err := secrets.Set(config.AppSecretKey(profile.Name), nonBusinessJSONSecret); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	return nonBusinessJSONFixture{store: store, stdout: stdout, stderr: stderr, options: cli.Options{
		Store: store, Secrets: secrets, Stdout: stdout, Stderr: stderr,
		LookupEnv:          func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
		InspectEnvironment: func(context.Context, int) invocation.Result { return invocation.Result{} },
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			t.Errorf("unexpected external request: %s", request.URL)
			return nil, errors.New("unexpected mock request")
		})},
	}}
}

func nonBusinessJSONSuccess(t *testing.T, body []byte) map[string]any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(body))
	var envelope struct {
		OK    *bool           `json:"ok"`
		Data  map[string]any  `json:"data"`
		Error json.RawMessage `json:"error"`
	}
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatalf("stdout is not a JSON result: %q (%v)", body, err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		t.Fatalf("stdout contains multiple results or trailing text: %q (%v)", body, err)
	}
	if envelope.OK == nil || !*envelope.OK || envelope.Data == nil || len(envelope.Error) != 0 {
		t.Fatalf("stdout is not one success envelope: %s", body)
	}
	nonBusinessJSONNoSecrets(t, string(body))
	return envelope.Data
}

func nonBusinessJSONRecords(t *testing.T, body []byte) []map[string]any {
	t.Helper()
	if len(bytes.TrimSpace(body)) == 0 {
		t.Fatal("expected stderr JSON records")
	}
	var records []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(body), []byte("\n")) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil || record == nil {
			t.Fatalf("stderr record is not a single JSON object: %q (%v)", line, err)
		}
		records = append(records, record)
	}
	return records
}

func nonBusinessJSONNoSecrets(t *testing.T, text string) {
	t.Helper()
	for _, secret := range []string{nonBusinessJSONUserToken, nonBusinessJSONAppToken, nonBusinessJSONSecret} {
		if strings.Contains(text, secret) {
			t.Fatal("command output leaked a fixture token or App Secret")
		}
	}
}

type nonBusinessJSONAuthorizationWriter struct {
	bytes.Buffer
	cancel           context.CancelFunc
	cancelOnGuidance bool
}

func (writer *nonBusinessJSONAuthorizationWriter) Write(body []byte) (int, error) {
	count, err := writer.Buffer.Write(body)
	if writer.cancelOnGuidance && (bytes.Contains(body, []byte("authorization_required")) || bytes.Contains(body, []byte("https://myaccount.qfei.cn/oauth/authorize/contract"))) {
		writer.cancel()
	}
	return count, err
}
