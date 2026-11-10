package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"cn.qfei/contract-cli/internal/openplatform"
)

const (
	reviewPathPrefix      = "/open-apis/contract/v3/review"
	reviewRulesPrefix     = "/open-apis/contract/v1/review-rules"
	reviewRulesChecklists = reviewRulesPrefix + "/review-checklists"
	reviewRulesGroups     = reviewRulesPrefix + "/review-rule-groups"
)

func (a *App) runReview(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing review subcommand")
	}
	switch args[0] {
	case "file":
		return a.runReviewFile(ctx, args[1:])
	case "cooperation":
		return a.runReviewCooperation(ctx, args[1:])
	case "subject":
		return a.runReviewSubject(ctx, args[1:])
	case "task":
		return a.runReviewTask(ctx, args[1:])
	case "checklist":
		return a.runChecklist(ctx, args[1:])
	case "rule":
		return a.runReviewRule(ctx, args[1:])
	case "run":
		return a.runReviewWorkflow(ctx, args[1:])
	case "pre-audit":
		return a.runPreAudit(ctx, args[1:])
	default:
		return fmt.Errorf("unknown review subcommand %q", args[0])
	}
}

func (a *App) runReviewFile(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing review file subcommand")
	}
	switch args[0] {
	case "upload":
		return a.runReviewUpload(ctx, args[1:])
	default:
		return fmt.Errorf("unknown review file subcommand %q", args[0])
	}
}

func (a *App) runChecklist(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing checklist subcommand")
	}
	switch args[0] {
	case "list":
		return a.runReviewRulesList(ctx, args[1:], reviewRulesChecklists, true)
	case "create":
		return a.runReviewRulesJSON(ctx, args[1:], http.MethodPost, reviewRulesChecklists, false)
	case "batch-create":
		return a.runReviewRulesJSON(ctx, args[1:], http.MethodPost, reviewRulesChecklists+"/batch", true)
	case "update":
		return a.runReviewRulesIDJSON(ctx, args[1:], http.MethodPut, reviewRulesChecklists)
	case "batch-update":
		return a.runReviewRulesJSON(ctx, args[1:], http.MethodPut, reviewRulesChecklists+"/batch", true)
	case "delete":
		return a.runReviewRulesDelete(ctx, args[1:], reviewRulesChecklists)
	case "batch-delete":
		return a.runReviewRulesBatchDelete(ctx, args[1:], reviewRulesChecklists)
	default:
		return fmt.Errorf("unknown checklist subcommand %q", args[0])
	}
}

func (a *App) runReviewRule(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing review-rule resource")
	}
	if args[0] == "group" {
		if len(args) < 2 {
			return fmt.Errorf("missing rule group subcommand")
		}
		switch args[1] {
		case "list":
			return a.runReviewRulesList(ctx, args[2:], reviewRulesGroups, false)
		case "create":
			return a.runReviewRulesJSON(ctx, args[2:], http.MethodPost, reviewRulesGroups, false)
		case "update":
			return a.runReviewRulesIDJSON(ctx, args[2:], http.MethodPut, reviewRulesGroups)
		case "delete":
			return a.runReviewRulesDelete(ctx, args[2:], reviewRulesGroups)
		default:
			return fmt.Errorf("unknown rule group subcommand %q", args[1])
		}
	}
	if len(args) == 1 && args[0] == "list" {
		return fmt.Errorf("--group-id is required")
	}
	if args[0] == "list" {
		parsed, err := parseReviewArgs(args[1:], []string{"--group-id", "--sort", "--page-index", "--page-size"}, nil)
		if err != nil {
			return err
		}
		groupID := strings.TrimSpace(parsed.String("--group-id"))
		if groupID == "" {
			return fmt.Errorf("--group-id is required")
		}
		return a.runReviewRulesList(ctx, withoutFlags(args[1:], "--group-id"), reviewRulesGroups+"/"+url.PathEscape(groupID)+"/rules", false)
	}
	parsed, err := parseArgs(args[1:], reviewValueFlags("--group-id", "--rule-id", "--id"), commonBoolFlags("--yes", "--dry-run", "--print-input"))
	if err != nil {
		return err
	}
	groupID := strings.TrimSpace(parsed.String("--group-id"))
	if groupID == "" {
		return fmt.Errorf("--group-id is required")
	}
	collection := reviewRulesGroups + "/" + url.PathEscape(groupID) + "/rules"
	command := args[0]
	commandArgs := withoutFlags(args[1:], "--group-id")
	switch command {
	case "list":
		return a.runReviewRulesList(ctx, commandArgs, collection, false)
	case "create":
		return a.runReviewRulesJSON(ctx, commandArgs, http.MethodPost, collection, false)
	case "batch-create":
		return a.runReviewRulesJSON(ctx, commandArgs, http.MethodPost, collection+"/batch", true)
	case "update":
		return a.runReviewRulesIDJSON(ctx, commandArgs, http.MethodPut, collection, "--rule-id")
	case "batch-update":
		return a.runReviewRulesJSON(ctx, commandArgs, http.MethodPut, collection+"/batch", true)
	case "delete":
		return a.runReviewRulesIDDelete(ctx, commandArgs, collection, "--rule-id")
	case "batch-delete":
		return a.runReviewRulesBatchDelete(ctx, commandArgs, collection)
	default:
		return fmt.Errorf("unknown rule subcommand %q", command)
	}
}

func (a *App) runReviewJSON(ctx context.Context, args []string, method, path string, array bool) error {
	parsed, err := parseArgs(args, reviewValueFlags(), commonBoolFlags("--dry-run", "--print-input"))
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	return a.executeJSONBody(ctx, parsed, method, path, array)
}

func (a *App) runReviewRulesJSON(ctx context.Context, args []string, method, path string, array bool) error {
	return a.runReviewJSON(ctx, args, method, path, array)
}

func (a *App) runReviewRulesList(ctx context.Context, args []string, path string, checklist bool) error {
	values := []string{"--sort", "--page-index", "--page-size"}
	var booleans []string
	if checklist {
		values = append(values, "--name", "--review-stage", "--contract-category", "--start-time", "--end-time", "--create-employee-id", "--update-employee-id")
		booleans = append(booleans, "--enabled")
	}
	parsed, err := parseReviewArgs(args, values, booleans)
	if err != nil {
		return err
	}
	query, err := reviewListValues(parsed, checklist)
	if err != nil {
		return err
	}
	return a.executeReviewResponse(ctx, parseReviewOptions(parsed), openplatform.Request{Method: http.MethodGet, Path: path, Query: query, IdentityPolicy: openplatform.IdentityPolicyUserOnly, OperationKind: openplatform.OperationRead})
}

func reviewResourceIDFlag(flags []string) string {
	if len(flags) > 0 {
		return flags[0]
	}
	return "--id"
}

func (a *App) runReviewRulesIDJSON(ctx context.Context, args []string, method, base string, idFlags ...string) error {
	idFlag := reviewResourceIDFlag(idFlags)
	parsed, err := parseArgs(args, reviewValueFlags(idFlag), commonBoolFlags("--dry-run", "--print-input"))
	if err != nil {
		return err
	}
	id := strings.TrimSpace(parsed.String(idFlag))
	if id == "" {
		return fmt.Errorf("%s is required", idFlag)
	}
	return a.executeJSONBody(ctx, parsed, method, base+"/"+url.PathEscape(id), false, id)
}

func (a *App) runReviewRulesDelete(ctx context.Context, args []string, base string) error {
	return a.runReviewRulesIDDelete(ctx, args, base)
}
func (a *App) runReviewRulesIDDelete(ctx context.Context, args []string, base string, idFlags ...string) error {
	idFlag := reviewResourceIDFlag(idFlags)
	parsed, err := parseReviewArgs(args, []string{idFlag}, []string{"--yes", "--dry-run", "--print-input"})
	if err != nil {
		return err
	}
	id := strings.TrimSpace(parsed.String(idFlag))
	if id == "" {
		return fmt.Errorf("%s is required", idFlag)
	}
	if parsed.Bool("--dry-run") || parsed.Bool("--print-input") {
		key := "id"
		if idFlag == "--rule-id" {
			key = "ruleId"
		}
		return a.renderReviewValue(parseReviewOptions(parsed), map[string]string{key: id})
	}
	if !parsed.Bool("--yes") {
		return fmt.Errorf("delete requires --yes")
	}
	options := parseReviewOptions(parsed)
	return a.executeReviewResponse(ctx, options, openplatform.Request{Method: http.MethodDelete, Path: base + "/" + url.PathEscape(id), IdentityPolicy: openplatform.IdentityPolicyUserOnly})
}

func (a *App) runReviewRulesBatchDelete(ctx context.Context, args []string, base string) error {
	parsed, err := parseArgs(args, reviewValueFlags("--id"), commonBoolFlags("--yes", "--dry-run", "--print-input"))
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	var ids []string
	if parsed.HasValue("--id") {
		for _, flag := range []string{"--input", "--input-file", "--data"} {
			if parsed.HasValue(flag) {
				return fmt.Errorf("--id 与 JSON 输入互斥")
			}
		}
		ids, err = reviewCSV(parsed, "--id")
		if err != nil {
			return err
		}
	} else {
		if err := readReviewJSON(ctx, parsed, &ids); err != nil {
			return fmt.Errorf("batch-delete input must be a JSON string array: %w", err)
		}
	}
	clean := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		value := strings.TrimSpace(id)
		if value == "" {
			return fmt.Errorf("batch-delete ID 不能为空")
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		clean = append(clean, value)
	}
	if len(clean) == 0 {
		return fmt.Errorf("batch-delete requires at least one --id")
	}
	body, err := json.Marshal(map[string]any{"ids": clean})
	if err != nil {
		return err
	}
	options := parseReviewOptions(parsed)
	if parsed.Bool("--dry-run") || parsed.Bool("--print-input") {
		return a.renderOpenPlatformResponse(options, openplatform.Response{Body: body})
	}
	if !parsed.Bool("--yes") {
		return fmt.Errorf("batch-delete requires --yes")
	}
	return a.executeReviewResponse(ctx, options, openplatform.Request{Method: http.MethodDelete, Path: base + "/batch", Body: body, IdentityPolicy: openplatform.IdentityPolicyUserOnly})
}

func reviewWriteBody(ctx context.Context, parsed parsedArgs, array bool, pathID string) ([]byte, error) {
	if len(parsed.positionals) != 0 {
		return nil, fmt.Errorf("unexpected positional arguments")
	}
	count := 0
	for _, flag := range []string{"--input", "--input-file", "--data"} {
		if parsed.HasValue(flag) {
			count++
		}
	}
	if count != 1 {
		return nil, fmt.Errorf("必须且只能提供 --input、--input-file、--data 之一")
	}
	options := parseReviewOptions(parsed)
	body, err := resolveRequiredRawBody(ctx, options)
	if err != nil {
		return nil, err
	}
	objects := []json.RawMessage{body}
	if array {
		if err := json.Unmarshal(body, &objects); err != nil || len(objects) == 0 {
			return nil, fmt.Errorf("command input must be a non-empty JSON object array")
		}
	}
	for _, object := range objects {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(object, &fields); err != nil || fields == nil {
			return nil, fmt.Errorf("command input must contain JSON objects")
		}
		if value, exists := fields["id"]; pathID != "" && exists {
			var bodyID string
			if err := json.Unmarshal(value, &bodyID); err != nil || strings.TrimSpace(bodyID) != pathID {
				return nil, fmt.Errorf("请求体 id 与路径 id 不一致，id 必须是字符串")
			}
		}
	}
	return body, nil
}

func (a *App) executeJSONBody(ctx context.Context, parsed parsedArgs, method, path string, array bool, pathIDs ...string) error {
	pathID := ""
	if len(pathIDs) > 0 {
		pathID = pathIDs[0]
	}
	body, err := reviewWriteBody(ctx, parsed, array, pathID)
	if err != nil {
		return err
	}
	options := parseReviewOptions(parsed)
	if parsed.Bool("--dry-run") || parsed.Bool("--print-input") {
		return a.renderOpenPlatformResponse(options, openplatform.Response{Body: body})
	}
	return a.executeReviewResponse(ctx, options, openplatform.Request{Method: method, Path: path, Body: body, IdentityPolicy: openplatform.IdentityPolicyUserOnly})
}

func withoutFlags(args []string, names ...string) []string {
	blocked := make(map[string]struct{}, len(names))
	for _, name := range names {
		blocked[name] = struct{}{}
	}
	result := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		name, _, inline := strings.Cut(args[index], "=")
		if _, ok := blocked[name]; !ok {
			result = append(result, args[index])
			continue
		}
		if !inline && index+1 < len(args) {
			index++
		}
	}
	return result
}
