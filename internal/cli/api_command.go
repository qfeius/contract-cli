package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"cn.qfei/contract-cli/internal/openplatform"
)

type apiCallOptions struct {
	commandOptions
	channel string
	headers http.Header
	query   url.Values
}

const maxOpenWebBodyBytes = 1024 * 1024

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
		a.logger.Error("parse api call failed")
		return err
	}
	a.logger.Info("api call requested", "channel", options.channel, "method", method, "path", path)

	if options.channel == "" {
		err = fmt.Errorf("--channel web is required")
		a.logger.Error("api call channel rejected", "method", method, "path", path, "error", err.Error())
		return err
	}
	if options.channel != "web" {
		err = fmt.Errorf("unsupported api channel %q; only web is available", options.channel)
		a.logger.Error("api call channel rejected", "channel", options.channel, "method", method, "path", path, "error", err.Error())
		return err
	}

	normalizedMethod, err := normalizeOpenWebMethod(method)
	if err != nil {
		a.logger.Error("api call method rejected", "channel", options.channel, "method", method, "path", path, "error", err.Error())
		return err
	}
	method = normalizedMethod
	if err = openplatform.ValidateOpenWebPath(path); err != nil {
		a.logger.Error("api call path rejected", "channel", options.channel, "method", method, "path", path, "error", err.Error())
		return err
	}

	body, err := resolveRawBody(ctx, options.commandOptions)
	if err != nil {
		a.logger.Error("resolve api call body failed", "channel", options.channel, "method", method, "path", path, "error", err.Error())
		return err
	}
	if len(body) > maxOpenWebBodyBytes {
		err = fmt.Errorf("open web request body exceeds %d bytes", maxOpenWebBodyBytes)
		a.logger.Error("api call body rejected", "channel", options.channel, "method", method, "path", path, "body_bytes", len(body), "error", err.Error())
		return err
	}
	a.logger.Info("dispatch api web call", "method", method, "path", path, "query_keys", len(options.query), "body_bytes", len(body))

	request := openplatform.Request{
		Method:         method,
		Path:           path,
		Query:          options.query,
		Headers:        options.headers,
		Body:           body,
		Raw:            options.raw,
		IdentityPolicy: openplatform.IdentityPolicyUserOnly,
	}
	if err = a.executeOpenPlatformCommandWithoutCommonQuery(ctx, options.commandOptions, request); err != nil {
		a.logger.Error("api web call failed", "method", method, "path", path)
		return err
	}
	return nil
}

func parseAPICallArgs(args []string) (apiCallOptions, string, string, error) {
	parsed, err := parseArgs(args, commonValueFlags("--channel", "--header", "--query"), commonBoolFlags())
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
		key = strings.TrimSpace(key)
		if isManagedOpenWebHeader(key) {
			return apiCallOptions{}, "", "", fmt.Errorf("header %q is managed by contract-cli and cannot be passed with --header", key)
		}
		headers.Add(key, strings.TrimSpace(headerValue))
	}

	query := make(url.Values)
	for _, value := range parsed.Strings("--query") {
		key, queryValue, ok := strings.Cut(value, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return apiCallOptions{}, "", "", fmt.Errorf("invalid --query %q; expected key=value", value)
		}
		query.Add(key, queryValue)
	}

	return apiCallOptions{
		commandOptions: parseCommandOptions(parsed),
		channel:        strings.TrimSpace(parsed.String("--channel")),
		headers:        headers,
		query:          query,
	}, parsed.positionals[0], parsed.positionals[1], nil
}

func normalizeOpenWebMethod(method string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(method))
	switch normalized {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported Open Web method %q", method)
	}
}

func isManagedOpenWebHeader(header string) bool {
	normalized := strings.ToLower(strings.TrimSpace(header))
	switch normalized {
	case "authorization", "cookie", "proxy-authorization",
		"z-tenant-id", "z-user-id", "x-gate-auth", "sourcetype",
		"forwarded", "x-real-ip",
		"x-qfei-channel-type", "x-qfei-agent-source-type", "x-qfei-product-code":
		return true
	default:
		return strings.HasPrefix(normalized, "x-forwarded-")
	}
}
