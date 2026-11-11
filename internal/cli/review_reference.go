package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Validate reference metadata before authentication or remote requests. Values are
// strings throughout: business IDs and content fingerprints are not JSON floats.
func reviewUploadFields(parsed parsedArgs) (map[string]string, error) {
	fields := make(map[string]string)
	for _, field := range []struct{ flag, wire string }{
		{"--business-id", "businessId"}, {"--business-file-id", "businessFileId"},
		{"--business-file-version", "businessFileVersion"}, {"--source-locator", "sourceLocator"},
		{"--source-revision-id", "sourceRevisionId"}, {"--source-content-md5", "sourceContentMd5"},
		{"--channel-type", "channelType"},
	} {
		value := parsed.String(field.flag)
		if value != "" {
			fields[field.wire] = value
		}
	}
	for _, required := range []struct{ flag, wire string }{{"--business-id", "businessId"}, {"--business-file-id", "businessFileId"}, {"--business-file-version", "businessFileVersion"}, {"--source-locator", "sourceLocator"}} {
		if strings.TrimSpace(fields[required.wire]) == "" {
			return nil, fmt.Errorf("引用上传必须提供 %s；本地文件请使用 contract-cli review pre-audit prepare", required.flag)
		}
	}
	if utf8.RuneCountInString(fields["businessFileVersion"]) > 64 {
		return nil, fmt.Errorf("--business-file-version 最多 64 字符")
	}
	if utf8.RuneCountInString(fields["sourceRevisionId"]) > 128 {
		return nil, fmt.Errorf("--source-revision-id 最多 128 字符")
	}
	if err := validateReviewSourceLocator(fields["sourceLocator"]); err != nil {
		return nil, err
	}
	return fields, nil
}

func validateReviewSourceLocator(value string) error {
	if len(value) > 4096 {
		return fmt.Errorf("--source-locator 最多 4096 UTF-8 字节")
	}
	var locator map[string]json.RawMessage
	if err := json.Unmarshal([]byte(value), &locator); err != nil || locator == nil {
		return fmt.Errorf("--source-locator 必须为 JSON 对象")
	}
	var sourceType int
	raw, ok := locator["sourceFileType"]
	if !ok || string(raw) == "null" || json.Unmarshal(raw, &sourceType) != nil || sourceType < 0 || sourceType > 2 {
		return fmt.Errorf("sourceLocator.sourceFileType 必须为整数 0、1 或 2")
	}
	for key, raw := range locator {
		switch key {
		case "sourceFileType":
		case "provider", "cooperationId", "cooperationFileId":
			var value string
			if string(raw) == "null" || json.Unmarshal(raw, &value) != nil {
				return fmt.Errorf("sourceLocator.%s 必须为字符串", key)
			}
		default:
			return fmt.Errorf("sourceLocator 不支持字段 %s", key)
		}
	}
	return nil
}

// This compatibility reader does not widen the identity policy of historical
// contract cooperation commands, nor select a file or create a review task.
func (a *App) runReviewCooperation(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "get" {
		return fmt.Errorf("expected review cooperation get")
	}
	parsed, err := parseReviewArgs(args[1:], []string{"--cooperation-id"}, nil)
	if err != nil {
		return err
	}
	request, err := cooperationDetailRequest(parsed.String("--cooperation-id"))
	if err != nil {
		return err
	}
	return a.executeReviewResponse(ctx, parseReviewOptions(parsed), request)
}
