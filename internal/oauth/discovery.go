package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

type ProtectedResourceMetadata struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
	ScopesSupported      []string `json:"scopes_supported"`
}

type AuthorizationServerMetadata struct {
	Issuer                          string   `json:"issuer"`
	AuthorizationEndpoint           string   `json:"authorization_endpoint"`
	TokenEndpoint                   string   `json:"token_endpoint"`
	RegistrationEndpoint            string   `json:"registration_endpoint"`
	CodeChallengeMethodsSupported   []string `json:"code_challenge_methods_supported"`
	GrantTypesSupported             []string `json:"grant_types_supported"`
	ResponseTypesSupported          []string `json:"response_types_supported"`
	TokenEndpointAuthMethodsSupport []string `json:"token_endpoint_auth_methods_supported"`
}

type DiscoveryResult struct {
	ProtectedResourceMetadataURL   string
	AuthorizationServerMetadataURL string
	ProtectedResource              ProtectedResourceMetadata
	AuthorizationServer            AuthorizationServerMetadata
}

func Discover(ctx context.Context, client *http.Client, logger *slog.Logger, protectedResourceURL string) (*DiscoveryResult, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if logger != nil {
		logger.Info("discover oauth metadata", "protected_resource_url", protectedResourceURL)
	}

	protected, err := getJSON[ProtectedResourceMetadata](ctx, client, protectedResourceURL)
	if err != nil {
		if logger != nil {
			logger.Error("discover protected resource failed", "error", err.Error())
		}
		return nil, err
	}
	if len(protected.AuthorizationServers) == 0 {
		return nil, fmt.Errorf("protected resource metadata missing authorization_servers")
	}

	authMetadataURL, err := AuthorizationServerMetadataURL(protected.AuthorizationServers[0])
	if err != nil {
		return nil, err
	}
	authServer, err := getJSON[AuthorizationServerMetadata](ctx, client, authMetadataURL)
	if err != nil {
		if logger != nil {
			logger.Error("discover authorization server failed", "error", err.Error())
		}
		return nil, err
	}

	result := &DiscoveryResult{
		ProtectedResourceMetadataURL:   protectedResourceURL,
		AuthorizationServerMetadataURL: authMetadataURL,
		ProtectedResource:              protected,
		AuthorizationServer:            authServer,
	}
	if logger != nil {
		logger.Info("oauth metadata discovered", "authorization_server_metadata_url", authMetadataURL)
	}
	return result, nil
}

func AuthorizationServerMetadataURL(rawServer string) (string, error) {
	parsed, err := url.Parse(rawServer)
	if err != nil {
		return "", fmt.Errorf("parse authorization server url: %w", err)
	}

	if strings.Contains(parsed.Path, ".well-known/oauth-authorization-server") {
		return parsed.String(), nil
	}

	path := strings.Trim(parsed.Path, "/")
	parsed.Path = "/.well-known/oauth-authorization-server"
	if path != "" {
		parsed.Path += "/" + path
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func getJSON[T any](ctx context.Context, client *http.Client, endpoint string) (T, error) {
	var zero T

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return zero, fmt.Errorf("build GET request for %s: %w", endpoint, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return zero, fmt.Errorf("perform GET request for %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return zero, fmt.Errorf("GET %s failed with status %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload T
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return zero, fmt.Errorf("decode GET %s response: %w", endpoint, err)
	}
	return payload, nil
}
