package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"cn.qfei/contract-cli/internal/openplatform"
)

type commandFailure struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	TraceID    string `json:"trace_id,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty"`
	Cancelled  bool   `json:"cancelled,omitempty"`
	Details    any    `json:"details,omitempty"`
}

type commandFailureEnvelope struct {
	OK     bool           `json:"ok"`
	Error  commandFailure `json:"error"`
	Notice map[string]any `json:"_notice,omitempty"`
}

// Execute 是进程使用的结果出口；Run 保留库调用方原有的返回错误约定。
// 错误只复用本次 Run 已准备的更新提示，不再发起网络请求。
func (a *App) Execute(ctx context.Context, args []string) int {
	err := a.Run(ctx, args)
	if err == nil {
		return 0
	}
	clean, options, _ := parseCommandLogOptions(args)
	if commandRequestsJSON(clean, options) {
		envelope := commandFailureEnvelope{Error: describeCommandFailure(err), Notice: a.updateNotice}
		encoder := json.NewEncoder(a.stderr)
		encoder.SetEscapeHTML(false)
		_ = encoder.Encode(envelope) // Broken stderr must not change the operation's exit status.
	} else {
		a.writeCommandFailureText(err)
	}
	if errors.Is(err, context.Canceled) {
		return 130
	}
	return 1
}

func (a *App) writeCommandFailureText(err error) {
	_, _ = fmt.Fprintln(a.stderr, "error:", err)
	update, _ := a.updateNotice["update"].(map[string]any)
	message, _ := update["message"].(string)
	if message != "" {
		_, _ = fmt.Fprintln(a.stderr, message)
	}
}

func describeCommandFailure(err error) commandFailure {
	failure := commandFailure{Type: "command", Message: err.Error(), Cancelled: errors.Is(err, context.Canceled)}
	var detailed commandFailureDetailer
	if errors.As(err, &detailed) {
		failure.Details = detailed.commandFailureDetails()
	}
	var traced *openplatform.TraceError
	if errors.As(err, &traced) {
		failure.TraceID = traced.TraceID
	}
	var status *openplatform.HTTPStatusError
	if errors.As(err, &status) {
		failure.Type, failure.HTTPStatus = "http", status.StatusCode
	}
	var uncertain *openplatform.UncertainWriteError
	switch {
	case errors.As(err, &uncertain):
		failure.Type = "uncertain_write"
	case failure.Cancelled:
		failure.Type = "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		failure.Type = "timeout"
	}
	return failure
}
