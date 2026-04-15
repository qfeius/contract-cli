package mdmvendor_test

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
	"cn.qfei/contract-cli/internal/openplatform/mdmvendor"
)

func TestServiceListUsesContractMCPVendorEndpoint(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/open-apis/contract/v1/mcp/vendors" {
					t.Fatalf("path = %q", req.URL.Path)
				}
				query := req.URL.Query()
				if query.Get("vendor") != "acme" {
					t.Fatalf("vendor = %q", query.Get("vendor"))
				}
				if query.Get("page_size") != "20" {
					t.Fatalf("page_size = %q", query.Get("page_size"))
				}
				if query.Get("page_token") != "next" {
					t.Fatalf("page_token = %q", query.Get("page_token"))
				}
				if query.Get("user_id_type") != "user_id" {
					t.Fatalf("user_id_type = %q", query.Get("user_id_type"))
				}
				return jsonResponse(`{"code":0,"data":[{"vendor_id":"1063197165850985296"}]}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	requestContext, err := client.RequestContext(profileWithUserToken(), config.IdentityUser)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}

	service := mdmvendor.NewService(client)
	response, err := service.List(context.Background(), requestContext, mdmvendor.ListInput{
		Name:      "acme",
		PageSize:  20,
		PageToken: "next",
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestServiceGetUsesContractMCPVendorDetailEndpoint(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://dev-open.qtech.cn/open-apis/contract/v1/mcp/vendors/1063197165850985296?user_id_type=user_id" {
					t.Fatalf("url = %q", req.URL.String())
				}
				return jsonResponse(`{"code":0,"data":{"vendor_id":"1063197165850985296"}}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	requestContext, err := client.RequestContext(profileWithUserToken(), config.IdentityUser)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}

	service := mdmvendor.NewService(client)
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

	service := mdmvendor.NewService(openplatform.New(openplatform.Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}))
	_, err := service.Get(context.Background(), openplatform.RequestContext{}, "")
	if err == nil || !strings.Contains(err.Error(), "vendor id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func profileWithUserToken() config.Profile {
	return config.Profile{
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
