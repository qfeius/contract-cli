package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/invocation"
	"cn.qfei/contract-cli/internal/openplatform"
	"cn.qfei/contract-cli/internal/tracecontext"
)

func TestBeforeOpenPlatformRequestDetectsAgainForEveryRequest(t *testing.T) {
	reports := []invocation.Result{
		{
			ChannelType:     "cli",
			AgentSourceType: "doubao",
			ProductCode:     invocation.ProductCodeContract,
			EvidenceType:    "macos_code_signature",
			Confidence:      "high",
			DetectorVersion: invocation.DetectorVersion,
			RuleID:          "client.doubao.signed-bundle",
		},
		{
			ChannelType:     "cli",
			AgentSourceType: "workbuddy",
			ProductCode:     invocation.ProductCodeContract,
			EvidenceType:    "macos_code_signature",
			Confidence:      "high",
			DetectorVersion: invocation.DetectorVersion,
			RuleID:          "client.workbuddy.signed-bundle",
		},
	}
	calls := 0
	app := &App{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		inspectEnvironment: func(_ context.Context, depth int) invocation.Result {
			if depth != invocation.DefaultMaxDepth {
				t.Fatalf("depth = %d", depth)
			}
			result := reports[calls]
			calls++
			return result
		},
	}

	for index, wantAgentSource := range []string{"doubao", "workbuddy"} {
		request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://open.qtech.cn/open-apis/test", nil)
		if err != nil {
			t.Fatalf("NewRequest() error = %v", err)
		}
		if err := app.beforeOpenPlatformRequest(context.Background(), request); err != nil {
			t.Fatalf("beforeOpenPlatformRequest() error = %v", err)
		}
		if got := request.Header.Get(invocation.HeaderChannelType); got != "cli" {
			t.Fatalf("request %d channel = %q, want cli", index, got)
		}
		if got := request.Header.Get(invocation.HeaderAgentSourceType); got != wantAgentSource {
			t.Fatalf("request %d agent source = %q, want %q", index, got, wantAgentSource)
		}
		if got := request.Header.Get(invocation.HeaderProductCode); got != invocation.ProductCodeContract {
			t.Fatalf("request %d product code = %q", index, got)
		}
	}

	if calls != 2 {
		t.Fatalf("detector calls = %d, want 2", calls)
	}
}

func TestOpenPlatformClientWiresFreshEnvironmentHookIntoEveryAttempt(t *testing.T) {
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(config.Profile{
		Name:                "contract",
		Environment:         "prod",
		OpenPlatformBaseURL: "https://open.qfei.cn",
		Resource:            "https://open.qfei.cn",
		DefaultIdentity:     config.IdentityUser,
		Identities: config.Identities{
			User: config.UserIdentity{Token: &config.Token{
				AccessToken: "user-token",
				Expiry:      time.Now().Add(time.Hour),
			}},
		},
	}, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	channels := []string{"doubao", "workbuddy"}
	detectorCalls := 0
	receivedChannels := make([]string, 0, len(channels))
	app := New(Options{
		Stdout:          io.Discard,
		Stderr:          io.Discard,
		Store:           store,
		CredentialStore: &deviceMemoryCredentialStore{},
		HTTPClient: &http.Client{Transport: roundTripFuncInternal(func(request *http.Request) (*http.Response, error) {
			if got := request.Header.Get(invocation.HeaderChannelType); got != "cli" {
				t.Fatalf("channel = %q, want cli", got)
			}
			receivedChannels = append(receivedChannels, request.Header.Get(invocation.HeaderAgentSourceType))
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"code":0}`)),
				Request:    request,
			}, nil
		})},
		InspectEnvironment: func(context.Context, int) invocation.Result {
			channel := channels[detectorCalls]
			detectorCalls++
			return invocation.Result{
				ChannelType:     "cli",
				AgentSourceType: channel,
				ProductCode:     invocation.ProductCodeContract,
				EvidenceType:    "test",
				Confidence:      "high",
				DetectorVersion: invocation.DetectorVersion,
			}
		},
	})

	client, requestContext, err := app.openPlatformClientAndContext("contract", "user", "/open-apis/contract/v1/mcp/contracts/search", openplatform.IdentityPolicyUserOnly)
	if err != nil {
		t.Fatalf("openPlatformClientAndContext() error = %v", err)
	}
	for range channels {
		if _, err := client.Do(context.Background(), requestContext, openplatform.Request{
			Method: http.MethodGet,
			Path:   "/open-apis/contract/v1/mcp/contracts/search",
		}); err != nil {
			t.Fatalf("Do() error = %v", err)
		}
	}

	if detectorCalls != 2 {
		t.Fatalf("detector calls = %d, want 2", detectorCalls)
	}
	if len(receivedChannels) != 2 || receivedChannels[0] != "doubao" || receivedChannels[1] != "workbuddy" {
		t.Fatalf("received channels = %#v", receivedChannels)
	}
}

type roundTripFuncInternal func(*http.Request) (*http.Response, error)

func (function roundTripFuncInternal) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestEnvironmentTimeoutDoesNotBlockBusinessOrPinNextRequestSource(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "buffered", true: "streaming"}[stream], func(t *testing.T) {
			var sends atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				index := sends.Add(1)
				wantAgent, wantRule := "unknown", ""
				if index == 2 {
					wantAgent, wantRule = "workbuddy", "client.workbuddy.signed-bundle"
				}
				for name, want := range map[string]string{
					invocation.HeaderChannelType: "cli", invocation.HeaderProductCode: "contract",
					invocation.HeaderAgentSourceType: wantAgent, invocation.HeaderRuleID: wantRule,
					"Authorization": "Bearer test-token", "X-Business-Header": "unchanged",
				} {
					if got := request.Header.Get(name); got != want {
						t.Errorf("request %d: %s = %q, want %q", index, name, got, want)
					}
				}
				trace := request.Header.Get(tracecontext.HeaderLogID)
				if len(trace) != 32 || !strings.HasPrefix(request.Header.Get(tracecontext.HeaderTraceparent), "00-"+trace+"-") {
					t.Error("business trace missing or changed")
				}
				body, err := io.ReadAll(request.Body)
				if err != nil || string(body) != "unchanged-body" {
					t.Errorf("business body changed: %q, %v", body, err)
				}
				_, _ = io.WriteString(writer, "original-response")
			}))
			defer server.Close()
			detections := 0
			app := &App{
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
				inspectEnvironment: func(parent context.Context, depth int) invocation.Result {
					detections++
					if detections == 1 {
						// The real helper's five-second kill/reap behavior is tested in
						// invocation; here compose an expired detection with real HTTP.
						ctx, cancel := context.WithTimeout(parent, 10*time.Millisecond)
						defer cancel()
						<-ctx.Done()
						return invocation.Analyze(nil, nil)
					}
					return invocation.Result{ChannelType: "cli", AgentSourceType: "workbuddy", ProductCode: "contract",
						EvidenceType: "macos_code_signature", Confidence: "high", DetectorVersion: invocation.DetectorVersion, RuleID: "client.workbuddy.signed-bundle"}
				},
			}
			client := openplatform.New(openplatform.Options{HTTPClient: server.Client(), Logger: app.logger,
				BeforeRequestHooks: []openplatform.BeforeRequestHook{app.beforeOpenPlatformRequest}})
			requestContext := openplatform.RequestContext{BaseURL: server.URL, Identity: config.IdentityUser, AccessToken: "test-token"}
			headers := make(http.Header)
			headers.Set(invocation.HeaderAgentSourceType, "codex")
			headers.Set(invocation.HeaderRuleID, "stale.rule")
			headers.Set("X-Business-Header", "unchanged")
			request := openplatform.Request{Method: http.MethodPost, Path: "/open-apis/contract/v1/mcp/contracts/search", Body: []byte("unchanged-body"), Headers: headers, OperationKind: openplatform.OperationRead}
			for attempt := 0; attempt < 2; attempt++ {
				var responseBody []byte
				if stream {
					var output bytes.Buffer
					if _, err := client.DoStream(context.Background(), requestContext, request, &output); err != nil {
						t.Fatal(err)
					}
					responseBody = output.Bytes()
				} else {
					response, err := client.Do(context.Background(), requestContext, request)
					if err != nil {
						t.Fatal(err)
					}
					responseBody = response.Body
				}
				if string(responseBody) != "original-response" {
					t.Fatalf("response changed: %s", responseBody)
				}
			}
			if detections != 2 || sends.Load() != 2 {
				t.Fatalf("detector calls=%d, sends=%d", detections, sends.Load())
			}
			if headers.Get(invocation.HeaderRuleID) != "stale.rule" {
				t.Fatal("mutated caller's reusable headers")
			}
		})
	}
}

func TestEnvironmentHookRespectsBusinessCancellationWithoutSending(t *testing.T) {
	for _, cancelBefore := range []bool{true, false} {
		ctx, cancel := context.WithCancel(context.Background())
		if cancelBefore {
			cancel()
		}
		detections, sends := 0, 0
		app := &App{logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			inspectEnvironment: func(received context.Context, _ int) invocation.Result {
				detections++
				if received != ctx {
					t.Fatal("lost caller context")
				}
				cancel()
				return invocation.Analyze(nil, nil)
			}}
		client := openplatform.New(openplatform.Options{Logger: app.logger,
			BeforeRequestHooks: []openplatform.BeforeRequestHook{app.beforeOpenPlatformRequest},
			HTTPClient:         &http.Client{Transport: roundTripFuncInternal(func(*http.Request) (*http.Response, error) { sends++; return nil, errors.New("must not send") })}})
		_, err := client.Do(ctx, openplatform.RequestContext{BaseURL: "https://test.invalid", Identity: config.IdentityUser, AccessToken: "test"},
			openplatform.Request{Method: http.MethodGet, Path: "/open-apis/contract/v1/mcp/contracts/search", OperationKind: openplatform.OperationRead})
		cancel()
		if !errors.Is(err, context.Canceled) || sends != 0 {
			t.Fatalf("cancellation: err=%v, sends=%d", err, sends)
		}
		if (cancelBefore && detections != 0) || (!cancelBefore && detections != 1) {
			t.Fatalf("detections=%d", detections)
		}
	}
}
