package cli

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNormalizeApprovalMatrixPublishAliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "top-level publish",
			args: []string{"approval-matrix", "publish", "--table-id", "table-1"},
			want: []string{"rule", "table", "release", "--table-id", "table-1"},
		},
		{
			name: "table publish",
			args: []string{"approval-matrix", "table", "publish", "--table-id", "table-1"},
			want: []string{"rule", "table", "release", "--table-id", "table-1"},
		},
		{
			name: "help top-level publish",
			args: []string{"help", "approval-matrix", "publish"},
			want: []string{"help", "rule", "table", "release"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeApprovalMatrixArgs(tt.args); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("normalizeApprovalMatrixArgs(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestNewApprovalMatrixTableCellNullClearsValue(t *testing.T) {
	t.Parallel()

	for _, contentType := range []string{"STRING", "NUMBER", "BOOLEAN", "COLLECTION", "EMPLOYEE_COLLECTION", "DEPARTMENT_COLLECTION", "ROLE_COLLECTION"} {
		t.Run(contentType, func(t *testing.T) {
			cell, err := newApprovalMatrixTableCell(approvalMatrixColumn{ID: "column-1", CellContentType: contentType}, json.RawMessage("null"))
			if err != nil {
				t.Fatalf("newApprovalMatrixTableCell() error = %v", err)
			}
			if cell.TableColumnID != "column-1" || cell.TableCellContentType != contentType {
				t.Fatalf("cell target = %#v", cell)
			}
			content := cell.TableCellContent
			if content.Bool != nil || content.Collection != nil || content.DepartmentCollection != nil || content.EmployeeCollection != nil || content.RoleCollection != nil || content.Number != nil || content.String != nil {
				t.Fatalf("null did not clear content: %#v", content)
			}
		})
	}
}

/* TestApprovalMatrixDepartmentImportModes 验证合同原生 ID 与其他规则组旧部门 ID 各按目标模式校验。 */
func TestApprovalMatrixDepartmentImportModes(t *testing.T) {
	t.Parallel()
	column := approvalMatrixColumn{ID: "department", CellContentType: "DEPARTMENT_COLLECTION"}
	legacy, err := newApprovalMatrixTableCell(column, json.RawMessage(`["od-department"]`))
	if err != nil || len(*legacy.TableCellContent.DepartmentCollection) != 1 || (*legacy.TableCellContent.DepartmentCollection)[0] != "od-department" {
		t.Fatalf("legacy department cell = %#v, err = %v", legacy, err)
	}
	native, err := newApprovalMatrixTableCellWithDepartmentIDType(column, json.RawMessage(`["1018399484738012242"]`), "tenant_department_id")
	if err != nil || len(*native.TableCellContent.DepartmentCollection) != 1 || (*native.TableCellContent.DepartmentCollection)[0] != "1018399484738012242" {
		t.Fatalf("native department cell = %#v, err = %v", native, err)
	}
}

/* TestApprovalMatrixImportRejectsMixedPersonnelModes 验证计划和执行不会混用原生与旧人员 ID。 */
func TestApprovalMatrixImportRejectsMixedPersonnelModes(t *testing.T) {
	t.Parallel()
	if err := validateApprovalMatrixImportIDType(commandOptions{userIDType: "user_id"}, "contract", "approve_matrix"); err == nil {
		t.Fatal("contract import accepted the legacy personnel mode")
	}
	if err := validateApprovalMatrixImportIDType(commandOptions{userIDType: "tenant_employee_id"}, "contract", "approve_matrix"); err != nil {
		t.Fatal(err)
	}
	if err := validateApprovalMatrixImportIDType(commandOptions{userIDType: "user_id"}, "other", "group"); err != nil {
		t.Fatal(err)
	}
}
