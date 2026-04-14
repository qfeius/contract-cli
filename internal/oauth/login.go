package oauth

import (
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
	query.Set("resource", request.Resource)
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
	form.Set("resource", request.Resource)

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
