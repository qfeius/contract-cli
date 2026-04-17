package mdmvendor

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
)

type Service struct {
	client *openplatform.Client
}

type ListInput struct {
	Name      string
	PageSize  int
	PageToken string
}

func NewService(client *openplatform.Client) *Service {
	return &Service{client: client}
}

func (s *Service) List(ctx context.Context, requestContext openplatform.RequestContext, input ListInput) (openplatform.Response, error) {
	query := url.Values{}
	if strings.TrimSpace(input.Name) != "" {
		query.Set("vendor", strings.TrimSpace(input.Name))
	}
	if input.PageSize > 0 {
		query.Set("page_size", strconv.Itoa(input.PageSize))
	}
	if strings.TrimSpace(input.PageToken) != "" {
		query.Set("page_token", strings.TrimSpace(input.PageToken))
	}

	switch requestContext.Identity {
	case config.IdentityUser:
		spec, ok := openplatformContractSpec("get-vendors")
		if !ok {
			return openplatform.Response{}, fmt.Errorf("vendor list spec is not configured")
		}

		return s.client.Do(ctx, requestContext, openplatform.Request{
			Method:         spec.Method,
			Path:           spec.Path,
			Query:          spec.Query(query),
			IdentityPolicy: spec.IdentityPolicy,
		})
	case config.IdentityBot:
		return s.client.Do(ctx, requestContext, openplatform.Request{
			Method:         http.MethodGet,
			Path:           "/open-apis/mdm/v1/vendors",
			Query:          query,
			IdentityPolicy: openplatform.IdentityPolicyAny,
		})
	default:
		return openplatform.Response{}, fmt.Errorf("unsupported identity %q for mdm vendor list", requestContext.Identity)
	}
}

func (s *Service) Get(ctx context.Context, requestContext openplatform.RequestContext, vendorID string) (openplatform.Response, error) {
	vendorID = strings.TrimSpace(vendorID)
	if vendorID == "" {
		return openplatform.Response{}, fmt.Errorf("vendor id is required")
	}

	switch requestContext.Identity {
	case config.IdentityUser:
		spec, ok := openplatformContractSpec("get-vendor-detail")
		if !ok {
			return openplatform.Response{}, fmt.Errorf("vendor detail spec is not configured")
		}

		return s.client.Do(ctx, requestContext, openplatform.Request{
			Method:         spec.Method,
			Path:           strings.ReplaceAll(spec.Path, "{vendor_id}", url.PathEscape(vendorID)),
			Query:          spec.Query(nil),
			IdentityPolicy: spec.IdentityPolicy,
		})
	case config.IdentityBot:
		return s.client.Do(ctx, requestContext, openplatform.Request{
			Method:         http.MethodGet,
			Path:           "/open-apis/mdm/v1/vendors/" + url.PathEscape(vendorID),
			IdentityPolicy: openplatform.IdentityPolicyAny,
		})
	default:
		return openplatform.Response{}, fmt.Errorf("unsupported identity %q for mdm vendor get", requestContext.Identity)
	}
}

func openplatformContractSpec(toolName string) (openplatform.ToolSpec, bool) {
	return openplatform.ContractMCPToolSpec(toolName)
}
