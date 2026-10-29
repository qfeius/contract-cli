package oauth_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"cn.qfei/contract-cli/internal/oauth"
)

func TestDiscoverChecksDeclaredAuthorizationOriginBeforeFollowingMetadata(t *testing.T) {
	const trusted = "https://accounts.example.test/.well-known/oauth-authorization-server/contract"
	tests := []struct {
		name, server, trusted string
		allowed               bool
	}{
		{"same origin", "https://accounts.example.test/contract", trusted, true},
		{"same origin different path", "https://accounts.example.test/another", trusted, true},
		{"different origin", "https://other.example.test/contract", trusted, false},
		{"http discovery", "http://accounts.example.test/contract", trusted, false},
		{"userinfo discovery", "https://user:secret@accounts.example.test/contract", trusted, false},
		{"different port", "https://accounts.example.test:8443/contract", trusted, false},
		{"invalid trusted scheme", "https://accounts.example.test/contract", "http://accounts.example.test/metadata", false},
		{"userinfo trusted URL", "https://accounts.example.test/contract", "https://user:secret@accounts.example.test/metadata", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				requests++
				if requests == 1 {
					return jsonResponse(fmt.Sprintf(`{"resource":"https://api.example.test","authorization_servers":[%q]}`, test.server)), nil
				}
				return jsonResponse(`{"token_endpoint":"https://accounts.example.test/token"}`), nil
			})}
			_, err := oauth.Discover(context.Background(), client, nil, "https://api.example.test/metadata", test.trusted)
			if test.allowed {
				if err != nil || requests != 2 {
					t.Fatalf("allowed discovery error=%v requests=%d", err, requests)
				}
			} else if err == nil || requests > 1 {
				t.Fatalf("untrusted discovery must stop before the second GET: error=%v requests=%d", err, requests)
			}
		})
	}
}

func TestDiscover(t *testing.T) {
	t.Parallel()

	protectedResource := oauth.ProtectedResourceMetadata{
		Resource:             "https://example.test/mcp-servers",
		AuthorizationServers: []string{"https://example.test/contract"},
		ScopesSupported:      []string{"mcp:tools", "mcp:resources"},
	}
	authServer := oauth.AuthorizationServerMetadata{
		Issuer:                "common-organization-v2",
		AuthorizationEndpoint: "https://example.test/oauth/authorize/contract",
		TokenEndpoint:         "https://example.test/oauth/token/contract",
		RegistrationEndpoint:  "https://example.test/oauth/register/contract",
	}

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/.well-known/oauth-protected-resource":
			return jsonResponse(`{"resource":"https://example.test/mcp-servers","authorization_servers":["https://example.test/contract"],"scopes_supported":["mcp:tools","mcp:resources"]}`), nil
		case "/.well-known/oauth-authorization-server/contract":
			return jsonResponse(`{"issuer":"common-organization-v2","authorization_endpoint":"https://example.test/oauth/authorize/contract","token_endpoint":"https://example.test/oauth/token/contract","registration_endpoint":"https://example.test/oauth/register/contract"}`), nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})}

	got, err := oauth.Discover(context.Background(), client, nil, "https://example.test/.well-known/oauth-protected-resource")
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if got.ProtectedResource.Resource != protectedResource.Resource {
		t.Fatalf("resource mismatch: got %q want %q", got.ProtectedResource.Resource, protectedResource.Resource)
	}
	if got.AuthorizationServer.TokenEndpoint != authServer.TokenEndpoint {
		t.Fatalf("token endpoint mismatch: got %q want %q", got.AuthorizationServer.TokenEndpoint, authServer.TokenEndpoint)
	}
	if got.AuthorizationServerMetadataURL != "https://example.test/.well-known/oauth-authorization-server/contract" {
		t.Fatalf("authorization server metadata url mismatch: got %q", got.AuthorizationServerMetadataURL)
	}
}

func TestAuthorizationServerMetadataURL(t *testing.T) {
	t.Parallel()

	got, err := oauth.AuthorizationServerMetadataURL("https://dev-myaccount.qtech.cn/contract")
	if err != nil {
		t.Fatalf("AuthorizationServerMetadataURL() error = %v", err)
	}
	want := "https://dev-myaccount.qtech.cn/.well-known/oauth-authorization-server/contract"
	if got != want {
		t.Fatalf("AuthorizationServerMetadataURL() = %q, want %q", got, want)
	}
}

func TestDiscoverFromAuthorizationServer(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://dev-myaccount.qtech.cn/.well-known/oauth-authorization-server/contract" {
			t.Fatalf("unexpected url: %s", req.URL.String())
		}
		return jsonResponse(`{"issuer":"common-organization-v2","authorization_endpoint":"https://dev-myaccount.qtech.cn/api/public/oauth/authorize/contract","token_endpoint":"https://dev-myaccount.qtech.cn/api/public/oauth/token/contract","registration_endpoint":"https://dev-myaccount.qtech.cn/api/public/oauth/register/contract"}`), nil
	})}

	got, err := oauth.DiscoverFromAuthorizationServer(
		context.Background(),
		client,
		nil,
		"https://dev-myaccount.qtech.cn/.well-known/oauth-authorization-server/contract",
		"http://higress-gateway.higress-system/mcp-servers",
	)
	if err != nil {
		t.Fatalf("DiscoverFromAuthorizationServer() error = %v", err)
	}
	if got.ProtectedResourceMetadataURL != "" {
		t.Fatalf("protected resource metadata url = %q", got.ProtectedResourceMetadataURL)
	}
	if got.AuthorizationServerMetadataURL != "https://dev-myaccount.qtech.cn/.well-known/oauth-authorization-server/contract" {
		t.Fatalf("authorization server metadata url = %q", got.AuthorizationServerMetadataURL)
	}
	if got.ProtectedResource.Resource != "http://higress-gateway.higress-system/mcp-servers" {
		t.Fatalf("resource = %q", got.ProtectedResource.Resource)
	}
	if got.AuthorizationServer.AuthorizationEndpoint != "https://dev-myaccount.qtech.cn/api/public/oauth/authorize/contract" {
		t.Fatalf("authorization endpoint = %q", got.AuthorizationServer.AuthorizationEndpoint)
	}
}
