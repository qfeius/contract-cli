package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
)

func (a *App) runContractSearchV2(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli contract search-v2 --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	body, err := resolveRequiredRawBody(options)
	if err != nil {
		return err
	}
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodPost, contractOpenAPIPathPrefix+"/contracts/searchV2", nil, body)
}

func (a *App) runContractField(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract field subcommand")
	}
	switch args[0] {
	case "update":
		parsed, err := parseArgs(args[1:], structuredValueFlags(), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return fmt.Errorf("usage: contract-cli contract field update --input-file <path>|--data <json> [flags]")
		}
		options := parseCommandOptions(parsed)
		body, err := resolveRequiredRawBody(options)
		if err != nil {
			return err
		}
		return a.executeAppOpenPlatformRequest(ctx, options, http.MethodPut, contractOpenAPIPathPrefix+"/attribute_definition", nil, body)
	default:
		return fmt.Errorf("unknown contract field subcommand %q", args[0])
	}
}

func (a *App) runContractSign(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract sign subcommand")
	}
	switch args[0] {
	case "switch-to-paper":
		parsed, err := parseArgs(args[1:], structuredValueFlags("--business-id", "--business-type-code"), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return fmt.Errorf("usage: contract-cli contract sign switch-to-paper --business-id <contract-id> --business-type-code <code> [flags]")
		}
		options := parseCommandOptions(parsed)
		if err := rejectRawBody(options, "contract sign switch-to-paper"); err != nil {
			return err
		}
		businessID, err := requiredParsedValue(parsed, "--business-id")
		if err != nil {
			return err
		}
		businessTypeCode, err := requiredParsedValue(parsed, "--business-type-code")
		if err != nil {
			return err
		}
		query := url.Values{
			"business_id":        {businessID},
			"business_type_code": {businessTypeCode},
		}
		return a.executeAppOpenPlatformRequest(ctx, options, http.MethodPost, contractOpenAPIPathPrefix+"/contracts/signType/switchToPaper", query, nil)
	default:
		return fmt.Errorf("unknown contract sign subcommand %q", args[0])
	}
}

func (a *App) runContractSignURL(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract sign-url subcommand")
	}
	if args[0] != "get" {
		return fmt.Errorf("unknown contract sign-url subcommand %q", args[0])
	}
	parsed, err := parseArgs(args[1:], structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli contract sign-url get <contract-id> [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "contract sign-url get"); err != nil {
		return err
	}
	path := contractOpenAPIPathPrefix + "/contracts/" + escapePathSegment(parsed.positionals[0]) + "/sign_url"
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodGet, path, nil, nil)
}

func (a *App) runContractForm(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract form resource")
	}
	switch args[0] {
	case "attribute":
		return a.runContractFormAttribute(ctx, args[1:])
	default:
		return fmt.Errorf("unknown contract form resource %q", args[0])
	}
}

func (a *App) runContractFormAttribute(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract form attribute subcommand")
	}
	if args[0] != "list" {
		return fmt.Errorf("unknown contract form attribute subcommand %q", args[0])
	}
	parsed, err := parseArgs(args[1:], structuredValueFlags("--category-id", "--business-type-code"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli contract form attribute list --category-id <category-id> --business-type-code <code> [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "contract form attribute list"); err != nil {
		return err
	}
	categoryID, err := requiredParsedValue(parsed, "--category-id")
	if err != nil {
		return err
	}
	businessTypeCode, err := requiredParsedValue(parsed, "--business-type-code")
	if err != nil {
		return err
	}
	query := url.Values{
		"category_id":        {categoryID},
		"business_type_code": {businessTypeCode},
	}
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodGet, contractOpenAPIPathPrefix+"/form_definition/attribute", query, nil)
}

func (a *App) runContractAuthorization(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract authorization subcommand")
	}
	switch args[0] {
	case "grant":
		parsed, err := parseArgs(args[1:], structuredValueFlags(), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return fmt.Errorf("usage: contract-cli contract authorization grant --input-file <path>|--data <json> [flags]")
		}
		options := parseCommandOptions(parsed)
		body, err := resolveRequiredRawBody(options)
		if err != nil {
			return err
		}
		return a.executeAppOpenPlatformRequest(ctx, options, http.MethodPost, contractOpenAPIPathPrefix+"/authorizations", nil, body)
	default:
		return fmt.Errorf("unknown contract authorization subcommand %q", args[0])
	}
}

func (a *App) runContractEsign(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract esign subcommand")
	}
	paths := map[string]string{
		"personal-auth-url": "/open-apis/esign/auth/psnAuthUrl",
		"org-auth-url":      "/open-apis/esign/auth/orgAuthUrl",
	}
	path, ok := paths[args[0]]
	if !ok {
		return fmt.Errorf("unknown contract esign subcommand %q", args[0])
	}
	parsed, err := parseArgs(args[1:], structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli contract esign %s --input-file <path>|--data <json> [flags]", args[0])
	}
	options := parseCommandOptions(parsed)
	body, err := resolveRequiredRawBody(options)
	if err != nil {
		return err
	}
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodPost, path, nil, body)
}

func (a *App) runContractShareBatchCreate(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli contract share batch-create --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	body, err := resolveRequiredRawBody(options)
	if err != nil {
		return err
	}
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodPost, contractOpenAPIPathPrefix+"/contracts/contract/batch_share", nil, body)
}

func (a *App) runContractCooperationFile(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing contract cooperation file subcommand")
	}
	switch args[0] {
	case "get":
		parsed, err := parseArgs(args[1:], structuredValueFlags(), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 1 {
			return fmt.Errorf("usage: contract-cli contract cooperation file get <contract-id> [flags]")
		}
		options := parseCommandOptions(parsed)
		if err := rejectRawBody(options, "contract cooperation file get"); err != nil {
			return err
		}
		path := contractOpenAPIPathPrefix + "/contracts/" + escapePathSegment(parsed.positionals[0]) + "/cooperation/file_info"
		return a.executeAppOpenPlatformRequest(ctx, options, http.MethodGet, path, nil, nil)
	case "download":
		parsed, err := parseArgs(args[1:], structuredValueFlags("--output-file"), commonBoolFlags("--force"))
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 1 {
			return fmt.Errorf("usage: contract-cli contract cooperation file download <file-id> [flags]")
		}
		options := parseCommandOptions(parsed)
		if err := rejectRawBody(options, "contract cooperation file download"); err != nil {
			return err
		}
		fileID := strings.TrimSpace(parsed.positionals[0])
		path := contractOpenAPIPathPrefix + "/contracts/cooperation/" + escapePathSegment(fileID) + "/download_file"
		return a.downloadAppOpenPlatformFile(ctx, options, path, fileID, parsed.String("--output-file"), parsed.Bool("--force"))
	default:
		return fmt.Errorf("unknown contract cooperation file subcommand %q", args[0])
	}
}

func (a *App) runContractCooperationSearch(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli contract cooperation search --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	body, err := resolveRequiredRawBody(options)
	if err != nil {
		return err
	}
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodPost, contractOpenAPIPathPrefix+"/cooperation/search", nil, body)
}

func (a *App) runMDMFixedExchangeRate(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing mdm fixed-exchange-rate subcommand")
	}
	switch args[0] {
	case "get":
		parsed, err := parseArgs(args[1:], structuredValueFlags("--source-currency", "--target-currency", "--effective-date"), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return fmt.Errorf("usage: contract-cli mdm fixed-exchange-rate get --source-currency <code> --target-currency <code> --effective-date <date> [flags]")
		}
		options := parseCommandOptions(parsed)
		if err := rejectRawBody(options, "mdm fixed-exchange-rate get"); err != nil {
			return err
		}
		sourceCurrency, err := requiredParsedValue(parsed, "--source-currency")
		if err != nil {
			return err
		}
		targetCurrency, err := requiredParsedValue(parsed, "--target-currency")
		if err != nil {
			return err
		}
		effectiveDate, err := requiredParsedValue(parsed, "--effective-date")
		if err != nil {
			return err
		}
		query := url.Values{
			"source_currency": {sourceCurrency},
			"target_currency": {targetCurrency},
			"date":            {effectiveDate},
		}
		return a.executeAppOpenPlatformRequest(ctx, options, http.MethodGet, "/open-apis/mdm/v1/fixed_exchange_rate", query, nil)
	case "update":
		parsed, err := parseArgs(args[1:], structuredValueFlags(), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return fmt.Errorf("usage: contract-cli mdm fixed-exchange-rate update --input-file <path>|--data <json> [flags]")
		}
		options := parseCommandOptions(parsed)
		body, err := resolveRequiredRawBody(options)
		if err != nil {
			return err
		}
		return a.executeAppOpenPlatformRequest(ctx, options, http.MethodPut, "/open-apis/mdm/v1/fixed_exchange_rate", nil, body)
	default:
		return fmt.Errorf("unknown mdm fixed-exchange-rate subcommand %q", args[0])
	}
}

func (a *App) runMDMVendorCreate(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--department-id-type"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli mdm vendor create --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	return a.executeVendorCreate(ctx, options)
}

func (a *App) runMDMVendorUpdate(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli mdm vendor update <vendor-id> --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	path := "/open-apis/mdm/v1/vendors/" + escapePathSegment(parsed.positionals[0])
	if err := rejectExplicitNonAppIdentity(options, path); err != nil {
		return err
	}
	if err := requireMDMWriteUserID("mdm vendor update", options); err != nil {
		return err
	}
	body, err := resolveRequiredRawBody(options)
	if err != nil {
		return err
	}
	if err := validateMDMVendorUpdateBody(body); err != nil {
		return err
	}
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodPut, path, nil, body)
}

func (a *App) runMDMVendorListAll(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--page-size", "--page-token"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli mdm vendor list-all [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "mdm vendor list-all"); err != nil {
		return err
	}
	query, err := pageQuery(parsed)
	if err != nil {
		return err
	}
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodGet, "/open-apis/mdm/v1/vendors/list_all", query, nil)
}

func (a *App) runMDMVendorQueryByCert(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--certification-id", "--ad-country"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli mdm vendor query-by-cert --certification-id <id> --ad-country <country> [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "mdm vendor query-by-cert"); err != nil {
		return err
	}
	certificationID, err := requiredParsedValue(parsed, "--certification-id")
	if err != nil {
		return err
	}
	adCountry, err := requiredParsedValue(parsed, "--ad-country")
	if err != nil {
		return err
	}
	query := url.Values{
		"certification_id": {certificationID},
		"ad_country":       {adCountry},
	}
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodGet, "/open-apis/mdm/v1/vendors/query_vendors", query, nil)
}

func (a *App) runMDMLegalCreate(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli mdm legal create --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectExplicitNonAppIdentity(options, "/open-apis/mdm/v1/legal_entities"); err != nil {
		return err
	}
	if err := requireMDMWriteUserID("mdm legal create", options); err != nil {
		return err
	}
	body, err := resolveRequiredRawBody(options)
	if err != nil {
		return err
	}
	if err := validateMDMLegalCreateBody(body); err != nil {
		return err
	}
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodPost, "/open-apis/mdm/v1/legal_entities", nil, body)
}

func (a *App) runMDMLegalUpdate(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli mdm legal update <legal-entity-id> --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	path := "/open-apis/mdm/v1/legal_entities/" + escapePathSegment(parsed.positionals[0])
	if err := rejectExplicitNonAppIdentity(options, path); err != nil {
		return err
	}
	if err := requireMDMWriteUserID("mdm legal update", options); err != nil {
		return err
	}
	body, err := resolveRequiredRawBody(options)
	if err != nil {
		return err
	}
	if err := validateMDMLegalUpdateBody(body); err != nil {
		return err
	}
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodPut, path, nil, body)
}

func (a *App) runMDMLegalGetByCode(ctx context.Context, parsed parsedArgs) error {
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli mdm legal get <legal-entity-id> [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "mdm legal get --code"); err != nil {
		return err
	}
	legalEntity, err := requiredParsedValue(parsed, "--code")
	if err != nil {
		return err
	}
	query, err := pageQuery(parsed)
	if err != nil {
		return err
	}
	query.Set("legalEntity", legalEntity)
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodGet, "/open-apis/mdm/v1/legal_entities", query, nil)
}

func (a *App) runMDMFile(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing mdm file subcommand")
	}
	if args[0] != "download" {
		return fmt.Errorf("unknown mdm file subcommand %q", args[0])
	}
	parsed, err := parseArgs(args[1:], structuredValueFlags("--output-file"), commonBoolFlags("--force"))
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli mdm file download <file-id> [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "mdm file download"); err != nil {
		return err
	}
	fileID := strings.TrimSpace(parsed.positionals[0])
	path := "/open-apis/mdm/v1/file/download/" + escapePathSegment(fileID)
	return a.downloadAppOpenPlatformFile(ctx, options, path, fileID, parsed.String("--output-file"), parsed.Bool("--force"))
}

func (a *App) runEvent(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing event resource")
	}
	if args[0] != "outbound-ip" {
		return fmt.Errorf("unknown event resource %q", args[0])
	}
	if len(args) < 2 {
		return fmt.Errorf("missing event outbound-ip subcommand")
	}
	if args[1] != "list" {
		return fmt.Errorf("unknown event outbound-ip subcommand %q", args[1])
	}
	parsed, err := parseArgs(args[2:], structuredValueFlags("--page-size", "--page-token"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli event outbound-ip list [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "event outbound-ip list"); err != nil {
		return err
	}
	query, err := pageQueryWithPageSizeRange(parsed, 10, 50)
	if err != nil {
		return err
	}
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodGet, "/open-apis/event/v1/outbound_ip", query, nil)
}

func (a *App) runRule(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing rule resource")
	}
	if args[0] != "table" {
		return fmt.Errorf("unknown rule resource %q", args[0])
	}
	return a.runRuleTable(ctx, args[1:])
}

func (a *App) runRuleTable(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing rule table subcommand")
	}
	switch args[0] {
	case "list":
		return a.runRuleTableList(ctx, args[1:])
	case "pre-release", "release":
		return a.runRuleTablePublish(ctx, args[0], args[1:])
	case "column-headers":
		return a.runRuleTableColumnHeaders(ctx, args[1:])
	case "row":
		return a.runRuleTableRow(ctx, args[1:])
	default:
		return fmt.Errorf("unknown rule table subcommand %q", args[0])
	}
}

func (a *App) runRuleTableList(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--product-id", "--group-id", "--page-size", "--page-token"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli rule table list --product-id <id> --group-id <id> [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "rule table list"); err != nil {
		return err
	}
	productID, groupID, err := requiredRuleProductGroup(parsed)
	if err != nil {
		return err
	}
	query, err := pageQuery(parsed)
	if err != nil {
		return err
	}
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodGet, ruleTableBasePath(productID, groupID), query, nil)
}

func (a *App) runRuleTablePublish(ctx context.Context, action string, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--product-id", "--group-id", "--table-id"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli rule table %s --product-id <id> --group-id <id> --table-id <id> [flags]", action)
	}
	options := parseCommandOptions(parsed)
	body, err := resolveRawBody(options)
	if err != nil {
		return err
	}
	productID, groupID, tableID, err := requiredRuleTable(parsed)
	if err != nil {
		return err
	}
	suffix := strings.ReplaceAll(action, "-", "_")
	path := ruleTablePath(productID, groupID, tableID) + "/" + suffix
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodPatch, path, nil, body)
}

func (a *App) runRuleTableColumnHeaders(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing rule table column-headers subcommand")
	}
	if args[0] != "list" {
		return fmt.Errorf("unknown rule table column-headers subcommand %q", args[0])
	}
	parsed, err := parseArgs(args[1:], structuredValueFlags("--product-id", "--group-id", "--table-id"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli rule table column-headers list --product-id <id> --group-id <id> --table-id <id> [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "rule table column-headers list"); err != nil {
		return err
	}
	productID, groupID, tableID, err := requiredRuleTable(parsed)
	if err != nil {
		return err
	}
	path := ruleTablePath(productID, groupID, tableID) + "/table_columns/column_headers"
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodGet, path, nil, nil)
}

func (a *App) runRuleTableRow(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing rule table row subcommand")
	}
	switch args[0] {
	case "create":
		return a.runRuleTableRowCreate(ctx, args[1:])
	case "get":
		return a.runRuleTableRowGet(ctx, args[1:])
	case "list":
		return a.runRuleTableRowList(ctx, args[1:])
	case "search":
		return a.runRuleTableRowSearch(ctx, args[1:])
	case "update":
		return a.runRuleTableRowUpdate(ctx, args[1:])
	case "delete":
		return a.runRuleTableRowDelete(ctx, args[1:])
	default:
		return fmt.Errorf("unknown rule table row subcommand %q", args[0])
	}
}

func (a *App) runRuleTableRowCreate(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--product-id", "--group-id", "--table-id"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli rule table row create --product-id <id> --group-id <id> --table-id <id> --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	body, err := resolveRequiredRawBody(options)
	if err != nil {
		return err
	}
	productID, groupID, tableID, err := requiredRuleTable(parsed)
	if err != nil {
		return err
	}
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodPost, ruleTablePath(productID, groupID, tableID)+"/table_rows", nil, body)
}

func (a *App) runRuleTableRowGet(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--product-id", "--group-id", "--table-id"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli rule table row get <row-id> --product-id <id> --group-id <id> --table-id <id> [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "rule table row get"); err != nil {
		return err
	}
	productID, groupID, tableID, err := requiredRuleTable(parsed)
	if err != nil {
		return err
	}
	path := ruleTablePath(productID, groupID, tableID) + "/table_rows/" + escapePathSegment(parsed.positionals[0])
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodGet, path, nil, nil)
}

func (a *App) runRuleTableRowList(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--product-id", "--group-id", "--table-id", "--page-size", "--page-token"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli rule table row list --product-id <id> --group-id <id> --table-id <id> [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "rule table row list"); err != nil {
		return err
	}
	productID, groupID, tableID, err := requiredRuleTable(parsed)
	if err != nil {
		return err
	}
	query, err := pageQuery(parsed)
	if err != nil {
		return err
	}
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodGet, ruleTablePath(productID, groupID, tableID)+"/table_rows", query, nil)
}

func (a *App) runRuleTableRowSearch(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--product-id", "--group-id", "--table-id", "--page-size", "--page-token"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli rule table row search --product-id <id> --group-id <id> --table-id <id> --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	body, err := resolveRequiredRawBody(options)
	if err != nil {
		return err
	}
	productID, groupID, tableID, err := requiredRuleTable(parsed)
	if err != nil {
		return err
	}
	query, err := pageQuery(parsed)
	if err != nil {
		return err
	}
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodPost, ruleTablePath(productID, groupID, tableID)+"/table_rows/search", query, body)
}

func (a *App) runRuleTableRowUpdate(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--product-id", "--group-id", "--table-id"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli rule table row update <row-id> --product-id <id> --group-id <id> --table-id <id> --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	body, err := resolveRequiredRawBody(options)
	if err != nil {
		return err
	}
	productID, groupID, tableID, err := requiredRuleTable(parsed)
	if err != nil {
		return err
	}
	path := ruleTablePath(productID, groupID, tableID) + "/table_rows/" + escapePathSegment(parsed.positionals[0])
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodPut, path, nil, body)
}

func (a *App) runRuleTableRowDelete(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--product-id", "--group-id", "--table-id"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli rule table row delete <row-id> --product-id <id> --group-id <id> --table-id <id> [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "rule table row delete"); err != nil {
		return err
	}
	productID, groupID, tableID, err := requiredRuleTable(parsed)
	if err != nil {
		return err
	}
	path := ruleTablePath(productID, groupID, tableID) + "/table_rows/" + escapePathSegment(parsed.positionals[0])
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodDelete, path, nil, nil)
}

func (a *App) executeAppOpenPlatformRequest(ctx context.Context, options commandOptions, method string, path string, query url.Values, body []byte) error {
	return a.executeOpenPlatformCommand(ctx, options, openplatform.Request{
		Method:         method,
		Path:           path,
		Query:          query,
		Body:           body,
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (a *App) downloadAppOpenPlatformFile(ctx context.Context, options commandOptions, path string, suggestedName string, outputFile string, force bool) error {
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, path, openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}

	writer, outputPath, closeOutput, err := a.contractDownloadWriter(ctx, strings.TrimSpace(suggestedName), outputFile, options.raw, force)
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

	if _, err := client.DoStream(ctx, requestContext, openplatform.Request{
		Method:         http.MethodGet,
		Path:           path,
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	}, writer); err != nil {
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
		_, _ = fmt.Fprintf(a.stdout, "Downloaded file to %s\n", outputPath)
	}
	return nil
}

func pageQuery(parsed parsedArgs) (url.Values, error) {
	query := url.Values{}
	pageSize, err := parsed.Int("--page-size")
	if err != nil {
		return nil, err
	}
	if pageSize > 0 {
		query.Set("page_size", fmt.Sprintf("%d", pageSize))
	}
	if value := strings.TrimSpace(parsed.String("--page-token")); value != "" {
		query.Set("page_token", value)
	}
	return query, nil
}

func pageQueryWithPageSizeRange(parsed parsedArgs, minValue, maxValue int) (url.Values, error) {
	query := url.Values{}
	pageSize, err := parsed.Int("--page-size")
	if err != nil {
		return nil, err
	}
	if pageSize > 0 {
		if pageSize < minValue || pageSize > maxValue {
			return nil, fmt.Errorf("--page-size must be between %d and %d", minValue, maxValue)
		}
		query.Set("page_size", fmt.Sprintf("%d", pageSize))
	}
	if value := strings.TrimSpace(parsed.String("--page-token")); value != "" {
		query.Set("page_token", value)
	}
	return query, nil
}

func rejectExplicitNonAppIdentity(options commandOptions, path string) error {
	if strings.TrimSpace(options.identity) == "" {
		return nil
	}
	identity, err := config.ParseIdentityKind(options.identity)
	if err != nil {
		return err
	}
	if identity != config.IdentityApp {
		return fmt.Errorf("open platform path %q only supports --as app", path)
	}
	return nil
}

func requireMDMWriteUserID(command string, options commandOptions) error {
	if strings.TrimSpace(options.userID) == "" {
		return fmt.Errorf("%s requires --user-id", command)
	}
	return nil
}

func validateMDMVendorCreateBody(body []byte) error {
	object, err := decodeJSONBodyObject("mdm vendor create", body)
	if err != nil {
		return err
	}
	if hasJSONField(object, "vendor") {
		return fmt.Errorf("mdm vendor create body must not include vendor")
	}
	return nil
}

func validateMDMVendorUpdateBody(body []byte) error {
	object, err := decodeJSONBodyObject("mdm vendor update", body)
	if err != nil {
		return err
	}
	if !hasNonEmptyJSONField(object, "id") || !hasNonEmptyJSONField(object, "vendor") {
		return fmt.Errorf("mdm vendor update body must include id and vendor")
	}
	return nil
}

func validateMDMLegalCreateBody(body []byte) error {
	object, err := decodeJSONBodyObject("mdm legal create", body)
	if err != nil {
		return err
	}
	if hasJSONField(object, "legalEntity") || hasJSONField(object, "legal_entity") {
		return fmt.Errorf("mdm legal create body must not include legalEntity")
	}
	return nil
}

func validateMDMLegalUpdateBody(body []byte) error {
	object, err := decodeJSONBodyObject("mdm legal update", body)
	if err != nil {
		return err
	}
	if hasJSONField(object, "legal_entity") {
		return fmt.Errorf("mdm legal update body must use legalEntity")
	}
	if !hasNonEmptyJSONField(object, "id") || !hasNonEmptyJSONField(object, "legalEntity") {
		return fmt.Errorf("mdm legal update body must include id and legalEntity")
	}
	return nil
}

func decodeJSONBodyObject(command string, body []byte) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil {
		return nil, fmt.Errorf("decode %s body json: %w", command, err)
	}
	if object == nil {
		return nil, fmt.Errorf("%s body must be a JSON object", command)
	}
	return object, nil
}

func hasJSONField(object map[string]json.RawMessage, key string) bool {
	_, ok := object[key]
	return ok
}

func hasNonEmptyJSONField(object map[string]json.RawMessage, key string) bool {
	value, ok := object[key]
	if !ok {
		return false
	}
	trimmed := strings.TrimSpace(string(value))
	return trimmed != "" && trimmed != "null" && trimmed != `""`
}

func requiredRuleProductGroup(parsed parsedArgs) (string, string, error) {
	productID, err := requiredParsedValue(parsed, "--product-id")
	if err != nil {
		return "", "", err
	}
	groupID, err := requiredParsedValue(parsed, "--group-id")
	if err != nil {
		return "", "", err
	}
	return productID, groupID, nil
}

func requiredRuleTable(parsed parsedArgs) (string, string, string, error) {
	productID, groupID, err := requiredRuleProductGroup(parsed)
	if err != nil {
		return "", "", "", err
	}
	tableID, err := requiredParsedValue(parsed, "--table-id")
	if err != nil {
		return "", "", "", err
	}
	return productID, groupID, tableID, nil
}

func ruleTableBasePath(productID, groupID string) string {
	return "/open-apis/rule_engine/v1/products/" + escapePathSegment(productID) + "/groups/" + escapePathSegment(groupID) + "/rule_tables"
}

func ruleTablePath(productID, groupID, tableID string) string {
	return ruleTableBasePath(productID, groupID) + "/" + escapePathSegment(tableID)
}

func escapePathSegment(value string) string {
	return url.PathEscape(strings.TrimSpace(value))
}
