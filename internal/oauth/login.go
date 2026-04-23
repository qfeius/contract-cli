package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"cn.qfei/contract-cli/internal/config"
)

type AuthorizationRequest struct {
	AuthorizationEndpoint string
	BusinessType          string
	ClientID              string
	RedirectURL           string
	Resource              string
	Scopes                []string
	State                 string
	CodeChallenge         string
}

type TokenExchangeRequest struct {
	TokenEndpoint string
	ClientID      string
	Code          string
	CodeVerifier  string
	RedirectURL   string
	Resource      string
}

type CallbackServer struct {
	listener net.Listener
	server   *http.Server
	results  chan callbackResult
	once     sync.Once
}

type callbackResult struct {
	code  string
	state string
	err   string
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
}

type tenantAccessTokenRequest struct {
	AppID     string `json:"appId"`
	AppSecret string `json:"appSecret"`
}

type tenantAccessTokenResponse struct {
	Code              int    `json:"code"`
	Expire            int64  `json:"expire"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
}

func BuildAuthorizationURL(request AuthorizationRequest) (string, error) {
	parsed, err := url.Parse(request.AuthorizationEndpoint)
	if err != nil {
		return "", fmt.Errorf("parse authorization endpoint: %w", err)
	}

	query := parsed.Query()
	query.Set("business_type", request.BusinessType)
	query.Set("client_id", request.ClientID)
	query.Set("code_challenge", request.CodeChallenge)
	query.Set("code_challenge_method", "S256")
	query.Set("redirect_uri", request.RedirectURL)
	if strings.TrimSpace(request.Resource) != "" {
		query.Set("resource", request.Resource)
	}
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(request.Scopes, " "))
	query.Set("state", request.State)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func ExchangeAuthorizationCode(ctx context.Context, client *http.Client, logger *slog.Logger, request TokenExchangeRequest) (*config.Token, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if logger != nil {
		logger.Info("exchange authorization code", "token_endpoint", request.TokenEndpoint)
	}

	form := url.Values{}
	form.Set("client_id", request.ClientID)
	form.Set("code", request.Code)
	form.Set("code_verifier", request.CodeVerifier)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", request.RedirectURL)
	if strings.TrimSpace(request.Resource) != "" {
		form.Set("resource", request.Resource)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, request.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build token request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("perform token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var payload tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if payload.AccessToken == "" {
		return nil, fmt.Errorf("token response missing access_token")
	}

	token := &config.Token{
		AccessToken:  payload.AccessToken,
		TokenType:    payload.TokenType,
		Scope:        payload.Scope,
		RefreshToken: payload.RefreshToken,
	}
	if payload.ExpiresIn > 0 {
		token.Expiry = time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	}
	return token, nil
}

func ExchangeTenantAccessToken(ctx context.Context, client *http.Client, logger *slog.Logger, endpoint, appID, appSecret string) (*config.Token, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if logger != nil {
		logger.Info("exchange tenant access token", "token_endpoint", endpoint)
	}

	payload, err := json.Marshal(tenantAccessTokenRequest{
		AppID:     appID,
		AppSecret: appSecret,
	})
	if err != nil {
		if logger != nil {
			logger.Error("encode tenant access token request failed", "token_endpoint", endpoint, "error", err.Error())
		}
		return nil, fmt.Errorf("encode tenant access token request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		if logger != nil {
			logger.Error("build tenant access token request failed", "token_endpoint", endpoint, "error", err.Error())
		}
		return nil, fmt.Errorf("build tenant access token request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpRequest)
	if err != nil {
		if logger != nil {
			logger.Error("perform tenant access token request failed", "token_endpoint", endpoint, "error", err.Error())
		}
		return nil, fmt.Errorf("perform tenant access token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		err = fmt.Errorf("tenant access token request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
		if logger != nil {
			logger.Error("tenant access token request returned non-success status", "token_endpoint", endpoint, "status_code", resp.StatusCode, "error", err.Error())
		}
		return nil, err
	}

	var response tenantAccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		if logger != nil {
			logger.Error("decode tenant access token response failed", "token_endpoint", endpoint, "error", err.Error())
		}
		return nil, fmt.Errorf("decode tenant access token response: %w", err)
	}
	if response.Code != 0 {
		err = fmt.Errorf("tenant access token request failed: %s", emptyTenantAccessTokenMessage(response.Msg))
		if logger != nil {
			logger.Error("tenant access token request returned business error", "token_endpoint", endpoint, "code", response.Code, "error", err.Error())
		}
		return nil, err
	}
	if response.TenantAccessToken == "" {
		err = fmt.Errorf("tenant access token response missing tenant_access_token")
		if logger != nil {
			logger.Error("tenant access token response missing token", "token_endpoint", endpoint, "error", err.Error())
		}
		return nil, err
	}

	token := &config.Token{
		AccessToken: response.TenantAccessToken,
		TokenType:   "Bearer",
	}
	if response.Expire > 0 {
		token.Expiry = time.Now().Add(time.Duration(response.Expire) * time.Second)
	}

	if logger != nil {
		attrs := []any{"token_endpoint", endpoint}
		if !token.Expiry.IsZero() {
			attrs = append(attrs, "expires_at", token.Expiry.Format(time.RFC3339))
		}
		logger.Info("tenant access token exchange completed", attrs...)
	}
	return token, nil
}

func emptyTenantAccessTokenMessage(message string) string {
	if strings.TrimSpace(message) == "" {
		return "unknown error"
	}
	return message
}

func StartCallbackServer(redirectURL string) (*CallbackServer, error) {
	parsed, err := url.Parse(redirectURL)
	if err != nil {
		return nil, fmt.Errorf("parse redirect url: %w", err)
	}
	if parsed.Scheme != "http" {
		return nil, fmt.Errorf("redirect url must use http")
	}

	path := parsed.Path
	if path == "" {
		path = "/"
	}

	listener, err := net.Listen("tcp", parsed.Host)
	if err != nil {
		return nil, fmt.Errorf("listen on redirect host %s: %w", parsed.Host, err)
	}

	callback := &CallbackServer{
		listener: listener,
		results:  make(chan callbackResult, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		callback.results <- callbackResult{
			code:  query.Get("code"),
			state: query.Get("state"),
			err:   query.Get("error"),
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "Authorization received. You can return to the terminal.\n")
	})

	callback.server = &http.Server{Handler: mux}
	go func() {
		_ = callback.server.Serve(listener)
	}()

	return callback, nil
}

func (s *CallbackServer) Wait(ctx context.Context, expectedState string) (string, error) {
	defer s.Close()

	select {
	case <-ctx.Done():
		return "", fmt.Errorf("authorization cancelled or timed out: %w", ctx.Err())
	case result := <-s.results:
		if result.err != "" {
			return "", fmt.Errorf("authorization failed: %s", result.err)
		}
		if result.state != expectedState {
			return "", fmt.Errorf("authorization state mismatch")
		}
		if result.code == "" {
			return "", fmt.Errorf("authorization callback missing code")
		}
		return result.code, nil
	}
}

func (s *CallbackServer) Close() {
	s.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.server.Shutdown(ctx)
	})
}

func OpenBrowser(authURL string) error {
	var command string
	switch runtime.GOOS {
	case "darwin":
		command = "open"
	case "linux":
		command = "xdg-open"
	case "windows":
		command = "rundll32"
	default:
		return fmt.Errorf("unsupported platform for browser auto-open")
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command(command, "url.dll,FileProtocolHandler", authURL)
	} else {
		cmd = exec.Command(command, authURL)
	}
	return cmd.Start()
}
