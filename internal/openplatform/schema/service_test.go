package schema_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
	"cn.qfei/contract-cli/internal/openplatform/schema"
)

func TestServiceFieldsUsesConfigListEndpoint(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://dev-open.qtech.cn/open-apis/contract/v1/mcp/config/config_list?biz_line=vendor&user_id_type=user_id" {
					t.Fatalf("url = %q", req.URL.String())
				}
				return jsonResponse(`{"code":0,"data":[]}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	requestContext, err := client.RequestContext(profileWithUserToken(), config.IdentityUser)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}

	service := schema.NewService(client)
	response, err := service.Fields(context.Background(), requestContext, "vendor")
	if err != nil {
		t.Fatalf("Fields() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestServiceFieldsUsesBotConfigListEndpoint(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/open-apis/mdm/v1/config/config_list" {
					t.Fatalf("path = %q", req.URL.Path)
				}
				query := req.URL.Query()
				if query.Get("biz_line") != "vendor" {
					t.Fatalf("biz_line = %q", query.Get("biz_line"))
				}
				if query.Get("user_id_type") != "employee_id" {
					t.Fatalf("user_id_type = %q", query.Get("user_id_type"))
				}
				return jsonResponse(`{"code":0,"data":{"config":[{"fieldCode":"V00000001"}]}}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	requestContext, err := client.RequestContext(profileWithBotToken(), config.IdentityBot)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}
	requestContext.CommonQuery = urlValues("user_id_type", "employee_id")

	service := schema.NewService(client)
	response, err := service.Fields(context.Background(), requestContext, "vendor")
	if err != nil {
		t.Fatalf("Fields() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestServiceFieldsRejectsEmptyBizLine(t *testing.T) {
	t.Parallel()

	service := schema.NewService(openplatform.New(openplatform.Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}))
	_, err := service.Fields(context.Background(), openplatform.RequestContext{}, "")
	if err == nil || !strings.Contains(err.Error(), "biz line is required") {
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

func profileWithBotToken() config.Profile {
	return config.Profile{
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
	}
}

func urlValues(key, value string) url.Values {
	return url.Values{key: {value}}
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
