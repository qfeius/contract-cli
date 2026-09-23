package cli

import (
	"encoding/json"
	"fmt"
	"strings"
)

/*
validateApprovalMatrixDepartmentIDs 校验原子行请求中的部门集合值。
入参 body（[]byte）为 row create/update/search 的 JSON 请求体；返回值为部门 ID 约定错误。
*/
func validateApprovalMatrixDepartmentIDs(body []byte) error {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		// 原子命令继续让服务端处理整体 JSON 格式，避免改变其原有透传行为。
		return nil
	}
	return walkApprovalMatrixDepartmentIDs(value, "$", false)
}

/*
walkApprovalMatrixDepartmentIDs 递归查找 department_collection 字段并校验其 open_department_id 值。
入参 value 为当前节点，path 为节点路径，inDepartmentField 表示当前节点是否位于部门集合中；返回首个部门 ID 校验错误。
*/
func walkApprovalMatrixDepartmentIDs(value any, path string, inDepartmentField bool) error {
	if inDepartmentField {
		if value == nil {
			return nil
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("%s: invalid department_collection", path)
		}
		if _, err := approvalMatrixDepartmentIDs(raw); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		return nil
	}

	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			childPath := path + "." + key
			if key == "department_collection" {
				if err := walkApprovalMatrixDepartmentIDs(child, childPath, true); err != nil {
					return err
				}
				continue
			}
			if err := walkApprovalMatrixDepartmentIDs(child, childPath, false); err != nil {
				return err
			}
		}
	case []any:
		for index, child := range current {
			if err := walkApprovalMatrixDepartmentIDs(child, fmt.Sprintf("%s[%d]", path, index), false); err != nil {
				return err
			}
		}
	}
	return nil
}

/*
validateApprovalMatrixDepartmentID 校验单个审批矩阵部门 ID 是否可供开放接口直接使用。
入参 value 为待校验的部门 ID；返回格式校验错误。
*/
func validateApprovalMatrixDepartmentID(value string) error {
	if !strings.HasPrefix(strings.TrimSpace(value), "od-") {
		return fmt.Errorf("department ID %q must be an open_department_id such as \"od-...\"; sys_department.id is not accepted", value)
	}
	return nil
}
