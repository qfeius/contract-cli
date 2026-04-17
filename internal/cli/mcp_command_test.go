package cli_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestMCPCommandsUseUserIdentityAndExpectedEndpoints(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := config.NewStore(dir)
	searchFile := filepath.Join(dir, "contract-search.json")
	if err := os.WriteFile(searchFile, []byte(`{"combine_condition":{"contract_name":"采购合同"}}`), 0o600); err != nil {
		t.Fatalf("WriteFile(search) error = %v", err)
	}
	templateFile := filepath.Join(dir, "template-instance.json")
	if err := os.WriteFile(templateFile, []byte(`{"template_number":"TMP001"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(template) error = %v", err)
	}

	profile := config.Profile{
		Name:                "contract-group",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityBot,
		Identities: config.Identities{
			User: config.UserIdentity{
				Token: &config.Token{
					AccessToken: "user-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
			Bot: config.BotIdentity{
				Token: &config.Token{
					AccessToken: "bot-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	testCases := []struct {
		name         string
		args         []string
		wantMethod   string
		wantPath     string
		wantQuery    map[string]string
		wantAuth     string
		wantBody     string
		responseBody string
		wantContains []string
	}{
		{
			name:         "contract get user",
			args:         []string{"contract", "get", "contract-1", "--profile", "contract-group", "--as", "user"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/mcp/contracts/contract-1",
			wantQuery:    map[string]string{"user_id_type": "user_id"},
			wantAuth:     "Bearer user-token",
			responseBody: `{"code":0,"data":{"contract_id":"contract-1"}}`,
			wantContains: []string{`"contract_id": "contract-1"`},
		},
		{
			name:         "contract get bot by default identity",
			args:         []string{"contract", "get", "contract-1", "--profile", "contract-group"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/contracts/contract-1",
			wantQuery:    map[string]string{},
			wantAuth:     "Bearer bot-token",
			responseBody: `{"code":0,"data":{"contract":{"contract_id":"contract-1"}}}`,
			wantContains: []string{`"contract_id": "contract-1"`},
		},
			{
				name:         "contract search user",
				args:         []string{"contract", "search", "--profile", "contract-group", "--as", "user", "--input-file", searchFile, "--contract-number", "CN-001", "--page-size", "20", "--user-id", "ou_user_1", "--user-id-type", "employee_id"},
				wantMethod:   http.MethodPost,
				wantPath:     "/open-apis/contract/v1/mcp/contracts/search",
				wantQuery:    map[string]string{"user_id_type": "user_id", "user_id": "ou_user_1"},
				wantAuth:     "Bearer user-token",
				wantBody:     `{"combine_condition":{"contract_name":"采购合同"},"contract_number":"CN-001","page_size":20}`,
				responseBody: `{"code":0,"data":{"has_more":false,"items":[{"contract_number":"CN-001"}],"page_token":"10"}}`,
				wantContains: []string{`"contract_number": "CN-001"`, `"has_more": false`, `"page_token": "10"`},
			},
		{
			name:         "contract search bot by default identity",
			args:         []string{"contract", "search", "--profile", "contract-group", "--input-file", searchFile, "--contract-number", "CN-001", "--page-size", "20"},
			wantMethod:   http.MethodPost,
			wantPath:     "/open-apis/contract/v1/contracts/search",
			wantQuery:    map[string]string{},
			wantAuth:     "Bearer bot-token",
			wantBody:     `{"combine_condition":{"contract_name":"采购合同"},"contract_number":"CN-001","page_size":20}`,
			responseBody: `{"code":0,"data":{"has_more":false,"items":[{"contract_number":"CN-001"}],"page_token":"10"}}`,
			wantContains: []string{`"contract_number": "CN-001"`, `"has_more": false`, `"page_token": "10"`},
		},
		{
			name:         "contract sync-user-groups",
			args:         []string{"contract", "sync-user-groups", "--profile", "contract-group", "--as", "user"},
			wantMethod:   http.MethodPost,
			wantPath:     "/open-apis/contract/v1/mcp/contracts/user-groups/sync",
			wantQuery:    map[string]string{"user_id_type": "user_id"},
			wantAuth:     "Bearer user-token",
			responseBody: `{"code":0,"data":{"synced":true}}`,
			wantContains: []string{`"synced": true`},
		},
		{
			name:         "contract sync-user-groups bot by default identity",
			args:         []string{"contract", "sync-user-groups", "--profile", "contract-group", "--user-id", "ou_bot_owner"},
			wantMethod:   http.MethodPost,
			wantPath:     "/open-apis/contract/v1/contracts/user-groups/sync",
			wantQuery:    map[string]string{"user_id": "ou_bot_owner"},
			wantAuth:     "Bearer bot-token",
			responseBody: `{"code":0,"msg":"success","data":true}`,
			wantContains: []string{`"data": true`},
		},
		{
			name:         "contract text",
			args:         []string{"contract", "text", "contract-1", "--profile", "contract-group", "--as", "user", "--full-text"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/mcp/contracts/contract-1/text",
			wantQuery:    map[string]string{"user_id_type": "user_id", "full_text": "true"},
			wantAuth:     "Bearer user-token",
			responseBody: `{"code":0,"data":{"text":"demo"}}`,
			wantContains: []string{`"text": "demo"`},
		},
		{
			name:         "contract text bot by default identity",
			args:         []string{"contract", "text", "contract-1", "--profile", "contract-group", "--full-text", "--user-id-type", "employee_id"},
			wantMethod:   http.MethodPost,
			wantPath:     "/open-apis/contract/v1/contracts/contract-1/text",
			wantQuery:    map[string]string{"full_text": "true", "user_id_type": "employee_id"},
			wantAuth:     "Bearer bot-token",
			responseBody: `{"code":0,"data":"demo"}`,
			wantContains: []string{`"data": "demo"`},
		},
		{
			name:         "contract create user",
			args:         []string{"contract", "create", "--profile", "contract-group", "--as", "user", "--data", `{"title":"demo"}`},
			wantMethod:   http.MethodPost,
			wantPath:     "/open-apis/contract/v1/mcp/contracts",
			wantQuery:    map[string]string{"user_id_type": "user_id"},
			wantAuth:     "Bearer user-token",
			wantBody:     `{"title":"demo"}`,
			responseBody: `{"code":0,"data":{"contract_id":"created"}}`,
			wantContains: []string{`"contract_id": "created"`},
		},
		{
			name:         "contract create bot by default identity",
			args:         []string{"contract", "create", "--profile", "contract-group", "--data", `{"title":"demo","create_user_id":"ou_creator"}`},
			wantMethod:   http.MethodPost,
			wantPath:     "/open-apis/contract/v1/contracts",
			wantQuery:    map[string]string{},
			wantAuth:     "Bearer bot-token",
			wantBody:     `{"create_user_id":"ou_creator","title":"demo"}`,
			responseBody: `{"code":0,"data":{"contract":{"contract_id":"created"}}}`,
			wantContains: []string{`"contract_id": "created"`},
		},
		{
			name:         "contract category list",
			args:         []string{"contract", "category", "list", "--profile", "contract-group", "--as", "user", "--lang", "zh-CN"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/mcp/contract_categorys",
			wantQuery:    map[string]string{"lang": "zh-CN"},
			wantAuth:     "Bearer user-token",
			responseBody: `{"code":0,"data":[{"category":"采购"}]}`,
			wantContains: []string{`"category": "采购"`},
		},
		{
			name:         "contract category list bot by default identity",
			args:         []string{"contract", "category", "list", "--profile", "contract-group", "--lang", "zh-CN"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/contract_categorys",
			wantQuery:    map[string]string{"lang": "zh-CN"},
			wantAuth:     "Bearer bot-token",
			responseBody: `{"code":0,"data":{"contract_category_resource_vo":{"category_resources":[{"name":"采购"}]}}}`,
			wantContains: []string{`"name": "采购"`},
		},
		{
			name:         "contract template list",
			args:         []string{"contract", "template", "list", "--profile", "contract-group", "--as", "user", "--category-number", "CAT-1", "--page-size", "20"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/mcp/templates",
			wantQuery:    map[string]string{"category_number": "CAT-1", "page_size": "20"},
			wantAuth:     "Bearer user-token",
			responseBody: `{"code":0,"data":[{"template_id":"tpl-1"}]}`,
			wantContains: []string{`"template_id": "tpl-1"`},
		},
		{
			name:         "contract template list bot by default identity",
			args:         []string{"contract", "template", "list", "--profile", "contract-group", "--category-number", "CAT-1", "--page-size", "20", "--page-token", "next", "--user-id", "ou_bot_owner", "--user-id-type", "employee_id"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/templates",
			wantQuery:    map[string]string{"category_number": "CAT-1", "page_size": "20", "page_token": "next", "user_id": "ou_bot_owner", "user_id_type": "employee_id"},
			wantAuth:     "Bearer bot-token",
			responseBody: `{"code":0,"data":{"template_brief_infos":[{"template_id":"tpl-1"}],"page_token":"next-2","has_more":true}}`,
			wantContains: []string{`"template_id": "tpl-1"`, `"has_more": true`},
		},
		{
			name:         "contract template get",
			args:         []string{"contract", "template", "get", "tpl-1", "--profile", "contract-group", "--as", "user"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/mcp/templates/tpl-1",
			wantAuth:     "Bearer user-token",
			responseBody: `{"code":0,"data":{"template_id":"tpl-1"}}`,
			wantContains: []string{`"template_id": "tpl-1"`},
		},
		{
			name:         "contract template get bot by default identity",
			args:         []string{"contract", "template", "get", "tpl-1", "--profile", "contract-group", "--user-id", "ou_bot_owner", "--user-id-type", "employee_id"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/templates/tpl-1",
			wantQuery:    map[string]string{"user_id": "ou_bot_owner", "user_id_type": "employee_id"},
			wantAuth:     "Bearer bot-token",
			responseBody: `{"code":0,"data":{"template":{"template_id":"tpl-1"}}}`,
			wantContains: []string{`"template_id": "tpl-1"`},
		},
		{
			name:         "contract template instantiate",
			args:         []string{"contract", "template", "instantiate", "--profile", "contract-group", "--as", "user", "--input-file", templateFile},
			wantMethod:   http.MethodPost,
			wantPath:     "/open-apis/contract/v1/mcp/template_instances",
			wantAuth:     "Bearer user-token",
			wantBody:     `{"template_number":"TMP001"}`,
			responseBody: `{"code":0,"data":{"instance_id":"instance-1"}}`,
			wantContains: []string{`"instance_id": "instance-1"`},
		},
		{
			name:         "contract template instantiate bot by default identity",
			args:         []string{"contract", "template", "instantiate", "--profile", "contract-group", "--data", `{"template_number":"TMP001","create_user_id":"ou_creator"}`, "--user-id-type", "employee_id"},
			wantMethod:   http.MethodPost,
			wantPath:     "/open-apis/contract/v1/template_instances",
			wantQuery:    map[string]string{"user_id_type": "employee_id"},
			wantAuth:     "Bearer bot-token",
			wantBody:     `{"create_user_id":"ou_creator","template_number":"TMP001"}`,
			responseBody: `{"code":0,"data":{"template_instance":{"template_instance_id":"instance-1"}}}`,
			wantContains: []string{`"template_instance_id": "instance-1"`},
		},
		{
			name:         "contract enum list",
			args:         []string{"contract", "enum", "list", "--profile", "contract-group", "--type", "contract_status"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/mcp/enum_values",
			wantQuery:    map[string]string{"enum_type": "contract_status"},
			wantAuth:     "Bearer user-token",
			responseBody: `{"code":0,"data":[{"label":"草稿","value":"0"}]}`,
			wantContains: []string{`"label": "草稿"`},
		},
		{
			name:         "mdm vendor list",
			args:         []string{"mdm", "vendor", "list", "--profile", "contract-group", "--as", "user", "--name", "供应商", "--page-size", "10"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/mcp/vendors",
			wantQuery:    map[string]string{"vendor": "供应商", "page_size": "10", "user_id_type": "user_id"},
			wantAuth:     "Bearer user-token",
			responseBody: `{"code":0,"data":[{"vendor_id":"vendor-1"}]}`,
			wantContains: []string{`"vendor_id": "vendor-1"`},
		},
		{
			name:         "mdm vendor list bot by default identity",
			args:         []string{"mdm", "vendor", "list", "--profile", "contract-group", "--name", "V00000001", "--page-size", "10", "--page-token", "next", "--user-id-type", "employee_id"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/mdm/v1/vendors",
			wantQuery:    map[string]string{"vendor": "V00000001", "page_size": "10", "page_token": "next", "user_id_type": "employee_id"},
			wantAuth:     "Bearer bot-token",
			responseBody: `{"code":0,"data":{"items":[{"vendor":"V00000001"}],"hasMore":false}}`,
			wantContains: []string{`"vendor": "V00000001"`},
		},
		{
			name:         "mdm vendor get",
			args:         []string{"mdm", "vendor", "get", "vendor-1", "--profile", "contract-group", "--as", "user"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/mcp/vendors/vendor-1",
			wantQuery:    map[string]string{"user_id_type": "user_id"},
			wantAuth:     "Bearer user-token",
			responseBody: `{"code":0,"data":{"vendor_id":"vendor-1"}}`,
			wantContains: []string{`"vendor_id": "vendor-1"`},
		},
		{
			name:         "mdm vendor get bot by default identity",
			args:         []string{"mdm", "vendor", "get", "7003410079584092448", "--profile", "contract-group", "--user-id-type", "employee_id"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/mdm/v1/vendors/7003410079584092448",
			wantQuery:    map[string]string{"user_id_type": "employee_id"},
			wantAuth:     "Bearer bot-token",
			responseBody: `{"code":0,"data":{"vendor":{"vendor":"V00108006"}}}`,
			wantContains: []string{`"vendor": "V00108006"`},
		},
		{
			name:         "mdm legal list",
			args:         []string{"mdm", "legal", "list", "--profile", "contract-group", "--as", "user", "--name", "主体A", "--page-size", "10"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/mcp/legal_entities",
			wantQuery:    map[string]string{"legalEntity": "主体A", "page_size": "10", "user_id_type": "user_id"},
			wantAuth:     "Bearer user-token",
			responseBody: `{"code":0,"data":[{"legal_entity_id":"entity-1"}]}`,
			wantContains: []string{`"legal_entity_id": "entity-1"`},
		},
		{
			name:         "mdm legal list bot by default identity",
			args:         []string{"mdm", "legal", "list", "--profile", "contract-group", "--name", "主体A", "--page-size", "10", "--page-token", "next", "--user-id-type", "employee_id"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/mdm/v1/legal_entities/list_all",
			wantQuery:    map[string]string{"legalEntity": "主体A", "page_size": "10", "page_token": "next", "user_id_type": "employee_id"},
			wantAuth:     "Bearer bot-token",
			responseBody: `{"code":0,"data":{"items":[{"legalEntity":"L00002002"}],"hasMore":false}}`,
			wantContains: []string{`"legalEntity": "L00002002"`},
		},
		{
			name:         "mdm legal get",
			args:         []string{"mdm", "legal", "get", "entity-1", "--profile", "contract-group", "--as", "user"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/mcp/legal_entities/entity-1",
			wantQuery:    map[string]string{"user_id_type": "user_id"},
			wantAuth:     "Bearer user-token",
			responseBody: `{"code":0,"data":{"legal_entity_id":"entity-1"}}`,
			wantContains: []string{`"legal_entity_id": "entity-1"`},
		},
		{
			name:         "mdm legal get bot by default identity",
			args:         []string{"mdm", "legal", "get", "7003410079584092448", "--profile", "contract-group", "--user-id-type", "employee_id"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/mdm/v1/legal_entities/7003410079584092448",
			wantQuery:    map[string]string{"legal_entity_id": "7003410079584092448", "user_id_type": "employee_id"},
			wantAuth:     "Bearer bot-token",
			responseBody: `{"code":0,"data":{"legalEntity":{"legalEntity":"L00002002"}}}`,
			wantContains: []string{`"legalEntity": "L00002002"`},
		},
		{
			name:         "mdm fields list",
			args:         []string{"mdm", "fields", "list", "--profile", "contract-group", "--as", "user", "--biz-line", "vendor"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/contract/v1/mcp/config/config_list",
			wantQuery:    map[string]string{"biz_line": "vendor", "user_id_type": "user_id"},
			wantAuth:     "Bearer user-token",
			responseBody: `{"code":0,"data":[{"field_key":"name"}]}`,
			wantContains: []string{`"field_key": "name"`},
		},
		{
			name:         "mdm fields list bot by default identity",
			args:         []string{"mdm", "fields", "list", "--profile", "contract-group", "--biz-line", "vendor", "--user-id-type", "employee_id"},
			wantMethod:   http.MethodGet,
			wantPath:     "/open-apis/mdm/v1/config/config_list",
			wantQuery:    map[string]string{"biz_line": "vendor", "user_id_type": "employee_id"},
			wantAuth:     "Bearer bot-token",
			responseBody: `{"code":0,"data":{"config":[{"fieldCode":"V00000001"}]}}`,
			wantContains: []string{`"fieldCode": "V00000001"`},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := cli.New(cli.Options{
				Stdout: stdout,
				Stderr: stderr,
				Store:  store,
				HTTPClient: &http.Client{
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						if req.Method != tc.wantMethod {
							t.Fatalf("method = %s, want %s", req.Method, tc.wantMethod)
						}
						if req.URL.Path != tc.wantPath {
							t.Fatalf("path = %s, want %s", req.URL.Path, tc.wantPath)
						}
						if req.Header.Get("Authorization") != tc.wantAuth {
							t.Fatalf("authorization = %q, want %q", req.Header.Get("Authorization"), tc.wantAuth)
						}
						for key, want := range tc.wantQuery {
							if got := req.URL.Query().Get(key); got != want {
								t.Fatalf("query %s = %q, want %q", key, got, want)
							}
						}
						if tc.wantBody != "" {
							body, err := io.ReadAll(req.Body)
							if err != nil {
								t.Fatalf("ReadAll() error = %v", err)
							}
							if string(body) != tc.wantBody {
								t.Fatalf("body = %s, want %s", string(body), tc.wantBody)
							}
						}
						return jsonResponse(tc.responseBody), nil
					}),
				},
			})

			if err := app.Run(context.Background(), tc.args); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			for _, want := range tc.wantContains {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("missing %q in output: %s", want, stdout.String())
				}
			}
		})
	}
}

func TestStructuredUserOnlyMCPCommandRejectsBotIdentity(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(config.Profile{
		Name:                "contract-group",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityBot,
		Identities: config.Identities{
			User: config.UserIdentity{
				Token: &config.Token{
					AccessToken: "user-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
			Bot: config.BotIdentity{
				Token: &config.Token{
					AccessToken: "bot-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}, true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: stderr,
		Store:  store,
	})

	err := app.Run(context.Background(), []string{
		"contract", "enum", "list", "--profile", "contract-group", "--as", "bot", "--type", "contract_status",
	})
	if err == nil || !strings.Contains(err.Error(), "only supports --as user") {
		t.Fatalf("unexpected bot error: %v", err)
	}
}

func TestLegacyMCPCommandNamesAreRejected(t *testing.T) {
	t.Parallel()

	store := config.NewStore(t.TempDir())
	app := cli.New(cli.Options{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Store:  store,
	})

	testCases := [][]string{
		{"vendor", "list"},
		{"entity", "list"},
		{"schema", "fields", "--biz-line", "vendor"},
		{"mdm-vendor", "list"},
		{"mdm-legal", "list"},
		{"mdm-fields", "--biz-line", "vendor"},
	}

	for _, args := range testCases {
		args := args
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()
			err := app.Run(context.Background(), args)
			if err == nil || !strings.Contains(err.Error(), "unknown command") {
				t.Fatalf("unexpected legacy command error: %v", err)
			}
		})
	}
}
