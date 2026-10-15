package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
	contractsvc "cn.qfei/contract-cli/internal/openplatform/contract"
	"cn.qfei/contract-cli/internal/output"
)

const contractMCPPathPrefix = "/open-apis/contract/v1/mcp"
const contractOpenAPIPathPrefix = "/open-apis/contract/v1"
const maxContractUploadFileBytes int64 = 200 * 1024 * 1024

func (a *App) runContract(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract subcommand")
	}

	switch args[0] {
	case "search":
		return a.runContractSearch(ctx, args[1:])
	case "search-fields":
		return a.runContractSearchFields(ctx, args[1:])
	case "search-v2":
		return a.runContractSearchV2(ctx, args[1:])
	case "summary":
		return a.runContractSummary(ctx, args[1:])
	case "get":
		return a.runContractGet(ctx, args[1:])
	case "sync-user-groups":
		return a.runContractSyncUserGroups(ctx, args[1:])
	case "text":
		return a.runContractText(ctx, args[1:])
	case "create":
		return a.runContractCreate(ctx, args[1:])
	case "submit":
		return a.runContractSubmit(ctx, args[1:])
	case "resubmit":
		return a.runContractResubmit(ctx, args[1:])
	case "patch":
		return a.runContractPatch(ctx, args[1:])
	case "download-file":
		return a.runContractDownloadFile(ctx, args[1:])
	case "delete":
		return a.runContractDelete(ctx, args[1:])
	case "print-file":
		return a.runContractPrintFile(ctx, args[1:])
	case "share":
		return a.runContractShare(ctx, args[1:])
	case "cooperation":
		return a.runContractCooperation(ctx, args[1:])
	case "field":
		return a.runContractField(ctx, args[1:])
	case "sign":
		return a.runContractSign(ctx, args[1:])
	case "sign-url":
		return a.runContractSignURL(ctx, args[1:])
	case "form":
		return a.runContractForm(ctx, args[1:])
	case "authorization":
		return a.runContractAuthorization(ctx, args[1:])
	case "esign":
		return a.runContractEsign(ctx, args[1:])
	case "approval":
		return a.runContractApproval(ctx, args[1:])
	case "category":
		return a.runContractCategory(ctx, args[1:])
	case "template":
		return a.runContractTemplate(ctx, args[1:])
	case "enum":
		return a.runContractEnum(ctx, args[1:])
	case "upload-file":
		return a.runContractUploadFile(ctx, args[1:])
	default:
		return fmt.Errorf("unknown contract subcommand %q", args[0])
	}
}

func (a *App) runContractSearch(ctx context.Context, args []string) (err error) {
	a.logger.Info("search contracts")
	defer func() {
		if err != nil {
			a.logger.Error("search contracts failed", "error_type", fmt.Sprintf("%T", err))
		}
	}()
	parsed, err := parseArgs(args, structuredValueFlags("--contract-number", "--page-size", "--page-token"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli contract search [flags]")
	}

	options := parseCommandOptions(parsed)
	pageSize, err := parsed.Int("--page-size")
	if err != nil {
		return err
	}
	bodyObject, err := resolveJSONObjectBody(options, false)
	if err != nil {
		return err
	}
	if value := parsed.String("--contract-number"); value != "" {
		bodyObject["contract_number"] = value
	}
	if pageSize > 0 {
		bodyObject["page_size"] = pageSize
	}
	if value := parsed.String("--page-token"); value != "" {
		bodyObject["page_token"] = value
	}
	body, err := marshalJSONObject(bodyObject)
	if err != nil {
		return err
	}

	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/contracts/search", openplatform.IdentityPolicyAny)
	if err != nil {
		return err
	}
	a.logger.Info("resolved contract search identity", "identity", requestContext.Identity)
	if requestContext.Identity == config.IdentityUser && parsed.HasValue("--user-id-type") && strings.TrimSpace(options.userIDType) != "user_id" {
		return fmt.Errorf("contract search with user identity requires --user-id-type user_id; omit the flag or use user_id to match the MCP contract")
	}
	response, err := contractsvc.NewService(client).Search(ctx, requestContext, contractsvc.SearchInput{
		Body: body,
	})
	if err != nil {
		return err
	}
	if requestContext.Identity == config.IdentityUser {
		return a.renderContractMCPResponse(options, response)
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runContractGet(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli contract get <contract-id> [flags]")
	}

	options := parseCommandOptions(parsed)
	contractID := parsed.positionals[0]
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/contracts/"+contractID, openplatform.IdentityPolicyAny)
	if err != nil {
		return err
	}
	response, err := contractsvc.NewService(client).Get(ctx, requestContext, contractID)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runContractSyncUserGroups(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli contract sync-user-groups [flags]")
	}

	options := parseCommandOptions(parsed)
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractMCPPathPrefix+"/contracts/user-groups/sync", openplatform.IdentityPolicyAny)
	if err != nil {
		return err
	}
	response, err := contractsvc.NewService(client).SyncUserGroups(ctx, requestContext)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runContractText(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--offset", "--limit"), commonBoolFlags("--full-text"))
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli contract text <contract-id> [flags]")
	}

	options := parseCommandOptions(parsed)
	offset, err := parsed.Int("--offset")
	if err != nil {
		return err
	}
	limit, err := parsed.Int("--limit")
	if err != nil {
		return err
	}
	contractID := parsed.positionals[0]
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractMCPPathPrefix+"/contracts/"+contractID+"/text", openplatform.IdentityPolicyAny)
	if err != nil {
		return err
	}
	response, err := contractsvc.NewService(client).GetText(ctx, requestContext, contractID, contractsvc.TextInput{
		FullText:    parsed.Bool("--full-text"),
		FullTextSet: parsed.HasBool("--full-text"),
		Offset:      offset,
		OffsetSet:   parsed.HasValue("--offset"),
		Limit:       limit,
		LimitSet:    parsed.HasValue("--limit"),
	})
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runContractCreate(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli contract create [flags]")
	}

	options := parseCommandOptions(parsed)
	bodyObject, err := resolveJSONObjectBody(options, true)
	if err != nil {
		return err
	}
	body, err := marshalJSONObject(bodyObject)
	if err != nil {
		return err
	}

	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, "/open-apis/contract/v1/contracts", openplatform.IdentityPolicyAny)
	if err != nil {
		return err
	}
	response, err := contractsvc.NewService(client).Create(ctx, requestContext, body)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runContractSubmit(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli contract submit <contract-id> [flags]")
	}
	options := parseCommandOptions(parsed)
	body, err := resolveRawBody(options)
	if err != nil {
		return err
	}

	contractID := parsed.positionals[0]
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/contracts/"+contractID+"/submit", openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}
	response, err := contractsvc.NewService(client).Submit(ctx, requestContext, contractID, body)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runContractResubmit(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli contract resubmit <contract-id> [flags]")
	}
	options := parseCommandOptions(parsed)
	body, err := resolveRawBody(options)
	if err != nil {
		return err
	}

	contractID := parsed.positionals[0]
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/contracts/"+contractID+"/resubmit", openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}
	response, err := contractsvc.NewService(client).Resubmit(ctx, requestContext, contractID, body)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runContractPatch(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli contract patch <contract-id> --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	body, err := resolveRequiredRawBody(options)
	if err != nil {
		return err
	}

	contractID := parsed.positionals[0]
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/contracts/"+contractID, openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}
	response, err := contractsvc.NewService(client).Patch(ctx, requestContext, contractID, body)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runContractDownloadFile(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--output-file", "--contract"), commonBoolFlags("--force"))
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli contract download-file <file-id> [flags]")
	}
	options := parseCommandOptions(parsed)
	if options.inputFile != "" || options.data != "" {
		return fmt.Errorf("contract download-file does not accept --input-file or --data")
	}

	format := output.Format(parsed.String("--output"))
	if err := output.ValidateFormat(format); err != nil {
		return err
	}
	if options.raw && parsed.HasValue("--output") {
		return fmt.Errorf("--raw cannot be combined with --output for file downloads")
	}
	fileID := parsed.positionals[0]
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/files/"+fileID, openplatform.IdentityPolicyAny)
	if err != nil {
		return err
	}
	if requestContext.Identity == config.IdentityUser {
		contractID := strings.TrimSpace(parsed.String("--contract"))
		if contractID == "" {
			return fmt.Errorf("--contract is required with --as user")
		}
		return a.runContractDownloadFileAsUser(ctx, contractsvc.NewService(client), requestContext, contractID, fileID, parsed.String("--output-file"), options.raw, parsed.Bool("--force"), format)
	}

	writer, outputPath, closeOutput, err := a.contractDownloadWriter(ctx, strings.TrimSpace(fileID), parsed.String("--output-file"), options.raw, parsed.Bool("--force"))
	if err != nil {
		return err
	}
	if closeOutput != nil {
		defer func() {
			if closeOutput != nil {
				_ = closeOutput()
			}
		}()
	}

	if _, err := contractsvc.NewService(client).DownloadFile(ctx, requestContext, fileID, writer); err != nil {
		return err
	}
	if closeOutput != nil {
		closeErr := closeOutput()
		closeOutput = nil
		if closeErr != nil {
			return fmt.Errorf("close download output file: %w", closeErr)
		}
	}
	if !options.raw {
		return a.renderDownloadedFile(format, "", fileID, filepath.Base(outputPath), outputPath)
	}
	return nil
}

func (a *App) contractDownloadWriter(ctx context.Context, fileID string, outputFile string, raw bool, force bool) (io.Writer, string, func() error, error) {
	if raw {
		return a.stdout, "", nil, nil
	}

	outputPath := strings.TrimSpace(outputFile)
	if outputPath == "" {
		selectedPath, err := a.saveFileDialog(ctx, fileID)
		if err != nil {
			return nil, "", nil, fmt.Errorf("select save path failed; pass --output-file <path> in non-GUI environments: %w", err)
		}
		outputPath = strings.TrimSpace(selectedPath)
		if outputPath == "" {
			return nil, "", nil, fmt.Errorf("select save path failed; pass --output-file <path> in non-GUI environments")
		}
	}

	flags := os.O_WRONLY | os.O_CREATE
	if force {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	file, err := os.OpenFile(outputPath, flags, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil, "", nil, fmt.Errorf("download output file %q already exists; pass --force to overwrite", outputPath)
		}
		return nil, "", nil, fmt.Errorf("open download output file: %w", err)
	}
	return file, outputPath, file.Close, nil
}

func (a *App) runContractDelete(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli contract delete <contract-id> [flags]")
	}
	options := parseCommandOptions(parsed)

	contractID := parsed.positionals[0]
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/contracts/"+contractID, openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}
	response, err := contractsvc.NewService(client).Delete(ctx, requestContext, contractID)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runContractPrintFile(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli contract print-file --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	body, err := resolveRequiredRawBody(options)
	if err != nil {
		return err
	}

	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/files", openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}
	response, err := contractsvc.NewService(client).PrintFile(ctx, requestContext, body)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runContractShare(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract share subcommand")
	}
	switch args[0] {
	case "batch-create":
		return a.runContractShareBatchCreate(ctx, args[1:])
	case "get":
		parsed, err := parseArgs(args[1:], structuredValueFlags(), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 1 {
			return fmt.Errorf("usage: contract-cli contract share get <contract-id> [flags]")
		}
		options := parseCommandOptions(parsed)
		contractID := parsed.positionals[0]
		client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/contracts/"+contractID+"/share_records", openplatform.IdentityPolicyAppOnly)
		if err != nil {
			return err
		}
		response, err := contractsvc.NewService(client).GetShareRecords(ctx, requestContext, contractID)
		if err != nil {
			return err
		}
		return a.renderOpenPlatformResponse(options, response)
	default:
		return fmt.Errorf("unknown contract share subcommand %q", args[0])
	}
}

func (a *App) runContractCooperation(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract cooperation resource")
	}
	switch args[0] {
	case "link":
		return a.runContractCooperationLink(ctx, args[1:])
	case "record":
		return a.runContractCooperationRecord(ctx, args[1:])
	case "file":
		return a.runContractCooperationFile(ctx, args[1:])
	case "search":
		return a.runContractCooperationSearch(ctx, args[1:])
	default:
		return fmt.Errorf("unknown contract cooperation resource %q", args[0])
	}
}

func (a *App) runContractCooperationLink(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract cooperation link subcommand")
	}
	if args[0] != "get" {
		return fmt.Errorf("unknown contract cooperation link subcommand %q", args[0])
	}
	parsed, err := parseArgs(args[1:], structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli contract cooperation link get <contract-id> [flags]")
	}
	options := parseCommandOptions(parsed)
	contractID := parsed.positionals[0]
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/contracts/"+contractID+"/cooperation_link", openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}
	response, err := contractsvc.NewService(client).GetCooperationLink(ctx, requestContext, contractID)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runContractCooperationRecord(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract cooperation record subcommand")
	}
	if args[0] != "get" {
		return fmt.Errorf("unknown contract cooperation record subcommand %q", args[0])
	}
	parsed, err := parseArgs(args[1:], structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli contract cooperation record get <contract-id> [flags]")
	}
	options := parseCommandOptions(parsed)
	contractID := parsed.positionals[0]
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/contracts/"+contractID+"/cooperation_record_info", openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}
	response, err := contractsvc.NewService(client).GetCooperationRecordInfo(ctx, requestContext, contractID)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runContractApproval(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract approval subcommand")
	}
	switch args[0] {
	case "start":
		return a.runContractApprovalStart(ctx, args[1:])
	case "get":
		return a.runContractApprovalGet(ctx, args[1:])
	case "comment":
		return a.runContractApprovalComment(ctx, args[1:])
	case "task":
		return a.runContractApprovalTask(ctx, args[1:])
	default:
		return fmt.Errorf("unknown contract approval subcommand %q", args[0])
	}
}

func (a *App) runContractApprovalStart(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli contract approval start <process-instance-id> --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	body, err := resolveRequiredRawBody(options)
	if err != nil {
		return err
	}

	processInstanceID := parsed.positionals[0]
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/process_instances/"+processInstanceID+"/task_approval", openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}
	response, err := contractsvc.NewService(client).StartApproval(ctx, requestContext, processInstanceID, body)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runContractApprovalGet(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--notice-filter", "--task-instance-filter"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli contract approval get <process-instance-id> [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "contract approval get"); err != nil {
		return err
	}

	processInstanceID := parsed.positionals[0]
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/process_instances/"+processInstanceID, openplatform.IdentityPolicyAny)
	if err != nil {
		return err
	}
	response, err := contractsvc.NewService(client).GetProcessInstance(ctx, requestContext, processInstanceID, contractsvc.ProcessInstanceInput{
		NoticeFilter:       parsed.String("--notice-filter"),
		TaskInstanceFilter: parsed.String("--task-instance-filter"),
	})
	if err != nil {
		return err
	}
	if requestContext.Identity == config.IdentityUser {
		return a.renderContractMCPResponse(options, response)
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runContractCategory(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract category subcommand")
	}
	switch args[0] {
	case "list":
		parsed, err := parseArgs(args[1:], structuredValueFlags("--lang"), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return fmt.Errorf("usage: contract-cli contract category list [flags]")
		}

		options := parseCommandOptions(parsed)
		client, requestContext, err := a.openPlatformClientAndContextForOptions(options, "/open-apis/contract/v1/contract_categorys", openplatform.IdentityPolicyAny)
		if err != nil {
			return err
		}
		response, err := contractsvc.NewService(client).ListCategories(ctx, requestContext, parsed.String("--lang"))
		if err != nil {
			return err
		}
		return a.renderOpenPlatformResponse(options, response)
	default:
		return fmt.Errorf("unknown contract category subcommand %q", args[0])
	}
}

func (a *App) runContractTemplate(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract template subcommand")
	}
	switch args[0] {
	case "list":
		parsed, err := parseArgs(args[1:], structuredValueFlags("--category-number", "--page-size", "--page-token"), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return fmt.Errorf("usage: contract-cli contract template list [flags]")
		}
		options := parseCommandOptions(parsed)
		pageSize, err := parsed.Int("--page-size")
		if err != nil {
			return err
		}
		client, requestContext, err := a.openPlatformClientAndContextForOptions(options, "/open-apis/contract/v1/templates", openplatform.IdentityPolicyAny)
		if err != nil {
			return err
		}
		response, err := contractsvc.NewService(client).ListTemplates(ctx, requestContext, contractsvc.ListTemplatesInput{
			CategoryNumber: parsed.String("--category-number"),
			PageSize:       pageSize,
			PageToken:      parsed.String("--page-token"),
		})
		if err != nil {
			return err
		}
		return a.renderOpenPlatformResponse(options, response)
	case "get":
		parsed, err := parseArgs(args[1:], structuredValueFlags(), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 1 {
			return fmt.Errorf("usage: contract-cli contract template get <template-id> [flags]")
		}
		options := parseCommandOptions(parsed)
		templateID := parsed.positionals[0]
		client, requestContext, err := a.openPlatformClientAndContextForOptions(options, "/open-apis/contract/v1/templates/"+templateID, openplatform.IdentityPolicyAny)
		if err != nil {
			return err
		}
		response, err := contractsvc.NewService(client).GetTemplate(ctx, requestContext, templateID)
		if err != nil {
			return err
		}
		return a.renderOpenPlatformResponse(options, response)
	case "instantiate":
		parsed, err := parseArgs(args[1:], structuredValueFlags(), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return fmt.Errorf("usage: contract-cli contract template instantiate [flags]")
		}
		options := parseCommandOptions(parsed)
		bodyObject, err := resolveJSONObjectBody(options, true)
		if err != nil {
			return err
		}
		body, err := marshalJSONObject(bodyObject)
		if err != nil {
			return err
		}
		client, requestContext, err := a.openPlatformClientAndContextForOptions(options, "/open-apis/contract/v1/template_instances", openplatform.IdentityPolicyAny)
		if err != nil {
			return err
		}
		response, err := contractsvc.NewService(client).InstantiateTemplate(ctx, requestContext, body)
		if err != nil {
			return err
		}
		return a.renderOpenPlatformResponse(options, response)
	default:
		return fmt.Errorf("unknown contract template subcommand %q", args[0])
	}
}

func (a *App) runContractEnum(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract enum subcommand")
	}
	switch args[0] {
	case "list":
		parsed, err := parseArgs(args[1:], structuredValueFlags("--type"), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return fmt.Errorf("usage: contract-cli contract enum list --type <enum-type> [flags]")
		}
		options := parseCommandOptions(parsed)
		client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractMCPPathPrefix+"/enum_values", openplatform.IdentityPolicyUserOnly)
		if err != nil {
			return err
		}
		response, err := contractsvc.NewService(client).ListEnums(ctx, requestContext, parsed.String("--type"))
		if err != nil {
			return err
		}
		return a.renderOpenPlatformResponse(options, response)
	default:
		return fmt.Errorf("unknown contract enum subcommand %q", args[0])
	}
}

func (a *App) runContractUploadFile(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--file", "--file-type", "--file-name"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli contract upload-file --file <path> --file-type <type> [flags]")
	}

	options := parseCommandOptions(parsed)
	if options.inputFile != "" || options.data != "" {
		return fmt.Errorf("contract upload-file does not accept --input-file or --data; use --file for binary upload")
	}
	uploadPath := strings.TrimSpace(parsed.String("--file"))
	if uploadPath == "" {
		return fmt.Errorf("--file is required")
	}
	fileType := strings.TrimSpace(parsed.String("--file-type"))
	if fileType == "" {
		return fmt.Errorf("--file-type is required")
	}
	fileName := strings.TrimSpace(parsed.String("--file-name"))
	if fileName == "" {
		fileName = filepath.Base(uploadPath)
	}
	if strings.TrimSpace(fileName) == "" {
		return fmt.Errorf("--file-name must not be empty")
	}
	fileInfo, err := validateContractUploadFile(uploadPath)
	if err != nil {
		return err
	}

	a.logger.Info("contract upload-file command started", "profile", emptyFallback(options.profileName, "<current>"), "identity", emptyFallback(options.identity, "<default>"), "file_name", fileName, "file_type", fileType, "size_bytes", fileInfo.Size())
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/files/upload", openplatform.IdentityPolicyAny)
	if err != nil {
		a.logger.Error("contract upload-file context failed", "profile", emptyFallback(options.profileName, "<current>"), "identity", emptyFallback(options.identity, "<default>"), "file_name", fileName, "file_type", fileType, "error", err.Error())
		return err
	}

	file, err := os.Open(uploadPath)
	if err != nil {
		a.logger.Error("open contract upload file failed", "profile", requestContext.Profile.Name, "identity", requestContext.Identity, "file_name", fileName, "file_type", fileType, "error", err.Error())
		return fmt.Errorf("open upload file: %w", err)
	}
	defer func() { _ = file.Close() }() // Read-only upload source; request errors are handled below.

	if requestContext.Identity == config.IdentityUser && (fileType == "reviewAttachment" || fileType == "approveAttachment") {
		return a.runContractUploadAttachmentAsUser(ctx, contractsvc.NewService(client), requestContext, options, contractsvc.UploadFileInput{
			FileName: fileName, FileType: fileType, File: file,
		}, fileInfo.Size())
	}

	response, err := contractsvc.NewService(client).UploadFile(ctx, requestContext, contractsvc.UploadFileInput{
		FileName: fileName,
		FileType: fileType,
		File:     file,
	})
	if err != nil {
		a.logger.Error("contract upload-file request failed", "profile", requestContext.Profile.Name, "identity", requestContext.Identity, "file_name", fileName, "file_type", fileType, "error", err.Error())
		return err
	}
	a.logger.Info("contract upload-file command completed", "profile", requestContext.Profile.Name, "identity", requestContext.Identity, "file_name", fileName, "file_type", fileType)
	return a.renderOpenPlatformResponse(options, response)
}

func validateContractUploadFile(path string) (os.FileInfo, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat upload file: %w", err)
	}
	if !fileInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("upload file %q must be a regular file", path)
	}
	if fileInfo.Size() > maxContractUploadFileBytes {
		return nil, fmt.Errorf("upload file %q size %d bytes must be <= 200MB", path, fileInfo.Size())
	}
	return fileInfo, nil
}
