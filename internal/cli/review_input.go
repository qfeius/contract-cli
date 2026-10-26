package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const reviewMaxJSONBytes = 2 << 20
const reviewMaxUploadBytes = 50 << 20

// Contract CLI usage attribution is fixed at the request boundary, never caller controlled.
const reviewUsageReportBusinessCode = "clm_100_openApi_cli"

func reviewValueFlags(extra ...string) map[string]struct{} {
	flags := commonValueFlags(extra...)
	flags["--input"] = struct{}{}
	return flags
}

func parseReviewOptions(parsed parsedArgs) commandOptions {
	options := parseCommandOptions(parsed)
	if options.inputFile == "" {
		options.inputFile = parsed.String("--input")
	}
	return options
}

type reviewStartInput struct {
	BusinessID string         `json:"businessId"`
	FileID     int64          `json:"fileId"`
	FileHash   string         `json:"fileHash"`
	Config     map[string]any `json:"config"`
}

type reviewRunSource struct {
	Type    string `json:"type"`
	Path    string `json:"path,omitempty"`
	FileURL string `json:"fileUrl,omitempty"`
	Name    string `json:"name"`
}

type reviewRunInput struct {
	Source          reviewRunSource `json:"source"`
	BusinessID      string          `json:"businessId,omitempty"`
	FileHash        string          `json:"fileHash,omitempty"`
	Config          map[string]any  `json:"config"`
	ExtractSubjects bool            `json:"extractSubjects,omitempty"`
	Wait            bool            `json:"wait"`
}

// review JSON decoding is deliberately local: legacy commands keep their input rules.
func readReviewJSON(parsed parsedArgs, target any) error {
	count := 0
	for _, flag := range []string{"--input", "--input-file", "--data"} {
		if parsed.String(flag) != "" {
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("必须且只能提供 --input（兼容 --input-file）或 --data 之一")
	}
	options := parseReviewOptions(parsed)
	var reader io.Reader = strings.NewReader(options.data)
	if options.inputFile != "" {
		file, err := os.Open(options.inputFile)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		reader = file
	}
	body, err := io.ReadAll(io.LimitReader(reader, reviewMaxJSONBytes+1))
	if err != nil {
		return err
	}
	if len(body) > reviewMaxJSONBytes {
		return fmt.Errorf("JSON 输入不能超过 2 MiB")
	}
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return err
	}
	if containsReviewNull(raw) {
		return fmt.Errorf("审查输入字段不能为 null")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("解析审查输入: %w", err)
	}
	return nil
}

func containsReviewNull(value any) bool {
	if value == nil {
		return true
	}
	switch v := value.(type) {
	case map[string]any:
		for _, item := range v {
			if containsReviewNull(item) {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if containsReviewNull(item) {
				return true
			}
		}
	}
	return false
}

func normalizedReviewConfig(input map[string]any) (map[string]any, error) {
	allowed := map[string]bool{"selectedPosition": true, "selectedAuditRole": true, "reviewStrength": true, "selectedCheckListIds": true, "matchContractTypeRulePackage": true}
	result := make(map[string]any, len(input))
	for key, value := range input {
		if !allowed[key] {
			return nil, fmt.Errorf("未知审查配置字段 config.%s", key)
		}
		result[key] = value
	}
	for _, key := range []string{"selectedPosition", "selectedAuditRole"} {
		text, ok := result[key].(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("config.%s 必须是非空字符串", key)
		}
		result[key] = strings.TrimSpace(text)
	}
	strength := -1
	switch value := result["reviewStrength"].(type) {
	case string:
		switch strings.TrimSpace(value) {
		case "弱势":
			strength = 0
		case "中立":
			strength = 1
		case "强势":
			strength = 2
		}
	case json.Number:
		n, err := strconv.ParseFloat(string(value), 64)
		if err == nil && (n == 0 || n == 1 || n == 2) {
			strength = int(n)
		}
	case int:
		strength = value
	}
	if strength < 0 || strength > 2 {
		return nil, fmt.Errorf("config.reviewStrength 必须是弱势、中立、强势或 0、1、2")
	}
	result["reviewStrength"] = strength
	selected := false
	if value, exists := result["selectedCheckListIds"]; exists {
		ids, ok := value.([]any)
		if !ok || len(ids) == 0 {
			return nil, fmt.Errorf("config.selectedCheckListIds 必须是非空字符串数组")
		}
		normalized := make([]any, len(ids))
		for i, id := range ids {
			text, ok := id.(string)
			if !ok || strings.TrimSpace(text) == "" {
				return nil, fmt.Errorf("清单 ID 不能为空")
			}
			normalized[i] = strings.TrimSpace(text)
		}
		result["selectedCheckListIds"] = normalized
		selected = true
	}
	matched := false
	if value, exists := result["matchContractTypeRulePackage"]; exists {
		var ok bool
		matched, ok = value.(bool)
		if !ok {
			return nil, fmt.Errorf("matchContractTypeRulePackage 必须是 boolean")
		}
	}
	if !selected && !matched {
		return nil, fmt.Errorf("至少提供一种规则来源：非空 selectedCheckListIds，或 matchContractTypeRulePackage=true")
	}
	return result, nil
}

func normalizedReviewHash(hash string) (string, error) {
	hash = strings.ToLower(strings.TrimSpace(hash))
	_, err := hex.DecodeString(hash)
	if len(hash) != 64 || err != nil {
		return "", fmt.Errorf("fileHash 必须是 64 位 SHA-256 十六进制字符串")
	}
	return hash, nil
}

func (input reviewStartInput) payload() (map[string]any, error) {
	business := strings.TrimSpace(input.BusinessID)
	if business == "" || input.FileID <= 0 {
		return nil, fmt.Errorf("businessId 不能为空且 fileId 必须大于 0")
	}
	hash, err := normalizedReviewHash(input.FileHash)
	if err != nil {
		return nil, err
	}
	config, err := normalizedReviewConfig(input.Config)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"businessId": business, "fileId": strconv.FormatInt(input.FileID, 10), "fileHash": hash, "config": config,
		"usageReportContext": map[string]string{"reportBusinessCode": reviewUsageReportBusinessCode},
	}, nil
}

func validateReviewName(name string) error {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".doc", ".docx", ".pdf":
		return nil
	default:
		return fmt.Errorf("name 仅支持 .doc、.docx 和 .pdf 文件")
	}
}
func validateReviewURL(value string) error {
	u, err := url.ParseRequestURI(value)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("file-url 必须是完整的 http/https URL")
	}
	return nil
}
func validateReviewFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("--file 不能为空")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("file 必须是普通文件")
	}
	if info.Size() > reviewMaxUploadBytes {
		return fmt.Errorf("文件大小不能超过 50 MiB")
	}
	return nil
}

func parseReviewArgs(args []string, values, booleans []string) (parsedArgs, error) {
	flags := map[string]struct{}{"--profile": {}, "--as": {}, "--output": {}}
	for _, flag := range values {
		flags[flag] = struct{}{}
	}
	parsed, err := parseArgs(args, flags, commonBoolFlags(booleans...))
	if err != nil {
		return parsed, err
	}
	if len(parsed.positionals) > 0 {
		return parsed, fmt.Errorf("unexpected positional arguments")
	}
	return parsed, nil
}

func reviewTaskValues(parsed parsedArgs) (url.Values, error) {
	id, err := strconv.ParseInt(parsed.String("--task-id"), 10, 64)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("task-id 必须是大于 0 的整数")
	}
	query := url.Values{"taskId": {strconv.FormatInt(id, 10)}}
	business := strings.TrimSpace(parsed.String("--business-id"))
	scope := strings.TrimSpace(parsed.String("--visibility-scope"))
	if business != "" {
		query.Set("businessId", business)
	}
	if scope != "" {
		if scope != "contractResult" {
			return nil, fmt.Errorf("visibility-scope 必须是 contractResult")
		}
		if business == "" {
			return nil, fmt.Errorf("visibility-scope=contractResult 必须提供 business-id")
		}
		query.Set("visibilityScope", scope)
	}
	return query, nil
}

func reviewDurations(parsed parsedArgs) (time.Duration, time.Duration, error) {
	durations := []time.Duration{2 * time.Second, 10 * time.Minute}
	for i, flag := range []string{"--interval", "--deadline"} {
		if parsed.HasValue(flag) {
			d, err := time.ParseDuration(parsed.String(flag))
			if err != nil || d <= 0 {
				return 0, 0, fmt.Errorf("%s 必须是正时长，例如 2s、10m", flag)
			}
			durations[i] = d
		}
	}
	return durations[0], durations[1], nil
}

// Match Cobra StringSlice semantics, including quoted CSV values such as '"name,desc"'.
func reviewCSV(parsed parsedArgs, flag string) ([]string, error) {
	var values []string
	for _, raw := range parsed.Strings(flag) {
		if raw == "" {
			continue
		}
		items, err := csv.NewReader(strings.NewReader(raw)).Read()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", flag, err)
		}
		values = append(values, items...)
	}
	return values, nil
}

func reviewListValues(parsed parsedArgs, checklist bool) (url.Values, error) {
	query := url.Values{}
	numbers := map[string]int64{}
	flags := []string{"--page-index", "--page-size"}
	if checklist {
		flags = append(flags, "--start-time", "--end-time")
	}
	keys := map[string]string{"--page-index": "pageIndex", "--page-size": "pageSize", "--start-time": "startTime", "--end-time": "endTime"}
	for _, flag := range flags {
		if !parsed.HasValue(flag) {
			continue
		}
		n, err := strconv.ParseInt(parsed.String(flag), 10, 64)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("%s 必须是非负整数", flag)
		}
		numbers[flag] = n
		if n > 0 {
			query.Set(keys[flag], strconv.FormatInt(n, 10))
		}
	}
	if numbers["--start-time"] > 0 && numbers["--end-time"] > 0 && numbers["--start-time"] > numbers["--end-time"] {
		return nil, fmt.Errorf("start-time 不能晚于 end-time")
	}
	lists := map[string]string{"--sort": "sort"}
	if checklist {
		lists["--review-stage"] = "reviewStages"
		lists["--contract-category"] = "contractCategories"
		lists["--create-employee-id"] = "createEmployeeIds"
		lists["--update-employee-id"] = "updateEmployeeIds"
		if name := parsed.String("--name"); name != "" {
			query.Set("name", name)
		}
		if parsed.HasBool("--enabled") {
			query.Set("enabled", strconv.FormatBool(parsed.Bool("--enabled")))
		}
	}
	for flag, key := range lists {
		values, err := reviewCSV(parsed, flag)
		if err != nil {
			return nil, err
		}
		for _, value := range values {
			query.Add(key, value)
		}
	}
	return query, nil
}
