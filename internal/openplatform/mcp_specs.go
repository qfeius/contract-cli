package openplatform

import (
	"net/http"
	"net/url"
	"strings"
)

type IdentityPolicy string

const (
	IdentityPolicyAny      IdentityPolicy = "any"
	IdentityPolicyUserOnly IdentityPolicy = "user_only"
)

type ToolSpec struct {
	ToolName       string
	Method         string
	Path           string
	FixedQuery     url.Values
	IdentityPolicy IdentityPolicy
}

func IdentityPolicyForPath(path string) IdentityPolicy {
	if strings.HasPrefix(strings.TrimSpace(path), "/open-apis/contract/v1/mcp/") {
		return IdentityPolicyUserOnly
	}
	return IdentityPolicyAny
}

func ContractMCPToolSpecs() []ToolSpec {
	specs := make([]ToolSpec, 0, len(contractMCPToolSpecs))
	for _, spec := range contractMCPToolSpecs {
		specs = append(specs, spec.clone())
	}
	return specs
}

func ContractMCPToolSpec(toolName string) (ToolSpec, bool) {
	for _, spec := range contractMCPToolSpecs {
		if spec.ToolName == toolName {
			return spec.clone(), true
		}
	}
	return ToolSpec{}, false
}

func (s ToolSpec) Query(extra url.Values) url.Values {
	query := cloneURLValues(s.FixedQuery)
	for key, values := range extra {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	return query
}

func (s ToolSpec) clone() ToolSpec {
	s.FixedQuery = cloneURLValues(s.FixedQuery)
	return s
}

func cloneURLValues(values url.Values) url.Values {
	if values == nil {
		return make(url.Values)
	}
	cloned := make(url.Values, len(values))
	for key, items := range values {
		copied := make([]string, len(items))
		copy(copied, items)
		cloned[key] = copied
	}
	return cloned
}

var contractMCPToolSpecs = []ToolSpec{
	{
		ToolName:       "search-contracts",
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/mcp/contracts/search",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
	},
	{
		ToolName:       "get-contract-detail",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/contracts/{contractId}",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
	},
	{
		ToolName:       "sync-user-groups",
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/mcp/contracts/user-groups/sync",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
	},
	{
		ToolName:       "get-contract-text",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/contracts/{contractId}/text",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
	},
	{
		ToolName:       "contract_category.list",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/contract_categorys",
		IdentityPolicy: IdentityPolicyUserOnly,
	},
	{
		ToolName:       "get-vendors",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/vendors",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
	},
	{
		ToolName:       "get-vendor-detail",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/vendors/{vendor_id}",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
	},
	{
		ToolName:       "get-legal-entities",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/legal_entities",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
	},
	{
		ToolName:       "get-legal-entity-detail",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/legal_entities/{legal_entity_id}",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
	},
	{
		ToolName:       "get-field-config",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/config/config_list",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
	},
	{
		ToolName:       "create-contracts",
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/mcp/contracts",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
	},
	{
		ToolName:       "list-templates",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/templates",
		IdentityPolicy: IdentityPolicyUserOnly,
	},
	{
		ToolName:       "get-template-detail",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/templates/{template_id}",
		IdentityPolicy: IdentityPolicyUserOnly,
	},
	{
		ToolName:       "create-template-instance",
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/mcp/template_instances",
		IdentityPolicy: IdentityPolicyUserOnly,
	},
	{
		ToolName:       "get-enum-values",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/enum_values",
		IdentityPolicy: IdentityPolicyUserOnly,
	},
}
