package schema

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"cn.qfei/contract-cli/internal/openplatform"
)

type Service struct {
	client *openplatform.Client
}

func NewService(client *openplatform.Client) *Service {
	return &Service{client: client}
}

func (s *Service) Fields(ctx context.Context, requestContext openplatform.RequestContext, bizLine string) (openplatform.Response, error) {
	bizLine = strings.TrimSpace(bizLine)
	if bizLine == "" {
		return openplatform.Response{}, fmt.Errorf("biz line is required")
	}

	spec, ok := openplatform.ContractMCPToolSpec("get-field-config")
	if !ok {
		return openplatform.Response{}, fmt.Errorf("schema fields spec is not configured")
	}

	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         spec.Method,
		Path:           spec.Path,
		Query:          spec.Query(url.Values{"biz_line": {bizLine}}),
		IdentityPolicy: spec.IdentityPolicy,
	})
}
