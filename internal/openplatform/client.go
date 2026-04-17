package openplatform

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cn.qfei/contract-cli/internal/config"
)

type AuthProvider interface {
	Resolve(config.Profile, config.IdentityKind) (RequestContext, error)
}

type ProfileAuthProvider struct{}

type Options struct {
	HTTPClient   *http.Client
	Logger       *slog.Logger
	AuthProvider AuthProvider
}

type Client struct {
	httpClient   *http.Client
	logger       *slog.Logger
	authProvider AuthProvider
}

type RequestContext struct {
	Profile     config.Profile
	Identity    config.IdentityKind
	BaseURL     string
	AccessToken string
	CommonQuery url.Values
}

type Request struct {
	Method         string
	Path           string
	Query          url.Values
	Headers        http.Header
	Body           []byte
	Raw            bool
	IdentityPolicy IdentityPolicy
}

type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

func New(options Options) *Client {
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	logger := options.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	}

	authProvider := options.AuthProvider
	if authProvider == nil {
		authProvider = ProfileAuthProvider{}
	}

	return &Client{
		httpClient:   httpClient,
		logger:       logger,
		authProvider: authProvider,
	}
}

func (c *Client) RequestContext(profile config.Profile, identity config.IdentityKind) (RequestContext, error) {
	return c.authProvider.Resolve(profile, identity)
}

func (c *Client) Do(ctx context.Context, requestContext RequestContext, request Request) (Response, error) {
	fullURL, err := buildURL(requestContext.BaseURL, request.Path, mergeQuery(request.Query, requestContext.CommonQuery))
	if err != nil {
		return Response{}, err
	}

	method := strings.ToUpper(strings.TrimSpace(request.Method))
	if method == "" {
		return Response{}, fmt.Errorf("open platform request method is required")
	}
	policy := request.IdentityPolicy
	if policy == "" {
		policy = IdentityPolicyForPath(request.Path)
	}
	if err := validateIdentityPolicy(requestContext.Identity, policy, request.Path); err != nil {
		c.logger.Error("open platform identity policy rejected", "method", method, "path", request.Path, "identity", requestContext.Identity, "error", err.Error())
		return Response{}, err
	}

	headers := cloneHeaders(request.Headers)
	if headers.Get("Authorization") == "" {
		headers.Set("Authorization", "Bearer "+requestContext.AccessToken)
	}
	if headers.Get("Accept") == "" {
		headers.Set("Accept", "application/json")
	}
	if len(request.Body) > 0 && headers.Get("Content-Type") == "" {
		headers.Set("Content-Type", "application/json")
	}

	c.logger.Info("open platform request started", "method", method, "path", request.Path, "identity", requestContext.Identity)

	httpRequest, err := http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(request.Body))
	if err != nil {
		c.logger.Error("build open platform request failed", "method", method, "path", request.Path, "error", err.Error())
		return Response{}, fmt.Errorf("build open platform request: %w", err)
	}
	httpRequest.Header = headers

	resp, err := c.httpClient.Do(httpRequest)
	if err != nil {
		c.logger.Error("perform open platform request failed", "method", method, "path", request.Path, "error", err.Error())
		return Response{}, fmt.Errorf("perform open platform request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error("read open platform response failed", "method", method, "path", request.Path, "error", err.Error())
		return Response{}, fmt.Errorf("read open platform response: %w", err)
	}

	response := Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header.Clone(),
		Body:       body,
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = fmt.Errorf("open platform request failed with status %d: %s", resp.StatusCode, responseSnippet(body))
		c.logger.Error("open platform request failed", "method", method, "path", request.Path, "status_code", resp.StatusCode, "error", err.Error())
		return response, err
	}

	c.logger.Info("open platform request completed", "method", method, "path", request.Path, "status_code", resp.StatusCode)
	return response, nil
}

func (ProfileAuthProvider) Resolve(profile config.Profile, identity config.IdentityKind) (RequestContext, error) {
	resolvedIdentity := identity
	if resolvedIdentity == "" {
		resolvedIdentity = defaultIdentity(profile)
	}

	if strings.TrimSpace(profile.OpenPlatformBaseURL) == "" {
		return RequestContext{}, fmt.Errorf(
			"open platform base url is not configured; run `contract-cli config add --env %s --name %s` first",
			emptyFallback(profile.Environment, "dev"),
			profile.Name,
		)
	}

	token, err := tokenForIdentity(profile, resolvedIdentity)
	if err != nil {
		return RequestContext{}, err
	}

	return RequestContext{
		Profile:     profile,
		Identity:    resolvedIdentity,
		BaseURL:     strings.TrimRight(profile.OpenPlatformBaseURL, "/"),
		AccessToken: token.AccessToken,
	}, nil
}

func tokenForIdentity(profile config.Profile, identity config.IdentityKind) (*config.Token, error) {
	var token *config.Token

	switch identity {
	case config.IdentityBot:
		token = profile.Identities.Bot.Token
	case config.IdentityUser:
		token = profile.Identities.User.Token
	default:
		return nil, fmt.Errorf("unsupported identity %q", identity)
	}

	if token == nil || strings.TrimSpace(token.AccessToken) == "" {
		return nil, fmt.Errorf(
			"%s identity is not authorized; run `contract-cli auth login --profile %s --as %s` first",
			identity,
			profile.Name,
			identity,
		)
	}
	if !token.Expiry.IsZero() && time.Now().After(token.Expiry) {
		return nil, fmt.Errorf(
			"%s identity token expired; run `contract-cli auth login --profile %s --as %s` again",
			identity,
			profile.Name,
			identity,
		)
	}
	return token, nil
}

func buildURL(baseURL, path string, query url.Values) (string, error) {
	if strings.Contains(path, "://") || !strings.HasPrefix(path, "/open-apis/") {
		return "", fmt.Errorf("open platform path must be a relative /open-apis/ path")
	}

	parsedBase, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return "", fmt.Errorf("parse open platform base url: %w", err)
	}
	parsedPath, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("parse open platform path: %w", err)
	}
	parsedURL := parsedBase.ResolveReference(parsedPath)
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func mergeQuery(requestQuery url.Values, commonQuery url.Values) url.Values {
	if len(requestQuery) == 0 && len(commonQuery) == 0 {
		return url.Values{}
	}

	merged := cloneQuery(requestQuery)
	for key, values := range commonQuery {
		cloned := make([]string, len(values))
		copy(cloned, values)
		merged[key] = cloned
	}
	return merged
}

func cloneQuery(values url.Values) url.Values {
	if len(values) == 0 {
		return url.Values{}
	}

	cloned := make(url.Values, len(values))
	for key, fieldValues := range values {
		copied := make([]string, len(fieldValues))
		copy(copied, fieldValues)
		cloned[key] = copied
	}
	return cloned
}

func responseSnippet(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 4<<10 {
		return text[:4<<10]
	}
	return text
}

func cloneHeaders(headers http.Header) http.Header {
	if headers == nil {
		return make(http.Header)
	}
	return headers.Clone()
}

func defaultIdentity(profile config.Profile) config.IdentityKind {
	if profile.DefaultIdentity == "" {
		return config.IdentityUser
	}
	return profile.DefaultIdentity
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func validateIdentityPolicy(identity config.IdentityKind, policy IdentityPolicy, path string) error {
	switch policy {
	case "", IdentityPolicyAny:
		return nil
	case IdentityPolicyUserOnly:
		if identity != config.IdentityUser {
			return fmt.Errorf("open platform path %q only supports --as user", path)
		}
		return nil
	default:
		return fmt.Errorf("unsupported identity policy %q", policy)
	}
}
