package schema

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"cn.qfei/contract-cli/internal/config"
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

	switch requestContext.Identity {
	case config.IdentityUser:
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
	case config.IdentityBot:
		botBizLine, err := normalizeBotBizLine(bizLine)
		if err != nil {
			return openplatform.Response{}, err
		}
		return s.client.Do(ctx, requestContext, openplatform.Request{
			Method:         http.MethodGet,
			Path:           "/open-apis/mdm/v1/config/config_list",
			Query:          url.Values{"biz_line": {botBizLine}},
			IdentityPolicy: openplatform.IdentityPolicyAny,
		})
	default:
		return openplatform.Response{}, fmt.Errorf("unsupported identity %q for mdm fields list", requestContext.Identity)
	}
}

func normalizeBotBizLine(bizLine string) (string, error) {
	switch bizLine {
	case "vendor":
		return "vendor", nil
	case "legalEntity", "legal_entity":
		return "legalEntity", nil
	default:
		return "", fmt.Errorf("biz line %q is not supported for bot identity; supported values: vendor, legalEntity", bizLine)
	}
}
