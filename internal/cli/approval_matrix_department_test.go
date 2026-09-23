package cli_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

/* TestRuleTableDepartmentIDsUseOpenDepartmentID 验证创建、搜索和更新规则行时保留 od- 部门 ID。 */
func TestRuleTableDepartmentIDsUseOpenDepartmentID(t *testing.T) {
	t.Parallel()

	const departmentID = "od-1b1b803a7df98989bf457d9ba203c350"
	cases := []struct {
		name string
		args []string
		body string
	}{
		{
			name: "create",
			args: []string{"rule", "table", "row", "create", "--profile", "contract", "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "table-1"},
			body: `{"table_cells":[{"table_cell_content":{"department_collection":["` + departmentID + `"]},"table_cell_content_type":"DEPARTMENT_COLLECTION","table_column_id":"column-1"}]}`,
		},
		{
			name: "search",
			args: []string{"rule", "table", "row", "search", "--profile", "contract", "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "table-1", "--page-size", "10"},
			body: `{"table_cell":{"table_cell_content":{"department_collection":["` + departmentID + `"]},"table_cell_content_type":"DEPARTMENT_COLLECTION","table_column_id":"column-1"}}`,
		},
		{
			name: "update",
			args: []string{"rule", "table", "row", "update", "row-1", "--profile", "contract", "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "table-1"},
			body: `{"table_cells":[{"table_cell_content":{"department_collection":["` + departmentID + `"]},"table_cell_content_type":"DEPARTMENT_COLLECTION","table_column_id":"column-1"}]}`,
		},
	}

	for _, identity := range []config.IdentityKind{config.IdentityUser, config.IdentityApp} {
		for _, tc := range cases {
			t.Run(string(identity)+"/"+tc.name, func(t *testing.T) {
				store := config.NewStore(t.TempDir())
				if err := store.UpsertProfile(uploadProfile(identity), true); err != nil {
					t.Fatal(err)
				}
				var gotBody string
				app := cli.New(cli.Options{
					Store:  store,
					Stdout: &bytes.Buffer{},
					Stderr: &bytes.Buffer{},
					HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						data, err := io.ReadAll(req.Body)
						if err != nil {
							t.Fatal(err)
						}
						gotBody = string(data)
						return jsonResponse(`{"code":0,"msg":"success","data":{}}`), nil
					})},
				})
				args := append(append([]string{}, tc.args...), "--as", string(identity), "--data", tc.body)
				if err := app.Run(context.Background(), args); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(gotBody, `"department_collection":["`+departmentID+`"]`) {
					t.Fatalf("department ID was not preserved: %s", gotBody)
				}
			})
		}
	}
}

/* TestRuleTableDepartmentIDsRejectInternalID 验证 CLI 会在发起 HTTP 请求前拒绝数字型 sys_department.id。 */
func TestRuleTableDepartmentIDsRejectInternalID(t *testing.T) {
	t.Parallel()

	for _, identity := range []config.IdentityKind{config.IdentityUser, config.IdentityApp} {
		t.Run(string(identity), func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(identity), true); err != nil {
				t.Fatal(err)
			}
			requests := 0
			app := cli.New(cli.Options{
				Store:  store,
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
				HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests++
					return jsonResponse(`{"code":0,"msg":"unexpected","data":{}}`), nil
				})},
			})
			err := app.Run(context.Background(), []string{
				"rule", "table", "row", "search", "--profile", "contract", "--as", string(identity),
				"--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "table-1", "--page-size", "10",
				"--data", `{"table_cell":{"table_cell_content":{"department_collection":[1018399484738012242]},"table_cell_content_type":"DEPARTMENT_COLLECTION","table_column_id":"column-1"}}`,
			})
			if err == nil || !strings.Contains(err.Error(), "sys_department.id is not accepted") {
				t.Fatalf("error = %v, want local department ID validation", err)
			}
			if requests != 0 {
				t.Fatalf("internal department ID sent to backend: %d requests", requests)
			}
		})
	}
}
