package contract_test

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
	"cn.qfei/contract-cli/internal/openplatform/contract"
)

func TestServiceSearchUsesSearchEndpoint(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost {
					t.Fatalf("method = %s", req.Method)
				}
				if req.URL.String() != "https://dev-open.qtech.cn/open-apis/contract/v1/mcp/contracts/search?user_id_type=user_id" {
					t.Fatalf("url = %q", req.URL.String())
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("ReadAll() error = %v", err)
				}
				if string(body) != `{"contract_number":"CN-001"}` {
					t.Fatalf("body = %s", string(body))
				}
				return jsonResponse(`{"code":0,"data":[{"contract_number":"CN-001"}]}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	requestContext, err := client.RequestContext(profileWithUserToken(), config.IdentityUser)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}

	service := contract.NewService(client)
	response, err := service.Search(context.Background(), requestContext, []byte(`{"contract_number":"CN-001"}`))
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestServiceGetTextUsesContractTextEndpoint(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://dev-open.qtech.cn/open-apis/contract/v1/mcp/contracts/contract-1/text?full_text=true&user_id_type=user_id" {
					t.Fatalf("url = %q", req.URL.String())
				}
				return jsonResponse(`{"code":0,"data":{"text":"demo"}}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	requestContext, err := client.RequestContext(profileWithUserToken(), config.IdentityUser)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}

	service := contract.NewService(client)
	response, err := service.GetText(context.Background(), requestContext, "contract-1", contract.TextInput{
		FullText: true,
	})
	if err != nil {
		t.Fatalf("GetText() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestServiceCreateTemplateInstantiateAndLookupsUseExpectedEndpoints(t *testing.T) {
	t.Parallel()

	requests := []string{}
	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests = append(requests, req.Method+" "+req.URL.String())
				return jsonResponse(`{"code":0}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	requestContext, err := client.RequestContext(profileWithUserToken(), config.IdentityUser)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}

	service := contract.NewService(client)
	if _, err := service.Create(context.Background(), requestContext, []byte(`{"title":"demo"}`)); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := service.SyncUserGroups(context.Background(), requestContext); err != nil {
		t.Fatalf("SyncUserGroups() error = %v", err)
	}
	if _, err := service.ListCategories(context.Background(), requestContext, "zh-CN"); err != nil {
		t.Fatalf("ListCategories() error = %v", err)
	}
	if _, err := service.ListTemplates(context.Background(), requestContext, contract.ListTemplatesInput{
		CategoryNumber: "CAT-1",
		PageSize:       20,
		PageToken:      "next",
	}); err != nil {
		t.Fatalf("ListTemplates() error = %v", err)
	}
	if _, err := service.GetTemplate(context.Background(), requestContext, "tpl-1"); err != nil {
		t.Fatalf("GetTemplate() error = %v", err)
	}
	if _, err := service.InstantiateTemplate(context.Background(), requestContext, []byte(`{"template_number":"TMP-1"}`)); err != nil {
		t.Fatalf("InstantiateTemplate() error = %v", err)
	}
	if _, err := service.ListEnums(context.Background(), requestContext, "contract_status"); err != nil {
		t.Fatalf("ListEnums() error = %v", err)
	}
	if _, err := service.Get(context.Background(), requestContext, "contract-1"); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	wantContains := []string{
		"POST https://dev-open.qtech.cn/open-apis/contract/v1/mcp/contracts?user_id_type=user_id",
		"POST https://dev-open.qtech.cn/open-apis/contract/v1/mcp/contracts/user-groups/sync?user_id_type=user_id",
		"GET https://dev-open.qtech.cn/open-apis/contract/v1/mcp/contract_categorys?lang=zh-CN",
		"GET https://dev-open.qtech.cn/open-apis/contract/v1/mcp/templates?category_number=CAT-1&page_size=20&page_token=next",
		"GET https://dev-open.qtech.cn/open-apis/contract/v1/mcp/templates/tpl-1",
		"POST https://dev-open.qtech.cn/open-apis/contract/v1/mcp/template_instances",
		"GET https://dev-open.qtech.cn/open-apis/contract/v1/mcp/enum_values?enum_type=contract_status",
		"GET https://dev-open.qtech.cn/open-apis/contract/v1/mcp/contracts/contract-1?user_id_type=user_id",
	}
	for _, want := range wantContains {
		if !containsString(requests, want) {
			t.Fatalf("missing request %q in %v", want, requests)
		}
	}
}

func TestServiceRequiresContractAndTemplateIdentifiers(t *testing.T) {
	t.Parallel()

	service := contract.NewService(openplatform.New(openplatform.Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}))

	if _, err := service.Get(context.Background(), openplatform.RequestContext{}, ""); err == nil || !strings.Contains(err.Error(), "contract id is required") {
		t.Fatalf("unexpected contract get error: %v", err)
	}
	if _, err := service.GetTemplate(context.Background(), openplatform.RequestContext{}, ""); err == nil || !strings.Contains(err.Error(), "template id is required") {
		t.Fatalf("unexpected template get error: %v", err)
	}
	if _, err := service.ListEnums(context.Background(), openplatform.RequestContext{}, ""); err == nil || !strings.Contains(err.Error(), "enum type is required") {
		t.Fatalf("unexpected enum error: %v", err)
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
