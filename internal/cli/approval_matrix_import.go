package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"cn.qfei/contract-cli/internal/openplatform"
	"cn.qfei/contract-cli/internal/output"
	"github.com/gofrs/flock"
)

const (
	approvalMatrixImportPlanVersion = 3
	approvalMatrixRowCountLimit     = 2000
)

type approvalMatrixImportInput struct {
	Rows []approvalMatrixImportInputRow `json:"rows"`
}

type approvalMatrixImportInputRow struct {
	Operation string                     `json:"operation,omitempty"`
	RowID     string                     `json:"row_id,omitempty"`
	Cells     map[string]json.RawMessage `json:"cells"`
}

type approvalMatrixColumn struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Type            int    `json:"type"`
	CellContentType string `json:"table_cell_content_type"`
}

type approvalMatrixCellContent struct {
	Bool                 *bool        `json:"bool"`
	Collection           *[]string    `json:"collection"`
	DepartmentCollection *[]string    `json:"department_collection"`
	EmployeeCollection   *[]int64     `json:"employee_collection"`
	RoleCollection       *[]string    `json:"role_collection"`
	Number               *json.Number `json:"number"`
	String               *string      `json:"string"`
}

type approvalMatrixTableCell struct {
	TableCellContent     approvalMatrixCellContent `json:"table_cell_content"`
	TableCellContentType string                    `json:"table_cell_content_type"`
	TableColumnID        string                    `json:"table_column_id"`
}

type approvalMatrixRowRequest struct {
	TableCells []approvalMatrixTableCell `json:"table_cells"`
}

type approvalMatrixImportOperation struct {
	Index             int                      `json:"index"`
	Operation         string                   `json:"operation"`
	RowID             string                   `json:"row_id,omitempty"`
	Request           approvalMatrixRowRequest `json:"request"`
	Status            string                   `json:"status"`
	Response          json.RawMessage          `json:"response,omitempty"`
	ExpectedRowDigest string                   `json:"expected_row_digest,omitempty"`
	Error             string                   `json:"error,omitempty"`
}

type approvalMatrixImportPlan struct {
	// 仅识别已停用的旧保护计划，执行时拒绝降级；新计划不设置该字段。
	Guarded            bool                            `json:"guarded,omitempty"`
	Version            int                             `json:"version"`
	PlanID             string                          `json:"plan_id"`
	CreatedAt          time.Time                       `json:"created_at"`
	UpdatedAt          time.Time                       `json:"updated_at"`
	Profile            string                          `json:"profile"`
	ProductID          string                          `json:"product_id"`
	GroupID            string                          `json:"group_id"`
	TableID            string                          `json:"table_id"`
	Status             string                          `json:"status"`
	Operations         []approvalMatrixImportOperation `json:"operations"`
	ColumnSnapshot     string                          `json:"column_snapshot"`
	RowSnapshot        map[string]string               `json:"row_snapshot"`
	ContextFingerprint string                          `json:"context_fingerprint"`
	Reason             string                          `json:"reason,omitempty"`
}

type approvalMatrixValidationIssue struct {
	Row          int      `json:"row"`
	Column       string   `json:"column,omitempty"`
	Code         string   `json:"code"`
	Message      string   `json:"message"`
	ExpectedType string   `json:"expected_type,omitempty"`
	Candidates   []string `json:"candidates,omitempty"`
}

type approvalMatrixImportSummary struct {
	Total      int `json:"total"`
	Pending    int `json:"pending"`
	Succeeded  int `json:"succeeded"`
	Failed     int `json:"failed"`
	Uncertain  int `json:"uncertain"`
	Unverified int `json:"unverified"`
}

/*
runApprovalMatrixImport 分发批量导入的计划、执行、暂停与只读核验命令。
入参 ctx（context.Context）为命令执行上下文，args（[]string）为 rule table import 后的参数。
返回值为命令解析或执行错误；成功输出结构化状态并返回 nil。
*/
func (a *App) runApprovalMatrixImport(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing rule table import subcommand")
	}
	switch args[0] {
	case "plan":
		return a.runApprovalMatrixImportPlan(ctx, args[1:])
	case "apply":
		return a.runApprovalMatrixImportApply(ctx, args[1:])
	case "get", "cancel", "pause", "resume", "verify":
		return a.runApprovalMatrixPlanControl(ctx, args[0], args[1:])
	default:
		return fmt.Errorf("unknown rule table import subcommand %q", args[0])
	}
}

/*
runApprovalMatrixImportPlan 读取矩阵列头和全量规则行，校验容量与输入并持久化可确认的计划，全程不写矩阵数据。
入参 ctx（context.Context）为命令执行上下文，args（[]string）为 plan 命令参数。
返回值为解析、鉴权、矩阵读取或计划保存错误；业务校验问题通过 status=needs_input 输出并返回 nil。
*/
func (a *App) runApprovalMatrixImportPlan(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--product-id", "--group-id", "--table-id"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli rule table import plan --product-id <id> --group-id <id> [--table-id <id>] --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	if options.raw {
		return fmt.Errorf("rule table import plan does not support --raw because it returns structured state")
	}
	body, err := resolveRequiredRawBody(options)
	if err != nil {
		return err
	}
	input, err := decodeApprovalMatrixImportInput(body)
	if err != nil {
		return err
	}
	productID, groupID, err := requiredRuleProductGroup(parsed)
	if err != nil {
		return err
	}
	tableID := strings.TrimSpace(parsed.String("--table-id"))
	if tableID == "" {
		return a.renderApprovalMatrixTableCandidates(ctx, options, productID, groupID)
	}

	// 列头用于类型映射；行快照用于容量计算和后续自动失效校验。
	columnsPath := ruleTablePath(productID, groupID, tableID) + "/table_columns/column_headers"
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, columnsPath, openplatform.IdentityPolicyAny)
	if err != nil {
		return err
	}
	response, err := client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodGet,
		Path:           columnsPath,
		IdentityPolicy: openplatform.IdentityPolicyAny,
	})
	if err != nil {
		return err
	}
	columns, err := decodeApprovalMatrixColumns(response.Body)
	if err != nil {
		return err
	}
	operations, issues := buildApprovalMatrixImportOperations(input, columns)
	if len(issues) > 0 {
		return a.renderApprovalMatrixImportValue(options, map[string]any{
			"status":            "needs_input",
			"profile":           requestContext.Profile.Name,
			"product_id":        productID,
			"group_id":          groupID,
			"table_id":          tableID,
			"issues":            issues,
			"available_columns": columns,
		})
	}
	rows, err := readApprovalMatrixRows(ctx, client, requestContext, ruleTablePath(productID, groupID, tableID))
	if err != nil {
		return fmt.Errorf("read approval matrix rows for import plan: %w", err)
	}
	rowSnapshot, err := approvalMatrixRowSnapshot(rows)
	if err != nil {
		return err
	}
	// 后端以当前行数小于 2000 作为新增条件；默认空白行也占一个名额。
	creates := 0
	for _, operation := range operations {
		if operation.Operation == "create" {
			creates++
		} else if _, exists := rowSnapshot[operation.RowID]; !exists {
			issues = append(issues, approvalMatrixValidationIssue{Row: operation.Index, Code: "missing_row", Message: "update row_id does not exist in the target matrix"})
		}
	}
	if len(rowSnapshot)+creates > approvalMatrixRowCountLimit {
		issues = append(issues, approvalMatrixValidationIssue{Code: "row_limit_exceeded", Message: fmt.Sprintf("current %d rows plus %d creates exceeds maximum %d", len(rowSnapshot), creates, approvalMatrixRowCountLimit)})
	}
	if len(issues) > 0 {
		return a.renderApprovalMatrixImportValue(options, map[string]any{
			"status": "needs_input", "profile": requestContext.Profile.Name,
			"product_id": productID, "group_id": groupID, "table_id": tableID,
			"current_rows": len(rowSnapshot), "planned_creates": creates, "row_limit": approvalMatrixRowCountLimit,
			"issues": issues,
		})
	}

	planID, err := newApprovalMatrixImportPlanID()
	if err != nil {
		return err
	}
	now := a.now().UTC()
	plan := approvalMatrixImportPlan{
		Version:            approvalMatrixImportPlanVersion,
		PlanID:             planID,
		CreatedAt:          now,
		UpdatedAt:          now,
		Profile:            requestContext.Profile.Name,
		ProductID:          productID,
		GroupID:            groupID,
		TableID:            tableID,
		Status:             "needs_confirmation",
		Operations:         operations,
		ColumnSnapshot:     approvalMatrixFingerprint(columns),
		RowSnapshot:        rowSnapshot,
		ContextFingerprint: approvalMatrixContextFingerprint(requestContext),
	}
	if err := a.saveApprovalMatrixImportPlan(plan); err != nil {
		return err
	}
	return a.renderApprovalMatrixImportValue(options, approvalMatrixImportPlanOutput(plan))
}

/*
renderApprovalMatrixTableCandidates 在计划缺少 table_id 时读取已有矩阵列表，返回可供 Agent 追问的候选项。
入参 ctx（context.Context）为命令执行上下文，options（commandOptions）为公共命令选项，productID 与 groupID（string）为规则表父级定位参数。
返回值为鉴权、列表读取、响应解析或结构化输出错误。
*/
func (a *App) renderApprovalMatrixTableCandidates(ctx context.Context, options commandOptions, productID string, groupID string) error {
	path := ruleTableBasePath(productID, groupID)
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, path, openplatform.IdentityPolicyAny)
	if err != nil {
		return err
	}
	response, err := client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodGet,
		Path:           path,
		Query:          url.Values{"page_size": {"100"}},
		IdentityPolicy: openplatform.IdentityPolicyAny,
	})
	if err != nil {
		return err
	}
	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(response.Body, &envelope); err != nil {
		return fmt.Errorf("decode approval matrix list: %w", err)
	}
	if envelope.Code != 0 {
		return fmt.Errorf("query approval matrix list failed with code %d: %s", envelope.Code, envelope.Msg)
	}
	var candidates any = map[string]any{}
	if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		decoder := json.NewDecoder(bytes.NewReader(envelope.Data))
		decoder.UseNumber()
		if err := decoder.Decode(&candidates); err != nil {
			return fmt.Errorf("decode approval matrix candidates: %w", err)
		}
	}
	return a.renderApprovalMatrixImportValue(options, map[string]any{
		"status":         "needs_input",
		"profile":        requestContext.Profile.Name,
		"product_id":     productID,
		"group_id":       groupID,
		"missing_fields": []string{"table_id"},
		"candidates":     candidates,
	})
}

/*
runApprovalMatrixImportApply 校验全量行快照后逐行写入并回读，重试时跳过已验证成功行并阻断不确定或未验证行。
入参 ctx（context.Context）为命令执行上下文，args（[]string）为 apply 命令参数。
返回值为参数、计划读取、鉴权或状态保存错误；逐行结果先通过结构化输出保留，批量失败或计划失效时额外返回错误。
*/
func (a *App) runApprovalMatrixImportApply(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--plan-id", "--rows", "--batch-size"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli rule table import apply --plan-id <id> [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "rule table import apply"); err != nil {
		return err
	}
	if options.raw {
		return fmt.Errorf("rule table import apply does not support --raw because it returns structured state")
	}
	planID, err := requiredParsedValue(parsed, "--plan-id")
	if err != nil {
		return err
	}
	planPath, err := a.approvalMatrixImportPlanPath(planID)
	if err != nil {
		return err
	}
	// 同一主机上的并发 apply 只能有一个进入，避免两个进程同时读取到 pending 后重复创建。
	planLock := flock.New(planPath + ".lock")
	locked, err := planLock.TryLock()
	if err != nil {
		return fmt.Errorf("acquire approval matrix import plan lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("approval matrix import plan %q is already being applied", planID)
	}
	defer func() {
		if unlockErr := planLock.Unlock(); unlockErr != nil {
			a.logger.Error("release approval matrix import plan lock failed", "plan_id", planID, "error", unlockErr.Error())
		}
	}()
	plan, err := a.loadApprovalMatrixImportPlan(planID)
	if err != nil {
		return err
	}
	selected, limit, err := approvalMatrixExecutionSelection(parsed.String("--rows"), parsed.String("--batch-size"), len(plan.Operations))
	if err != nil {
		return err
	}
	if strings.TrimSpace(options.profileName) == "" {
		options.profileName = plan.Profile
	} else if options.profileName != plan.Profile {
		return fmt.Errorf("approval matrix import plan %q belongs to profile %q", plan.PlanID, plan.Profile)
	}
	if plan.Status == "success" || plan.Status == "cancelled" {
		return a.renderApprovalMatrixImportApplyResult(options, plan)
	}

	// 停用回执接口后，旧保护计划必须核验已成功的行并重新确认，不静默切换写入模式。
	if plan.Guarded {
		plan.Status = "invalidated"
		plan.Reason = "guarded import has been removed; reconcile existing rows and confirm a new plan"
		if err := a.saveApprovalMatrixImportPlan(plan); err != nil {
			return err
		}
		return a.renderApprovalMatrixImportApplyResult(options, plan)
	}

	// 整个批次复用同一请求上下文，并严格串行写入，避免并发放大开放平台限流。
	basePath := ruleTablePath(plan.ProductID, plan.GroupID, plan.TableID) + "/table_rows"
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, basePath, openplatform.IdentityPolicyAny)
	if err != nil {
		return err
	}
	if plan.Status == "invalidated" {
		return a.renderApprovalMatrixImportApplyResult(options, plan)
	}
	// 旧计划缺少行快照时必须重新确认；app 凭证轮换保持兼容。
	if plan.Version != approvalMatrixImportPlanVersion || plan.RowSnapshot == nil || plan.ContextFingerprint != approvalMatrixContextFingerprint(requestContext) || a.now().Sub(plan.CreatedAt) > 24*time.Hour {
		plan.Status = "invalidated"
		plan.Reason = "plan expired or identity/credential/application/environment changed; generate a new plan"
		if err := a.saveApprovalMatrixImportPlan(plan); err != nil {
			return err
		}
		return a.renderApprovalMatrixImportApplyResult(options, plan)
	}
	checkPath := ruleTablePath(plan.ProductID, plan.GroupID, plan.TableID) + "/table_columns/column_headers"
	columnsResponse, err := client.Do(ctx, requestContext, openplatform.Request{Method: http.MethodGet, Path: checkPath, IdentityPolicy: openplatform.IdentityPolicyAny})
	if err != nil {
		return err
	}
	columns, err := decodeApprovalMatrixColumns(columnsResponse.Body)
	if err != nil {
		return err
	}
	if approvalMatrixFingerprint(columns) != plan.ColumnSnapshot {
		plan.Status = "invalidated"
		plan.Reason = "matrix snapshot changed; generate a new plan"
		if err := a.saveApprovalMatrixImportPlan(plan); err != nil {
			return err
		}
		return a.renderApprovalMatrixImportApplyResult(options, plan)
	}
	// 请求前落盘的 running 表示上次进程可能在服务端成功后退出，恢复时先核验。
	for i := range plan.Operations {
		if plan.Operations[i].Status == "running" {
			plan.Operations[i].Status = "uncertain"
		}
		if plan.Operations[i].Status == "uncertain" {
			plan.Status = "needs_input"
			plan.Reason = "query and reconcile uncertain rows before continuing"
			if err := a.saveApprovalMatrixImportPlan(plan); err != nil {
				return err
			}
			return a.renderApprovalMatrixImportApplyResult(options, plan)
		}
		if plan.Operations[i].Status == "unverified" {
			plan.Status = "needs_verification"
			plan.Reason = "run import verify to check the previous write before continuing"
			if err := a.saveApprovalMatrixImportPlan(plan); err != nil {
				return err
			}
			return a.renderApprovalMatrixImportApplyResult(options, plan)
		}
	}
	// 每次启动或恢复 apply 都对整张矩阵做行快照比较，外部增删改会使确认失效。
	currentRows, err := readApprovalMatrixRows(ctx, client, requestContext, ruleTablePath(plan.ProductID, plan.GroupID, plan.TableID))
	if err != nil {
		return fmt.Errorf("read approval matrix rows before import apply: %w", err)
	}
	currentSnapshot, err := approvalMatrixRowSnapshot(currentRows)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(currentSnapshot, plan.RowSnapshot) {
		plan.Status = "invalidated"
		plan.Reason = "matrix rows changed since the plan was confirmed; generate a new plan"
		if err := a.saveApprovalMatrixImportPlan(plan); err != nil {
			return err
		}
		return a.renderApprovalMatrixImportApplyResult(options, plan)
	}
	plan.Reason = ""
	executed := 0
	for index := range plan.Operations {
		operation := &plan.Operations[index]
		if len(selected) > 0 && !selected[operation.Index] {
			continue
		}
		if limit > 0 && executed >= limit {
			break
		}
		if operation.Status == "success" || operation.Status == "uncertain" {
			continue
		}
		pauseRequested, pauseErr := approvalMatrixPauseRequested(planPath)
		if pauseErr != nil {
			return pauseErr
		}
		if pauseRequested {
			plan.Status = "paused"
			plan.Reason = "pause requested; no further row writes started"
			break
		}
		if operation.Operation == "create" && len(plan.RowSnapshot) >= approvalMatrixRowCountLimit {
			plan.Status = "invalidated"
			plan.Reason = "approval matrix row limit reached; generate a new plan"
			break
		}
		if operation.Operation == "update" {
			// 目标行在批次运行中仍可能变化；单行写入前核对并预先保存期望的完整行摘要。
			before, readErr := readApprovalMatrixImportRow(ctx, client, requestContext, basePath, operation.RowID)
			if readErr != nil {
				return fmt.Errorf("read row %q before update: %w", operation.RowID, readErr)
			}
			beforeCells, readErr := approvalMatrixCanonicalRowCells(before)
			if readErr != nil {
				return readErr
			}
			if approvalMatrixFingerprint(beforeCells) != plan.RowSnapshot[operation.RowID] {
				plan.Status = "invalidated"
				plan.Reason = fmt.Sprintf("row %q changed before update; generate a new plan", operation.RowID)
				break
			}
			requestedCells, readErr := approvalMatrixCanonicalRequestCells(operation.Request)
			if readErr != nil {
				return readErr
			}
			for columnID, value := range requestedCells {
				beforeCells[columnID] = value
			}
			operation.ExpectedRowDigest = approvalMatrixFingerprint(beforeCells)
		}
		requestBody, marshalErr := json.Marshal(operation.Request)
		if marshalErr != nil {
			return fmt.Errorf("encode approval matrix import row %d: %w", operation.Index, marshalErr)
		}
		method := http.MethodPost
		path := basePath
		if operation.Operation == "update" {
			method = http.MethodPut
			path += "/" + escapePathSegment(operation.RowID)
		}
		if ctx.Err() != nil {
			break
		}
		// 暂停和启动下一条写入共用短锁，明确并发请求的先后顺序。
		started, startErr := a.startApprovalMatrixImportWrite(planPath, &plan, operation)
		if startErr != nil {
			return startErr
		}
		if !started {
			plan.Status = "paused"
			plan.Reason = "pause requested; no further row writes started"
			break
		}
		response, requestErr := client.Do(ctx, requestContext, openplatform.Request{
			Method:         method,
			Path:           path,
			Body:           requestBody,
			IdentityPolicy: openplatform.IdentityPolicyAny,
		})
		executed++
		operation.Response = nil
		operation.Error = ""
		if requestErr != nil {
			operation.Status = "failed"
			operation.Error = requestErr.Error()
			var uncertainErr *openplatform.UncertainWriteError
			if errors.As(requestErr, &uncertainErr) {
				// 写请求结果不确定时禁止自动重试，避免重复创建矩阵行。
				operation.Status = "uncertain"
			}
		} else if apiErr, unknown := approvalMatrixAPIError(response.Body); apiErr != "" {
			operation.Status = "failed"
			if unknown {
				operation.Status = "uncertain"
			}
			operation.Error = apiErr
			operation.Response = append(json.RawMessage(nil), response.Body...)
			var business struct {
				Code int `json:"code"`
			}
			if !unknown && json.Unmarshal(response.Body, &business) == nil && business.Code == 20302 {
				plan.Status = "invalidated"
				plan.Reason = "approval matrix row limit reached on server; generate a new plan"
			}
		} else {
			operation.Response = append(json.RawMessage(nil), response.Body...)
			if operation.Operation == "create" {
				var result struct {
					Data struct {
						TableRowID string `json:"table_row_id"`
					} `json:"data"`
				}
				if json.Unmarshal(response.Body, &result) != nil || strings.TrimSpace(result.Data.TableRowID) == "" {
					operation.Status = "uncertain"
					operation.Error = "create response has no row id; query and reconcile before retrying"
				} else {
					operation.RowID = result.Data.TableRowID
				}
			}
			if operation.Status != "uncertain" {
				// 成功响应先落盘为 unverified，进程中断或回读失败时不会重放写请求。
				operation.Status = "unverified"
				plan.UpdatedAt = a.now().UTC()
				if err := a.saveApprovalMatrixImportPlan(plan); err != nil {
					return err
				}
				observedDigest, verifyErr := verifyApprovalMatrixImportOperation(ctx, client, requestContext, basePath, *operation)
				if verifyErr != nil {
					operation.Error = verifyErr.Error()
					plan.Status = "needs_verification"
					plan.Reason = "readback failed or differed; run import verify before continuing"
				} else {
					operation.Status = "success"
					plan.RowSnapshot[operation.RowID] = observedDigest
				}
			}
		}
		plan.UpdatedAt = a.now().UTC()
		if err := a.saveApprovalMatrixImportPlan(plan); err != nil {
			return err
		}
		if operation.Status == "uncertain" || operation.Status == "unverified" || plan.Status == "invalidated" {
			break
		}
		var statusErr *openplatform.HTTPStatusError
		if errors.As(requestErr, &statusErr) {
			switch statusErr.StatusCode {
			case 401, 403, 404, 409, 412, 429:
				plan.Reason = fmt.Sprintf("HTTP %d: resolve authentication, conflict or rate limit before continuing", statusErr.StatusCode)
				if statusErr.StatusCode == 404 || statusErr.StatusCode == 409 || statusErr.StatusCode == 412 {
					plan.Status = "invalidated"
				}
			}
			if plan.Reason != "" {
				break
			}
		}
	}

	summary := summarizeApprovalMatrixImportPlan(plan)
	switch {
	case plan.Status == "invalidated":
		// 冲突或目标消失时保留终止状态，禁止再次执行旧计划。
	case summary.Unverified > 0:
		plan.Status = "needs_verification"
	case summary.Succeeded == summary.Total:
		plan.Status = "success"
	case summary.Uncertain > 0:
		plan.Status = "needs_input"
	case summary.Pending > 0:
		plan.Status = "paused"
	case summary.Succeeded > 0:
		plan.Status = "partial_success"
	default:
		plan.Status = "failed"
	}
	plan.UpdatedAt = a.now().UTC()
	if err := a.saveApprovalMatrixImportPlan(plan); err != nil {
		return err
	}
	return a.renderApprovalMatrixImportApplyResult(options, plan)
}

/*
startApprovalMatrixImportWrite 原子检查暂停意图并将当前行记为 running，之后调用方才发送写请求。
入参 planPath（string）为计划路径，plan（*approvalMatrixImportPlan）为持锁计划，operation（*approvalMatrixImportOperation）为当前行；返回 bool 表示是否可开始，error 为锁或落盘错误。
*/
func (a *App) startApprovalMatrixImportWrite(planPath string, plan *approvalMatrixImportPlan, operation *approvalMatrixImportOperation) (bool, error) {
	controlLock := flock.New(planPath + ".pause.lock")
	if err := controlLock.Lock(); err != nil {
		return false, fmt.Errorf("lock approval matrix pause control: %w", err)
	}
	defer controlLock.Unlock()
	requested, err := approvalMatrixPauseRequested(planPath)
	if err != nil || requested {
		return false, err
	}
	operation.Status = "running"
	if err := a.saveApprovalMatrixImportPlan(*plan); err != nil {
		return false, err
	}
	return true, nil
}

/*
approvalMatrixExecutionSelection 解析用户确认的计划行范围和每次执行上限。
入参 rows/batch（string）为可选参数，total（int）为总行数；返回行集合、上限和 error。
*/
func approvalMatrixExecutionSelection(rows, batch string, total int) (map[int]bool, int, error) {
	selected := map[int]bool{}
	limit := 0
	if batch != "" {
		var err error
		limit, err = strconv.Atoi(batch)
		if err != nil || limit < 1 {
			return nil, 0, fmt.Errorf("--batch-size must be positive")
		}
	}
	if rows != "" {
		for _, item := range strings.Split(rows, ",") {
			index, err := strconv.Atoi(strings.TrimSpace(item))
			if err != nil || index < 1 || index > total {
				return nil, 0, fmt.Errorf("--rows must contain plan row indexes from 1 to %d", total)
			}
			selected[index] = true
		}
	}
	return selected, limit, nil
}

/*
runApprovalMatrixPlanControl 读取、暂停、恢复、核验或取消本地计划；状态写入与执行共用计划锁。
入参 ctx（context.Context）为调用上下文，action（string）为控制动作，args（[]string）为参数；返回 error 为解析、状态保存或回读错误。
*/
func (a *App) runApprovalMatrixPlanControl(ctx context.Context, action string, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--plan-id"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("use --plan-id to select a plan")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "import "+action); err != nil {
		return err
	}
	if options.raw {
		return fmt.Errorf("import %s requires structured output", action)
	}
	id, err := requiredParsedValue(parsed, "--plan-id")
	if err != nil {
		return err
	}
	path, err := a.approvalMatrixImportPlanPath(id)
	if err != nil {
		return err
	}
	lock := flock.New(path + ".lock")
	if action == "cancel" || action == "resume" || action == "verify" {
		locked, err := lock.TryLock()
		if err != nil {
			return err
		}
		if !locked {
			return fmt.Errorf("plan is running; wait for the current row request to finish before %s", action)
		}
		defer lock.Unlock()
	}
	plan, err := a.loadApprovalMatrixImportPlan(id)
	if err != nil {
		return err
	}
	if options.profileName != "" && options.profileName != plan.Profile {
		return fmt.Errorf("plan belongs to profile %q", plan.Profile)
	}
	switch action {
	case "pause":
		if plan.Status != "success" && plan.Status != "cancelled" && plan.Status != "invalidated" {
			if err := requestApprovalMatrixPause(path); err != nil {
				return err
			}
		}
	case "resume":
		controlLock := flock.New(path + ".pause.lock")
		if err := controlLock.Lock(); err != nil {
			return err
		}
		removeErr := os.Remove(path + ".pause")
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			_ = controlLock.Unlock()
			return fmt.Errorf("clear approval matrix pause request: %w", removeErr)
		}
		// 只有仍有待执行行的已暂停计划才能转为可续跑；核验和不确定状态仍由原流程阻断。
		summary := summarizeApprovalMatrixImportPlan(plan)
		if plan.Status == "paused" && summary.Pending > 0 && summary.Uncertain == 0 && summary.Unverified == 0 {
			plan.Status = "ready"
			plan.Reason = ""
			plan.UpdatedAt = a.now().UTC()
			if err := a.saveApprovalMatrixImportPlan(plan); err != nil {
				_ = controlLock.Unlock()
				return err
			}
		}
		if err := controlLock.Unlock(); err != nil {
			return err
		}
	case "verify":
		if plan.Version != approvalMatrixImportPlanVersion || plan.RowSnapshot == nil {
			return fmt.Errorf("import plan lacks a compatible row snapshot; generate a new plan")
		}
		if summarizeApprovalMatrixImportPlan(plan).Unverified == 0 {
			break
		}
		if options.profileName == "" {
			options.profileName = plan.Profile
		}
		rowsPath := ruleTablePath(plan.ProductID, plan.GroupID, plan.TableID) + "/table_rows"
		client, rc, err := a.openPlatformClientAndContextForOptions(options, rowsPath, openplatform.IdentityPolicyAny)
		if err != nil {
			return err
		}
		if plan.ContextFingerprint != approvalMatrixContextFingerprint(rc) {
			return fmt.Errorf("import verification identity or environment changed; query rows with the original context")
		}
		terminalStatus := plan.Status
		for i := range plan.Operations {
			operation := &plan.Operations[i]
			if operation.Status != "unverified" {
				continue
			}
			digest, verifyErr := verifyApprovalMatrixImportOperation(ctx, client, rc, rowsPath, *operation)
			if verifyErr != nil {
				operation.Error = verifyErr.Error()
				continue
			}
			operation.Status = "success"
			operation.Error = ""
			plan.RowSnapshot[operation.RowID] = digest
		}
		summary := summarizeApprovalMatrixImportPlan(plan)
		switch {
		case terminalStatus == "cancelled" || terminalStatus == "invalidated":
			// 已终止计划可只读核验历史副作用，但核验不会恢复后续写入权限。
		case summary.Unverified > 0:
			plan.Status = "needs_verification"
			plan.Reason = "readback still failed or differed; inspect the affected rows"
		case summary.Succeeded == summary.Total:
			plan.Status = "success"
			plan.Reason = ""
		default:
			plan.Status = "paused"
			plan.Reason = "verification completed; remaining rows require apply"
		}
		plan.UpdatedAt = a.now().UTC()
		if err := a.saveApprovalMatrixImportPlan(plan); err != nil {
			return err
		}
	case "cancel":
		if plan.Status != "success" {
			plan.Status = "cancelled"
			plan.UpdatedAt = a.now().UTC()
			if err := a.saveApprovalMatrixImportPlan(plan); err != nil {
				return err
			}
		}
	}
	pauseRequested, err := approvalMatrixPauseRequested(path)
	if err != nil {
		return err
	}
	result := approvalMatrixImportPlanOutput(plan)
	result["pause_requested"] = pauseRequested
	return a.renderApprovalMatrixImportValue(options, result)
}

/*
requestApprovalMatrixPause 在计划锁之外持久记录暂停意图，使正在执行的批次可在当前行后停止。
入参 planPath（string）为已校验的计划路径；返回 error 为文件创建错误。
*/
func requestApprovalMatrixPause(planPath string) error {
	controlLock := flock.New(planPath + ".pause.lock")
	if err := controlLock.Lock(); err != nil {
		return fmt.Errorf("lock approval matrix pause control: %w", err)
	}
	defer controlLock.Unlock()
	file, err := os.OpenFile(planPath+".pause", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("request approval matrix pause: %w", err)
	}
	return file.Close()
}

/*
approvalMatrixPauseRequested 查询持久暂停标记，执行端在每条行写请求前调用。
入参 planPath（string）为已校验的计划路径；返回 bool 表示是否请求暂停，error 为文件查询错误。
*/
func approvalMatrixPauseRequested(planPath string) (bool, error) {
	_, err := os.Stat(planPath + ".pause")
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read approval matrix pause request: %w", err)
	}
	return true, nil
}

/*
decodeApprovalMatrixImportInput 解析 Agent 提供的批量行输入，并保留单元格原始 JSON 类型用于后续严格校验。
入参 body（[]byte）为 --input-file 或 --data 读取出的 JSON 字节。
返回值为批量输入和解析错误；rows 为空时返回明确错误。
*/
func decodeApprovalMatrixImportInput(body []byte) (approvalMatrixImportInput, error) {
	var input approvalMatrixImportInput
	if !json.Valid(body) {
		return approvalMatrixImportInput{}, fmt.Errorf("decode rule table import input: invalid JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return approvalMatrixImportInput{}, fmt.Errorf("decode rule table import input: %w", err)
	}
	if len(input.Rows) == 0 {
		return approvalMatrixImportInput{}, fmt.Errorf("rule table import input must include at least one row")
	}
	return input, nil
}

/*
decodeApprovalMatrixColumns 解析开放平台列头响应并校验业务响应码。
入参 body（[]byte）为列头接口的 JSON 响应体。
返回值为当前矩阵的列定义和响应格式或业务错误。
*/
func decodeApprovalMatrixColumns(body []byte) ([]approvalMatrixColumn, error) {
	var response struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Columns []approvalMatrixColumn `json:"columns_headers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode approval matrix column headers: %w", err)
	}
	if response.Code != 0 {
		return nil, fmt.Errorf("query approval matrix column headers failed with code %d: %s", response.Code, response.Msg)
	}
	if len(response.Data.Columns) == 0 {
		return nil, fmt.Errorf("approval matrix has no writable column headers")
	}
	return response.Data.Columns, nil
}

/*
buildApprovalMatrixImportOperations 将按列名或列 ID 表达的输入转换成开放平台逐行请求，并汇总所有可追问问题。
入参 input（approvalMatrixImportInput）为用户确认前的批量输入，columns（[]approvalMatrixColumn）为目标矩阵当前列头。
返回值为规范化操作列表和校验问题列表；存在问题时调用方不保存计划。
*/
func buildApprovalMatrixImportOperations(input approvalMatrixImportInput, columns []approvalMatrixColumn) ([]approvalMatrixImportOperation, []approvalMatrixValidationIssue) {
	columnsByID := make(map[string]approvalMatrixColumn, len(columns))
	columnsByName := make(map[string][]approvalMatrixColumn, len(columns))
	for _, column := range columns {
		columnsByID[column.ID] = column
		columnsByName[column.Name] = append(columnsByName[column.Name], column)
	}

	operations := make([]approvalMatrixImportOperation, 0, len(input.Rows))
	issues := make([]approvalMatrixValidationIssue, 0)
	for rowIndex, row := range input.Rows {
		operationName := strings.ToLower(strings.TrimSpace(row.Operation))
		if operationName == "" {
			operationName = "create"
		}
		if operationName != "create" && operationName != "update" {
			issues = append(issues, approvalMatrixValidationIssue{Row: rowIndex + 1, Code: "invalid_operation", Message: "operation must be create or update"})
			continue
		}
		rowID := strings.TrimSpace(row.RowID)
		if operationName == "update" && rowID == "" {
			issues = append(issues, approvalMatrixValidationIssue{Row: rowIndex + 1, Code: "missing_row_id", Message: "update operation requires row_id"})
			continue
		}
		if len(row.Cells) == 0 {
			issues = append(issues, approvalMatrixValidationIssue{Row: rowIndex + 1, Code: "missing_cells", Message: "row must include at least one cell"})
			continue
		}

		columnKeys := make([]string, 0, len(row.Cells))
		for key := range row.Cells {
			columnKeys = append(columnKeys, key)
		}
		sort.Strings(columnKeys)
		cells := make([]approvalMatrixTableCell, 0, len(columnKeys))
		rowIssueCount := len(issues)
		for _, columnKey := range columnKeys {
			column, candidates, ok := resolveApprovalMatrixColumn(columnKey, columnsByID, columnsByName)
			if !ok {
				code := "unknown_column"
				message := "column does not exist in the target matrix"
				if len(candidates) > 1 {
					code = "ambiguous_column"
					message = "column name matches multiple columns; use a column id"
				}
				issues = append(issues, approvalMatrixValidationIssue{Row: rowIndex + 1, Column: columnKey, Code: code, Message: message, Candidates: candidates})
				continue
			}
			cell, err := newApprovalMatrixTableCell(column, row.Cells[columnKey])
			if err != nil {
				issues = append(issues, approvalMatrixValidationIssue{Row: rowIndex + 1, Column: columnKey, Code: "type_mismatch", Message: err.Error(), ExpectedType: column.CellContentType})
				continue
			}
			cells = append(cells, cell)
		}
		if len(issues) != rowIssueCount {
			continue
		}
		operations = append(operations, approvalMatrixImportOperation{
			Index:     rowIndex + 1,
			Operation: operationName,
			RowID:     rowID,
			Request:   approvalMatrixRowRequest{TableCells: cells},
			Status:    "pending",
		})
	}
	return operations, issues
}

/*
resolveApprovalMatrixColumn 优先按列 ID、其次按精确列名解析单元格目标，并识别重名列。
入参 key（string）为输入中的列键，columnsByID（map[string]approvalMatrixColumn）与 columnsByName（map[string][]approvalMatrixColumn）为当前列头索引。
返回值依次为命中的列、重名候选 ID、是否唯一命中。
*/
func resolveApprovalMatrixColumn(key string, columnsByID map[string]approvalMatrixColumn, columnsByName map[string][]approvalMatrixColumn) (approvalMatrixColumn, []string, bool) {
	key = strings.TrimSpace(key)
	if column, ok := columnsByID[key]; ok {
		return column, nil, true
	}
	matches := columnsByName[key]
	if len(matches) == 1 {
		return matches[0], nil, true
	}
	candidates := make([]string, 0, len(matches))
	for _, match := range matches {
		candidates = append(candidates, match.ID)
	}
	return approvalMatrixColumn{}, candidates, false
}

/*
newApprovalMatrixTableCell 按列头声明的内容类型校验输入值，并生成开放平台要求的单元格对象。
入参 column（approvalMatrixColumn）为目标列定义，rawValue（json.RawMessage）为用户输入的原始 JSON 值。
返回值为规范化单元格和类型不匹配错误。
*/
func newApprovalMatrixTableCell(column approvalMatrixColumn, rawValue json.RawMessage) (approvalMatrixTableCell, error) {
	contentType := strings.ToUpper(strings.TrimSpace(column.CellContentType))
	cell := approvalMatrixTableCell{
		TableCellContentType: contentType,
		TableColumnID:        column.ID,
	}
	if bytes.Equal(bytes.TrimSpace(rawValue), []byte("null")) {
		// 显式 null 清空这个单元格；未提供的列保持不变，与后端局部行更新一致。
		return cell, nil
	}
	switch contentType {
	case "STRING":
		var value string
		if err := json.Unmarshal(rawValue, &value); err != nil {
			return approvalMatrixTableCell{}, fmt.Errorf("value must be a string")
		}
		cell.TableCellContent.String = &value
	case "NUMBER":
		decoder := json.NewDecoder(bytes.NewReader(rawValue))
		decoder.UseNumber()
		var value json.Number
		if err := decoder.Decode(&value); err != nil {
			return approvalMatrixTableCell{}, fmt.Errorf("value must be a number")
		}
		if !validMatrixDecimal(value.String()) {
			return approvalMatrixTableCell{}, fmt.Errorf("value must be a decimal with at most 30 integer digits and 8 fractional digits")
		}
		cell.TableCellContent.Number = &value
	case "BOOLEAN":
		var value bool
		if err := json.Unmarshal(rawValue, &value); err != nil {
			return approvalMatrixTableCell{}, fmt.Errorf("value must be true or false")
		}
		cell.TableCellContent.Bool = &value
	case "EMPLOYEE_COLLECTION":
		ids, err := approvalMatrixResourceIDs(rawValue)
		if err != nil {
			return cell, err
		}
		cell.TableCellContent.EmployeeCollection = &ids
	case "DEPARTMENT_COLLECTION":
		ids, err := approvalMatrixDepartmentIDs(rawValue)
		if err != nil {
			return cell, err
		}
		cell.TableCellContent.DepartmentCollection = &ids
	case "COLLECTION", "ROLE_COLLECTION":
		var value []string
		if err := json.Unmarshal(rawValue, &value); err != nil || value == nil {
			return approvalMatrixTableCell{}, fmt.Errorf("value must be an array of ids or collection values")
		}
		for _, item := range value {
			if strings.TrimSpace(item) == "" {
				return approvalMatrixTableCell{}, fmt.Errorf("collection values must not be empty")
			}
		}
		switch contentType {
		case "COLLECTION":
			cell.TableCellContent.Collection = &value
		case "ROLE_COLLECTION":
			cell.TableCellContent.RoleCollection = &value
		}
	default:
		return approvalMatrixTableCell{}, fmt.Errorf("unsupported column content type %q", column.CellContentType)
	}
	return cell, nil
}

/*
newApprovalMatrixImportPlanID 生成不可预测的确认令牌，兼作本地计划文件名。
入参为空。
返回值为 32 位十六进制计划 ID 和随机源错误。
*/
func newApprovalMatrixImportPlanID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate approval matrix import plan id: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

/*
saveApprovalMatrixImportPlan 以仅当前用户可读写的权限原子保存计划，支持执行过程中逐行落盘。
入参 plan（approvalMatrixImportPlan）为需要持久化的完整计划状态。
返回值为目录创建、编码或文件替换错误。
*/
func (a *App) saveApprovalMatrixImportPlan(plan approvalMatrixImportPlan) error {
	dir := filepath.Join(filepath.Dir(a.store.Path()), "approval-matrix-plans")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create approval matrix plan directory: %w", err)
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("encode approval matrix import plan: %w", err)
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(dir, ".approval-matrix-plan-*")
	if err != nil {
		return fmt.Errorf("create temporary approval matrix import plan: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set approval matrix import plan permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write approval matrix import plan: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close approval matrix import plan: %w", err)
	}
	path := filepath.Join(dir, plan.PlanID+".json")
	if err := os.Rename(temporaryPath, path); err != nil {
		// Windows 的 os.Rename 不覆盖已存在文件；持有 plan lock 时使用兼容回退。
		if runtime.GOOS != "windows" {
			return fmt.Errorf("replace approval matrix import plan: %w", err)
		}
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("remove previous approval matrix import plan: %w", removeErr)
		}
		if retryErr := os.Rename(temporaryPath, path); retryErr != nil {
			return fmt.Errorf("replace approval matrix import plan: %w", retryErr)
		}
	}
	return nil
}

/*
approvalMatrixImportPlanPath 校验计划 ID 并构造限定在 CLI 配置目录内的计划文件路径。
入参 planID（string）为 plan 命令生成的十六进制确认令牌。
返回值为计划绝对路径和格式校验错误。
*/
func (a *App) approvalMatrixImportPlanPath(planID string) (string, error) {
	planID = strings.TrimSpace(planID)
	if len(planID) != 32 {
		return "", fmt.Errorf("invalid approval matrix import plan id %q", planID)
	}
	if _, err := hex.DecodeString(planID); err != nil {
		return "", fmt.Errorf("invalid approval matrix import plan id %q", planID)
	}
	return filepath.Join(filepath.Dir(a.store.Path()), "approval-matrix-plans", planID+".json"), nil
}

/*
loadApprovalMatrixImportPlan 从本地状态目录读取计划，并校验 ID、版本和文件内容的一致性。
入参 planID（string）为 plan 命令输出的确认令牌。
返回值为完整计划和路径、读取、解码或兼容性错误。
*/
func (a *App) loadApprovalMatrixImportPlan(planID string) (approvalMatrixImportPlan, error) {
	path, err := a.approvalMatrixImportPlanPath(planID)
	if err != nil {
		return approvalMatrixImportPlan{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return approvalMatrixImportPlan{}, fmt.Errorf("approval matrix import plan %q not found", planID)
		}
		return approvalMatrixImportPlan{}, fmt.Errorf("read approval matrix import plan: %w", err)
	}
	var plan approvalMatrixImportPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return approvalMatrixImportPlan{}, fmt.Errorf("decode approval matrix import plan: %w", err)
	}
	if plan.PlanID != planID {
		return approvalMatrixImportPlan{}, fmt.Errorf("approval matrix import plan %q is invalid or incompatible", planID)
	}
	return plan, nil
}

/*
approvalMatrixAPIError 将 HTTP 200 响应中的开放平台业务错误转换成逐行失败原因。
入参 body（[]byte）为创建或更新行接口的 JSON 响应体。
返回 string 为错误文本（空表示成功），bool 表示响应缺少明确结果，需要先查询核验。
*/
func approvalMatrixAPIError(body []byte) (string, bool) {
	var response struct {
		Code *int   `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Sprintf("decode approval matrix row response: %v", err), true
	}
	if response.Code == nil {
		return "approval matrix response missing code; query before retrying", true
	}
	if *response.Code == 0 {
		return "", false
	}
	return fmt.Sprintf("open platform returned code %d: %s", *response.Code, response.Msg), false
}

type approvalMatrixCanonicalCell struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

/*
approvalMatrixCanonicalCellValue 提取单元格当前类型对应的值，统一空值、数字精度和无序部门集合。
入参 raw（map[string]any）为请求或回读中的单元格；返回 approvalMatrixCanonicalCell 为比较值，error 为残缺或未知类型错误。
*/
func approvalMatrixCanonicalCellValue(raw map[string]any) (approvalMatrixCanonicalCell, error) {
	cellType, ok := raw["table_cell_content_type"].(string)
	if !ok || cellType == "" {
		return approvalMatrixCanonicalCell{}, fmt.Errorf("missing cell content type")
	}
	content, ok := raw["table_cell_content"].(map[string]any)
	if !ok {
		return approvalMatrixCanonicalCell{}, fmt.Errorf("missing cell content")
	}
	key := strings.ToLower(cellType)
	switch cellType {
	case "STRING", "NUMBER", "BOOLEAN", "COLLECTION", "EMPLOYEE_COLLECTION", "DEPARTMENT_COLLECTION", "ROLE_COLLECTION":
	default:
		return approvalMatrixCanonicalCell{}, fmt.Errorf("unsupported cell content type %q", cellType)
	}
	value := content[key]
	// 服务端将空字符串和空集合回读为 null；两者都表示已清空单元格。
	if text, ok := value.(string); ok && text == "" {
		value = nil
	}
	if items, ok := value.([]any); ok && len(items) == 0 {
		value = nil
	}
	if value != nil {
		wrapper := map[string]any{key: value}
		matrixExactValues(wrapper)
		value = wrapper[key]
	}
	return approvalMatrixCanonicalCell{Type: cellType, Value: value}, nil
}

/*
approvalMatrixCanonicalRowCells 将已规范化的行转换为可精确比较的列值集合。
入参 row（map[string]any）为分页或单行查询结果；返回 map[string]approvalMatrixCanonicalCell 为列 ID 到值的映射，error 为异常单元格结构。
*/
func approvalMatrixCanonicalRowCells(row map[string]any) (map[string]approvalMatrixCanonicalCell, error) {
	rawCells, ok := row["table_cells"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing row cells")
	}
	cells := make(map[string]approvalMatrixCanonicalCell, len(rawCells))
	for columnID, raw := range rawCells {
		cell, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid cell %q", columnID)
		}
		value, err := approvalMatrixCanonicalCellValue(cell)
		if err != nil {
			return nil, fmt.Errorf("column %q: %w", columnID, err)
		}
		cells[columnID] = value
	}
	return cells, nil
}

/*
approvalMatrixCanonicalRequestCells 将计划中的写入请求转换为回读比较值，保留 JSON 数字精度。
入参 request（approvalMatrixRowRequest）为计划行请求；返回 map[string]approvalMatrixCanonicalCell 为列值，error 为编码或重复列错误。
*/
func approvalMatrixCanonicalRequestCells(request approvalMatrixRowRequest) (map[string]approvalMatrixCanonicalCell, error) {
	encoded, err := json.Marshal(request.TableCells)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var rawCells []map[string]any
	if err := decoder.Decode(&rawCells); err != nil {
		return nil, err
	}
	cells := make(map[string]approvalMatrixCanonicalCell, len(rawCells))
	for _, raw := range rawCells {
		columnID, ok := raw["table_column_id"].(string)
		if !ok || columnID == "" {
			return nil, fmt.Errorf("request cell missing column id")
		}
		if _, exists := cells[columnID]; exists {
			return nil, fmt.Errorf("request has duplicate column %q", columnID)
		}
		value, err := approvalMatrixCanonicalCellValue(raw)
		if err != nil {
			return nil, fmt.Errorf("column %q: %w", columnID, err)
		}
		cells[columnID] = value
	}
	return cells, nil
}

/*
approvalMatrixRowSnapshot 保存整张矩阵中每行的列值摘要，供导入计划自动失效判断。
入参 rows（map[string]any）为完整分页读取结果；返回 map[string]string 为行 ID 到摘要的映射，error 为不完整行错误。
*/
func approvalMatrixRowSnapshot(rows map[string]any) (map[string]string, error) {
	snapshot := make(map[string]string, len(rows))
	for rowID, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid row %q", rowID)
		}
		cells, err := approvalMatrixCanonicalRowCells(row)
		if err != nil {
			return nil, fmt.Errorf("row %q: %w", rowID, err)
		}
		snapshot[rowID] = approvalMatrixFingerprint(cells)
	}
	return snapshot, nil
}

/*
readApprovalMatrixImportRow 按 ID 回读单行并复用发布路径的严格行结构校验。
入参 ctx（context.Context）为请求上下文，client（*openplatform.Client）和 rc（openplatform.RequestContext）为调用身份，rowsPath/rowID（string）定位规则行；返回 map[string]any 为规范化行，error 为读取或结构错误。
*/
func readApprovalMatrixImportRow(ctx context.Context, client *openplatform.Client, rc openplatform.RequestContext, rowsPath, rowID string) (map[string]any, error) {
	response, err := client.Do(ctx, rc, openplatform.Request{Method: http.MethodGet, Path: rowsPath + "/" + escapePathSegment(rowID), IdentityPolicy: openplatform.IdentityPolicyAny})
	if err != nil {
		return nil, err
	}
	data, err := matrixPublishJSON(response.Body)
	if err != nil {
		return nil, err
	}
	row, err := matrixNormalizedRow(data["table_row"])
	if err != nil {
		return nil, err
	}
	if row["id"] != rowID {
		return nil, fmt.Errorf("row readback returned id %v, expected %q", row["id"], rowID)
	}
	return row, nil
}

/*
verifyApprovalMatrixImportOperation 回读已写入行并比对计划值，返回可推进快照的实际行摘要。
入参 ctx（context.Context）、client（*openplatform.Client）、rc（openplatform.RequestContext）及 rowsPath（string）用于查询，operation（approvalMatrixImportOperation）为待核验的计划行；返回 string 为回读行摘要，error 为查询或内容不一致。
*/
func verifyApprovalMatrixImportOperation(ctx context.Context, client *openplatform.Client, rc openplatform.RequestContext, rowsPath string, operation approvalMatrixImportOperation) (string, error) {
	if operation.RowID == "" {
		return "", fmt.Errorf("write response has no row id; reconcile before retrying")
	}
	row, err := readApprovalMatrixImportRow(ctx, client, rc, rowsPath, operation.RowID)
	if err != nil {
		return "", err
	}
	actual, err := approvalMatrixCanonicalRowCells(row)
	if err != nil {
		return "", err
	}
	expected, err := approvalMatrixCanonicalRequestCells(operation.Request)
	if err != nil {
		return "", err
	}
	for columnID, value := range expected {
		if !reflect.DeepEqual(actual[columnID], value) {
			return "", fmt.Errorf("readback mismatch for column %q", columnID)
		}
	}
	digest := approvalMatrixFingerprint(actual)
	if operation.Operation == "update" && digest != operation.ExpectedRowDigest {
		return "", fmt.Errorf("readback changed other cells of row %q", operation.RowID)
	}
	return digest, nil
}

/*
summarizeApprovalMatrixImportPlan 汇总计划行状态，区分已验证成功、未验证写入和结果不确定写入。
入参 plan（approvalMatrixImportPlan）为当前计划快照。
返回 approvalMatrixImportSummary 为各状态数量。
*/
func summarizeApprovalMatrixImportPlan(plan approvalMatrixImportPlan) approvalMatrixImportSummary {
	summary := approvalMatrixImportSummary{Total: len(plan.Operations)}
	for _, operation := range plan.Operations {
		switch operation.Status {
		case "success":
			summary.Succeeded++
		case "failed":
			summary.Failed++
		case "uncertain":
			summary.Uncertain++
		case "unverified":
			summary.Unverified++
		default:
			summary.Pending++
		}
	}
	return summary
}

/*
approvalMatrixImportPlanOutput 构造稳定的结构化输出，在计划和执行阶段共享同一状态模型。
入参 plan（approvalMatrixImportPlan）为需要展示的计划快照。
返回值为包含状态、定位信息、摘要和逐行结果的 JSON 对象。
*/
func approvalMatrixImportPlanOutput(plan approvalMatrixImportPlan) map[string]any {
	return map[string]any{
		"status":     plan.Status,
		"plan_id":    plan.PlanID,
		"profile":    plan.Profile,
		"product_id": plan.ProductID,
		"group_id":   plan.GroupID,
		"table_id":   plan.TableID,
		"created_at": plan.CreatedAt,
		"updated_at": plan.UpdatedAt,
		"summary":    summarizeApprovalMatrixImportPlan(plan),
		"operations": plan.Operations,
		"reason":     plan.Reason,
	}
}

/*
renderApprovalMatrixImportApplyResult 输出批量 apply 的完整结果，并将失败状态映射为命令错误。
入参 options（commandOptions）为公共输出选项，plan（approvalMatrixImportPlan）为已持久化的执行状态。
返回值为输出错误，或用于产生非零进程退出码的批量状态错误。
*/
func (a *App) renderApprovalMatrixImportApplyResult(options commandOptions, plan approvalMatrixImportPlan) error {
	// 先写出逐行状态和原因，确保调用方即使收到非零退出码也能拿到可恢复的执行明细。
	if err := a.renderApprovalMatrixImportValue(options, approvalMatrixImportPlanOutput(plan)); err != nil {
		return err
	}

	switch plan.Status {
	case "partial_success", "failed":
		summary := summarizeApprovalMatrixImportPlan(plan)
		return fmt.Errorf("approval matrix import apply returned %s: %d of %d rows failed", plan.Status, summary.Failed, summary.Total)
	case "invalidated":
		reason := strings.TrimSpace(plan.Reason)
		if reason == "" {
			reason = "generate a new plan before applying"
		}
		return fmt.Errorf("approval matrix import apply returned invalidated: %s", reason)
	default:
		return nil
	}
}

/*
approvalMatrixResourceIDs 将人员外部 ID 转为后端 List<Long>，避免浮点精度损失。
入参 raw（json.RawMessage）为数字或十进制字符串数组；返回 []int64 和校验 error。
*/
func approvalMatrixResourceIDs(raw json.RawMessage) ([]int64, error) {
	var items []json.RawMessage
	if json.Unmarshal(raw, &items) != nil || items == nil {
		return nil, fmt.Errorf("resource ids must be an array of positive 64-bit integer IDs")
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		value := string(item)
		if len(item) > 0 && item[0] == '"' {
			if err := json.Unmarshal(item, &value); err != nil {
				return nil, err
			}
		}
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("resource id %q must be a positive 64-bit integer external ID", value)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

/*
approvalMatrixDepartmentIDs 校验并保留部门 open_department_id，避免把 sys_department.id 当成规则值。
入参 raw（json.RawMessage）为 open_department_id 字符串数组；返回 []string 和校验 error。
*/
func approvalMatrixDepartmentIDs(raw json.RawMessage) ([]string, error) {
	var items []json.RawMessage
	if json.Unmarshal(raw, &items) != nil || items == nil {
		return nil, fmt.Errorf("department ids must be an array of open_department_id strings such as \"od-...\"")
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		var id string
		if err := json.Unmarshal(item, &id); err != nil {
			return nil, fmt.Errorf("department ID %s must be an open_department_id such as \"od-...\"; sys_department.id is not accepted", strings.TrimSpace(string(item)))
		}
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, fmt.Errorf("department ID must not be empty")
		}
		if err := validateApprovalMatrixDepartmentID(id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

/*
approvalMatrixFingerprint 计算稳定 JSON 摘要，仅保存摘要避免持久化鉴权信息。
入参 value（any）为可编码的数据，返回 string 为 SHA-256 十六进制摘要。
*/
func approvalMatrixFingerprint(value any) string {
	data, _ := json.Marshal(value)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

/*
approvalMatrixContextFingerprint 绑定身份、实际应用和环境；app 保留旧指纹，user 额外绑定凭证摘要（轮换后需重新确认计划）。
入参 rc（openplatform.RequestContext）为解析后的上下文；返回 string 为指纹。
*/
func approvalMatrixContextFingerprint(rc openplatform.RequestContext) string {
	values := []string{rc.BaseURL, rc.Profile.Environment, rc.Profile.Identities.App.AppID, rc.Profile.Resource}
	if rc.Identity == "user" {
		// 仅存不可逆摘要，避免 user/app 或不同用户间复用已确认的写入计划。
		values = append(values, "user", rc.AccessToken)
	}
	return approvalMatrixFingerprint(values)
}

/*
renderApprovalMatrixImportValue 使用项目既有输出器渲染批量导入状态，保持 json、yaml、table 与更新提示行为一致。
入参 options（commandOptions）为公共命令输出选项，value（any）为待渲染的结构化值。
返回值为格式校验或写出错误。
*/
func (a *App) renderApprovalMatrixImportValue(options commandOptions, value any) error {
	// 先统一转成 JSON 树，确保嵌套 struct 在 yaml/table 输出下也沿用现有 renderer 语义。
	normalized, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode approval matrix import output: %w", err)
	}
	return output.NewRenderer(a.stdout).WithNotice(a.updateNotice).Render(options.output, normalized)
}
