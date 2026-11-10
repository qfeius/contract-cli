package openplatform_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
	"cn.qfei/contract-cli/internal/tracecontext"
)

func TestClientCommandTraceSharedAcrossClientsAndStreaming(t *testing.T) {
	t.Parallel()
	ctx, trace, err := tracecontext.NewCommandContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var headers []http.Header
	options := openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		headers = append(headers, req.Header.Clone())
		return jsonResponse(`{"code":0}`), nil
	})}, BeforeRequestHooks: []openplatform.BeforeRequestHook{func(_ context.Context, req *http.Request) error {
		req.Header.Set("traceparent", "caller-value")
		req.Header.Set("X-Log-Id", "caller-value")
		return nil
	}}}
	rc := openplatform.RequestContext{Identity: config.IdentityUser, BaseURL: "https://open.qtech.cn", AccessToken: "test-token"}
	request := openplatform.Request{Method: http.MethodGet, Path: "/open-apis/contract/v1/mcp/contracts/search"}
	first, err := openplatform.New(options).Do(ctx, rc, request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := openplatform.New(options).DoStream(ctx, rc, request, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if first.TraceID != trace.TraceID || second.TraceID != trace.TraceID {
		t.Fatalf("responses have trace IDs %q, %q, want %q", first.TraceID, second.TraceID, trace.TraceID)
	}
	assertClientCommandHeaders(t, headers, trace.TraceID)
}

func assertClientCommandHeaders(t *testing.T, headers []http.Header, id string) {
	t.Helper()
	if len(headers) < 2 {
		t.Fatalf("request count = %d", len(headers))
	}
	parents := map[string]bool{}
	for _, h := range headers {
		parts := traceparentPattern.FindStringSubmatch(h.Get("traceparent"))
		if parts == nil || parts[1] != id || h.Get("X-Log-Id") != id {
			t.Fatalf("headers = %v, want trace ID %s", h, id)
		}
		if parents[parts[2]] {
			t.Fatal("explicit sends reused a parent ID")
		}
		parents[parts[2]] = true
	}
}

func TestClientCommandTraceSurvivesRetryAndTokenRefresh(t *testing.T) {
	for _, kind := range []string{"network", "token"} {
		t.Run(kind, func(t *testing.T) {
			ctx, trace, err := tracecontext.NewCommandContext(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			var headers []http.Header
			client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				headers = append(headers, req.Header.Clone())
				if len(headers) == 1 && kind == "network" {
					return nil, temporaryNetworkError{}
				}
				if len(headers) == 1 {
					return responseWithAuthenticationError(http.StatusUnauthorized, "token_expired", `{"code":401,"data":{"error_type":"token_expired"}}`), nil
				}
				return jsonResponse(`{"code":0}`), nil
			})}})
			rc := openplatform.RequestContext{Identity: config.IdentityUser, BaseURL: "https://open.qtech.cn", AccessToken: "expired",
				RefreshAccessToken: func(callback context.Context, _ string) (string, error) {
					if got, ok := tracecontext.FromContext(callback); !ok || got != trace {
						t.Fatal("refresh lost command context")
					}
					return "refreshed", nil
				}}
			request := openplatform.Request{Method: http.MethodGet, Path: "/open-apis/contract/v1/mcp/contracts/search"}
			for range 2 {
				if _, err := client.Do(ctx, rc, request); err != nil {
					t.Fatal(err)
				}
			}
			if len(headers) != 3 {
				t.Fatalf("attempt count = %d", len(headers))
			}
			assertClientCommandHeaders(t, headers, trace.TraceID)
		})
	}
}

func TestClientWithoutCommandContextRetainsRequestLevelTrace(t *testing.T) {
	t.Parallel()
	client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"code":0}`), nil
	})}})
	rc := openplatform.RequestContext{Identity: config.IdentityUser, BaseURL: "https://open.qtech.cn", AccessToken: "test-token"}
	request := openplatform.Request{Method: http.MethodGet, Path: "/open-apis/contract/v1/mcp/contracts/search"}
	first, err := client.Do(context.Background(), rc, request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.DoStream(context.Background(), rc, request, &strings.Builder{})
	if err != nil || first.TraceID == second.TraceID {
		t.Fatalf("standalone traces = %s, %s, error = %v", first.TraceID, second.TraceID, err)
	}
}
