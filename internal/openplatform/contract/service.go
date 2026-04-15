package contract

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"cn.qfei/contract-cli/internal/openplatform"
)

type Service struct {
	client *openplatform.Client
}

type TextInput struct {
	FullText bool
	Offset   int
	Limit    int
}

type ListTemplatesInput struct {
	CategoryNumber string
	PageSize       int
	PageToken      string
}

func NewService(client *openplatform.Client) *Service {
	return &Service{client: client}
}

func (s *Service) Search(ctx context.Context, requestContext openplatform.RequestContext, body []byte) (openplatform.Response, error) {
	return s.do(ctx, requestContext, "search-contracts", nil, nil, body)
}

func (s *Service) Get(ctx context.Context, requestContext openplatform.RequestContext, contractID string) (openplatform.Response, error) {
	contractID = strings.TrimSpace(contractID)
	if contractID == "" {
		return openplatform.Response{}, fmt.Errorf("contract id is required")
	}
	return s.do(ctx, requestContext, "get-contract-detail", map[string]string{"{contractId}": url.PathEscape(contractID)}, nil, nil)
}

func (s *Service) SyncUserGroups(ctx context.Context, requestContext openplatform.RequestContext) (openplatform.Response, error) {
	return s.do(ctx, requestContext, "sync-user-groups", nil, nil, nil)
}

func (s *Service) GetText(ctx context.Context, requestContext openplatform.RequestContext, contractID string, input TextInput) (openplatform.Response, error) {
	contractID = strings.TrimSpace(contractID)
	if contractID == "" {
		return openplatform.Response{}, fmt.Errorf("contract id is required")
	}
	query := url.Values{
		"full_text": {strconv.FormatBool(input.FullText)},
	}
	if input.Offset > 0 {
		query.Set("offset", strconv.Itoa(input.Offset))
	}
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	return s.do(ctx, requestContext, "get-contract-text", map[string]string{"{contractId}": url.PathEscape(contractID)}, query, nil)
}

func (s *Service) ListCategories(ctx context.Context, requestContext openplatform.RequestContext, lang string) (openplatform.Response, error) {
	query := url.Values{}
	if strings.TrimSpace(lang) != "" {
		query.Set("lang", strings.TrimSpace(lang))
	}
	return s.do(ctx, requestContext, "contract_category.list", nil, query, nil)
}

func (s *Service) Create(ctx context.Context, requestContext openplatform.RequestContext, body []byte) (openplatform.Response, error) {
	return s.do(ctx, requestContext, "create-contracts", nil, nil, body)
}

func (s *Service) ListTemplates(ctx context.Context, requestContext openplatform.RequestContext, input ListTemplatesInput) (openplatform.Response, error) {
	query := url.Values{}
	if strings.TrimSpace(input.CategoryNumber) != "" {
		query.Set("category_number", strings.TrimSpace(input.CategoryNumber))
	}
	if input.PageSize > 0 {
		query.Set("page_size", strconv.Itoa(input.PageSize))
	}
	if strings.TrimSpace(input.PageToken) != "" {
		query.Set("page_token", strings.TrimSpace(input.PageToken))
	}
	return s.do(ctx, requestContext, "list-templates", nil, query, nil)
}

func (s *Service) GetTemplate(ctx context.Context, requestContext openplatform.RequestContext, templateID string) (openplatform.Response, error) {
	templateID = strings.TrimSpace(templateID)
	if templateID == "" {
		return openplatform.Response{}, fmt.Errorf("template id is required")
	}
	return s.do(ctx, requestContext, "get-template-detail", map[string]string{"{template_id}": url.PathEscape(templateID)}, nil, nil)
}

func (s *Service) InstantiateTemplate(ctx context.Context, requestContext openplatform.RequestContext, body []byte) (openplatform.Response, error) {
	return s.do(ctx, requestContext, "create-template-instance", nil, nil, body)
}

func (s *Service) ListEnums(ctx context.Context, requestContext openplatform.RequestContext, enumType string) (openplatform.Response, error) {
	enumType = strings.TrimSpace(enumType)
	if enumType == "" {
		return openplatform.Response{}, fmt.Errorf("enum type is required")
	}
	query := url.Values{"enum_type": {enumType}}
	return s.do(ctx, requestContext, "get-enum-values", nil, query, nil)
}

func (s *Service) do(ctx context.Context, requestContext openplatform.RequestContext, toolName string, replacements map[string]string, query url.Values, body []byte) (openplatform.Response, error) {
	spec, ok := openplatform.ContractMCPToolSpec(toolName)
	if !ok {
		return openplatform.Response{}, fmt.Errorf("contract tool spec %q is not configured", toolName)
	}

	path := spec.Path
	for old, newValue := range replacements {
		path = strings.ReplaceAll(path, old, newValue)
	}

	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         spec.Method,
		Path:           path,
		Query:          spec.Query(query),
		Body:           body,
		IdentityPolicy: spec.IdentityPolicy,
	})
}
