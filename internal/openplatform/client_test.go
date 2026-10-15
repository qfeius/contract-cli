package openplatform_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
	"cn.qfei/contract-cli/internal/tracecontext"
)

var traceparentPattern = regexp.MustCompile(`^00-([0-9a-f]{32})-([0-9a-f]{16})-01$`)

func TestClientDoAddsAuthorizationAndQuery(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://dev-open.qtech.cn/open-apis/mdm/v1/vendors/123?name=acme&user_id=ou_123&user_id_type=employee_id" {
					t.Fatalf("url = %q", req.URL.String())
				}
				if req.Header.Get("Authorization") != "Bearer app-token" {
					t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
				}
				if req.Header.Get("Accept") != "application/json" {
					t.Fatalf("accept = %q", req.Header.Get("Accept"))
				}
				return jsonResponse(`{"code":0,"data":{"vendorId":"123"}}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	requestContext, err := client.RequestContext(config.Profile{
		Name:                "contract",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityApp,
		Identities: config.Identities{
			App: config.AppIdentity{
				Token: &config.Token{
					AccessToken: "app-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, "")
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}
	requestContext.CommonQuery = map[string][]string{
		"user_id_type": {"employee_id"},
		"user_id":      {"ou_123"},
	}

	response, err := client.Do(context.Background(), requestContext, openplatform.Request{
		Method: http.MethodGet,
		Path:   "/open-apis/mdm/v1/vendors/123",
		Query: map[string][]string{
			"name": {"acme"},
		},
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestClientDoAddsCanonicalTraceHeaders(t *testing.T) {
	t.Parallel()

	var receivedTraceID string
	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			matches := traceparentPattern.FindStringSubmatch(req.Header.Get(tracecontext.HeaderTraceparent))
			if matches == nil {
				t.Fatalf("traceparent = %q", req.Header.Get(tracecontext.HeaderTraceparent))
			}
			receivedTraceID = matches[1]
			if got := req.Header.Get(tracecontext.HeaderLogID); got != receivedTraceID {
				t.Fatalf("X-Log-Id = %q, want trace id %q", got, receivedTraceID)
			}
			return jsonResponse(`{"code":0}`), nil
		})},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	response, err := client.Do(context.Background(), openplatform.RequestContext{
		Identity: config.IdentityUser, BaseURL: "https://open.qtech.cn", AccessToken: "user-token",
	}, openplatform.Request{
		Method: http.MethodGet,
		Path:   "/open-apis/contract/v1/mcp/contracts/search",
		Headers: http.Header{
			tracecontext.HeaderTraceparent: {"00-11111111111111111111111111111111-2222222222222222-01"},
			tracecontext.HeaderLogID:       {"caller-supplied-log-id"},
		},
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if response.TraceID != receivedTraceID {
		t.Fatalf("response trace id = %q, want %q", response.TraceID, receivedTraceID)
	}
	if receivedTraceID == "11111111111111111111111111111111" {
		t.Fatal("caller-supplied trace headers were not replaced")
	}
}

func TestClientDoRunsBeforeRequestHookForEveryAttempt(t *testing.T) {
	t.Parallel()

	hookCalls := 0
	channels := []string{"doubao", "workbuddy"}
	receivedChannels := make([]string, 0, len(channels))
	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				receivedChannels = append(receivedChannels, req.Header.Get("X-Qfei-Agent-Source-Type"))
				return jsonResponse(`{"code":0}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		BeforeRequestHooks: []openplatform.BeforeRequestHook{
			func(_ context.Context, req *http.Request) error {
				req.Header.Set("X-Qfei-Agent-Source-Type", channels[hookCalls])
				hookCalls++
				return nil
			},
		},
	})
	requestContext := openplatform.RequestContext{
		Identity:    config.IdentityUser,
		BaseURL:     "https://open.qtech.cn",
		AccessToken: "user-token",
	}

	for range channels {
		if _, err := client.Do(context.Background(), requestContext, openplatform.Request{
			Method: http.MethodGet,
			Path:   "/open-apis/contract/v1/mcp/contracts/search",
		}); err != nil {
			t.Fatalf("Do() error = %v", err)
		}
	}

	if hookCalls != 2 {
		t.Fatalf("hook calls = %d, want 2", hookCalls)
	}
	if len(receivedChannels) != 2 || receivedChannels[0] != "doubao" || receivedChannels[1] != "workbuddy" {
		t.Fatalf("received channels = %#v", receivedChannels)
	}
}

func TestClientDoStreamRunsBeforeRequestHook(t *testing.T) {
	t.Parallel()

	hookCalls := 0
	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if got := req.Header.Get("X-Qfei-Agent-Source-Type"); got != "doubao" {
					t.Fatalf("channel header = %q, want doubao", got)
				}
				matches := traceparentPattern.FindStringSubmatch(req.Header.Get(tracecontext.HeaderTraceparent))
				if matches == nil || req.Header.Get(tracecontext.HeaderLogID) != matches[1] {
					t.Fatalf("stream trace headers = traceparent %q, X-Log-Id %q", req.Header.Get(tracecontext.HeaderTraceparent), req.Header.Get(tracecontext.HeaderLogID))
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("stream body")),
					Request:    req,
				}, nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		BeforeRequestHooks: []openplatform.BeforeRequestHook{
			func(_ context.Context, req *http.Request) error {
				hookCalls++
				req.Header.Set("X-Qfei-Agent-Source-Type", "doubao")
				return nil
			},
		},
	})
	requestContext := openplatform.RequestContext{
		Identity:    config.IdentityApp,
		BaseURL:     "https://open.qtech.cn",
		AccessToken: "app-token",
	}

	var output bytes.Buffer
	response, err := client.DoStream(context.Background(), requestContext, openplatform.Request{
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/files/file-1/download",
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	}, &output)
	if err != nil {
		t.Fatalf("DoStream() error = %v", err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(response.TraceID) {
		t.Fatalf("response trace id = %q", response.TraceID)
	}
	if hookCalls != 1 {
		t.Fatalf("hook calls = %d, want 1", hookCalls)
	}
	if output.String() != "stream body" {
		t.Fatalf("stream output = %q", output.String())
	}
}

func TestClientDoCommonQueryPreservesUserOnlyRequestQuery(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://dev-open.qtech.cn/open-apis/contract/v1/mcp/contracts/search?contract_number=CN-001&user_id=ou_123&user_id_type=user_id" {
					t.Fatalf("url = %q", req.URL.String())
				}
				return jsonResponse(`{"code":0}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	requestContext, err := client.RequestContext(config.Profile{
		Name:                "contract",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityUser,
		Identities: config.Identities{
			User: config.UserIdentity{
				Token: &config.Token{
					AccessToken: "user-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, "")
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}
	requestContext.CommonQuery = map[string][]string{
		"user_id_type": {"employee_id"},
		"user_id":      {"ou_123"},
	}

	_, err = client.Do(context.Background(), requestContext, openplatform.Request{
		Method: http.MethodGet,
		Path:   "/open-apis/contract/v1/mcp/contracts/search",
		Query: map[string][]string{
			"contract_number": {"CN-001"},
			"user_id_type":    {"user_id"},
		},
		IdentityPolicy: openplatform.IdentityPolicyUserOnly,
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
}

func TestClientDoCommonQueryOverridesAnyPolicyRequestQuery(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://dev-open.qtech.cn/open-apis/mdm/v1/vendors/123?user_id_type=employee_id" {
					t.Fatalf("url = %q", req.URL.String())
				}
				return jsonResponse(`{"code":0}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	requestContext, err := client.RequestContext(config.Profile{
		Name:                "contract",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityApp,
		Identities: config.Identities{
			App: config.AppIdentity{
				Token: &config.Token{
					AccessToken: "app-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, config.IdentityApp)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}
	requestContext.CommonQuery = map[string][]string{
		"user_id_type": {"employee_id"},
	}

	_, err = client.Do(context.Background(), requestContext, openplatform.Request{
		Method: http.MethodGet,
		Path:   "/open-apis/mdm/v1/vendors/123",
		Query: map[string][]string{
			"user_id_type": {"user_id"},
		},
		IdentityPolicy: openplatform.IdentityPolicyAny,
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
}

func TestClientDoStreamsBodyReaderWithoutJSONContentType(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if got := req.Header.Get("Content-Type"); got != "" {
					t.Fatalf("content-type = %q", got)
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("ReadAll() error = %v", err)
				}
				if string(body) != "streamed body" {
					t.Fatalf("body = %q", string(body))
				}
				return jsonResponse(`{"code":0}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	requestContext, err := client.RequestContext(config.Profile{
		Name:                "contract",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityApp,
		Identities: config.Identities{
			App: config.AppIdentity{
				Token: &config.Token{
					AccessToken: "app-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, config.IdentityApp)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}

	if _, err := client.Do(context.Background(), requestContext, openplatform.Request{
		Method:     http.MethodPost,
		Path:       "/open-apis/contract/v1/files/upload",
		BodyReader: strings.NewReader("streamed body"),
	}); err != nil {
		t.Fatalf("Do() error = %v", err)
	}
}

func TestClientDoPreservesMultipartContentType(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if got := req.Header.Get("Content-Type"); got != "multipart/form-data; boundary=test-boundary" {
					t.Fatalf("content-type = %q", got)
				}
				return jsonResponse(`{"code":0}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	requestContext, err := client.RequestContext(config.Profile{
		Name:                "contract",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityApp,
		Identities: config.Identities{
			App: config.AppIdentity{
				Token: &config.Token{
					AccessToken: "app-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, config.IdentityApp)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}

	if _, err := client.Do(context.Background(), requestContext, openplatform.Request{
		Method:     http.MethodPost,
		Path:       "/open-apis/contract/v1/files/upload",
		BodyReader: strings.NewReader("multipart body"),
		Headers: http.Header{
			"Content-Type": {"multipart/form-data; boundary=test-boundary"},
		},
	}); err != nil {
		t.Fatalf("Do() error = %v", err)
	}
}

func TestClientDoStreamWritesSuccessBody(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("method = %s", req.Method)
				}
				if req.URL.String() != "https://dev-open.qtech.cn/open-apis/contract/v1/files/file-123?user_id=ou_123&user_id_type=user_id" {
					t.Fatalf("url = %q", req.URL.String())
				}
				if req.Header.Get("Authorization") != "Bearer app-token" {
					t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type": {"application/pdf"},
					},
					Body: io.NopCloser(strings.NewReader("download bytes")),
				}, nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	requestContext, err := client.RequestContext(config.Profile{
		Name:                "contract",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityApp,
		Identities: config.Identities{
			App: config.AppIdentity{
				Token: &config.Token{
					AccessToken: "app-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, config.IdentityApp)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}
	requestContext.CommonQuery = map[string][]string{
		"user_id_type": {"user_id"},
		"user_id":      {"ou_123"},
	}

	out := &bytes.Buffer{}
	response, err := client.DoStream(context.Background(), requestContext, openplatform.Request{
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/files/file-123",
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	}, out)
	if err != nil {
		t.Fatalf("DoStream() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if response.Headers.Get("Content-Type") != "application/pdf" {
		t.Fatalf("content-type = %q", response.Headers.Get("Content-Type"))
	}
	if out.String() != "download bytes" {
		t.Fatalf("streamed body = %q", out.String())
	}
	if len(response.Body) != 0 {
		t.Fatalf("stream response should not buffer body, got %q", string(response.Body))
	}
}

func TestClientDoStreamWrapsNon2xxWithoutWritingBody(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return responseWithStatus(http.StatusBadRequest, `{"code":400,"msg":"bad file"}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	requestContext, err := client.RequestContext(config.Profile{
		Name:                "contract",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityApp,
		Identities: config.Identities{
			App: config.AppIdentity{
				Token: &config.Token{
					AccessToken: "app-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, config.IdentityApp)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}

	out := &bytes.Buffer{}
	response, err := client.DoStream(context.Background(), requestContext, openplatform.Request{
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/files/file-123",
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	}, out)
	if err == nil || !strings.Contains(err.Error(), "open platform request failed with status 400") || !strings.Contains(err.Error(), "bad file") {
		t.Fatalf("unexpected DoStream() error: %v", err)
	}
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if out.Len() != 0 {
		t.Fatalf("error response should not be streamed to output, got %q", out.String())
	}
}

func TestClientDoRejectsInvalidPathAndWrapsNon2xx(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return responseWithStatus(http.StatusBadGateway, `gateway failed`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	requestContext, err := client.RequestContext(config.Profile{
		Name:                "contract",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityApp,
		Identities: config.Identities{
			App: config.AppIdentity{
				Token: &config.Token{
					AccessToken: "app-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, config.IdentityApp)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}

	if _, err := client.Do(context.Background(), requestContext, openplatform.Request{
		Method: http.MethodGet,
		Path:   "https://dev-open.qtech.cn/open-apis/mdm/v1/vendors/123",
	}); err == nil || !strings.Contains(err.Error(), "must be a relative /open-apis/ path") {
		t.Fatalf("unexpected invalid-path error: %v", err)
	}

	if _, err := client.Do(context.Background(), requestContext, openplatform.Request{
		Method: http.MethodGet,
		Path:   "/open-apis/mdm/v1/vendors/123",
	}); err == nil || !strings.Contains(err.Error(), "open platform request failed with status 502") {
		t.Fatalf("unexpected non-2xx error: %v", err)
	}
}

func TestClientDoRejectsAppOnlyRequestForUserIdentity(t *testing.T) {
	t.Parallel()

	transportUsed := false
	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				transportUsed = true
				return jsonResponse(`{"code":0}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	requestContext, err := client.RequestContext(config.Profile{
		Name:                "contract",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityUser,
		Identities: config.Identities{
			User: config.UserIdentity{
				Token: &config.Token{
					AccessToken: "user-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, config.IdentityUser)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}

	_, err = client.Do(context.Background(), requestContext, openplatform.Request{
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/files/upload",
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
	if err == nil || !strings.Contains(err.Error(), "only supports --as app") {
		t.Fatalf("unexpected app-only error: %v", err)
	}
	if transportUsed {
		t.Fatalf("request transport should not be used for rejected app-only requests")
	}
}

func TestClientDoRejectsUserOnlyRequestForAppIdentity(t *testing.T) {
	t.Parallel()

	transportUsed := false
	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				transportUsed = true
				return jsonResponse(`{"code":0}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	requestContext, err := client.RequestContext(config.Profile{
		Name:                "contract",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityApp,
		Identities: config.Identities{
			App: config.AppIdentity{
				Token: &config.Token{
					AccessToken: "app-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, config.IdentityApp)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}

	_, err = client.Do(context.Background(), requestContext, openplatform.Request{
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/vendors/123",
		IdentityPolicy: openplatform.IdentityPolicyUserOnly,
	})
	if err == nil || !strings.Contains(err.Error(), "only supports --as user") {
		t.Fatalf("unexpected user-only error: %v", err)
	}
	if transportUsed {
		t.Fatalf("request transport should not be used for rejected user-only requests")
	}
}

func TestIdentityPolicyForPathRecognizesContractMCPAsUserOnly(t *testing.T) {
	t.Parallel()

	if got := openplatform.IdentityPolicyForPath("/open-apis/contract/v1/mcp/vendors"); got != openplatform.IdentityPolicyUserOnly {
		t.Fatalf("policy = %q", got)
	}
	if got := openplatform.IdentityPolicyForPath("/open-apis/mdm/v1/vendors"); got != openplatform.IdentityPolicyAny {
		t.Fatalf("policy = %q", got)
	}
}

func TestRequestContextRequiresConfiguredBaseURLAndToken(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	_, err := client.RequestContext(config.Profile{
		Name:        "contract",
		Environment: "test",
	}, config.IdentityApp)
	if err == nil ||
		!strings.Contains(err.Error(), "open platform base url is not configured") ||
		!strings.Contains(err.Error(), "contract-cli config add --env prod --name contract") ||
		strings.Contains(err.Error(), "`--env test` for test") {
		t.Fatalf("unexpected missing-base-url error: %v", err)
	}

	_, err = client.RequestContext(config.Profile{
		Name:                "contract",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
	}, config.IdentityApp)
	if err == nil || !strings.Contains(err.Error(), "app identity is not authorized") {
		t.Fatalf("unexpected missing-token error: %v", err)
	}
}

func TestClientDoRetriesClassifiedReadNetworkErrorOnce(t *testing.T) {
	calls, hookCalls := 0, 0
	traceIDs := make([]string, 0, 2)
	spanIDs := make([]string, 0, 2)
	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			if req.Header.Get("X-Attempt") != fmt.Sprintf("%d", calls) {
				t.Fatalf("attempt header = %q, want %d", req.Header.Get("X-Attempt"), calls)
			}
			matches := traceparentPattern.FindStringSubmatch(req.Header.Get(tracecontext.HeaderTraceparent))
			if matches == nil {
				t.Fatalf("attempt %d traceparent = %q", calls, req.Header.Get(tracecontext.HeaderTraceparent))
			}
			traceIDs = append(traceIDs, matches[1])
			spanIDs = append(spanIDs, matches[2])
			if req.Header.Get(tracecontext.HeaderLogID) != matches[1] {
				t.Fatalf("attempt %d X-Log-Id = %q, want %q", calls, req.Header.Get(tracecontext.HeaderLogID), matches[1])
			}
			if calls == 1 {
				return nil, temporaryNetworkError{}
			}
			return responseWithStatus(http.StatusOK, `{}`), nil
		})},
		BeforeRequestHooks: []openplatform.BeforeRequestHook{func(_ context.Context, req *http.Request) error {
			hookCalls++
			req.Header.Set("X-Attempt", fmt.Sprintf("%d", hookCalls))
			return nil
		}},
	})
	_, err := client.Do(context.Background(), openplatform.RequestContext{BaseURL: "https://example.test", AccessToken: "token", Identity: config.IdentityUser}, openplatform.Request{
		Method: http.MethodPost, Path: "/open-apis/contract/v1/mcp/contracts/search", Body: []byte(`{}`), OperationKind: openplatform.OperationRead,
	})
	if err != nil || calls != 2 || hookCalls != 2 {
		t.Fatalf("err=%v calls=%d hookCalls=%d", err, calls, hookCalls)
	}
	if traceIDs[0] != traceIDs[1] {
		t.Fatalf("retry trace ids = %#v, want one logical trace", traceIDs)
	}
	if spanIDs[0] == spanIDs[1] {
		t.Fatalf("retry span ids = %#v, want one span per HTTP attempt", spanIDs)
	}
}

func TestClientDoNeverRetriesWriteAndReturnsUncertainError(t *testing.T) {
	calls := 0
	client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return nil, temporaryNetworkError{}
	})}})
	_, err := client.Do(context.Background(), openplatform.RequestContext{BaseURL: "https://example.test", AccessToken: "token", Identity: config.IdentityUser}, openplatform.Request{
		Method: http.MethodPost, Path: "/open-apis/contract/v1/mcp/contracts", Body: []byte(`{}`), OperationKind: openplatform.OperationWrite,
	})
	var uncertain *openplatform.UncertainWriteError
	if !errors.As(err, &uncertain) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestClientDoTreatsWriteServerErrorAsUncertainWithoutRetry(t *testing.T) {
	calls := 0
	client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return responseWithStatus(http.StatusBadGateway, `{"code":502}`), nil
	})}})
	_, err := client.Do(context.Background(), openplatform.RequestContext{BaseURL: "https://example.test", AccessToken: "token", Identity: config.IdentityUser}, openplatform.Request{
		Method: http.MethodPost, Path: "/open-apis/contract/v1/mcp/contracts", Body: []byte(`{}`), OperationKind: openplatform.OperationWrite,
	})
	var uncertain *openplatform.UncertainWriteError
	if !errors.As(err, &uncertain) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestClientDoDoesNotRefreshOnGenericUnauthorized(t *testing.T) {
	refreshCalls := 0
	client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return responseWithStatus(http.StatusUnauthorized, `{"code":401}`), nil
	})}})
	response, err := client.Do(context.Background(), openplatform.RequestContext{
		BaseURL: "https://example.test", AccessToken: "token", Identity: config.IdentityUser,
		RefreshAccessToken: func(context.Context, string) (string, error) {
			refreshCalls++
			return "refreshed", nil
		},
	}, openplatform.Request{Method: http.MethodGet, Path: "/open-apis/contract/v1/mcp/templates", OperationKind: openplatform.OperationRead})
	var statusErr *openplatform.HTTPStatusError
	if !errors.As(err, &statusErr) || refreshCalls != 0 {
		t.Fatalf("err=%v refreshCalls=%d", err, refreshCalls)
	}
	if response.TraceID == "" || !strings.Contains(err.Error(), "trace_id="+response.TraceID) {
		t.Fatalf("response trace id = %q, error = %v", response.TraceID, err)
	}
	var traceErr *openplatform.TraceError
	if !errors.As(err, &traceErr) || traceErr.TraceID != response.TraceID {
		t.Fatalf("trace error = %#v, response trace id = %q", traceErr, response.TraceID)
	}
}

func TestClientDoRefreshesOnceOnExplicitTokenExpired(t *testing.T) {
	calls, refreshCalls, hookCalls := 0, 0, 0
	traceIDs := make([]string, 0, 2)
	spanIDs := make([]string, 0, 2)
	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			if req.Header.Get("X-Attempt") != fmt.Sprintf("%d", calls) {
				t.Fatalf("attempt header = %q, want %d", req.Header.Get("X-Attempt"), calls)
			}
			matches := traceparentPattern.FindStringSubmatch(req.Header.Get(tracecontext.HeaderTraceparent))
			if matches == nil {
				t.Fatalf("attempt %d traceparent = %q", calls, req.Header.Get(tracecontext.HeaderTraceparent))
			}
			traceIDs = append(traceIDs, matches[1])
			spanIDs = append(spanIDs, matches[2])
			if calls == 1 {
				return responseWithAuthenticationError(http.StatusUnauthorized, "token_expired", `{"code":401,"data":{"error_type":"token_expired"}}`), nil
			}
			if req.Header.Get("Authorization") != "Bearer refreshed" {
				t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
			}
			return responseWithStatus(http.StatusOK, `{}`), nil
		})},
		BeforeRequestHooks: []openplatform.BeforeRequestHook{func(_ context.Context, req *http.Request) error {
			hookCalls++
			req.Header.Set("X-Attempt", fmt.Sprintf("%d", hookCalls))
			return nil
		}},
	})
	requestContext := openplatform.RequestContext{
		BaseURL: "https://example.test", AccessToken: "expired", Identity: config.IdentityUser,
		RefreshAccessToken: func(context.Context, string) (string, error) {
			refreshCalls++
			return "refreshed", nil
		},
	}
	_, err := client.Do(context.Background(), requestContext, openplatform.Request{
		Method: http.MethodPost, Path: "/open-apis/contract/v1/mcp/contracts", Body: []byte(`{}`), OperationKind: openplatform.OperationWrite,
	})
	if err != nil || calls != 2 || refreshCalls != 1 || hookCalls != 2 {
		t.Fatalf("err=%v calls=%d refreshCalls=%d hookCalls=%d", err, calls, refreshCalls, hookCalls)
	}
	if traceIDs[0] != traceIDs[1] || spanIDs[0] == spanIDs[1] {
		t.Fatalf("token refresh trace ids = %#v, span ids = %#v", traceIDs, spanIDs)
	}
}

func TestClientDoDoesNotRefreshOnUntrustedTokenExpiredResponse(t *testing.T) {
	tests := []struct {
		name     string
		response *http.Response
	}{
		{
			name:     "body without trusted header",
			response: responseWithStatus(http.StatusUnauthorized, `{"data":{"error_type":"token_expired"}}`),
		},
		{
			name:     "trusted header without body marker",
			response: responseWithAuthenticationError(http.StatusUnauthorized, "token_expired", `{"code":401}`),
		},
		{
			name:     "non unauthorized status",
			response: responseWithAuthenticationError(http.StatusForbidden, "token_expired", `{"data":{"error_type":"token_expired"}}`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls, refreshCalls := 0, 0
			client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				return test.response, nil
			})}})

			_, err := client.Do(context.Background(), openplatform.RequestContext{
				BaseURL: "https://example.test", AccessToken: "expired", Identity: config.IdentityUser,
				RefreshAccessToken: func(context.Context, string) (string, error) {
					refreshCalls++
					return "refreshed", nil
				},
			}, openplatform.Request{
				Method: http.MethodPost, Path: "/open-apis/contract/v1/mcp/contracts", Body: []byte(`{}`), OperationKind: openplatform.OperationWrite,
			})

			var statusErr *openplatform.HTTPStatusError
			if !errors.As(err, &statusErr) || calls != 1 || refreshCalls != 0 {
				t.Fatalf("err=%v calls=%d refreshCalls=%d", err, calls, refreshCalls)
			}
		})
	}
}

func TestClientDoDoesNotReplayStreamingBodyAfterTokenExpired(t *testing.T) {
	calls, refreshCalls := 0, 0
	client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return responseWithAuthenticationError(http.StatusUnauthorized, "token_expired", `{"data":{"error_type":"token_expired"}}`), nil
	})}})
	_, err := client.Do(context.Background(), openplatform.RequestContext{
		BaseURL: "https://example.test", AccessToken: "expired", Identity: config.IdentityUser,
		RefreshAccessToken: func(context.Context, string) (string, error) {
			refreshCalls++
			return "refreshed", nil
		},
	}, openplatform.Request{
		Method: http.MethodPost, Path: "/open-apis/contract/v1/files/upload",
		BodyReader: strings.NewReader("multipart body"), OperationKind: openplatform.OperationWrite,
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be replayed") || calls != 1 || refreshCalls != 1 {
		t.Fatalf("err=%v calls=%d refreshCalls=%d", err, calls, refreshCalls)
	}
}

type temporaryNetworkError struct{}

func (temporaryNetworkError) Error() string   { return "temporary network error" }
func (temporaryNetworkError) Timeout() bool   { return true }
func (temporaryNetworkError) Temporary() bool { return true }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(payload string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(payload)),
	}
}

func responseWithStatus(statusCode int, payload string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(payload)),
	}
}

func responseWithAuthenticationError(statusCode int, errorType, payload string) *http.Response {
	response := responseWithStatus(statusCode, payload)
	response.Header.Set("X-Qfei-Open-Platform-Auth-Error", errorType)
	return response
}
