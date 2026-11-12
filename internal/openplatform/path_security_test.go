package openplatform_test

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
)

func TestClientRejectsUnsafePathsBeforeCredentialsOrHTTP(t *testing.T) {
	paths := []string{
		"/open-apis/../open-web/web/clm/contract/info",
		"/open-apis/rule_engine/v1/tables/rows/..",
		"/open-apis/x/./item", "/open-apis/x//item", `/open-apis/x\item`,
		"/open-apis/x/%2e%2e/item", "/open-apis/x/.%2e/item",
		"/open-apis/x/%252e%252e/item", "/open-apis/x/%2fitem",
		"/open-apis/x/%255citem", "/open-apis/x/%00item",
		"/open-apis/x/item?other=route", "/open-apis/x/item#fragment",
		"/open-apis/x/item;route", " /open-apis/x/item", "/open-apis/x/%0aitem",
	}
	for _, stream := range []bool{false, true} {
		for _, path := range paths {
			t.Run(path+map[bool]string{false: "/normal", true: "/stream"}[stream], func(t *testing.T) {
				calls, preparations := 0, 0
				client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					calls++
					return jsonResponse(`{}`), nil
				})}})
				rc := openplatform.RequestContext{Identity: config.IdentityApp, BaseURL: "https://open.qfei.cn", AccessToken: "fixture", PrepareAccessToken: func(context.Context, string) (string, error) {
					preparations++
					return "fixture", nil
				}}
				request := openplatform.Request{Method: http.MethodDelete, Path: path, IdentityPolicy: openplatform.IdentityPolicyAny}
				var err error
				if stream {
					_, err = client.DoStream(context.Background(), rc, request, io.Discard)
				} else {
					_, err = client.Do(context.Background(), rc, request)
				}
				if err == nil || calls != 0 || preparations != 0 {
					t.Fatalf("path accepted or side effects occurred: error=%v HTTP=%d credentials=%d", err, calls, preparations)
				}
			})
		}
	}
}

func TestClientCannotWeakenRouteIdentityPolicy(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, path := range []string{"/open-apis/contract/v1/mcp/contracts/search", "/open-apis/contract/v1/%6dcp/contracts/search", "/open-web/web/unregistered/action"} {
			client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("APP must not reach a USER-only route")
				return nil, nil
			})}})
			rc := openplatform.RequestContext{Identity: config.IdentityApp, BaseURL: "https://open.qfei.cn", AccessToken: "fixture"}
			request := openplatform.Request{Method: http.MethodGet, Path: path, IdentityPolicy: openplatform.IdentityPolicyAny}
			var err error
			if stream {
				_, err = client.DoStream(context.Background(), rc, request, io.Discard)
			} else {
				_, err = client.Do(context.Background(), rc, request)
			}
			if err == nil {
				t.Fatal("explicit Any weakened the route's required identity")
			}
		}
	}
}

func TestClientPreservesSafeEncodedIDsAndUnrestrictedData(t *testing.T) {
	path := "/open-apis/mdm/v1/vendors/" + url.PathEscape("研发 A%20")
	body := `{"text":"../合同","url":"https://example.com/a%20b?x=../"}`
	client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if r.URL.Path != "/open-apis/mdm/v1/vendors/研发 A%20" || r.URL.Query().Get("name") != "../研发/%" || string(data) != body {
			t.Fatalf("valid request changed: %s %s", r.URL, data)
		}
		return jsonResponse(`{}`), nil
	})}})
	_, err := client.Do(context.Background(), openplatform.RequestContext{Identity: config.IdentityApp, BaseURL: "https://open.qfei.cn", AccessToken: "fixture"}, openplatform.Request{Method: http.MethodPost, Path: path, Body: []byte(body), Query: url.Values{"name": {"../研发/%"}}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientUsesDecodedRuleRouteForIdentityHeader(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, identity := range []config.IdentityKind{config.IdentityUser, config.IdentityApp} {
			client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				want := ""
				if identity == config.IdentityUser {
					want = "user"
				}
				if r.Header.Get("X-Qfei-Identity") != want || !strings.HasPrefix(r.URL.Path, "/open-apis/rule_engine/v1/") {
					t.Fatal("route and identity header disagree")
				}
				return jsonResponse(`{}`), nil
			})}})
			rc := openplatform.RequestContext{Identity: identity, BaseURL: "https://open.qfei.cn", AccessToken: "fixture"}
			request := openplatform.Request{Method: http.MethodGet, Path: "/open-apis/%72ule_engine/v1/products", Headers: http.Header{"X-Qfei-Identity": {"untrusted"}}}
			var err error
			if stream {
				_, err = client.DoStream(context.Background(), rc, request, io.Discard)
			} else {
				_, err = client.Do(context.Background(), rc, request)
			}
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
