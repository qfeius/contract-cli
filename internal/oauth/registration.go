package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type ClientRegistrationRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

type ClientRegistrationResponse struct {
	ClientID string `json:"client_id"`
}

func RegisterClient(ctx context.Context, client *http.Client, logger *slog.Logger, endpoint string, request ClientRegistrationRequest) (*ClientRegistrationResponse, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if logger != nil {
		logger.Info("register oauth client", "registration_endpoint", endpoint, "client_name", request.ClientName)
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode client registration request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build client registration request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("perform client registration request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("client registration failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var payload ClientRegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode client registration response: %w", err)
	}
	if payload.ClientID == "" {
		return nil, fmt.Errorf("client registration response missing client_id")
	}
	return &payload, nil
}
