package cli

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"cn.qfei/contract-cli/internal/openplatform"
)

type apiCallOptions struct {
	commandOptions
	headers http.Header
}

func (a *App) runAPI(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing api subcommand")
	}

	switch args[0] {
	case "call":
		return a.runAPICall(ctx, args[1:])
	default:
		return fmt.Errorf("unknown api subcommand %q", args[0])
	}
}

func (a *App) runAPICall(ctx context.Context, args []string) error {
	options, method, path, err := parseAPICallArgs(args)
	if err != nil {
		return err
	}
	if strings.Contains(path, "://") || !strings.HasPrefix(path, "/open-apis/") {
		return fmt.Errorf("open platform path must be a relative /open-apis/ path")
	}

	body, err := resolveRawBody(options.commandOptions)
	if err != nil {
		return err
	}

	request := openplatform.Request{
		Method:         method,
		Path:           path,
		Headers:        options.headers,
		Body:           body,
		Raw:            options.raw,
		IdentityPolicy: openplatform.IdentityPolicyForPath(path),
	}
	return a.executeOpenPlatformCommand(ctx, options.commandOptions, request)
}

func parseAPICallArgs(args []string) (apiCallOptions, string, string, error) {
	parsed, err := parseArgs(args, structuredValueFlags("--header"), commonBoolFlags())
	if err != nil {
		return apiCallOptions{}, "", "", err
	}
	if len(parsed.positionals) != 2 {
		return apiCallOptions{}, "", "", fmt.Errorf("usage: contract-cli api call [flags] <METHOD> <PATH>")
	}

	headers := make(http.Header)
	for _, value := range parsed.Strings("--header") {
		key, headerValue, ok := strings.Cut(value, ":")
		if !ok || strings.TrimSpace(key) == "" {
			return apiCallOptions{}, "", "", fmt.Errorf("invalid --header %q; expected Key: Value", value)
		}
		headers.Add(strings.TrimSpace(key), strings.TrimSpace(headerValue))
	}

	return apiCallOptions{
		commandOptions: parseCommandOptions(parsed),
		headers:        headers,
	}, parsed.positionals[0], parsed.positionals[1], nil
}
