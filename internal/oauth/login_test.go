package oauth_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/oauth"
)

func TestBuildAuthorizationURL(t *testing.T) {
	t.Parallel()

	got, err := oauth.BuildAuthorizationURL(oauth.AuthorizationRequest{
		AuthorizationEndpoint: "https://dev-myaccount.qtech.cn/api/public/oauth/authorize/contract",
		BusinessType:          "contract",
		ClientID:              "zsdcli_test",
		RedirectURL:           "http://127.0.0.1:8000/callback",
		Resource:              "http://higress-gateway.higress-system/mcp-servers",
		Scopes:                []string{"mcp:tools", "mcp:resources"},
		State:                 "state-value",
		CodeChallenge:         "challenge-value",
	})
	if err != nil {
		t.Fatalf("BuildAuthorizationURL() error = %v", err)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	query := parsed.Query()
	if query.Get("business_type") != "contract" {
		t.Fatalf("business_type = %q, want contract", query.Get("business_type"))
	}
	if query.Get("scope") != "mcp:tools mcp:resources" {
		t.Fatalf("scope = %q", query.Get("scope"))
	}
	if query.Get("code_challenge_method") != "S256" {
		t.Fatalf("code_challenge_method = %q", query.Get("code_challenge_method"))
	}
	if query.Get("resource") != "http://higress-gateway.higress-system/mcp-servers" {
		t.Fatalf("resource = %q", query.Get("resource"))
	}
}

func TestBuildAuthorizationURLOmitsEmptyResource(t *testing.T) {
	t.Parallel()

	got, err := oauth.BuildAuthorizationURL(oauth.AuthorizationRequest{
		AuthorizationEndpoint: "https://dev-myaccount.qtech.cn/api/public/oauth/authorize/contract",
		BusinessType:          "contract",
		ClientID:              "zsdcli_test",
		RedirectURL:           "http://127.0.0.1:8000/callback",
		Scopes:                []string{"mcp:tools", "mcp:resources"},
		State:                 "state-value",
		CodeChallenge:         "challenge-value",
	})
	if err != nil {
		t.Fatalf("BuildAuthorizationURL() error = %v", err)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if _, ok := parsed.Query()["resource"]; ok {
		t.Fatalf("resource query should be omitted when empty, got url %s", got)
	}
}

func TestExchangeAuthorizationCode(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		if r.Form.Get("client_id") != "zsdcli_test" {
			t.Fatalf("client_id = %q", r.Form.Get("client_id"))
		}
		if r.Form.Get("code_verifier") != "verifier" {
			t.Fatalf("code_verifier = %q", r.Form.Get("code_verifier"))
		}
		if r.Form.Get("resource") != "http://higress-gateway.higress-system/mcp-servers" {
			t.Fatalf("resource = %q", r.Form.Get("resource"))
		}
		return jsonResponse(`{"access_token":"token-value","token_type":"Bearer","expires_in":3600,"scope":"mcp:tools mcp:resources"}`), nil
	})}

	token, err := oauth.ExchangeAuthorizationCode(context.Background(), client, nil, oauth.TokenExchangeRequest{
		TokenEndpoint: "https://example.test/oauth/token/contract",
		ClientID:      "zsdcli_test",
		Code:          "code-value",
		CodeVerifier:  "verifier",
		RedirectURL:   "http://127.0.0.1:8000/callback",
		Resource:      "http://higress-gateway.higress-system/mcp-servers",
	})
	if err != nil {
		t.Fatalf("ExchangeAuthorizationCode() error = %v", err)
	}
	if token.AccessToken != "token-value" {
		t.Fatalf("access_token = %q", token.AccessToken)
	}
	if token.TokenType != "Bearer" {
		t.Fatalf("token_type = %q", token.TokenType)
	}
	if time.Until(token.Expiry) <= 0 {
		t.Fatalf("expected future expiry, got %v", token.Expiry)
	}
}

func TestExchangeAuthorizationCodeOmitsEmptyResource(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		if _, ok := r.Form["resource"]; ok {
			t.Fatalf("resource form field should be omitted when empty, got form %v", r.Form)
		}
		return jsonResponse(`{"access_token":"token-value","token_type":"Bearer","expires_in":3600}`), nil
	})}

	_, err := oauth.ExchangeAuthorizationCode(context.Background(), client, nil, oauth.TokenExchangeRequest{
		TokenEndpoint: "https://example.test/oauth/token/contract",
		ClientID:      "zsdcli_test",
		Code:          "code-value",
		CodeVerifier:  "verifier",
		RedirectURL:   "http://127.0.0.1:8000/callback",
	})
	if err != nil {
		t.Fatalf("ExchangeAuthorizationCode() error = %v", err)
	}
}

func TestExchangeTenantAccessToken(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content-type = %q", got)
		}
		if r.URL.String() != "https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal" {
			t.Fatalf("url = %q", r.URL.String())
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if payload["appId"] != "cli_bot_123" {
			t.Fatalf("appId = %q", payload["appId"])
		}
		if payload["appSecret"] != "bot-secret" {
			t.Fatalf("appSecret = %q", payload["appSecret"])
		}

		return jsonResponse(`{"code":0,"expire":7200,"msg":"ok","tenant_access_token":"tenant-token"}`), nil
	})}

	token, err := oauth.ExchangeTenantAccessToken(
		context.Background(),
		client,
		nil,
		"https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
		"cli_bot_123",
		"bot-secret",
	)
	if err != nil {
		t.Fatalf("ExchangeTenantAccessToken() error = %v", err)
	}
	if token.AccessToken != "tenant-token" {
		t.Fatalf("access_token = %q", token.AccessToken)
	}
	if token.TokenType != "Bearer" {
		t.Fatalf("token_type = %q", token.TokenType)
	}
	if time.Until(token.Expiry) <= 0 {
		t.Fatalf("expected future expiry, got %v", token.Expiry)
	}
}

func TestExchangeTenantAccessTokenReturnsErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		response   *http.Response
		wantErr    string
		checkToken func(*testing.T, *config.Token)
	}{
		{
			name:     "non-2xx status",
			response: responseWithStatus(http.StatusBadGateway, `upstream failed`),
			wantErr:  "tenant access token request failed with status 502",
		},
		{
			name:     "business error",
			response: jsonResponse(`{"code":999,"msg":"invalid app"}`),
			wantErr:  "tenant access token request failed: invalid app",
		},
		{
			name:     "missing token",
			response: jsonResponse(`{"code":0,"expire":7200,"msg":"ok"}`),
			wantErr:  "tenant access token response missing tenant_access_token",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if !strings.Contains(r.URL.String(), "tenant_access_token/internal") {
					t.Fatalf("unexpected url: %s", r.URL.String())
				}
				return tc.response, nil
			})}

			token, err := oauth.ExchangeTenantAccessToken(
				context.Background(),
				client,
				nil,
				"https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal",
				"cli_bot_123",
				"bot-secret",
			)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
			if token != nil {
				t.Fatalf("token = %+v, want nil", token)
			}
		})
	}
}

func TestStartCallbackServerRejectsNonHTTPURL(t *testing.T) {
	t.Parallel()

	_, err := oauth.StartCallbackServer("https://127.0.0.1:18080/callback")
	if err == nil {
		t.Fatalf("expected error for non-http redirect url")
	}
}

func TestRegisterClient(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
			t.Fatalf("content-type = %q", contentType)
		}
		return jsonResponse(`{"client_id":"zsdcli_registered"}`), nil
	})}

	response, err := oauth.RegisterClient(context.Background(), client, nil, "https://example.test/oauth/register/contract", oauth.ClientRegistrationRequest{
		ClientName:              "contract-cli",
		RedirectURIs:            []string{"http://127.0.0.1:8000/callback"},
		GrantTypes:              []string{"authorization_code"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "",
	})
	if err != nil {
		t.Fatalf("RegisterClient() error = %v", err)
	}
	if response.ClientID != "zsdcli_registered" {
		t.Fatalf("client_id = %q", response.ClientID)
	}
}
