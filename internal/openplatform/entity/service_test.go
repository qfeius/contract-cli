package entity_test

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
	"cn.qfei/contract-cli/internal/openplatform/entity"
)

func TestServiceListAndGetUseLegalEntityEndpoints(t *testing.T) {
	t.Parallel()

	requests := []string{}
	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests = append(requests, req.URL.String())
				return jsonResponse(`{"code":0}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	requestContext, err := client.RequestContext(profileWithUserToken(), config.IdentityUser)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}

	service := entity.NewService(client)
	if _, err := service.List(context.Background(), requestContext, entity.ListInput{
		Name:      "上海主体",
		PageSize:  10,
		PageToken: "next",
	}); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if _, err := service.Get(context.Background(), requestContext, "legal-1"); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	wantContains := []string{
		"https://dev-open.qtech.cn/open-apis/contract/v1/mcp/legal_entities?legalEntity=%E4%B8%8A%E6%B5%B7%E4%B8%BB%E4%BD%93&page_size=10&page_token=next&user_id_type=user_id",
		"https://dev-open.qtech.cn/open-apis/contract/v1/mcp/legal_entities/legal-1?user_id_type=user_id",
	}
	for _, want := range wantContains {
		if !containsString(requests, want) {
			t.Fatalf("missing request %q in %v", want, requests)
		}
	}
}

func TestServiceGetRejectsEmptyEntityID(t *testing.T) {
	t.Parallel()

	service := entity.NewService(openplatform.New(openplatform.Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}))
	_, err := service.Get(context.Background(), openplatform.RequestContext{}, "")
	if err == nil || !strings.Contains(err.Error(), "legal entity id is required") {
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

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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
