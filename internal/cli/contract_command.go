package cli

import (
	"context"
	"fmt"

	"cn.qfei/contract-cli/internal/openplatform"
	contractsvc "cn.qfei/contract-cli/internal/openplatform/contract"
)

const contractMCPPathPrefix = "/open-apis/contract/v1/mcp"

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
	default:
		return fmt.Errorf("unknown contract subcommand %q", args[0])
	}
}

func (a *App) runContractSearch(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, commonValueFlags("--contract-number", "--page-size", "--page-token"), commonBoolFlags())
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

	client, requestContext, err := a.openPlatformClientAndContext(options.profileName, options.identity, contractMCPPathPrefix+"/contracts/search", openplatform.IdentityPolicyUserOnly)
	if err != nil {
		return err
	}
	response, err := contractsvc.NewService(client).Search(ctx, requestContext, body)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runContractGet(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, commonValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli contract get <contract-id> [flags]")
	}

	options := parseCommandOptions(parsed)
	contractID := parsed.positionals[0]
	client, requestContext, err := a.openPlatformClientAndContext(options.profileName, options.identity, contractMCPPathPrefix+"/contracts/"+contractID, openplatform.IdentityPolicyUserOnly)
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
	parsed, err := parseArgs(args, commonValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli contract sync-user-groups [flags]")
	}

	options := parseCommandOptions(parsed)
	client, requestContext, err := a.openPlatformClientAndContext(options.profileName, options.identity, contractMCPPathPrefix+"/contracts/user-groups/sync", openplatform.IdentityPolicyUserOnly)
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
	parsed, err := parseArgs(args, commonValueFlags("--offset", "--limit"), commonBoolFlags("--full-text"))
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
	client, requestContext, err := a.openPlatformClientAndContext(options.profileName, options.identity, contractMCPPathPrefix+"/contracts/"+contractID+"/text", openplatform.IdentityPolicyUserOnly)
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
	parsed, err := parseArgs(args, commonValueFlags(), commonBoolFlags())
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

	client, requestContext, err := a.openPlatformClientAndContext(options.profileName, options.identity, contractMCPPathPrefix+"/contracts", openplatform.IdentityPolicyUserOnly)
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
		parsed, err := parseArgs(args[1:], commonValueFlags("--lang"), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return fmt.Errorf("usage: contract-cli contract category list [flags]")
		}

		options := parseCommandOptions(parsed)
		client, requestContext, err := a.openPlatformClientAndContext(options.profileName, options.identity, contractMCPPathPrefix+"/contract_categorys", openplatform.IdentityPolicyUserOnly)
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
	servicePath := contractMCPPathPrefix + "/templates"
	switch args[0] {
	case "list":
		parsed, err := parseArgs(args[1:], commonValueFlags("--category-number", "--page-size", "--page-token"), commonBoolFlags())
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
		client, requestContext, err := a.openPlatformClientAndContext(options.profileName, options.identity, servicePath, openplatform.IdentityPolicyUserOnly)
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
		parsed, err := parseArgs(args[1:], commonValueFlags(), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 1 {
			return fmt.Errorf("usage: contract-cli contract template get <template-id> [flags]")
		}
		options := parseCommandOptions(parsed)
		templateID := parsed.positionals[0]
		client, requestContext, err := a.openPlatformClientAndContext(options.profileName, options.identity, servicePath+"/"+templateID, openplatform.IdentityPolicyUserOnly)
		if err != nil {
			return err
		}
		response, err := contractsvc.NewService(client).GetTemplate(ctx, requestContext, templateID)
		if err != nil {
			return err
		}
		return a.renderOpenPlatformResponse(options, response)
	case "instantiate":
		parsed, err := parseArgs(args[1:], commonValueFlags(), commonBoolFlags())
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
		client, requestContext, err := a.openPlatformClientAndContext(options.profileName, options.identity, contractMCPPathPrefix+"/template_instances", openplatform.IdentityPolicyUserOnly)
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
		parsed, err := parseArgs(args[1:], commonValueFlags("--type"), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return fmt.Errorf("usage: contract-cli contract enum list --type <enum-type> [flags]")
		}
		options := parseCommandOptions(parsed)
		client, requestContext, err := a.openPlatformClientAndContext(options.profileName, options.identity, contractMCPPathPrefix+"/enum_values", openplatform.IdentityPolicyUserOnly)
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
