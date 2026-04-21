package openplatform_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
)

func TestClientDoAddsAuthorizationAndQuery(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://dev-open.qtech.cn/open-apis/mdm/v1/vendors/123?name=acme&user_id=ou_123&user_id_type=employee_id" {
					t.Fatalf("url = %q", req.URL.String())
				}
				if req.Header.Get("Authorization") != "Bearer bot-token" {
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
		Name:                "contract-group",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityBot,
		Identities: config.Identities{
			Bot: config.BotIdentity{
				Token: &config.Token{
					AccessToken: "bot-token",
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
		Name:                "contract-group",
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
		Name:                "contract-group",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityBot,
		Identities: config.Identities{
			Bot: config.BotIdentity{
				Token: &config.Token{
					AccessToken: "bot-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, config.IdentityBot)
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
		Name:                "contract-group",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityBot,
		Identities: config.Identities{
			Bot: config.BotIdentity{
				Token: &config.Token{
					AccessToken: "bot-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, config.IdentityBot)
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
		Name:                "contract-group",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityBot,
		Identities: config.Identities{
			Bot: config.BotIdentity{
				Token: &config.Token{
					AccessToken: "bot-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, config.IdentityBot)
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
		Name:                "contract-group",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityBot,
		Identities: config.Identities{
			Bot: config.BotIdentity{
				Token: &config.Token{
					AccessToken: "bot-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, config.IdentityBot)
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

func TestClientDoRejectsBotOnlyRequestForUserIdentity(t *testing.T) {
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
		Name:                "contract-group",
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
		IdentityPolicy: openplatform.IdentityPolicyBotOnly,
	})
	if err == nil || !strings.Contains(err.Error(), "only supports --as bot") {
		t.Fatalf("unexpected bot-only error: %v", err)
	}
	if transportUsed {
		t.Fatalf("request transport should not be used for rejected bot-only requests")
	}
}

func TestClientDoRejectsUserOnlyRequestForBotIdentity(t *testing.T) {
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
		Name:                "contract-group",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityBot,
		Identities: config.Identities{
			Bot: config.BotIdentity{
				Token: &config.Token{
					AccessToken: "bot-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, config.IdentityBot)
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
		Name:        "contract-group",
		Environment: "dev",
	}, config.IdentityBot)
	if err == nil || !strings.Contains(err.Error(), "open platform base url is not configured") {
		t.Fatalf("unexpected missing-base-url error: %v", err)
	}

	_, err = client.RequestContext(config.Profile{
		Name:                "contract-group",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
	}, config.IdentityBot)
	if err == nil || !strings.Contains(err.Error(), "bot identity is not authorized") {
		t.Fatalf("unexpected missing-token error: %v", err)
	}
}

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
