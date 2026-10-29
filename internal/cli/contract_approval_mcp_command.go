package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"cn.qfei/contract-cli/internal/openplatform"
	contractsvc "cn.qfei/contract-cli/internal/openplatform/contract"
)

const (
	approvalTaskTypeTodo   = 0
	approvalTaskTypeDone   = 1
	approvalTaskTypeNotice = 2
	approvalTaskPageMin    = 1
	approvalTaskPageMax    = 100
)

func (a *App) runContractApprovalComment(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract approval comment subcommand")
	}
	switch args[0] {
	case "list":
		return a.runContractApprovalCommentList(ctx, args[1:])
	case "create":
		return a.runContractApprovalCommentCreate(ctx, args[1:])
	default:
		return fmt.Errorf("unknown contract approval comment subcommand %q", args[0])
	}
}

func (a *App) runContractApprovalCommentList(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, commonValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli contract approval comment list <process-instance-id> [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "contract approval comment list"); err != nil {
		return err
	}

	processInstanceID := strings.TrimSpace(parsed.positionals[0])
	a.logger.Info("list contract approval comments", "process_instance_id", processInstanceID)
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractMCPPathPrefix+"/process_instances/"+processInstanceID+"/comments", openplatform.IdentityPolicyUserOnly)
	if err != nil {
		return err
	}
	response, err := contractsvc.NewService(client).ListProcessComments(ctx, requestContext, processInstanceID)
	if err != nil {
		a.logger.Error("list contract approval comments failed", "process_instance_id", processInstanceID, "error", err.Error())
		return err
	}
	return a.renderContractMCPResponse(options, response)
}

func (a *App) runContractApprovalCommentCreate(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, commonValueFlags("--mention-id-type"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli contract approval comment create <process-instance-id> --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	body, err := resolveRequiredRawBody(ctx, options)
	if err != nil {
		return err
	}

	processInstanceID := strings.TrimSpace(parsed.positionals[0])
	a.logger.Info("create contract approval comment", "process_instance_id", processInstanceID)
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractMCPPathPrefix+"/process_instances/"+processInstanceID+"/comments", openplatform.IdentityPolicyUserOnly)
	if err != nil {
		return err
	}
	response, err := contractsvc.NewService(client).CreateProcessComment(ctx, requestContext, processInstanceID, contractsvc.CreateProcessCommentInput{
		MentionIDType: parsed.String("--mention-id-type"),
		Body:          body,
	})
	if err != nil {
		a.logger.Error("create contract approval comment failed", "process_instance_id", processInstanceID, "error", err.Error())
		return err
	}
	return a.renderContractMCPResponse(options, response)
}

func (a *App) runContractApprovalTask(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract approval task subcommand")
	}
	switch args[0] {
	case "list":
		return a.runContractApprovalTaskList(ctx, args[1:])
	case "approve", "reject":
		return a.runContractApprovalTaskAction(ctx, args[0], args[1:])
	default:
		return fmt.Errorf("unknown contract approval task subcommand %q", args[0])
	}
}

func (a *App) runContractApprovalTaskList(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, commonValueFlags("--query", "--task-type", "--page-index", "--page-size"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli contract approval task list [flags]")
	}
	options := parseCommandOptions(parsed)
	bodyObject, err := resolveJSONObjectBody(ctx, options, false)
	if err != nil {
		return err
	}
	if parsed.HasValue("--query") {
		bodyObject["query"] = parsed.String("--query")
	}
	if parsed.HasValue("--task-type") {
		taskTypeCode, err := parseApprovalTaskType(parsed.String("--task-type"))
		if err != nil {
			return err
		}
		bodyObject["task_type_code"] = taskTypeCode
	}
	if parsed.HasValue("--page-index") {
		pageIndex, err := parsed.Int("--page-index")
		if err != nil {
			return err
		}
		if pageIndex < 0 {
			return fmt.Errorf("--page-index must be zero or greater")
		}
		bodyObject["page_index"] = pageIndex
	}
	if parsed.HasValue("--page-size") {
		pageSize, err := parsed.Int("--page-size")
		if err != nil {
			return err
		}
		if pageSize < approvalTaskPageMin || pageSize > approvalTaskPageMax {
			return fmt.Errorf("--page-size must be between 1 and 100")
		}
		bodyObject["page_size"] = pageSize
	}
	body, err := marshalJSONObject(bodyObject)
	if err != nil {
		return err
	}

	a.logger.Info("list personal contract approval tasks")
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractMCPPathPrefix+"/tasks", openplatform.IdentityPolicyUserOnly)
	if err != nil {
		return err
	}
	response, err := contractsvc.NewService(client).ListPersonalTasks(ctx, requestContext, body)
	if err != nil {
		a.logger.Error("list personal contract approval tasks failed", "error", err.Error())
		return err
	}
	return a.renderContractMCPResponse(options, response)
}

func parseApprovalTaskType(value string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "todo":
		return approvalTaskTypeTodo, nil
	case "done":
		return approvalTaskTypeDone, nil
	case "notice":
		return approvalTaskTypeNotice, nil
	default:
		return 0, fmt.Errorf("--task-type must be one of todo, done, notice")
	}
}

func (a *App) runContractApprovalTaskAction(ctx context.Context, action string, args []string) error {
	parsed, err := parseArgs(args, commonValueFlags("--comment", "--file-id"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli contract approval task %s <task-instance-id> [flags]", action)
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "contract approval task "+action); err != nil {
		return err
	}
	comment := parsed.String("--comment")
	if action == "reject" && strings.TrimSpace(comment) == "" {
		return fmt.Errorf("--comment is required for reject")
	}
	fileIDs, err := validateApprovalFileIDs(parsed.Strings("--file-id"))
	if err != nil {
		return err
	}
	bodyObject := map[string]any{"action": action}
	if comment != "" {
		bodyObject["comment"] = comment
	}
	if len(fileIDs) > 0 {
		bodyObject["file_ids"] = fileIDs
	}
	body, err := marshalJSONObject(bodyObject)
	if err != nil {
		return err
	}

	taskInstanceID := strings.TrimSpace(parsed.positionals[0])
	a.logger.Info("run contract approval task action", "task_instance_id", taskInstanceID, "action", action)
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractMCPPathPrefix+"/tasks/"+taskInstanceID+"/approval", openplatform.IdentityPolicyUserOnly)
	if err != nil {
		return err
	}
	response, err := contractsvc.NewService(client).ProcessApprovalTask(ctx, requestContext, taskInstanceID, body)
	if err != nil {
		a.logger.Error("run contract approval task action failed", "task_instance_id", taskInstanceID, "action", action, "error", err.Error())
		return err
	}
	return a.renderContractMCPResponse(options, response)
}

func validateApprovalFileIDs(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		parsed, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil || parsed <= 0 {
			return nil, fmt.Errorf("--file-id must be a positive integer within signed 64-bit range")
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result, nil
}
