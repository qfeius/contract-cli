package openplatform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/tracecontext"
)

type AuthProvider interface {
	Resolve(config.Profile, config.IdentityKind) (RequestContext, error)
}

type ProfileAuthProvider struct{}

type Options struct {
	HTTPClient         *http.Client
	Logger             *slog.Logger
	AuthProvider       AuthProvider
	BeforeRequestHooks []BeforeRequestHook
	MaxResponseBytes   int64
}

type BeforeRequestHook func(context.Context, *http.Request) error

type Client struct {
	httpClient         *http.Client
	logger             *slog.Logger
	authProvider       AuthProvider
	beforeRequestHooks []BeforeRequestHook
	maxResponseBytes   int64
}

type RequestContext struct {
	Profile            config.Profile
	Identity           config.IdentityKind
	BaseURL            string
	AccessToken        string
	CommonQuery        url.Values
	PrepareAccessToken func(context.Context, string) (string, error)
	RefreshAccessToken func(context.Context, string) (string, error)
}

type OperationKind string

const (
	OperationRead             OperationKind = "read"
	OperationWrite            OperationKind = "write"
	maxReadNetworkRetries                   = 1
	authenticationErrorHeader               = "X-Qfei-Open-Platform-Auth-Error"
	tokenExpiredErrorType                   = "token_expired"
	defaultMaxResponseBytes                 = int64(32 << 20)
	defaultRequestTimeout                   = 30 * time.Second
	defaultStreamTimeout                    = 10 * time.Minute
)

type Request struct {
	Method         string
	Path           string
	Query          url.Values
	Headers        http.Header
	Body           []byte
	BodyReader     io.Reader
	Raw            bool
	IdentityPolicy IdentityPolicy
	OperationKind  OperationKind
	// FileDownload enables business-error validation for file endpoints only.
	FileDownload bool
}

type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	TraceID    string
}

type HTTPStatusError struct {
	StatusCode int
	ErrorType  string
	Body       []byte
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("open platform request failed with status %d: %s", e.StatusCode, responseSnippet(e.Body))
}

type UncertainWriteError struct {
	Cause error
}

func (e *UncertainWriteError) Error() string {
	if e != nil && e.Cause != nil {
		return fmt.Sprintf("执行结果不确定，请先查询确认: %s", e.Cause)
	}
	return "执行结果不确定，请先查询确认"
}

func (e *UncertainWriteError) Unwrap() error {
	return e.Cause
}

// TraceError preserves the concrete request error through Unwrap while adding
// the trace ID that operators can use for correlation if the request reached
// the server.
type TraceError struct {
	TraceID string
	Cause   error
}

func (e *TraceError) Error() string {
	return fmt.Sprintf("%s (trace_id=%s)", e.Cause, e.TraceID)
}

func (e *TraceError) Unwrap() error {
	return e.Cause
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
	limit := options.MaxResponseBytes
	if limit <= 0 {
		limit = defaultMaxResponseBytes
	}

	return &Client{
		httpClient:         httpClient,
		logger:             logger,
		authProvider:       authProvider,
		beforeRequestHooks: append([]BeforeRequestHook(nil), options.BeforeRequestHooks...),
		maxResponseBytes:   limit,
	}
}

func (c *Client) RequestContext(profile config.Profile, identity config.IdentityKind) (RequestContext, error) {
	return c.authProvider.Resolve(profile, identity)
}

/*
Do 验证所选身份并发送请求；规则 user 请求附加验证分支标记，其他模块行为保持不变。
入参 ctx（context.Context）为执行上下文、requestContext（RequestContext）为已解析身份凭证、request（Request）为接口参数。
返回 Response 为远端响应，error 为校验或网络错误。
*/
func (c *Client) Do(ctx context.Context, requestContext RequestContext, request Request) (response Response, err error) {
	fallback := defaultRequestTimeout
	if request.BodyReader != nil {
		fallback = defaultStreamTimeout
	}
	ctx, cancel := requestBudget(ctx, fallback)
	defer cancel()
	dispatched := false
	defer func() {
		if err != nil && !dispatched {
			err = &requestNotSentError{cause: err}
		}
	}()
	method := strings.ToUpper(strings.TrimSpace(request.Method))
	if method == "" {
		return Response{}, fmt.Errorf("open platform request method is required")
	}
	target, err := prepareRequestTarget(requestContext, request)
	if err != nil {
		c.logger.Error("open platform request target rejected", "method", method, "identity", requestContext.Identity, "error", err.Error())
		return Response{}, err
	}
	fullURL := target.String()
	if requestContext.PrepareAccessToken != nil {
		preparedToken, prepareErr := requestContext.PrepareAccessToken(ctx, requestContext.AccessToken)
		if prepareErr != nil {
			return Response{}, fmt.Errorf("prepare access token: %w", prepareErr)
		}
		if strings.TrimSpace(preparedToken) == "" {
			return Response{}, fmt.Errorf("prepare access token returned an empty token")
		}
		requestContext.AccessToken = preparedToken
	}

	headers := cloneHeaders(request.Headers)
	// 审批矩阵 user 显式选择用户验签分支；此标记不授予权限，后端仍须验证 Bearer。
	if strings.HasPrefix(target.Path, "/open-apis/rule_engine/v1/") {
		headers.Del("X-Qfei-Identity")
		if requestContext.Identity == config.IdentityUser {
			headers.Set("X-Qfei-Identity", "user")
		}
	}
	if headers.Get("Authorization") == "" {
		headers.Set("Authorization", "Bearer "+requestContext.AccessToken)
	}
	if headers.Get("Accept") == "" {
		headers.Set("Accept", "application/json")
	}
	if request.BodyReader == nil && len(request.Body) > 0 && headers.Get("Content-Type") == "" {
		headers.Set("Content-Type", "application/json")
	}

	operation := request.OperationKind
	if operation == "" {
		operation = OperationWrite
		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			operation = OperationRead
		}
	}
	if operation != OperationRead && operation != OperationWrite {
		return Response{}, fmt.Errorf("unsupported open platform operation kind %q", operation)
	}
	requestTrace, err := tracecontext.ForRequest(ctx)
	if err != nil {
		return Response{}, fmt.Errorf("create open platform request trace: %w", err)
	}

	c.logger.Info("open platform request started", "method", method, "path", request.Path, "identity", requestContext.Identity, "trace_id", requestTrace.TraceID)

	refreshed := false
	for attempt := 0; ; attempt++ {
		response, requestErr := c.doOnce(ctx, method, fullURL, headers, request, requestTrace, &dispatched)
		if requestErr == nil {
			c.logger.Info("open platform request completed", "method", method, "path", request.Path, "status_code", response.StatusCode, "trace_id", requestTrace.TraceID)
			return response, nil
		}

		var statusErr *HTTPStatusError
		if errors.As(requestErr, &statusErr) {
			if isTrustedTokenExpired(response, statusErr) && !refreshed && requestContext.RefreshAccessToken != nil {
				newToken, refreshErr := requestContext.RefreshAccessToken(ctx, requestContext.AccessToken)
				if refreshErr != nil {
					return response, wrapTraceError(requestTrace.TraceID, fmt.Errorf("refresh access token: %w", refreshErr))
				}
				if strings.TrimSpace(newToken) == "" {
					return response, wrapTraceError(requestTrace.TraceID, fmt.Errorf("refresh access token returned an empty token"))
				}
				requestContext.AccessToken = newToken
				headers.Set("Authorization", "Bearer "+newToken)
				refreshed = true
				if request.BodyReader != nil {
					return response, wrapTraceError(requestTrace.TraceID, errors.New("access token refreshed, but streaming request body cannot be replayed; retry the command"))
				}
				continue
			}
			if operation == OperationWrite && statusErr.StatusCode >= http.StatusInternalServerError {
				return response, wrapTraceError(requestTrace.TraceID, &UncertainWriteError{Cause: statusErr})
			}
			return response, wrapTraceError(requestTrace.TraceID, statusErr)
		}

		if operation == OperationWrite {
			return response, wrapTraceError(requestTrace.TraceID, &UncertainWriteError{Cause: requestErr})
		}
		if attempt < maxReadNetworkRetries && request.BodyReader == nil && isRetryableNetworkError(requestErr) {
			c.logger.Warn("retrying open platform read after temporary network error", "method", method, "path", request.Path, "trace_id", requestTrace.TraceID)
			continue
		}
		return response, wrapTraceError(requestTrace.TraceID, requestErr)
	}
}

func isTrustedTokenExpired(response Response, statusErr *HTTPStatusError) bool {
	return statusErr != nil &&
		statusErr.StatusCode == http.StatusUnauthorized &&
		statusErr.ErrorType == tokenExpiredErrorType &&
		response.Headers.Get(authenticationErrorHeader) == tokenExpiredErrorType
}

func (c *Client) doOnce(ctx context.Context, method, fullURL string, headers http.Header, request Request, requestTrace tracecontext.RequestTrace, dispatched *bool) (Response, error) {
	response := Response{TraceID: requestTrace.TraceID}
	bodyReader := io.Reader(bytes.NewReader(request.Body))
	if request.BodyReader != nil {
		bodyReader = request.BodyReader
	}
	httpRequest, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return response, fmt.Errorf("build open platform request: %w", err)
	}
	httpRequest.Header = headers.Clone()
	if err := c.runBeforeRequestHooks(ctx, httpRequest); err != nil {
		return response, err
	}
	if err := requestTrace.Apply(httpRequest.Header); err != nil {
		return response, fmt.Errorf("apply open platform request trace: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return response, err
	}
	*dispatched = true
	resp, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return response, fmt.Errorf("perform open platform request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() // Read/status errors determine the request result.

	body, err := c.readResponseBody(resp.Body)
	if err != nil {
		return response, fmt.Errorf("read open platform response: %w", err)
	}
	response.StatusCode = resp.StatusCode
	response.Headers = resp.Header.Clone()
	response.Body = body
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		httpErr := &HTTPStatusError{StatusCode: resp.StatusCode, ErrorType: responseErrorType(body), Body: body}
		c.logger.Error("open platform request failed", "method", method, "path", request.Path, "status_code", resp.StatusCode, "error_type", httpErr.ErrorType, "trace_id", requestTrace.TraceID)
		return response, httpErr
	}
	return response, nil
}

func responseErrorType(body []byte) string {
	var payload struct {
		Data struct {
			ErrorType string `json:"error_type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Data.ErrorType)
}

func isRetryableNetworkError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var networkErr net.Error
	return errors.As(err, &networkErr) && (networkErr.Timeout() || networkErr.Temporary()) //nolint:staticcheck // Preserve legacy retry behavior for existing transports implementing Temporary.
}

func wrapTraceError(traceID string, err error) error {
	if err == nil || strings.TrimSpace(traceID) == "" {
		return err
	}
	if traced, ok := err.(*TraceError); ok && traced.TraceID == traceID {
		return err
	}
	return &TraceError{TraceID: traceID, Cause: err}
}

func (c *Client) DoStream(ctx context.Context, requestContext RequestContext, request Request, writer io.Writer) (Response, error) {
	ctx, cancel := requestBudget(ctx, defaultStreamTimeout)
	defer cancel()
	if writer == nil {
		return Response{}, fmt.Errorf("open platform stream writer is required")
	}

	method := strings.ToUpper(strings.TrimSpace(request.Method))
	if method == "" {
		return Response{}, fmt.Errorf("open platform request method is required")
	}
	target, err := prepareRequestTarget(requestContext, request)
	if err != nil {
		c.logger.Error("open platform stream target rejected", "method", method, "identity", requestContext.Identity, "error", err.Error())
		return Response{}, err
	}
	fullURL := target.String()
	if requestContext.PrepareAccessToken != nil {
		token, err := requestContext.PrepareAccessToken(ctx, requestContext.AccessToken)
		if err != nil {
			return Response{}, fmt.Errorf("prepare access token: %w", err)
		}
		if strings.TrimSpace(token) == "" {
			return Response{}, fmt.Errorf("prepare access token returned an empty token")
		}
		requestContext.AccessToken = token
	}

	headers := cloneHeaders(request.Headers)
	if strings.HasPrefix(target.Path, "/open-apis/rule_engine/v1/") {
		headers.Del("X-Qfei-Identity")
		if requestContext.Identity == config.IdentityUser {
			headers.Set("X-Qfei-Identity", "user")
		}
	}
	if headers.Get("Authorization") == "" {
		headers.Set("Authorization", "Bearer "+requestContext.AccessToken)
	}
	if headers.Get("Accept") == "" {
		headers.Set("Accept", "*/*")
	}
	if request.BodyReader == nil && len(request.Body) > 0 && headers.Get("Content-Type") == "" {
		headers.Set("Content-Type", "application/json")
	}

	requestTrace, err := tracecontext.ForRequest(ctx)
	if err != nil {
		return Response{}, fmt.Errorf("create open platform request trace: %w", err)
	}
	c.logger.Info("open platform stream request started", "method", method, "path", request.Path, "identity", requestContext.Identity, "trace_id", requestTrace.TraceID)
	refreshed := false
	for {
		response, requestErr := c.doStreamOnce(ctx, method, fullURL, headers, request, writer, requestTrace)
		if requestErr == nil {
			c.logger.Info("open platform stream request completed", "method", method, "path", request.Path, "status_code", response.StatusCode, "trace_id", requestTrace.TraceID)
			return response, nil
		}
		var statusErr *HTTPStatusError
		if response.StatusCode == http.StatusUnauthorized && errors.As(requestErr, &statusErr) && isTrustedTokenExpired(response, statusErr) && !refreshed && requestContext.RefreshAccessToken != nil {
			token, err := requestContext.RefreshAccessToken(ctx, requestContext.AccessToken)
			if err != nil {
				return response, wrapTraceError(requestTrace.TraceID, fmt.Errorf("refresh access token: %w", err))
			}
			if strings.TrimSpace(token) == "" {
				return response, wrapTraceError(requestTrace.TraceID, fmt.Errorf("refresh access token returned an empty token"))
			}
			requestContext.AccessToken = token
			headers.Set("Authorization", "Bearer "+token)
			refreshed = true
			if request.BodyReader != nil {
				return response, wrapTraceError(requestTrace.TraceID, errors.New("access token refreshed, but streaming request body cannot be replayed; retry the command"))
			}
			c.logger.Info("retrying stream after trusted token expiry", "method", method, "trace_id", requestTrace.TraceID)
			continue
		}
		c.logger.Error("open platform stream request stopped", "method", method, "status_code", response.StatusCode, "error_type", fmt.Sprintf("%T", requestErr), "trace_id", requestTrace.TraceID)
		return response, wrapTraceError(requestTrace.TraceID, requestErr)
	}
}

func (c *Client) doStreamOnce(ctx context.Context, method, fullURL string, headers http.Header, request Request, writer io.Writer, requestTrace tracecontext.RequestTrace) (Response, error) {
	response := Response{TraceID: requestTrace.TraceID}

	bodyReader := io.Reader(bytes.NewReader(request.Body))
	if request.BodyReader != nil {
		bodyReader = request.BodyReader
	}
	httpRequest, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		c.logger.Error("build open platform stream request failed", "method", method, "path", request.Path, "error", err.Error(), "trace_id", requestTrace.TraceID)
		return response, wrapTraceError(requestTrace.TraceID, fmt.Errorf("build open platform request: %w", err))
	}
	httpRequest.Header = headers.Clone()
	if err := c.runBeforeRequestHooks(ctx, httpRequest); err != nil {
		return response, wrapTraceError(requestTrace.TraceID, err)
	}
	if err := requestTrace.Apply(httpRequest.Header); err != nil {
		return response, wrapTraceError(requestTrace.TraceID, fmt.Errorf("apply open platform request trace: %w", err))
	}

	resp, err := c.httpClient.Do(httpRequest)
	if err != nil {
		c.logger.Error("perform open platform stream request failed", "method", method, "path", request.Path, "error", err.Error(), "trace_id", requestTrace.TraceID)
		return response, wrapTraceError(requestTrace.TraceID, fmt.Errorf("perform open platform request: %w", err))
	}
	defer func() { _ = resp.Body.Close() }() // Read/status errors determine the request result.

	response.StatusCode = resp.StatusCode
	response.Headers = resp.Header.Clone()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := c.readResponseBody(resp.Body)
		if readErr != nil {
			c.logger.Error("read open platform stream error response failed", "method", method, "path", request.Path, "error", readErr.Error(), "trace_id", requestTrace.TraceID)
			return response, wrapTraceError(requestTrace.TraceID, fmt.Errorf("read open platform response: %w", readErr))
		}
		response.Body = body
		return response, &HTTPStatusError{StatusCode: resp.StatusCode, ErrorType: responseErrorType(body), Body: body}
	}

	var source io.Reader = resp.Body
	if request.FileDownload && method != http.MethodHead {
		c.logger.Debug("validate file download response", "trace_id", requestTrace.TraceID)
		source, err = c.fileDownloadBody(resp)
		if err != nil {
			c.logger.Error("file download response rejected", "status_code", resp.StatusCode, "trace_id", requestTrace.TraceID)
			return response, err
		}
	}
	written, err := io.Copy(writer, source)
	if err != nil {
		c.logger.Error("copy open platform stream response failed", "method", method, "path", request.Path, "error", err.Error(), "trace_id", requestTrace.TraceID)
		return response, wrapTraceError(requestTrace.TraceID, fmt.Errorf("copy open platform response: %w", err))
	}
	if method != http.MethodHead && resp.ContentLength >= 0 && written != resp.ContentLength {
		return response, fmt.Errorf("open platform stream size mismatch: expected %d bytes, got %d", resp.ContentLength, written)
	}

	return response, nil
}

func (c *Client) runBeforeRequestHooks(ctx context.Context, request *http.Request) error {
	for index, hook := range c.beforeRequestHooks {
		if hook == nil {
			continue
		}
		if err := hook(ctx, request); err != nil {
			return fmt.Errorf("run open platform before-request hook %d: %w", index, err)
		}
	}
	return nil
}

func (ProfileAuthProvider) Resolve(profile config.Profile, identity config.IdentityKind) (RequestContext, error) {
	resolvedIdentity := identity
	if resolvedIdentity == "" {
		resolvedIdentity = defaultIdentity(profile)
	}

	if strings.TrimSpace(profile.OpenPlatformBaseURL) == "" {
		return RequestContext{}, fmt.Errorf(
			"open platform base url is not configured; run `contract-cli config add --env prod --name %s` first",
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
	case config.IdentityApp:
		token = profile.Identities.App.Token
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
	parsedPath, err := validateRequestPath(path)
	if err != nil {
		return "", err
	}

	parsedBase, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return "", fmt.Errorf("parse open platform base url: %w", err)
	}
	parsedURL := parsedBase.ResolveReference(parsedPath)
	if parsedURL.Scheme != parsedBase.Scheme || parsedURL.Host != parsedBase.Host || parsedURL.Path != parsedPath.Path {
		return "", fmt.Errorf("open platform path changed its target during resolution")
	}
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func mergeQuery(policy IdentityPolicy, requestQuery url.Values, commonQuery url.Values) url.Values {
	if len(requestQuery) == 0 && len(commonQuery) == 0 {
		return url.Values{}
	}

	merged := cloneQuery(requestQuery)
	for key, values := range commonQuery {
		if policy == IdentityPolicyUserOnly {
			if _, exists := merged[key]; exists {
				continue
			}
		}
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

func validateIdentityPolicy(identity config.IdentityKind, policy IdentityPolicy, path string) error {
	switch policy {
	case "", IdentityPolicyAny:
		return nil
	case IdentityPolicyUserOnly:
		if identity != config.IdentityUser {
			return fmt.Errorf("open platform path %q only supports --as user", path)
		}
		return nil
	case IdentityPolicyAppOnly:
		if identity != config.IdentityApp {
			return fmt.Errorf("open platform path %q only supports --as app", path)
		}
		return nil
	default:
		return fmt.Errorf("unsupported identity policy %q", policy)
	}
}
