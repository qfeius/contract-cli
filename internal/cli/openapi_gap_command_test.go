package cli_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestOpenAPIGapCommandsUseExpectedEndpointsAsBot(t *testing.T) {
	t.Parallel()

	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	testCases := []struct {
		name       string
		args       []string
		wantMethod string
		wantPath   string
		wantQuery  map[string]string
		wantBody   string
	}{
		{
			name:       "contract search v2",
			args:       []string{"contract", "search-v2", "--profile", "contract", "--data", `{"contract_number":"CN-001"}`},
			wantMethod: http.MethodPost,
			wantPath:   "/open-apis/contract/v1/contracts/searchV2",
			wantQuery:  map[string]string{"user_id_type": "user_id"},
			wantBody:   `{"contract_number":"CN-001"}`,
		},
		{
			name:       "contract field update",
			args:       []string{"contract", "field", "update", "--profile", "contract", "--data", `{"module_name":"签约信息","attribute_name":"城市"}`},
			wantMethod: http.MethodPut,
			wantPath:   "/open-apis/contract/v1/attribute_definition",
			wantQuery:  map[string]string{"user_id_type": "user_id"},
			wantBody:   `{"module_name":"签约信息","attribute_name":"城市"}`,
		},
		{
			name:       "contract sign switch to paper",
			args:       []string{"contract", "sign", "switch-to-paper", "--profile", "contract", "--business-id", "contract-1", "--business-type-code", "0"},
			wantMethod: http.MethodPost,
			wantPath:   "/open-apis/contract/v1/contracts/signType/switchToPaper",
			wantQuery:  map[string]string{"business_id": "contract-1", "business_type_code": "0", "user_id_type": "user_id"},
		},
		{
			name:       "contract sign url get",
			args:       []string{"contract", "sign-url", "get", "contract-1", "--profile", "contract"},
			wantMethod: http.MethodGet,
			wantPath:   "/open-apis/contract/v1/contracts/contract-1/sign_url",
			wantQuery:  map[string]string{"user_id_type": "user_id"},
		},
		{
			name:       "contract form attribute list",
			args:       []string{"contract", "form", "attribute", "list", "--profile", "contract", "--category-id", "cat-1", "--business-type-code", "0"},
			wantMethod: http.MethodGet,
			wantPath:   "/open-apis/contract/v1/form_definition/attribute",
			wantQuery:  map[string]string{"category_id": "cat-1", "business_type_code": "0", "user_id_type": "user_id"},
		},
		{
			name:       "contract authorization grant",
			args:       []string{"contract", "authorization", "grant", "--profile", "contract", "--data", `{"business_id":"contract-1","authorized_user_id":"u1","start_time":"1","end_time":"2"}`},
			wantMethod: http.MethodPost,
			wantPath:   "/open-apis/contract/v1/authorizations",
			wantQuery:  map[string]string{"user_id_type": "user_id"},
			wantBody:   `{"business_id":"contract-1","authorized_user_id":"u1","start_time":"1","end_time":"2"}`,
		},
		{
			name:       "contract share batch create",
			args:       []string{"contract", "share", "batch-create", "--profile", "contract", "--data", `{"contract_id":"contract-1","user_ids":["u1"]}`},
			wantMethod: http.MethodPost,
			wantPath:   "/open-apis/contract/v1/contracts/contract/batch_share",
			wantQuery:  map[string]string{"user_id_type": "user_id"},
			wantBody:   `{"contract_id":"contract-1","user_ids":["u1"]}`,
		},
		{
			name:       "contract cooperation file get",
			args:       []string{"contract", "cooperation", "file", "get", "contract-1", "--profile", "contract"},
			wantMethod: http.MethodGet,
			wantPath:   "/open-apis/contract/v1/contracts/contract-1/cooperation/file_info",
			wantQuery:  map[string]string{"user_id_type": "user_id"},
		},
		{
			name:       "contract cooperation search",
			args:       []string{"contract", "cooperation", "search", "--profile", "contract", "--data", `{"user_id":"u1","page_size":10}`},
			wantMethod: http.MethodPost,
			wantPath:   "/open-apis/contract/v1/cooperation/search",
			wantQuery:  map[string]string{"user_id_type": "user_id"},
			wantBody:   `{"user_id":"u1","page_size":10}`,
		},
		{
			name:       "contract esign personal auth url",
			args:       []string{"contract", "esign", "personal-auth-url", "--profile", "contract", "--data", `{"psnAuthConfig":{"psnAccount":"18500000000"}}`},
			wantMethod: http.MethodPost,
			wantPath:   "/open-apis/esign/auth/psnAuthUrl",
			wantQuery:  map[string]string{"user_id_type": "user_id"},
			wantBody:   `{"psnAuthConfig":{"psnAccount":"18500000000"}}`,
		},
		{
			name:       "contract esign org auth url",
			args:       []string{"contract", "esign", "org-auth-url", "--profile", "contract", "--data", `{"orgAuthConfig":{"orgName":"北京优矩"}}`},
			wantMethod: http.MethodPost,
			wantPath:   "/open-apis/esign/auth/orgAuthUrl",
			wantQuery:  map[string]string{"user_id_type": "user_id"},
			wantBody:   `{"orgAuthConfig":{"orgName":"北京优矩"}}`,
		},
		{
			name:       "mdm fixed exchange rate get",
			args:       []string{"mdm", "fixed-exchange-rate", "get", "--profile", "contract", "--source-currency", "CNY", "--target-currency", "USD", "--effective-date", "2026-06-01"},
			wantMethod: http.MethodGet,
			wantPath:   "/open-apis/mdm/v1/fixed_exchange_rate",
			wantQuery:  map[string]string{"source_currency": "CNY", "target_currency": "USD", "date": "2026-06-01", "user_id_type": "user_id"},
		},
		{
			name:       "mdm fixed exchange rate update",
			args:       []string{"mdm", "fixed-exchange-rate", "update", "--profile", "contract", "--data", `{"source_currency":"CNY","target_currency":"USD","exchange_rate":"7.1"}`},
			wantMethod: http.MethodPut,
			wantPath:   "/open-apis/mdm/v1/fixed_exchange_rate",
			wantQuery:  map[string]string{"user_id_type": "user_id"},
			wantBody:   `{"source_currency":"CNY","target_currency":"USD","exchange_rate":"7.1"}`,
		},
		{
			name:       "mdm vendor create",
			args:       []string{"mdm", "vendor", "create", "--profile", "contract", "--user-id", "operator-1", "--data", `{"vendorText":"供应商A"}`},
			wantMethod: http.MethodPost,
			wantPath:   "/open-apis/mdm/v1/vendors",
			wantQuery:  map[string]string{"user_id_type": "user_id", "user_id": "operator-1"},
			wantBody:   `{"vendorText":"供应商A"}`,
		},
		{
			name:       "mdm vendor update",
			args:       []string{"mdm", "vendor", "update", "vendor-1", "--profile", "contract", "--user-id", "operator-1", "--data", `{"id":"vendor-1","vendor":"V0001","vendorText":"供应商A"}`},
			wantMethod: http.MethodPut,
			wantPath:   "/open-apis/mdm/v1/vendors/vendor-1",
			wantQuery:  map[string]string{"user_id_type": "user_id", "user_id": "operator-1"},
			wantBody:   `{"id":"vendor-1","vendor":"V0001","vendorText":"供应商A"}`,
		},
		{
			name:       "mdm vendor list all",
			args:       []string{"mdm", "vendor", "list-all", "--profile", "contract", "--page-size", "10", "--page-token", "next"},
			wantMethod: http.MethodGet,
			wantPath:   "/open-apis/mdm/v1/vendors/list_all",
			wantQuery:  map[string]string{"page_size": "10", "page_token": "next", "user_id_type": "user_id"},
		},
		{
			name:       "mdm vendor query by cert",
			args:       []string{"mdm", "vendor", "query-by-cert", "--profile", "contract", "--certification-id", "CERT-1", "--ad-country", "CN"},
			wantMethod: http.MethodGet,
			wantPath:   "/open-apis/mdm/v1/vendors/query_vendors",
			wantQuery:  map[string]string{"certification_id": "CERT-1", "ad_country": "CN", "user_id_type": "user_id"},
		},
		{
			name:       "mdm legal create",
			args:       []string{"mdm", "legal", "create", "--profile", "contract", "--user-id", "operator-1", "--data", `{"legalEntityText":"法人A"}`},
			wantMethod: http.MethodPost,
			wantPath:   "/open-apis/mdm/v1/legal_entities",
			wantQuery:  map[string]string{"user_id_type": "user_id", "user_id": "operator-1"},
			wantBody:   `{"legalEntityText":"法人A"}`,
		},
		{
			name:       "mdm legal update",
			args:       []string{"mdm", "legal", "update", "legal-1", "--profile", "contract", "--user-id", "operator-1", "--data", `{"id":"legal-1","legalEntity":"L0001","legalEntityText":"法人A"}`},
			wantMethod: http.MethodPut,
			wantPath:   "/open-apis/mdm/v1/legal_entities/legal-1",
			wantQuery:  map[string]string{"user_id_type": "user_id", "user_id": "operator-1"},
			wantBody:   `{"id":"legal-1","legalEntity":"L0001","legalEntityText":"法人A"}`,
		},
		{
			name:       "mdm legal get by code",
			args:       []string{"mdm", "legal", "get", "--profile", "contract", "--code", "L0001", "--page-size", "10", "--page-token", "next"},
			wantMethod: http.MethodGet,
			wantPath:   "/open-apis/mdm/v1/legal_entities",
			wantQuery:  map[string]string{"legalEntity": "L0001", "page_size": "10", "page_token": "next", "user_id_type": "user_id"},
		},
		{
			name:       "event outbound ip list",
			args:       []string{"event", "outbound-ip", "list", "--profile", "contract", "--page-size", "10", "--page-token", "next"},
			wantMethod: http.MethodGet,
			wantPath:   "/open-apis/event/v1/outbound_ip",
			wantQuery:  map[string]string{"page_size": "10", "page_token": "next", "user_id_type": "user_id"},
		},
		{
			name:       "rule table list",
			args:       []string{"rule", "table", "list", "--profile", "contract", "--product-id", "prod-1", "--group-id", "group-1", "--page-size", "10", "--page-token", "next"},
			wantMethod: http.MethodGet,
			wantPath:   "/open-apis/rule_engine/v1/products/prod-1/groups/group-1/rule_tables",
			wantQuery:  map[string]string{"page_size": "10", "page_token": "next", "user_id_type": "user_id"},
		},
		{
			name:       "rule table column headers list",
			args:       []string{"rule", "table", "column-headers", "list", "--profile", "contract", "--product-id", "prod-1", "--group-id", "group-1", "--table-id", "table-1"},
			wantMethod: http.MethodGet,
			wantPath:   "/open-apis/rule_engine/v1/products/prod-1/groups/group-1/rule_tables/table-1/table_columns/column_headers",
			wantQuery:  map[string]string{"user_id_type": "user_id"},
		},
		{
			name:       "rule table row create",
			args:       []string{"rule", "table", "row", "create", "--profile", "contract", "--product-id", "prod-1", "--group-id", "group-1", "--table-id", "table-1", "--data", `{"table_cells":[]}`},
			wantMethod: http.MethodPost,
			wantPath:   "/open-apis/rule_engine/v1/products/prod-1/groups/group-1/rule_tables/table-1/table_rows",
			wantQuery:  map[string]string{"user_id_type": "user_id"},
			wantBody:   `{"table_cells":[]}`,
		},
		{
			name:       "rule table row get",
			args:       []string{"rule", "table", "row", "get", "row-1", "--profile", "contract", "--product-id", "prod-1", "--group-id", "group-1", "--table-id", "table-1"},
			wantMethod: http.MethodGet,
			wantPath:   "/open-apis/rule_engine/v1/products/prod-1/groups/group-1/rule_tables/table-1/table_rows/row-1",
			wantQuery:  map[string]string{"user_id_type": "user_id"},
		},
		{
			name:       "rule table row list",
			args:       []string{"rule", "table", "row", "list", "--profile", "contract", "--product-id", "prod-1", "--group-id", "group-1", "--table-id", "table-1", "--page-size", "10", "--page-token", "next"},
			wantMethod: http.MethodGet,
			wantPath:   "/open-apis/rule_engine/v1/products/prod-1/groups/group-1/rule_tables/table-1/table_rows",
			wantQuery:  map[string]string{"page_size": "10", "page_token": "next", "user_id_type": "user_id"},
		},
		{
			name:       "rule table row search",
			args:       []string{"rule", "table", "row", "search", "--profile", "contract", "--product-id", "prod-1", "--group-id", "group-1", "--table-id", "table-1", "--page-size", "5", "--page-token", "next", "--data", `{"table_cell":{"table_column_id":"column-1","table_cell_content_type":"STRING","table_cell_content":{"string":"value"}}}`},
			wantMethod: http.MethodPost,
			wantPath:   "/open-apis/rule_engine/v1/products/prod-1/groups/group-1/rule_tables/table-1/table_rows/search",
			wantQuery:  map[string]string{"page_size": "5", "page_token": "next", "user_id_type": "user_id"},
			wantBody:   `{"table_cell":{"table_column_id":"column-1","table_cell_content_type":"STRING","table_cell_content":{"string":"value"}}}`,
		},
		{
			name:       "rule table row update",
			args:       []string{"rule", "table", "row", "update", "row-1", "--profile", "contract", "--product-id", "prod-1", "--group-id", "group-1", "--table-id", "table-1", "--data", `{"table_cells":[]}`},
			wantMethod: http.MethodPut,
			wantPath:   "/open-apis/rule_engine/v1/products/prod-1/groups/group-1/rule_tables/table-1/table_rows/row-1",
			wantQuery:  map[string]string{"user_id_type": "user_id"},
			wantBody:   `{"table_cells":[]}`,
		},
		{
			name:       "rule table row delete",
			args:       []string{"rule", "table", "row", "delete", "row-1", "--profile", "contract", "--product-id", "prod-1", "--group-id", "group-1", "--table-id", "table-1"},
			wantMethod: http.MethodDelete,
			wantPath:   "/open-apis/rule_engine/v1/products/prod-1/groups/group-1/rule_tables/table-1/table_rows/row-1",
			wantQuery:  map[string]string{"user_id_type": "user_id"},
		},
	}

	for _, tc := range testCases {
		identities := []string{"app"}
		if tc.args[0] == "rule" {
			identities = append(identities, "user")
		}
		for _, identity := range identities {
			tc := tc
			identity := identity
			t.Run(tc.name+"/"+identity, func(t *testing.T) {
				t.Parallel()

				stdout := &bytes.Buffer{}
				app := cli.New(cli.Options{
					Stdout: stdout,
					Stderr: &bytes.Buffer{},
					Store:  store,
					HTTPClient: &http.Client{
						Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
							if req.Method != tc.wantMethod {
								t.Fatalf("method = %s, want %s", req.Method, tc.wantMethod)
							}
							if req.URL.Path != tc.wantPath {
								t.Fatalf("path = %s, want %s", req.URL.Path, tc.wantPath)
							}
							assertQuery(t, req, tc.wantQuery)
							if req.Header.Get("Authorization") != "Bearer "+identity+"-token" {
								t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
							}
							wantMarker := ""
							if identity == "user" {
								wantMarker = "user"
							}
							if req.Header.Get("X-Qfei-Identity") != wantMarker {
								t.Fatal("unexpected identity selector")
							}
							body, err := io.ReadAll(req.Body)
							if err != nil {
								t.Fatalf("ReadAll() error = %v", err)
							}
							if string(body) != tc.wantBody {
								t.Fatalf("body = %q, want %q", string(body), tc.wantBody)
							}
							return jsonResponse(`{"code":0,"data":{"ok":true}}`), nil
						}),
					},
				})

				if err := app.Run(context.Background(), slices.Concat(tc.args, []string{"--as", identity})); err != nil {
					t.Fatalf("Run() error = %v", err)
				}
				if !strings.Contains(stdout.String(), `"code": 0`) {
					t.Fatalf("unexpected output: %s", stdout.String())
				}
			})
		}
	}
}

/*
TestRuleTableRowSearchEmptyResults 验证规则行搜索在服务端返回空对象时仍向调用方提供稳定的空行列表。
入参 t（*testing.T）为测试上下文；返回值为空，断言失败时报告测试错误。
*/
func TestRuleTableRowSearchEmptyResults(t *testing.T) {
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatal(err)
	}
	for _, identity := range []string{"app", "user"} {
		t.Run(identity, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			app := cli.New(cli.Options{Store: store, Stdout: stdout, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost || !strings.HasSuffix(req.URL.Path, "/table_rows/search") {
					t.Fatalf("unexpected search request: %s %s", req.Method, req.URL)
				}
				return jsonResponse(`{"code":0,"data":{}}`), nil
			})}})
			err := app.Run(context.Background(), []string{"rule", "table", "row", "search", "--profile", "contract", "--as", identity, "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "t", "--page-size", "10", "--data", `{"table_cell":{"table_column_id":"c","table_cell_content_type":"STRING","table_cell_content":{"string":"missing"}}}`})
			if err != nil {
				t.Fatal(err)
			}
			result := decodeApprovalMatrixOutput(t, stdout.Bytes())
			data, ok := result["data"].(map[string]any)
			if !ok {
				t.Fatalf("search data=%v", result["data"])
			}
			rows, ok := data["table_rows"].([]any)
			if !ok || len(rows) != 0 || data["has_more"] != false {
				t.Fatalf("empty search data=%v", data)
			}
		})
	}
}

/*
TestRuleTableRowSearchPreservesOtherResponses 验证原始输出、业务错误和非空分页不会被空结果规范化误改。
入参 t（*testing.T）为测试上下文；返回值为空，断言失败时报告测试错误。
*/
func TestRuleTableRowSearchPreservesOtherResponses(t *testing.T) {
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, body     string
		raw, wantError bool
	}{
		{"raw", `{"code":0,"data":{}}`, true, false},
		{"business_error", `{"code":41001,"msg":"rejected","data":{}}`, false, true},
		{"nonempty", `{"code":0,"data":{"table_rows":[{"id":"r1"}],"has_more":true,"page_token":"next"}}`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			app := cli.New(cli.Options{Store: store, Stdout: stdout, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(tc.body), nil
			})}})
			args := []string{"rule", "table", "row", "search", "--profile", "contract", "--as", "app", "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "t", "--page-size", "10", "--data", `{"table_cell":{"table_column_id":"c","table_cell_content_type":"STRING","table_cell_content":{"string":"x"}}}`}
			if tc.raw {
				args = append(args, "--raw")
			}
			err := app.Run(context.Background(), args)
			if (err != nil) != tc.wantError {
				t.Fatalf("Run() error=%v, wantError=%v", err, tc.wantError)
			}
			if tc.raw {
				if stdout.String() != tc.body {
					t.Fatalf("raw output=%q, want %q", stdout.String(), tc.body)
				}
				return
			}
			result := decodeApprovalMatrixOutput(t, stdout.Bytes())
			data, ok := result["data"].(map[string]any)
			if !ok {
				t.Fatalf("response data=%v", result["data"])
			}
			if tc.wantError {
				if len(data) != 0 || result["code"] != float64(41001) {
					t.Fatalf("business error was changed: %v", result)
				}
				return
			}
			rows, ok := data["table_rows"].([]any)
			if !ok || len(rows) != 1 || data["has_more"] != true || data["page_token"] != "next" {
				t.Fatalf("nonempty search was changed: %v", data)
			}
		})
	}
}

func TestOpenAPIGapDownloadCommandsWriteOutputFileAsBot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := config.NewStore(dir)
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	testCases := []struct {
		name     string
		args     []string
		wantPath string
	}{
		{
			name:     "contract cooperation file download",
			args:     []string{"contract", "cooperation", "file", "download", "file-1", "--profile", "contract"},
			wantPath: "/open-apis/contract/v1/contracts/cooperation/file-1/download_file",
		},
		{
			name:     "mdm file download",
			args:     []string{"mdm", "file", "download", "file-1", "--profile", "contract"},
			wantPath: "/open-apis/mdm/v1/file/download/file-1",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			outputPath := filepath.Join(dir, strings.ReplaceAll(tc.name, " ", "-")+".bin")
			args := slices.Concat(tc.args, []string{"--output-file", outputPath})
			stdout := &bytes.Buffer{}
			app := cli.New(cli.Options{
				Stdout: stdout,
				Stderr: &bytes.Buffer{},
				Store:  store,
				HTTPClient: &http.Client{
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						if req.Method != http.MethodGet {
							t.Fatalf("method = %s", req.Method)
						}
						if req.URL.Path != tc.wantPath {
							t.Fatalf("path = %s, want %s", req.URL.Path, tc.wantPath)
						}
						assertQuery(t, req, map[string]string{"user_id_type": "user_id"})
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     make(http.Header),
							Body:       io.NopCloser(strings.NewReader("download bytes")),
						}, nil
					}),
				},
			})

			if err := app.Run(context.Background(), args); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			content, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatalf("ReadFile(output) error = %v", err)
			}
			if string(content) != "download bytes" {
				t.Fatalf("downloaded content = %q", string(content))
			}
			if !strings.Contains(stdout.String(), "Downloaded file to "+outputPath) {
				t.Fatalf("missing download message: %s", stdout.String())
			}
		})
	}
}

func TestOpenAPIGapBotOnlyCommandsRejectUserIdentityBeforeHTTP(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := config.NewStore(dir)
	if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	testCases := [][]string{
		{"contract", "search-v2", "--profile", "contract", "--as", "user", "--data", `{"contract_number":"CN-001"}`},
		{"contract", "field", "update", "--profile", "contract", "--as", "user", "--data", `{"module_name":"签约信息","attribute_name":"城市"}`},
		{"contract", "sign", "switch-to-paper", "--profile", "contract", "--as", "user", "--business-id", "contract-1", "--business-type-code", "0"},
		{"contract", "sign-url", "get", "contract-1", "--profile", "contract", "--as", "user"},
		{"contract", "form", "attribute", "list", "--profile", "contract", "--as", "user", "--category-id", "cat-1", "--business-type-code", "0"},
		{"contract", "authorization", "grant", "--profile", "contract", "--as", "user", "--data", `{"business_id":"contract-1"}`},
		{"contract", "share", "batch-create", "--profile", "contract", "--as", "user", "--data", `{"contract_id":"contract-1","user_ids":["u1"]}`},
		{"contract", "cooperation", "search", "--profile", "contract", "--as", "user", "--data", `{"user_id":"u1"}`},
		{"contract", "cooperation", "file", "download", "file-1", "--profile", "contract", "--as", "user", "--output-file", filepath.Join(dir, "cooperation.bin")},
		{"contract", "esign", "personal-auth-url", "--profile", "contract", "--as", "user", "--data", `{"psnAuthConfig":{"psnAccount":"18500000000"}}`},
		{"mdm", "fixed-exchange-rate", "update", "--profile", "contract", "--as", "user", "--data", `{"source_currency":"CNY"}`},
		{"mdm", "vendor", "create", "--profile", "contract", "--as", "user", "--data", `{"vendor":"V0001"}`},
		{"mdm", "legal", "create", "--profile", "contract", "--as", "user", "--data", `{"legal_entity":"L0001"}`},
		{"mdm", "legal", "get", "--profile", "contract", "--as", "user", "--code", "L0001"},
		{"event", "outbound-ip", "list", "--profile", "contract", "--as", "user"},
	}

	for _, args := range testCases {
		args := args
		t.Run(strings.Join(args[:min(4, len(args))], " "), func(t *testing.T) {
			t.Parallel()

			requests := 0
			app := cli.New(cli.Options{
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
				Store:  store,
				HTTPClient: &http.Client{
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						requests++
						return jsonResponse(`{"code":0}`), nil
					}),
				},
			})

			err := app.Run(context.Background(), args)
			if err == nil || !strings.Contains(err.Error(), "only supports --as app") {
				t.Fatalf("unexpected user error: %v", err)
			}
			if requests != 0 {
				t.Fatalf("user rejection should not send HTTP, got %d requests", requests)
			}
		})
	}
}

func TestOpenAPIGapCommandValidationErrors(t *testing.T) {
	t.Parallel()

	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	removedCodeCommand := strings.Join([]string{"query", "by", "code"}, "-")
	removedLegalEntityFlag := "--legal" + "-entity"
	testCases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "search v2 missing body",
			args:    []string{"contract", "search-v2", "--profile", "contract"},
			wantErr: "--input-file or --data is required",
		},
		{
			name:    "sign missing business id",
			args:    []string{"contract", "sign", "switch-to-paper", "--profile", "contract", "--business-type-code", "0"},
			wantErr: "--business-id is required",
		},
		{
			name:    "form attribute missing category id",
			args:    []string{"contract", "form", "attribute", "list", "--profile", "contract", "--business-type-code", "0"},
			wantErr: "--category-id is required",
		},
		{
			name:    "sign url get rejects body",
			args:    []string{"contract", "sign-url", "get", "contract-1", "--profile", "contract", "--data", `{}`},
			wantErr: "contract sign-url get does not accept --input-file or --data",
		},
		{
			name:    "cooperation search missing body",
			args:    []string{"contract", "cooperation", "search", "--profile", "contract"},
			wantErr: "--input-file or --data is required",
		},
		{
			name:    "event list rejects body",
			args:    []string{"event", "outbound-ip", "list", "--profile", "contract", "--data", `{}`},
			wantErr: "event outbound-ip list does not accept --input-file or --data",
		},
		{
			name:    "event list rejects page size below documented minimum",
			args:    []string{"event", "outbound-ip", "list", "--profile", "contract", "--page-size", "3"},
			wantErr: "--page-size must be between 10 and 50",
		},
		{
			name:    "event list rejects page size above documented maximum",
			args:    []string{"event", "outbound-ip", "list", "--profile", "contract", "--page-size", "51"},
			wantErr: "--page-size must be between 10 and 50",
		},
		{
			name:    "mdm vendor create requires user id",
			args:    []string{"mdm", "vendor", "create", "--profile", "contract", "--data", `{"vendorText":"供应商A"}`},
			wantErr: "mdm vendor create requires --user-id",
		},
		{
			name:    "mdm vendor create rejects generated code",
			args:    []string{"mdm", "vendor", "create", "--profile", "contract", "--user-id", "operator-1", "--data", `{"vendor":"V0001","vendorText":"供应商A"}`},
			wantErr: "mdm vendor create body must not include vendor",
		},
		{
			name:    "mdm vendor update requires generated id and code",
			args:    []string{"mdm", "vendor", "update", "vendor-1", "--profile", "contract", "--user-id", "operator-1", "--data", `{"vendorText":"供应商A"}`},
			wantErr: "mdm vendor update body must include id and vendor",
		},
		{
			name:    "mdm legal create requires user id",
			args:    []string{"mdm", "legal", "create", "--profile", "contract", "--data", `{"legalEntityText":"法人A"}`},
			wantErr: "mdm legal create requires --user-id",
		},
		{
			name:    "mdm legal create rejects generated code",
			args:    []string{"mdm", "legal", "create", "--profile", "contract", "--user-id", "operator-1", "--data", `{"legalEntity":"L0001","legalEntityText":"法人A"}`},
			wantErr: "mdm legal create body must not include legalEntity",
		},
		{
			name:    "mdm legal update rejects snake case generated code",
			args:    []string{"mdm", "legal", "update", "legal-1", "--profile", "contract", "--user-id", "operator-1", "--data", `{"id":"legal-1","legal_entity":"L0001"}`},
			wantErr: "mdm legal update body must use legalEntity",
		},
		{
			name:    "mdm legal update requires generated id and code",
			args:    []string{"mdm", "legal", "update", "legal-1", "--profile", "contract", "--user-id", "operator-1", "--data", `{"legalEntityText":"法人A"}`},
			wantErr: "mdm legal update body must include id and legalEntity",
		},
		{
			name:    "rule table list missing product id",
			args:    []string{"rule", "table", "list", "--profile", "contract", "--group-id", "group-1"},
			wantErr: "--product-id is required",
		},
		{
			name:    "rule table list missing page size",
			args:    []string{"rule", "table", "list", "--profile", "contract", "--product-id", "prod-1", "--group-id", "group-1"},
			wantErr: "--page-size is required",
		},
		{
			name:    "rule table list rejects page size above maximum",
			args:    []string{"rule", "table", "list", "--profile", "contract", "--product-id", "prod-1", "--group-id", "group-1", "--page-size", "101"},
			wantErr: "--page-size must be between 1 and 100",
		},
		{
			name:    "rule table row search missing page size",
			args:    []string{"rule", "table", "row", "search", "--profile", "contract", "--product-id", "prod-1", "--group-id", "group-1", "--table-id", "table-1", "--data", `{"table_cell":{"table_column_id":"column-1","table_cell_content_type":"STRING","table_cell_content":{"string":"value"}}}`},
			wantErr: "--page-size is required",
		},
		{
			name:    "rule table row search rejects page size below minimum",
			args:    []string{"rule", "table", "row", "search", "--profile", "contract", "--product-id", "prod-1", "--group-id", "group-1", "--table-id", "table-1", "--page-size", "0", "--data", `{"table_cell":{"table_column_id":"column-1","table_cell_content_type":"STRING","table_cell_content":{"string":"value"}}}`},
			wantErr: "--page-size must be between 1 and 100",
		},
		{
			name:    "rule table row search rejects page size above maximum",
			args:    []string{"rule", "table", "row", "search", "--profile", "contract", "--product-id", "prod-1", "--group-id", "group-1", "--table-id", "table-1", "--page-size", "101", "--data", `{"table_cell":{"table_column_id":"column-1","table_cell_content_type":"STRING","table_cell_content":{"string":"value"}}}`},
			wantErr: "--page-size must be between 1 and 100",
		},
		{
			name:    "rule table row create missing body",
			args:    []string{"rule", "table", "row", "create", "--profile", "contract", "--product-id", "prod-1", "--group-id", "group-1", "--table-id", "table-1"},
			wantErr: "--input-file or --data is required",
		},
		{
			name:    "mdm file download rejects body",
			args:    []string{"mdm", "file", "download", "file-1", "--profile", "contract", "--output-file", "out.bin", "--data", `{}`},
			wantErr: "mdm file download does not accept --input-file or --data",
		},
		{
			name:    "mdm legal get code rejects body",
			args:    []string{"mdm", "legal", "get", "--profile", "contract", "--code", "L0001", "--data", `{}`},
			wantErr: "mdm legal get --code does not accept --input-file or --data",
		},
		{
			name:    "mdm legal get code rejects id",
			args:    []string{"mdm", "legal", "get", "legal-1", "--profile", "contract", "--code", "L0001"},
			wantErr: "usage: contract-cli mdm legal get <legal-entity-id> [flags]",
		},
		{
			name:    "mdm legal query by code removed",
			args:    []string{"mdm", "legal", removedCodeCommand, "--profile", "contract", removedLegalEntityFlag, "L0001"},
			wantErr: "unknown mdm legal subcommand",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := cli.New(cli.Options{
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
				Store:  store,
				HTTPClient: &http.Client{
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						t.Fatalf("validation error should not send HTTP")
						return nil, nil
					}),
				},
			})

			err := app.Run(context.Background(), tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("unexpected error: %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func assertQuery(t *testing.T, req *http.Request, want map[string]string) {
	t.Helper()

	query := req.URL.Query()
	if len(query) != len(want) {
		t.Fatalf("query keys = %v, want %v", query, want)
	}
	for key, value := range want {
		if got := query.Get(key); got != value {
			t.Fatalf("query %s = %q, want %q", key, got, value)
		}
	}
}
