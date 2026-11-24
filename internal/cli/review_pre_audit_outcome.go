package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"cn.qfei/contract-cli/internal/openplatform"
)

func (a *App) restoreUnsentPreAuditRun(run *preAuditRun, previous preAuditRun, requestErr error) error {
	if err := a.preAuditStore().save(&previous); err != nil {
		a.logger.Error("pre-audit unsent state restoration failed", "run_id", run.RunID, "stage", run.Stage)
		return fmt.Errorf("业务请求未发送，但恢复状态保存失败；保留原run核实，不自动重试: %w", errors.Join(requestErr, err))
	}
	*run = previous
	a.logger.Info("pre-audit unsent state restored", "run_id", run.RunID, "stage", run.Stage)
	return fmt.Errorf("业务请求未发送，已恢复原阶段；修复原因后可使用原run继续: %w", requestErr)
}

func preAuditStartRejected(response openplatform.Response, requestErr error) bool {
	// Only the business response itself is proof. A nested HTTP error may
	// belong to a later token refresh after a business attempt was dispatched.
	if traced, ok := requestErr.(*openplatform.TraceError); ok {
		requestErr = traced.Cause
	}
	statusErr, ok := requestErr.(*openplatform.HTTPStatusError)
	if !ok || statusErr.StatusCode != response.StatusCode || !bytes.Equal(statusErr.Body, response.Body) {
		return false
	}
	var envelope struct {
		Detail      string          `json:"detail"`
		Data        json.RawMessage `json:"data"`
		Message     string          `json:"msg"`
		Code        json.Number     `json:"code"`
		TaskID      json.RawMessage `json:"taskId"`
		TaskIDSnake json.RawMessage `json:"task_id"`
	}
	if json.Unmarshal(response.Body, &envelope) != nil || len(envelope.TaskID) > 0 || len(envelope.TaskIDSnake) > 0 {
		return false
	}
	// AI handle_validate_error rejects schema input before entering start_review.
	// Its code/data shape is distinct from business errors after task creation.
	if response.StatusCode == http.StatusUnprocessableEntity && envelope.Code == "422" && strings.HasPrefix(envelope.Message, "参数错误") {
		var issues []string
		return json.Unmarshal(envelope.Data, &issues) == nil && len(issues) > 0
	}
	if response.StatusCode != http.StatusUnauthorized && response.StatusCode != http.StatusForbidden {
		return false
	}
	var data map[string]json.RawMessage
	if len(envelope.Data) > 0 && json.Unmarshal(envelope.Data, &data) != nil {
		return false
	}
	if envelope.Code == "401000001" && response.StatusCode == http.StatusUnauthorized && len(data) == 0 {
		return true // AI auth_error_handler: NO_VALID_AUTH, before task dispatch.
	}
	if envelope.Code != "" && string(envelope.Code) != fmt.Sprint(response.StatusCode) {
		return false
	}
	// Auth dependency rejection precedes task creation. Do not reuse CLM's
	// business-code list or infer non-creation from arbitrary 4xx responses.
	if (envelope.Detail != "" || (envelope.Code != "" && envelope.Message != "")) && len(data) == 0 {
		return true
	}
	return len(data) == 1 && response.Headers.Get("X-Qfei-Open-Platform-Auth-Error") == "token_expired" && bytes.Equal(bytes.TrimSpace(data["error_type"]), []byte(`"token_expired"`))
}

func preAuditCandidateTask(body []byte) string {
	var envelope map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if decoder.Decode(&envelope) != nil {
		return ""
	}
	data, _ := envelope["data"].(map[string]any)
	for _, object := range []map[string]any{data, envelope} {
		if id, err := reviewResponseID(object, "taskId"); err == nil {
			return fmt.Sprint(id)
		}
	}
	return ""
}
