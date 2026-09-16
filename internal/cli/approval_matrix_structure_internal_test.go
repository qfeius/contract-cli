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
