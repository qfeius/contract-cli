package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"cn.qfei/contract-cli/internal/openplatform"
)

func (a *App) renderReviewValue(options commandOptions, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, openplatform.Response{Body: body})
}

func reviewJSONRequest(path string, value any) (openplatform.Request, error) {
	body, err := json.Marshal(value)
	return openplatform.Request{Method: http.MethodPost, Path: reviewPathPrefix + path, Body: body, IdentityPolicy: openplatform.IdentityPolicyUserOnly}, err
}

// Keep authentication and request logging on the existing shared HTTP path.
func (a *App) doReview(ctx context.Context, options commandOptions, request openplatform.Request) (openplatform.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client, rc, err := a.openPlatformClientAndContextForOptions(options, request.Path, openplatform.IdentityPolicyUserOnly)
	if err != nil {
		return openplatform.Response{}, err
	}
	response, err := client.Do(ctx, rc, request)
	if err != nil {
		return response, err
	}
	if len(bytes.TrimSpace(response.Body)) == 0 && response.StatusCode >= 200 && response.StatusCode < 300 {
		return response, nil
	}
	var envelope struct {
		Code json.Number `json:"code"`
	}
	if err := json.Unmarshal(response.Body, &envelope); err != nil {
		return response, fmt.Errorf("invalid review response: %w", err)
	}
	if envelope.Code != "" && envelope.Code != "0" && envelope.Code != "200" {
		a.logger.Error("review business request failed", "method", request.Method, "path", request.Path, "code", envelope.Code)
		return response, fmt.Errorf("review 请求失败 (%s %s, code %s)；请核对参数与权限", request.Method, request.Path, envelope.Code)
	}
	return response, nil
}

// Reuse the shared authentication, HTTP and renderer while limiting review policy to this tree.
func (a *App) executeReviewResponse(ctx context.Context, options commandOptions, request openplatform.Request) error {
	response, err := a.doReview(ctx, options, request)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) reviewDocument(ctx context.Context, options commandOptions, request openplatform.Request) (map[string]any, error) {
	response, err := a.doReview(ctx, options, request)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	decoder := json.NewDecoder(bytes.NewReader(response.Body))
	decoder.UseNumber()
	if err := decoder.Decode(&envelope); err != nil {
		return nil, fmt.Errorf("invalid review response data: %w", err)
	}
	if envelope.Data == nil {
		return nil, fmt.Errorf("review response missing data object")
	}
	return envelope.Data, nil
}

func (a *App) executeReview(ctx context.Context, parsed parsedArgs, request openplatform.Request, preview any) error {
	options := parseReviewOptions(parsed)
	if parsed.Bool("--dry-run") || parsed.Bool("--print-input") {
		return a.renderReviewValue(options, preview)
	}
	return a.executeReviewResponse(ctx, options, request)
}

func reviewUploadRequest(reader io.Reader, filename, name string, metadata ...map[string]string) (openplatform.Request, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if reader != nil {
		content, err := io.ReadAll(io.LimitReader(reader, reviewMaxUploadBytes+1))
		if err != nil {
			return openplatform.Request{}, err
		}
		if len(content) > reviewMaxUploadBytes {
			return openplatform.Request{}, fmt.Errorf("文件大小不能超过 50 MiB")
		}
		part, err := writer.CreateFormFile("file", filepath.Base(filename))
		if err != nil {
			return openplatform.Request{}, err
		}
		if _, err = part.Write(content); err != nil {
			return openplatform.Request{}, err
		}
	}
	if err := writer.WriteField("name", name); err != nil {
		return openplatform.Request{}, err
	}
	for _, fields := range metadata {
		keys := make([]string, 0, len(fields))
		for key := range fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := writer.WriteField(key, fields[key]); err != nil {
				return openplatform.Request{}, err
			}
		}
	}
	if err := writer.Close(); err != nil {
		return openplatform.Request{}, err
	}
	return openplatform.Request{Method: http.MethodPost, Path: reviewPathPrefix + "/file/upload", Headers: http.Header{"Content-Type": {writer.FormDataContentType()}}, Body: body.Bytes(), IdentityPolicy: openplatform.IdentityPolicyUserOnly}, nil
}

func (a *App) runReviewUpload(ctx context.Context, args []string) error {
	parsed, err := parseReviewArgs(args, []string{"--file", "--name", "--business-id", "--business-file-id", "--business-file-version", "--source-locator", "--source-revision-id", "--source-content-md5", "--channel-type"}, []string{"--stdin", "--dry-run", "--print-input"})
	if err != nil {
		return err
	}
	path, name := parsed.String("--file"), parsed.String("--name")
	if parsed.Bool("--stdin") && path != "" {
		return fmt.Errorf("--file 与 --stdin 只能选择一个")
	}
	if err := validateReviewName(name); err != nil {
		return err
	}
	stream := parsed.Bool("--stdin") || path != ""
	fields, err := reviewUploadFields(parsed, stream)
	if err != nil {
		return err
	}
	if !stream {
		preview := map[string]string{"name": name}
		for key, value := range fields {
			preview[key] = value
		}
		request, err := reviewUploadRequest(nil, "", name, fields)
		if err != nil {
			return err
		}
		return a.executeReview(ctx, parsed, request, preview)
	}
	source := path
	if parsed.Bool("--stdin") {
		source = "stdin"
	} else if err := validateReviewFile(path); err != nil {
		return err
	}
	if parsed.Bool("--dry-run") || parsed.Bool("--print-input") {
		preview := map[string]string{"file": source, "name": name}
		for key, value := range fields {
			preview[key] = value
		}
		return a.renderReviewValue(parseReviewOptions(parsed), preview)
	}
	reader := a.stdin
	filename := name
	if !parsed.Bool("--stdin") {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		reader = file
		filename = path
	}
	request, err := reviewUploadRequest(reader, filename, name, fields)
	if err != nil {
		return err
	}
	return a.executeReview(ctx, parsed, request, nil)
}

func (a *App) runReviewUploadURL(ctx context.Context, args []string) error {
	parsed, err := parseReviewArgs(args, []string{"--file-url", "--name"}, []string{"--dry-run", "--print-input"})
	if err != nil {
		return err
	}
	u, name := parsed.String("--file-url"), parsed.String("--name")
	if err := validateReviewURL(u); err != nil {
		return err
	}
	if err := validateReviewName(name); err != nil {
		return err
	}
	body := map[string]string{"fileUrl": u, "name": name}
	request, err := reviewJSONRequest("/file/uploadByUrl", body)
	if err != nil {
		return err
	}
	return a.executeReview(ctx, parsed, request, body)
}

func (a *App) runReviewSubject(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "extract" {
		return fmt.Errorf("expected review subject extract")
	}
	parsed, err := parseReviewArgs(args[1:], []string{"--business-id", "--file-id", "--file-hash"}, []string{"--dry-run", "--print-input"})
	if err != nil {
		return err
	}
	business := strings.TrimSpace(parsed.String("--business-id"))
	id, err := strconv.ParseInt(parsed.String("--file-id"), 10, 64)
	if business == "" || err != nil || id <= 0 {
		return fmt.Errorf("business-id 不能为空且 file-id 必须是大于 0 的整数")
	}
	body := map[string]any{"businessId": business, "fileId": strconv.FormatInt(id, 10)}
	if hash := strings.TrimSpace(parsed.String("--file-hash")); hash != "" {
		body["fileHash"] = strings.ToLower(hash)
	}
	request, err := reviewJSONRequest("/subjects", body)
	if err != nil {
		return err
	}
	return a.executeReview(ctx, parsed, request, body)
}

func (a *App) runReviewTask(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing review task subcommand")
	}
	if args[0] == "start" {
		parsed, err := parseReviewArgs(args[1:], []string{"--input", "--input-file", "--data"}, []string{"--dry-run", "--print-input"})
		if err != nil {
			return err
		}
		var input reviewStartInput
		if err := readReviewJSON(parsed, &input); err != nil {
			return err
		}
		payload, err := input.payload()
		if err != nil {
			return err
		}
		request, err := reviewJSONRequest("/startReview", payload)
		if err != nil {
			return err
		}
		return a.executeReview(ctx, parsed, request, payload)
	}
	if args[0] != "status" && args[0] != "info" && args[0] != "result" {
		return fmt.Errorf("unknown review task subcommand %q", args[0])
	}
	flags := []string{"--task-id", "--business-id", "--visibility-scope"}
	if args[0] == "result" {
		flags = append(flags, "--interval", "--deadline")
	}
	parsed, err := parseReviewArgs(args[1:], flags, nil)
	if err != nil {
		return err
	}
	query, err := reviewTaskValues(parsed)
	if err != nil {
		return err
	}
	if args[0] == "result" {
		interval, deadline, err := reviewDurations(parsed)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(ctx, deadline)
		defer cancel()
		result, err := a.waitReviewResult(ctx, parseReviewOptions(parsed), query, interval)
		return a.finishReviewWorkflow(parseReviewOptions(parsed), result, err)
	}
	response, err := a.doReview(ctx, parseReviewOptions(parsed), reviewQueryRequest("/"+args[0], query))
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(parseReviewOptions(parsed), response)
}

func reviewQueryRequest(path string, query url.Values) openplatform.Request {
	return openplatform.Request{Method: http.MethodGet, Path: reviewPathPrefix + path, Query: query, IdentityPolicy: openplatform.IdentityPolicyUserOnly, OperationKind: openplatform.OperationRead}
}

func (a *App) waitReviewResult(ctx context.Context, options commandOptions, query url.Values, interval time.Duration) (map[string]any, error) {
	var last map[string]any
	for {
		if err := ctx.Err(); err != nil {
			return last, err
		}
		snapshot, err := a.reviewDocument(ctx, options, reviewQueryRequest("/status", query))
		if err != nil {
			return last, err
		}
		last = snapshot
		status, _ := snapshot["status"].(string)
		a.logger.Info("review task status", "status", status)
		switch status {
		case "success":
			info, err := a.reviewDocument(ctx, options, reviewQueryRequest("/info", query))
			if err != nil {
				return last, err
			}
			for _, key := range []string{"url", "reviewDetailUrl", "review_detail_url", "detailUrl", "detail_url", "resultUrl", "result_url", "reportUrl", "report_url"} {
				if u, ok := info[key].(string); ok && validateReviewURL(strings.TrimSpace(u)) == nil {
					info["reviewDetailUrl"] = strings.TrimSpace(u)
					break
				}
			}
			return info, nil
		case "fail":
			return last, fmt.Errorf("审查任务失败")
		case "":
			return last, fmt.Errorf("任务不存在或当前用户无权访问")
		case "running":
		default:
			return last, fmt.Errorf("未知审查任务状态 %q", status)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return last, ctx.Err()
		case <-timer.C:
		}
	}
}

func (a *App) finishReviewWorkflow(options commandOptions, result any, workflowErr error) error {
	if err := a.renderReviewValue(options, result); err != nil {
		if workflowErr != nil {
			return fmt.Errorf("%w; output failed: %v", workflowErr, err)
		}
		return err
	}
	return workflowErr
}

func reviewResponseID(document map[string]any, key string) (int64, error) {
	var text string
	switch v := document[key].(type) {
	case string:
		text = v
	case json.Number:
		text = string(v)
	}
	id, err := strconv.ParseInt(text, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("响应缺少有效 %s", key)
	}
	return id, nil
}

func (a *App) runReviewWorkflow(ctx context.Context, args []string) error {
	parsed, err := parseReviewArgs(args, []string{"--input", "--input-file", "--data", "--interval", "--deadline"}, []string{"--dry-run", "--print-input"})
	if err != nil {
		return err
	}
	var input reviewRunInput
	if err := readReviewJSON(parsed, &input); err != nil {
		return err
	}
	interval, deadline, err := reviewDurations(parsed)
	if err != nil {
		return err
	}
	config, err := normalizedReviewConfig(input.Config)
	if err != nil {
		return err
	}
	if err := validateReviewName(input.Source.Name); err != nil {
		return err
	}
	switch input.Source.Type {
	case "file":
		if input.Source.FileURL != "" {
			return fmt.Errorf("file source 不支持 fileUrl")
		}
		if err := validateReviewFile(input.Source.Path); err != nil {
			return err
		}
	case "url":
		if input.Source.Path != "" {
			return fmt.Errorf("url source 不支持 path")
		}
		if err := validateReviewURL(input.Source.FileURL); err != nil {
			return err
		}
	default:
		return fmt.Errorf("source.type 必须为 file 或 url")
	}
	input.BusinessID = strings.TrimSpace(input.BusinessID)
	if input.FileHash != "" {
		hash, err := normalizedReviewHash(input.FileHash)
		if err != nil {
			return err
		}
		input.FileHash = hash
	}
	options := parseReviewOptions(parsed)
	if parsed.Bool("--dry-run") || parsed.Bool("--print-input") {
		// Run previews show normalized workflow input; enum conversion belongs to startReview.
		strength := input.Config["reviewStrength"]
		if text, ok := strength.(string); ok {
			strength = strings.TrimSpace(text)
		}
		input.Config = make(map[string]any, len(config))
		for key, value := range config {
			input.Config[key] = value
		}
		input.Config["reviewStrength"] = strength
		return a.renderReviewValue(options, input)
	}
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	a.logger.Info("review workflow started", "source_type", input.Source.Type, "wait", input.Wait)
	var request openplatform.Request
	if input.Source.Type == "file" {
		file, openErr := os.Open(input.Source.Path)
		if openErr != nil {
			return openErr
		}
		request, err = reviewUploadRequest(file, input.Source.Path, input.Source.Name)
		_ = file.Close()
	} else {
		request, err = reviewJSONRequest("/file/uploadByUrl", map[string]string{"fileUrl": input.Source.FileURL, "name": input.Source.Name})
	}
	if err != nil {
		return err
	}
	result, workflowErr := a.performReviewWorkflow(ctx, options, input, config, request, interval)
	if workflowErr != nil {
		a.logger.Error("review workflow failed")
	} else {
		a.logger.Info("review workflow completed")
	}
	return a.finishReviewWorkflow(options, result, workflowErr)
}

func (a *App) performReviewWorkflow(ctx context.Context, options commandOptions, input reviewRunInput, config map[string]any, uploadRequest openplatform.Request, interval time.Duration) (map[string]any, error) {
	result := map[string]any{}
	upload, err := a.reviewDocument(ctx, options, uploadRequest)
	if err != nil {
		return result, err
	}
	result["upload"] = upload
	fileID, err := reviewResponseID(upload, "fileId")
	if err != nil {
		return result, err
	}
	business, _ := upload["businessId"].(string)
	if strings.TrimSpace(business) == "" {
		business = input.BusinessID
	}
	hash, _ := upload["fileHash"].(string)
	if strings.TrimSpace(hash) == "" {
		hash = input.FileHash
	}
	payload, err := (reviewStartInput{BusinessID: business, FileID: fileID, FileHash: hash, Config: config}).payload()
	if err != nil {
		return result, err
	}
	if input.ExtractSubjects {
		request, err := reviewJSONRequest("/subjects", map[string]any{"businessId": payload["businessId"], "fileId": payload["fileId"], "fileHash": payload["fileHash"]})
		if err != nil {
			return result, err
		}
		subjects, err := a.reviewDocument(ctx, options, request)
		if err != nil {
			return result, err
		}
		result["subjects"] = subjects
	}
	request, err := reviewJSONRequest("/startReview", payload)
	if err != nil {
		return result, err
	}
	start, err := a.reviewDocument(ctx, options, request)
	if err != nil {
		return result, err
	}
	result["start"] = start
	if !input.Wait {
		return result, nil
	}
	taskID, err := reviewResponseID(start, "taskId")
	if err != nil {
		return result, err
	}
	final, err := a.waitReviewResult(ctx, options, url.Values{"taskId": {strconv.FormatInt(taskID, 10)}, "businessId": {payload["businessId"].(string)}}, interval)
	if final != nil {
		result["final"] = final
	}
	return result, err
}
