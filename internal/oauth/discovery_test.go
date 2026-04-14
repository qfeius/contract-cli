package oauth_test

import (
	"context"
	"net/http"
	"testing"

	"cn.qfei/contract-cli/internal/oauth"
)

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
