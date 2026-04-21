package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cn.qfei/contract-cli/internal/openplatform"
	contractsvc "cn.qfei/contract-cli/internal/openplatform/contract"
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
	case "get":
		return a.runContractGet(ctx, args[1:])
	case "sync-user-groups":
		return a.runContractSyncUserGroups(ctx, args[1:])
	case "text":
		return a.runContractText(ctx, args[1:])
	case "create":
		return a.runContractCreate(ctx, args[1:])
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

func (a *App) runContractSearch(ctx context.Context, args []string) error {
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
	response, err := contractsvc.NewService(client).Search(ctx, requestContext, contractsvc.SearchInput{
		Body: body,
	})
	if err != nil {
		return err
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
		FullText: parsed.Bool("--full-text"),
		Offset:   offset,
		Limit:    limit,
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
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/files/upload", openplatform.IdentityPolicyBotOnly)
	if err != nil {
		a.logger.Error("contract upload-file context failed", "profile", emptyFallback(options.profileName, "<current>"), "identity", emptyFallback(options.identity, "<default>"), "file_name", fileName, "file_type", fileType, "error", err.Error())
		return err
	}

	file, err := os.Open(uploadPath)
	if err != nil {
		a.logger.Error("open contract upload file failed", "profile", requestContext.Profile.Name, "identity", requestContext.Identity, "file_name", fileName, "file_type", fileType, "error", err.Error())
		return fmt.Errorf("open upload file: %w", err)
	}
	defer file.Close()

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
