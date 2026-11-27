package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"cn.qfei/contract-cli/internal/config"
)

const DeviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"

type DeviceAuthorizationRequest struct {
	Endpoint string
	ClientID string
	Scope    string
	Resource string
}

type DeviceAuthorizationResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval,omitempty"`
}

type DeviceTokenRequest struct {
	Endpoint   string
	ClientID   string
	DeviceCode string
}

type DeviceRefreshRequest struct {
	Endpoint     string
	ClientID     string
	RefreshToken string
}

type DeviceRevokeRequest struct {
	Endpoint     string
	ClientID     string
	RefreshToken string
}

type DeviceGrantError struct {
	Code        string
	Description string
}

type deviceGrantRequestError struct {
	cause          error
	requestWritten bool
}

func (e *deviceGrantRequestError) Error() string {
	return "perform device grant request: " + e.cause.Error()
}

func (e *deviceGrantRequestError) Unwrap() error {
	return e.cause
}

func (e *DeviceGrantError) Error() string {
	if e.Description == "" {
		return "device grant failed: " + e.Code
	}
	return "device grant failed: " + e.Code + ": " + e.Description
}

func IsDeviceGrantError(err error, code string) bool {
	var grantError *DeviceGrantError
	return errors.As(err, &grantError) && grantError.Code == code
}

// IsDeviceGrantRequestNotSent reports whether a transport failure happened
// during TCP dialing before any HTTP request bytes were written.
func IsDeviceGrantRequestNotSent(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var requestError *deviceGrantRequestError
	if !errors.As(err, &requestError) || requestError.requestWritten {
		return false
	}
	var operationError *net.OpError
	return errors.As(requestError.cause, &operationError) && operationError.Op == "dial"
}

func StartDeviceAuthorization(ctx context.Context, client *http.Client, request DeviceAuthorizationRequest) (*DeviceAuthorizationResponse, error) {
	if err := validateDeviceRequestEndpoint(request.Endpoint); err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.ClientID) == "" || strings.TrimSpace(request.Scope) == "" || strings.TrimSpace(request.Resource) == "" {
		return nil, errors.New("device authorization requires client_id, scope and resource")
	}
	form := url.Values{
		"client_id": {request.ClientID},
		"scope":     {request.Scope},
		"resource":  {request.Resource},
	}
	var response DeviceAuthorizationResponse
	if err := postDeviceForm(ctx, client, request.Endpoint, form, &response); err != nil {
		return nil, err
	}
	if response.DeviceCode == "" || response.UserCode == "" || response.VerificationURIComplete == "" || response.ExpiresIn <= 0 {
		return nil, errors.New("device authorization response is incomplete")
	}
	return &response, nil
}

func CompleteDeviceAuthorization(ctx context.Context, client *http.Client, request DeviceTokenRequest) (*config.Token, error) {
	if err := validateDeviceRequestEndpoint(request.Endpoint); err != nil {
		return nil, err
	}
	if request.ClientID == "" || request.DeviceCode == "" {
		return nil, errors.New("device token request requires client_id and device_code")
	}
	form := url.Values{
		"grant_type":  {DeviceGrantType},
		"client_id":   {request.ClientID},
		"device_code": {request.DeviceCode},
	}
	return exchangeDeviceToken(ctx, client, request.Endpoint, form)
}

func RefreshDeviceToken(ctx context.Context, client *http.Client, request DeviceRefreshRequest) (*config.Token, error) {
	if err := validateDeviceRequestEndpoint(request.Endpoint); err != nil {
		return nil, err
	}
	if request.ClientID == "" || request.RefreshToken == "" {
		return nil, errors.New("refresh token request requires client_id and refresh_token")
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {request.ClientID},
		"refresh_token": {request.RefreshToken},
	}
	return exchangeDeviceToken(ctx, client, request.Endpoint, form)
}

func RevokeDeviceToken(ctx context.Context, client *http.Client, request DeviceRevokeRequest) error {
	if err := validateDeviceRequestEndpoint(request.Endpoint); err != nil {
		return err
	}
	if strings.TrimSpace(request.ClientID) == "" || strings.TrimSpace(request.RefreshToken) == "" {
		return errors.New("device revoke request requires client_id and refresh_token")
	}
	if client == nil {
		client = http.DefaultClient
	}
	form := url.Values{
		"client_id":       {request.ClientID},
		"token":           {request.RefreshToken},
		"token_type_hint": {"refresh_token"},
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, request.Endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build device revoke request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("perform device revoke request: %w", err)
	}
	defer func() { _ = response.Body.Close() }() // Transport/status/decode errors determine the request result.
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("device revoke request failed with status %d", response.StatusCode)
	}
	return nil
}

func exchangeDeviceToken(ctx context.Context, client *http.Client, endpoint string, form url.Values) (*config.Token, error) {
	var response tokenResponse
	if err := postDeviceForm(ctx, client, endpoint, form, &response); err != nil {
		return nil, err
	}
	if response.AccessToken == "" || response.RefreshToken == "" {
		return nil, errors.New("device token response is incomplete")
	}
	token := &config.Token{
		AccessToken: response.AccessToken, TokenType: response.TokenType,
		Scope: response.Scope, RefreshToken: response.RefreshToken,
	}
	if response.ExpiresIn > 0 {
		token.Expiry = time.Now().Add(time.Duration(response.ExpiresIn) * time.Second)
	}
	return token, nil
}

func postDeviceForm(ctx context.Context, client *http.Client, endpoint string, form url.Values, result any) error {
	if client == nil {
		client = http.DefaultClient
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build device grant request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var requestWritten atomic.Bool
	request = request.WithContext(httptrace.WithClientTrace(request.Context(), &httptrace.ClientTrace{
		WroteRequest: func(httptrace.WroteRequestInfo) {
			requestWritten.Store(true)
		},
	}))
	response, err := client.Do(request)
	if err != nil {
		return &deviceGrantRequestError{cause: err, requestWritten: requestWritten.Load()}
	}
	defer func() { _ = response.Body.Close() }() // Transport/status/decode errors determine the request result.
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var payload struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil || payload.Error == "" {
			return fmt.Errorf("device grant request failed with status %d", response.StatusCode)
		}
		return &DeviceGrantError{Code: payload.Error, Description: payload.ErrorDescription}
	}
	if err := json.NewDecoder(response.Body).Decode(result); err != nil {
		return fmt.Errorf("decode device grant response: %w", err)
	}
	return nil
}

func validateDeviceRequestEndpoint(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || !parsed.IsAbs() || (parsed.Scheme != "https" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost") {
		return errors.New("device grant endpoint must be an absolute https url")
	}
	return nil
}
