package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

/*
validateApprovalMatrixDepartmentIDs 校验原子行请求中的部门集合值。
入参 body（[]byte）为 row create/update/search 的 JSON 请求体，departmentIDType（string）为请求模式；返回值为部门 ID 约定错误。
*/
func validateApprovalMatrixDepartmentIDs(body []byte, departmentIDType string) error {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		// 原子命令继续让服务端处理整体 JSON 格式，避免改变其原有透传行为。
		return nil
	}
	return walkApprovalMatrixDepartmentIDs(value, "$", false, departmentIDType)
}

/*
walkApprovalMatrixDepartmentIDs 递归查找 department_collection 字段并按模式校验 ID。
入参 value 为当前节点，path 为节点路径，inDepartmentField 表示是否位于部门集合中，departmentIDType 为请求模式；返回首个部门 ID 校验错误。
*/
func walkApprovalMatrixDepartmentIDs(value any, path string, inDepartmentField bool, departmentIDType string) error {
	if inDepartmentField {
		if value == nil {
			return nil
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("%s: invalid department_collection", path)
		}
		if _, err := approvalMatrixDepartmentIDs(raw, departmentIDType); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		return nil
	}

	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			childPath := path + "." + key
			if key == "department_collection" {
				if err := walkApprovalMatrixDepartmentIDs(child, childPath, true, departmentIDType); err != nil {
					return err
				}
				continue
			}
			if err := walkApprovalMatrixDepartmentIDs(child, childPath, false, departmentIDType); err != nil {
				return err
			}
		}
	case []any:
		for index, child := range current {
			if err := walkApprovalMatrixDepartmentIDs(child, fmt.Sprintf("%s[%d]", path, index), false, departmentIDType); err != nil {
				return err
			}
		}
	}
	return nil
}

/*
validateApprovalMatrixDepartmentID 校验单个审批矩阵部门 ID 是否符合所选模式。
入参 value（string）为待校验的部门 ID，departmentIDType（string）为请求模式；返回格式校验错误。
*/
func validateApprovalMatrixDepartmentID(value, departmentIDType string) error {
	switch departmentIDType {
	case "", "tenant_department_id":
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return fmt.Errorf("department ID %q must be a positive 64-bit SaaS id", value)
		}
	case "open_department_id":
		if !strings.HasPrefix(value, "od-") && !strings.HasPrefix(value, "od_") {
			return fmt.Errorf("department ID %q must be an open_department_id such as \"od-...\"", value)
		}
	case "department_id":
		// 旧矩阵类型继续由网关按组织目录解析，不改变原有外部 ID 约定。
	default:
		return fmt.Errorf("unsupported department_id_type %q", departmentIDType)
	}
	return nil
}
