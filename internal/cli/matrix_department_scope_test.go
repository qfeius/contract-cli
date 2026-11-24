package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestMatrixDepartmentDefaultsRespectProductAndGroup(t *testing.T) {
	for _, tc := range []struct {
		product, group, id, explicit string
		valid                        bool
	}{
		{"contract", "approve_matrix", "123", "", true},
		{"contract", "approve_matrix", "od-external", "", false},
		{"other", "approve_matrix", "od-external", "", true},
		{"contract", "other", "od-external", "", true},
		{"other", "other", "123", "", false},
		{"other", "other", "123", "tenant_department_id", true},
		{"contract", "approve_matrix", "od-external", "open_department_id", true},
	} {
		for _, action := range []string{"create", "search", "update"} {
			t.Run(tc.product+"/"+tc.group+"/"+tc.id+"/"+tc.explicit+"/"+action, func(t *testing.T) {
				store := config.NewStore(t.TempDir())
				if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
					t.Fatal(err)
				}
				calls := 0
				app := cli.New(cli.Options{Store: store, Stdout: &bytes.Buffer{}, Stderr: io.Discard, HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					calls++
					return jsonResponse(`{"code":0,"data":{}}`), nil
				})}})
				args := []string{"rule", "table", "row", action}
				if action == "update" {
					args = append(args, "row-1")
				}
				args = append(args, "--product-id", tc.product, "--group-id", tc.group, "--table-id", "table-1", "--data", `{"table_cells":[{"table_cell_content":{"department_collection":["`+tc.id+`"]}}]}`)
				if action == "search" {
					args = append(args, "--page-size", "10")
				}
				if tc.explicit != "" {
					args = append(args, "--department-id-type", tc.explicit)
				}
				err := app.Run(context.Background(), args)
				if (err == nil) != tc.valid || (calls == 1) != tc.valid {
					t.Fatalf("valid=%t calls=%d err=%v", tc.valid, calls, err)
				}
			})
		}
	}
}

func TestMatrixImportRecognizesDepartmentIDTypeBeforeBusinessInput(t *testing.T) {
	app, _ := alignmentApp(t, nil)
	err := app.Run(context.Background(), []string{"rule", "table", "import", "plan", "--product-id", "contract", "--group-id", "approve_matrix", "--department-id-type", "open_department_id", "--data", `{"rows":[]}`})
	if err == nil || strings.Contains(err.Error(), "unknown flag") || !strings.Contains(err.Error(), "tenant_department_id") {
		t.Fatalf("unexpected department mode error: %v", err)
	}
}

func TestMatrixImportKeepsExplicitDepartmentTypeThroughApply(t *testing.T) {
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatal(err)
	}
	stdout := &bytes.Buffer{}
	created := false
	const row = `{"id":"created","table_cells":[{"table_column_id":"c","table_cell_content_type":"DEPARTMENT_COLLECTION","table_cell_content":{"department_collection":["external-code"]}}]}`
	app := cli.New(cli.Options{Store: store, Stdout: stdout, Stderr: io.Discard, HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "column_headers") {
			return jsonResponse(`{"code":0,"data":{"columns_headers":[{"id":"c","name":"department","type":1,"table_cell_content_type":"DEPARTMENT_COLLECTION"}]}}`), nil
		}
		if !strings.Contains(r.URL.Path, "table_rows") {
			t.Fatalf("unexpected request %s", r.URL.Path)
		}
		if r.URL.Query().Get("department_id_type") != "department_id" {
			t.Fatal("confirmed department type was lost between plan and execution")
		}
		if r.Method == "POST" {
			created = true
			return jsonResponse(`{"code":0,"data":{"table_row_id":"created"}}`), nil
		}
		if strings.HasSuffix(r.URL.Path, "/created") {
			return jsonResponse(`{"code":0,"data":{"table_row":` + row + `}}`), nil
		}
		rows := ""
		if created {
			rows = row
		}
		return jsonResponse(fmt.Sprintf(`{"code":0,"data":{"table_rows":[%s],"has_more":false}}`, rows)), nil
	})}})
	if err := app.Run(context.Background(), []string{"rule", "table", "import", "plan", "--as", "app", "--product-id", "other", "--group-id", "other", "--table-id", "t", "--department-id-type", "department_id", "--data", `{"rows":[{"cells":{"department":["external-code"]}}]}`}); err != nil {
		t.Fatal(err)
	}
	result := decodeApprovalMatrixOutput(t, stdout.Bytes())
	planID, _ := result["plan_id"].(string)
	if planID == "" {
		t.Fatalf("no plan created: %v", result)
	}
	stdout.Reset()
	if err := app.Run(context.Background(), []string{"rule", "table", "import", "apply", "--as", "app", "--plan-id", planID}); err != nil {
		t.Fatal(err)
	}
	result = decodeApprovalMatrixOutput(t, stdout.Bytes())
	if result["status"] != "success" || !created {
		t.Fatalf("typed plan failed: %v", result)
	}
}
