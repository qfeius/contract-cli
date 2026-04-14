package vendor_test

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
	"cn.qfei/contract-cli/internal/openplatform/vendor"
)

func TestServiceGetUsesVendorEndpoint(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://dev-open.qtech.cn/open-apis/mdm/v1/vendors/1063197165850985296" {
					t.Fatalf("url = %q", req.URL.String())
				}
				return jsonResponse(`{"code":0,"data":{"vendorId":"1063197165850985296"}}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	requestContext, err := client.RequestContext(config.Profile{
		Name:                "contract-group",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		Identities: config.Identities{
			Bot: config.BotIdentity{
				Token: &config.Token{
					AccessToken: "bot-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
		DefaultIdentity: config.IdentityBot,
	}, "")
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}

	service := vendor.NewService(client)
	response, err := service.Get(context.Background(), requestContext, "1063197165850985296")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestServiceGetRejectsEmptyVendorID(t *testing.T) {
	t.Parallel()

	service := vendor.NewService(openplatform.New(openplatform.Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}))
	_, err := service.Get(context.Background(), openplatform.RequestContext{}, "")
	if err == nil || !strings.Contains(err.Error(), "vendor id is required") {
		t.Fatalf("unexpected error: %v", err)
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
