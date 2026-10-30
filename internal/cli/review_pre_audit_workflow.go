package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"cn.qfei/contract-cli/internal/openplatform"
)

func (a *App) runPreAuditReview(ctx context.Context, action string, args []string) error {
	flags := []string{"--run-id", "--interval", "--deadline"}
	if action == "start" {
		flags = append(flags, "--input", "--input-file", "--data")
	}
	parsed, err := parseReviewArgs(args, flags, []string{"--dry-run", "--print-input"})
	if err != nil {
		return err
	}
	interval, deadline, err := reviewDurations(parsed)
	if err != nil {
		return err
	}
	var config map[string]any
	if action == "start" {
		var input struct {
			Config map[string]any `json:"config"`
		}
		if err := readReviewJSON(ctx, parsed, &input); err != nil {
			return err
		}
		config, err = normalizedReviewConfig(input.Config)
		if err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	return a.withPreAuditRun(ctx, parseReviewOptions(parsed), parsed.String("--run-id"), parsed.Bool("--dry-run") || parsed.Bool("--print-input"), func(run *preAuditRun, options commandOptions) error {
		if parsed.Bool("--dry-run") || parsed.Bool("--print-input") {
			return nil
		}
		if action == "resume" {
			return a.resumePreAudit(ctx, options, run, interval)
		}
		switch run.Stage {
		case "awaiting_config", "start_rejected", "review_failed":
			if err := a.startPreAudit(ctx, options, run, config); err != nil {
				return err
			}
		default:
			return fmt.Errorf("当前阶段%s不允许重新发起，请使用review pre-audit resume续查；未知启动结果须人工核实", run.Stage)
		}
		return a.resumePreAudit(ctx, options, run, interval)
	})
}

func (a *App) preAuditDetail(ctx context.Context, options commandOptions, run *preAuditRun) (map[string]any, error) {
	data, err := a.reviewDocument(ctx, options, openplatform.Request{Method: http.MethodGet, Path: preAuditPath + "/" + run.PreAuditID, IdentityPolicy: openplatform.IdentityPolicyUserOnly, OperationKind: openplatform.OperationRead})
	if err != nil {
		return nil, err
	}
	if err := bindPreAuditDetail(run, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (a *App) startPreAudit(ctx context.Context, options commandOptions, run *preAuditRun, config map[string]any) error {
	a.logger.Info("pre-audit review start requested", "run_id", run.RunID, "pre_audit_id", run.PreAuditID)
	if _, err := a.preAuditDetail(ctx, options, run); err != nil {
		return err
	}
	fileID, err := strconv.ParseInt(run.AIFileID, 10, 64)
	if err != nil {
		return fmt.Errorf("原run的AI文件标识无效")
	}
	payload, err := (reviewStartInput{BusinessID: run.PreAuditID, FileID: fileID, FileHash: run.FileHash, Config: config}).payload()
	if err != nil {
		return err
	}
	payload["usageReportContext"] = map[string]string{"reportBusinessCode": reviewUsageReportBusinessCode, "businessId": run.PreAuditID, "biz1": "SMART_PRE_AUDIT"}
	request, err := reviewJSONRequest("/startReview", payload)
	if err != nil {
		return err
	}
	// A crash after this durable marker means the outcome is unknown. Never
	// infer that no task was created from a timeout or a missing response.
	run.Stage = "start_unknown"
	run.TaskID, run.Start, run.Final = "", nil, nil
	if err := a.preAuditStore().save(run); err != nil {
		return err
	}
	response, err := a.doReview(ctx, options, request)
	if err != nil {
		if preAuditStartRejected(response, err) {
			run.Stage = "start_rejected"
			if saveErr := a.preAuditStore().save(run); saveErr != nil {
				return saveErr
			}
			return err
		}
		return fmt.Errorf("启动结果不确定，任务可能已创建；没有taskId时不自动重发，请保留runId/preAuditId人工核实: %w", err)
	}
	data, err := preAuditResponseData(response)
	if err != nil {
		return fmt.Errorf("启动结果不确定，未取得taskId，不自动重发: %w", err)
	}
	taskID, err := reviewResponseID(data, "taskId")
	if err != nil {
		return fmt.Errorf("启动结果不确定，未取得taskId，不自动重发: %w", err)
	}
	run.TaskID = strconv.FormatInt(taskID, 10)
	run.Start = map[string]any{"taskId": run.TaskID}
	run.Stage = "running"
	return a.preAuditStore().save(run)
}

func preAuditStartRejected(response openplatform.Response, requestErr error) bool {
	var uncertain *openplatform.UncertainWriteError
	if errors.As(requestErr, &uncertain) {
		return false
	}
	if response.StatusCode >= 400 && response.StatusCode < 500 {
		return true
	}
	var envelope struct {
		Code json.Number `json:"code"`
	}
	return json.Unmarshal(response.Body, &envelope) == nil && envelope.Code != "" && envelope.Code != "0" && envelope.Code != "200"
}

func (a *App) resumePreAudit(ctx context.Context, options commandOptions, run *preAuditRun, interval time.Duration) error {
	a.logger.Info("pre-audit recovery requested", "run_id", run.RunID, "stage", run.Stage)
	switch run.Stage {
	case "create_pending":
		a.logger.Warn("pre-audit creation outcome unknown; recovery unsupported", "run_id", run.RunID)
		return fmt.Errorf("创建结果未知，暂不支持重试恢复；请保留原run并人工核实，不要重新发起创建")
	case "pre_audit_created":
		if err := a.createPreAudit(ctx, options, run, false); err != nil {
			return err
		}
		return fmt.Errorf("预审及引用已准备，请选择配置后执行review pre-audit start")
	case "running":
		if run.TaskID == "" {
			return fmt.Errorf("原任务ID缺失，不能重新发起")
		}
		final, err := a.waitReviewResult(ctx, options, url.Values{"taskId": {run.TaskID}, "businessId": {run.PreAuditID}}, interval)
		if final != nil {
			run.Final = cleanPreAuditResult(final)
		}
		if err != nil {
			if final != nil && final["status"] == "fail" {
				run.Stage = "review_failed"
			}
			if saveErr := a.preAuditStore().save(run); saveErr != nil {
				return saveErr
			}
			return err
		}
		run.Stage = "review_succeeded"
		if err := a.preAuditStore().save(run); err != nil {
			return err
		}
	case "review_succeeded", "completed":
		// Already have the successful info; only the original CLM URL is read.
	case "start_unknown":
		return fmt.Errorf("启动结果不确定，没有taskId；请人工核实，不自动重发")
	case "upload_unknown", "reference_unknown", "reference_not_ready":
		return fmt.Errorf("上传/引用结果未知或未就绪，请保留run核实，不自动重新上传")
	default:
		return fmt.Errorf("当前阶段%s尚未完成审查；选择分类/配置或核对失败后使用对应命令继续", run.Stage)
	}
	return a.deliverPreAuditLink(ctx, options, run)
}

func (a *App) deliverPreAuditLink(ctx context.Context, options commandOptions, run *preAuditRun) error {
	a.logger.Info("pre-audit result link requested", "run_id", run.RunID, "pre_audit_id", run.PreAuditID, "task_id", run.TaskID)
	// Discard any old/AI navigation fields, including a previous successful URL.
	run.Final = cleanPreAuditResult(run.Final)
	run.Stage = "review_succeeded"
	data, err := a.preAuditDetail(ctx, options, run)
	if err == nil {
		page, _ := data["page_url"].(string)
		err = validatePreAuditPageURL(page, run.PreAuditID)
		if err == nil {
			run.Stage = "completed"
			if err := a.preAuditStore().save(run); err != nil {
				return err
			}
			// Only attach the fresh URL to the completed command output. Resume
			// fetches it again, rather than trusting a cached navigation address.
			run.Final["reviewDetailUrl"] = page
			a.logger.Info("pre-audit review completed", "run_id", run.RunID, "task_id", run.TaskID)
			return nil
		}
	}
	a.logger.Error("pre-audit link delivery failed", "run_id", run.RunID, "task_id", run.TaskID)
	return fmt.Errorf("审查已成功，但预审地址获取失败；已保留明细和IDs，请使用review pre-audit resume仅重取地址: %w", err)
}

func validatePreAuditPageURL(page, preAuditID string) error {
	u, err := url.Parse(page)
	if err != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || strings.Contains(page, "#") || u.Path != "/pre-audit/detail" || u.RawPath != "" {
		return fmt.Errorf("CLM预审page_url无效")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(query["preAuditId"]) != 1 || query.Get("preAuditId") != preAuditID || len(query["from"]) != 1 || query.Get("from") != "cli" {
		return fmt.Errorf("CLM预审page_url定位参数无效")
	}
	return nil
}

// Preserve risk/config data while excluding AI navigation aliases and temporary
// signed download URLs from persisted recovery results and user output.
func cleanPreAuditResult(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		switch strings.ToLower(key) {
		case "url", "reviewdetailurl", "review_detail_url", "detailurl", "detail_url", "resulturl", "result_url", "reporturl", "report_url", "pageurl", "page_url":
			continue
		}
		result[key] = cleanPreAuditValue(item)
	}
	return result
}

func cleanPreAuditValue(value any) any {
	switch item := value.(type) {
	case string:
		if !preAuditSignedURL(item) {
			return item
		}
		return preAuditResultURLPattern.ReplaceAllStringFunc(item, func(candidate string) string {
			if preAuditSignedURL(candidate) {
				return "[临时下载地址已省略]"
			}
			return candidate
		})
	case map[string]any:
		return cleanPreAuditResult(item)
	case []any:
		items := make([]any, 0, len(item))
		for _, child := range item {
			items = append(items, cleanPreAuditValue(child))
		}
		return items
	default:
		return value
	}
}

var preAuditResultURLPattern = regexp.MustCompile(`(?i)https?://[^\s<>"'）)】，。；！？、]+`)

func preAuditSignedURL(text string) bool {
	text = strings.ToLower(text)
	for _, marker := range []string{"x-amz-", "signature=", "x-oss-", "ossaccesskeyid=", "access_token=", "x-tos-", "security-token=", "token="} {
		if strings.Contains(text, marker) && (strings.Contains(text, "https://") || strings.Contains(text, "http://")) {
			return true
		}
	}
	return false
}
