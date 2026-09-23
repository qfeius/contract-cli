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
	IdentityPolicyAppOnly  IdentityPolicy = "app_only"
)

type ToolSpec struct {
	ToolName       string
	Method         string
	Path           string
	FixedQuery     url.Values
	IdentityPolicy IdentityPolicy
	OperationKind  OperationKind
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
		ToolName: "get-employees", Method: http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/employees",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly, OperationKind: OperationRead,
	},
	{
		ToolName: "get-departments", Method: http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/departments",
		IdentityPolicy: IdentityPolicyUserOnly, OperationKind: OperationRead,
	},
	{
		ToolName:       "list-contract-search-filter-fields",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/contracts/search/filter_fields",
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationRead,
	},
	{
		ToolName:       "search-contracts",
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/mcp/contracts/search",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationRead,
	},
	{
		ToolName:       "get-contract-detail",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/contracts/{contractId}",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationRead,
	},
	{
		ToolName:       "sync-user-groups",
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/mcp/contracts/user-groups/sync",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationWrite,
	},
	{
		ToolName:       "get-contract-text",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/contracts/{contractId}/text",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationRead,
	},
	{
		ToolName:       "contract_category.list",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/contract_categorys",
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationRead,
	},
	{
		ToolName:       "get-vendors",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/vendors",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationRead,
	},
	{
		ToolName:       "get-vendor-detail",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/vendors/{vendor_id}",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationRead,
	},
	{
		ToolName:       "create-vendor",
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/mcp/vendors",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationWrite,
	},
	{
		ToolName:       "patch-vendor",
		Method:         http.MethodPatch,
		Path:           "/open-apis/contract/v1/mcp/vendors/{vendor_id}",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationWrite,
	},
	{
		ToolName:       "set-vendor-status",
		Method:         http.MethodPut,
		Path:           "/open-apis/contract/v1/mcp/vendors/{vendor_id}/status",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationWrite,
	},
	{
		ToolName:       "get-legal-entities",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/legal_entities",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationRead,
	},
	{
		ToolName:       "get-legal-entity-detail",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/legal_entities/{legal_entity_id}",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationRead,
	},
	{
		ToolName:       "get-field-config",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/config/config_list",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationRead,
	},
	{
		ToolName:       "create-contracts",
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/mcp/contracts",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationWrite,
	},
	{
		ToolName:       "list-templates",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/templates",
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationRead,
	},
	{
		ToolName:       "get-template-detail",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/templates/{template_id}",
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationRead,
	},
	{
		ToolName:       "create-template-instance",
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/mcp/template_instances",
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationWrite,
	},
	{
		ToolName:       "list-process-comments",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/process_instances/{process_instance_id}/comments",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationRead,
	},
	{
		ToolName:       "download-contract-file",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/contracts/{contract_id}/files/{file_id}/download",
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationRead,
	},
	{
		ToolName:       "get-process-instance",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/process_instances/{process_instance_id}",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationRead,
	},
	{
		ToolName:       "create-process-comment",
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/mcp/process_instances/{process_instance_id}/comments",
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationWrite,
	},
	{
		ToolName:       "list-personal-tasks",
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/mcp/tasks",
		FixedQuery:     url.Values{"user_id_type": {"user_id"}},
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationRead,
	},
	{
		ToolName:       "process-approval-task",
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/mcp/tasks/{task_instance_id}/approval",
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationWrite,
	},
	{
		ToolName:       "get-enum-values",
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/mcp/enum_values",
		IdentityPolicy: IdentityPolicyUserOnly,
		OperationKind:  OperationRead,
	},
}
