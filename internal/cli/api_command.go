package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
	"cn.qfei/contract-cli/internal/output"
)

type apiCallOptions struct {
	profileName string
	identity    string
	filePath    string
	data        string
	output      output.Format
	raw         bool
	headers     http.Header
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
	if options.filePath != "" && options.data != "" {
		return fmt.Errorf("only one of --file or --data may be provided")
	}
	if strings.Contains(path, "://") || !strings.HasPrefix(path, "/open-apis/") {
		return fmt.Errorf("open platform path must be a relative /open-apis/ path")
	}

	profile, err := a.store.GetProfile(options.profileName)
	if err != nil {
		return err
	}

	identity, err := config.ParseIdentityKind(options.identity)
	if err != nil {
		return err
	}
	if strings.TrimSpace(options.identity) == "" {
		identity = defaultIdentity(profile)
	}

	body, err := a.resolveAPICallBody(options)
	if err != nil {
		return err
	}

	client := openplatform.New(openplatform.Options{
		HTTPClient: a.httpClient,
		Logger:     a.logger,
	})
	requestContext, err := client.RequestContext(profile, identity)
	if err != nil {
		return err
	}

	response, err := client.Do(ctx, requestContext, openplatform.Request{
		Method:  method,
		Path:    path,
		Headers: options.headers,
		Body:    body,
		Raw:     options.raw,
	})
	if err != nil {
		return err
	}

	renderer := output.NewRenderer(a.stdout)
	if options.raw {
		return renderer.RenderRaw(response.Body)
	}
	if len(response.Body) == 0 {
		return nil
	}
	return renderer.Render(options.output, response.Body)
}

func parseAPICallArgs(args []string) (apiCallOptions, string, string, error) {
	options := apiCallOptions{
		output:  output.FormatJSON,
		headers: make(http.Header),
	}

	var method string
	var path string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			switch {
			case method == "":
				method = arg
			case path == "":
				path = arg
			default:
				return apiCallOptions{}, "", "", fmt.Errorf("unexpected extra argument %q", arg)
			}
			continue
		}

		name, inlineValue, hasInlineValue := strings.Cut(arg, "=")
		switch name {
		case "--raw":
			options.raw = true
		case "--profile":
			value, err := apiFlagValue(args, &i, inlineValue, hasInlineValue)
			if err != nil {
				return apiCallOptions{}, "", "", err
			}
			options.profileName = value
		case "--as":
			value, err := apiFlagValue(args, &i, inlineValue, hasInlineValue)
			if err != nil {
				return apiCallOptions{}, "", "", err
			}
			options.identity = value
		case "--file":
			value, err := apiFlagValue(args, &i, inlineValue, hasInlineValue)
			if err != nil {
				return apiCallOptions{}, "", "", err
			}
			options.filePath = value
		case "--data":
			value, err := apiFlagValue(args, &i, inlineValue, hasInlineValue)
			if err != nil {
				return apiCallOptions{}, "", "", err
			}
			options.data = value
		case "--output":
			value, err := apiFlagValue(args, &i, inlineValue, hasInlineValue)
			if err != nil {
				return apiCallOptions{}, "", "", err
			}
			options.output = output.Format(value)
		case "--header":
			value, err := apiFlagValue(args, &i, inlineValue, hasInlineValue)
			if err != nil {
				return apiCallOptions{}, "", "", err
			}
			key, headerValue, ok := strings.Cut(value, ":")
			if !ok || strings.TrimSpace(key) == "" {
				return apiCallOptions{}, "", "", fmt.Errorf("invalid --header %q; expected Key: Value", value)
			}
			options.headers.Add(strings.TrimSpace(key), strings.TrimSpace(headerValue))
		default:
			return apiCallOptions{}, "", "", fmt.Errorf("unknown flag %q", name)
		}
	}

	if method == "" || path == "" {
		return apiCallOptions{}, "", "", fmt.Errorf("usage: contract-cli api call [flags] <METHOD> <PATH>")
	}
	return options, method, path, nil
}

func apiFlagValue(args []string, index *int, inlineValue string, hasInlineValue bool) (string, error) {
	if hasInlineValue {
		return inlineValue, nil
	}
	if *index+1 >= len(args) {
		return "", fmt.Errorf("missing value for %s", args[*index])
	}
	*index += 1
	return args[*index], nil
}

func (a *App) resolveAPICallBody(options apiCallOptions) ([]byte, error) {
	switch {
	case options.filePath != "":
		body, err := os.ReadFile(options.filePath)
		if err != nil {
			return nil, fmt.Errorf("read api request body file: %w", err)
		}
		return body, nil
	case options.data != "":
		return []byte(options.data), nil
	default:
		return nil, nil
	}
}
