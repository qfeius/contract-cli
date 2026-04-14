package oauth_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

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
