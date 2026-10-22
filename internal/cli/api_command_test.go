package cli_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
)

func TestAPIWebCallUsesCLIUserAuthorizationAndExactRequest(t *testing.T) {
	t.Parallel()

	store := config.NewStore(t.TempDir())
	profile := apiWebTestProfile()
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	stdout := &bytes.Buffer{}
	requestCount := 0
	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
		Store:  store,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestCount++
			if req.Method != http.MethodPost {
				t.Fatalf("method = %q", req.Method)
			}
			if req.URL.Path != "/open-web/web/clm/statistic/contractInfoByTradingParty" {
				t.Fatalf("path = %q", req.URL.Path)
			}
			if got := req.URL.Query()["status"]; len(got) != 2 || got[0] != "draft" || got[1] != "signed" {
				t.Fatalf("status query = %#v", got)
			}
			if got := req.URL.Query().Get("name"); got != "张三 &+" {
				t.Fatalf("name query = %q", got)
			}
			if _, exists := req.URL.Query()["user_id_type"]; exists {
				t.Fatalf("web call must not add open-platform common query: %s", req.URL.RawQuery)
			}
			if got := req.Header.Get("Authorization"); got != "Bearer user-token" {
				t.Fatalf("authorization = %q", got)
			}
			if got := req.Header.Get("X-Business-Header"); got != "business-value" {
				t.Fatalf("business header = %q", got)
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll(body) error = %v", err)
			}
			if string(body) != `{"pageNum":1,"pageSize":20}` {
				t.Fatalf("body = %q", body)
			}
			return jsonResponse(`{"code":0,"data":{"total":3}}`), nil
		})},
	})

	err := app.Run(context.Background(), []string{
		"api", "call",
		"--channel", "web",
		"--profile", "contract",
		"--as", "user",
		"--output", "json",
		"POST", "/open-web/web/clm/statistic/contractInfoByTradingParty",
		"--query", "status=draft",
		"--query", "status=signed",
		"--query", "name=张三 &+",
		"--header", "X-Business-Header: business-value",
		"--data", `{"pageNum":1,"pageSize":20}`,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
	if output := stdout.String(); !strings.Contains(output, `"total": 3`) {
		t.Fatalf("output = %q", output)
	}
}

func TestAPIWebCallDefaultsToUserIdentity(t *testing.T) {
	t.Parallel()

	store := config.NewStore(t.TempDir())
	profile := apiWebTestProfile()
	profile.DefaultIdentity = config.IdentityApp
	profile.Identities.App.Token = &config.Token{AccessToken: "app-token", Expiry: time.Now().Add(time.Hour)}
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Store:  store,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("Authorization"); got != "Bearer user-token" {
				t.Fatalf("authorization = %q", got)
			}
			return jsonResponse(`{"code":0}`), nil
		})},
	})

	if err := app.Run(context.Background(), []string{
		"api", "call", "--channel", "web", "--profile", "contract",
		"GET", "/open-web/web/clm/contract/info",
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestAPIWebCallReusesDeviceGrantCredential(t *testing.T) {
	t.Parallel()

	now := time.Now()
	store := config.NewStore(t.TempDir())
	profile := apiWebTestProfile()
	profile.Identities.User.AuthMode = config.UserAuthModeDevice
	profile.Identities.User.DeviceClientID = "device-client"
	profile.Identities.User.Token = nil
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	credentials := &memoryDeviceCredentialStore{values: map[string]credential.DeviceCredential{
		"contract": {Token: &config.Token{AccessToken: "device-user-token", Expiry: now.Add(time.Hour)}},
	}}
	app := cli.New(cli.Options{
		Stdout:          &bytes.Buffer{},
		Stderr:          &bytes.Buffer{},
		Store:           store,
		CredentialStore: credentials,
		Now:             func() time.Time { return now },
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("Authorization"); got != "Bearer device-user-token" {
				t.Fatalf("authorization = %q", got)
			}
			return jsonResponse(`{"code":0}`), nil
		})},
	})

	if err := app.Run(context.Background(), []string{
		"api", "call", "--channel", "web", "--profile", "contract",
		"GET", "/open-web/web/clm/contract/info",
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestAPIWebCallRejectsUnsupportedChannelMethodAndPathBeforeHTTP(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing channel", args: []string{"GET", "/open-web/web/clm/contract/info"}, wantErr: "--channel web is required"},
		{name: "other channel", args: []string{"--channel", "open-platform", "GET", "/open-web/web/clm/contract/info"}, wantErr: `unsupported api channel "open-platform"`},
		{name: "invalid query", args: []string{"--channel", "web", "--query", "missing-equals", "GET", "/open-web/web/clm/contract/info"}, wantErr: "expected key=value"},
		{name: "unsupported method", args: []string{"--channel", "web", "OPTIONS", "/open-web/web/clm/contract/info"}, wantErr: `unsupported Open Web method "OPTIONS"`},
		{name: "open apis path", args: []string{"--channel", "web", "GET", "/open-apis/mdm/v1/vendors"}, wantErr: "relative /open-web/web/ path"},
		{name: "absolute URL", args: []string{"--channel", "web", "GET", "https://example.com/open-web/web/clm/contract/info"}, wantErr: "relative /open-web/web/ path"},
		{name: "embedded query", args: []string{"--channel", "web", "GET", "/open-web/web/clm/contract/info?a=b"}, wantErr: "--query"},
		{name: "encoded path", args: []string{"--channel", "web", "GET", "/open-web/web/clm/%2e%2e/internal"}, wantErr: "unsafe"},
		{name: "dot segment", args: []string{"--channel", "web", "GET", "/open-web/web/clm/../internal"}, wantErr: "unsafe"},
		{name: "double slash", args: []string{"--channel", "web", "GET", "/open-web/web/clm//internal"}, wantErr: "unsafe"},
		{name: "backslash", args: []string{"--channel", "web", "GET", `/open-web/web/clm\internal`}, wantErr: "unsafe"},
		{name: "semicolon", args: []string{"--channel", "web", "GET", "/open-web/web/clm/info;internal"}, wantErr: "unsafe"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := cli.New(cli.Options{
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
				Store:  config.NewStore(t.TempDir()),
				HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					t.Fatalf("rejected api call sent HTTP request to %s", req.URL.String())
					return nil, nil
				})},
			})
			err := app.Run(context.Background(), append([]string{"api", "call"}, tc.args...))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestAPIWebCallRejectsTrustedIdentityHeaders(t *testing.T) {
	t.Parallel()

	for _, header := range []string{
		"Authorization: Bearer conversation-token",
		"Cookie: zs_session=conversation-token",
		"Z-Tenant-Id: 123",
		"Z-User-Id: 456",
		"X-Gate-Auth: trusted",
		"sourceType: web",
		"Forwarded: for=127.0.0.1",
		"X-Forwarded-For: 127.0.0.1",
	} {
		header := header
		t.Run(strings.SplitN(header, ":", 2)[0], func(t *testing.T) {
			t.Parallel()
			app := cli.New(cli.Options{
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
				Store:  config.NewStore(t.TempDir()),
				HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					t.Fatalf("rejected header sent HTTP request to %s", req.URL.String())
					return nil, nil
				})},
			})
			err := app.Run(context.Background(), []string{
				"api", "call", "--channel", "web", "--header", header,
				"GET", "/open-web/web/clm/contract/info",
			})
			if err == nil || !strings.Contains(err.Error(), "managed by contract-cli") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestAPIWebCallRejectsAppIdentityAndOversizedBodyBeforeHTTP(t *testing.T) {
	t.Parallel()

	store := config.NewStore(t.TempDir())
	profile := apiWebTestProfile()
	profile.Identities.App.Token = &config.Token{AccessToken: "app-token", Expiry: time.Now().Add(time.Hour)}
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	app := cli.New(cli.Options{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Store:  store,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("rejected api call sent HTTP request to %s", req.URL.String())
			return nil, nil
		})},
	})

	err := app.Run(context.Background(), []string{
		"api", "call", "--channel", "web", "--profile", "contract", "--as", "app",
		"GET", "/open-web/web/clm/contract/info",
	})
	if err == nil || !strings.Contains(err.Error(), "only supports --as user") {
		t.Fatalf("unexpected app identity error: %v", err)
	}

	err = app.Run(context.Background(), []string{
		"api", "call", "--channel", "web", "--profile", "contract",
		"POST", "/open-web/web/clm/contract/info", "--data", strings.Repeat("x", 1024*1024+1),
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds 1048576 bytes") {
		t.Fatalf("unexpected body size error: %v", err)
	}
}

func TestAPICommandHelpIsExposedForWebChannel(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		args     []string
		contains []string
	}{
		{args: []string{"help", "api"}, contains: []string{"contract-cli api call"}},
		{args: []string{"help", "api", "call"}, contains: []string{"contract-cli api call", "--channel <web>", "/open-web/web/"}},
		{args: []string{"api", "call", "--help"}, contains: []string{"contract-cli api call", "--channel <web>", "/open-web/web/"}},
	} {
		stdout := &bytes.Buffer{}
		app := cli.New(cli.Options{Stdout: stdout, Stderr: &bytes.Buffer{}, Store: config.NewStore(t.TempDir())})
		if err := app.Run(context.Background(), test.args); err != nil {
			t.Fatalf("Run(%v) error = %v", test.args, err)
		}
		output := stdout.String()
		for _, fragment := range test.contains {
			if !strings.Contains(output, fragment) {
				t.Fatalf("help output for %v missing %q: %q", test.args, fragment, output)
			}
		}
	}
}

func apiWebTestProfile() config.Profile {
	return productionProfileFixture(config.Profile{
		Name:                           "contract",
		Environment:                    "prod",
		OpenPlatformBaseURL:            "https://open.qfei.cn",
		AppTokenEndpoint:               "https://open.qfei.cn/open-apis/auth/v3/tenant_access_token/internal",
		ProtectedResourceMetadataURL:   "https://open.qfei.cn/.well-known/oauth-protected-resource",
		AuthorizationServerMetadataURL: "https://myaccount.qfei.cn/.well-known/oauth-authorization-server/contract",
		Resource:                       "https://open.qfei.cn",
		BusinessType:                   "contract",
		ClientName:                     "contract-cli",
		DefaultIdentity:                config.IdentityUser,
		Identities: config.Identities{User: config.UserIdentity{
			AuthMode:              config.UserAuthModeAuthorizationCode,
			AuthorizationEndpoint: "https://myaccount.qfei.cn/api/public/oauth/authorize/contract",
			TokenEndpoint:         "https://myaccount.qfei.cn/api/public/oauth/token/contract",
			Token: &config.Token{
				AccessToken: "user-token",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(time.Hour),
			},
		}},
	})
}
