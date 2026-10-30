package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"cn.qfei/contract-cli/internal/openplatform"
	contractsvc "cn.qfei/contract-cli/internal/openplatform/contract"
)

const preAuditPath = "/open-apis/contract/v1/pre_audits"

// Claims are only a local recovery-account binding. Server-side verification
// remains authoritative; no signature or authorization is inferred here.
func preAuditTokenIdentity(token string) (tenant, user string, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("预审恢复需要有效登录JWT身份，请重新登录")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("无法解析登录JWT身份")
	}
	var claims struct {
		Issuer   string `json:"iss"`
		UserInfo struct {
			Tenant json.Number `json:"tenantId"`
			User   json.Number `json:"userId"`
		} `json:"userInfo"`
	}
	if err := json.Unmarshal(body, &claims); err != nil || claims.Issuer != "common-organization-v2" {
		return "", "", fmt.Errorf("登录JWT缺少受支持的用户/租户身份")
	}
	for _, id := range []json.Number{claims.UserInfo.Tenant, claims.UserInfo.User} {
		if v, err := strconv.ParseInt(string(id), 10, 64); err != nil || v <= 0 {
			return "", "", fmt.Errorf("登录JWT用户/租户标识无效")
		}
	}
	return string(claims.UserInfo.Tenant), string(claims.UserInfo.User), nil
}

func (a *App) newPreAuditSession(ctx context.Context, options commandOptions, run *preAuditRun) (*reviewRequestSession, error) {
	client, rc, err := a.openPlatformClientAndContextForOptions(options, preAuditPath, openplatform.IdentityPolicyUserOnly)
	if err != nil {
		return nil, err
	}
	if rc.PrepareAccessToken != nil {
		rc.AccessToken, err = rc.PrepareAccessToken(ctx, rc.AccessToken)
		if err != nil {
			return nil, err
		}
	}
	tenant, user, err := preAuditTokenIdentity(rc.AccessToken)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(rc.BaseURL)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("无效API origin")
	}
	origin := u.Scheme + "://" + strings.ToLower(u.Host)
	if run.Profile == "" {
		run.Profile, run.Origin, run.TenantID, run.UserID = rc.Profile.Name, origin, tenant, user
	} else if run.Profile != rc.Profile.Name || run.Origin != origin || run.TenantID != tenant || run.UserID != user {
		return nil, fmt.Errorf("当前profile/API origin/登录用户或租户与原run不一致，已停止")
	}
	guard := func(next func(context.Context, string) (string, error)) func(context.Context, string) (string, error) {
		return func(ctx context.Context, current string) (string, error) {
			token := current
			var err error
			if next != nil {
				token, err = next(ctx, current)
				if err != nil {
					return "", err
				}
			}
			tenant, user, err := preAuditTokenIdentity(token)
			if err != nil || tenant != run.TenantID || user != run.UserID {
				return "", fmt.Errorf("实际发送的登录身份与原run不一致，已停止请求")
			}
			return token, nil
		}
	}
	rc.PrepareAccessToken = guard(rc.PrepareAccessToken)
	if rc.RefreshAccessToken != nil {
		rc.RefreshAccessToken = guard(rc.RefreshAccessToken)
	}
	return &reviewRequestSession{client: client, rc: rc}, nil
}

func preAuditResponseData(response openplatform.Response) (map[string]any, error) {
	var envelope struct {
		Code json.Number    `json:"code"`
		Data map[string]any `json:"data"`
	}
	decoder := json.NewDecoder(bytes.NewReader(response.Body))
	decoder.UseNumber()
	if err := decoder.Decode(&envelope); err != nil {
		return nil, fmt.Errorf("预审接口响应格式无效")
	}
	if envelope.Code != "0" && envelope.Code != "200" {
		return nil, fmt.Errorf("预审请求失败(code=%s)，请核对参数、权限与依赖版本", envelope.Code)
	}
	if envelope.Data == nil {
		return nil, fmt.Errorf("预审接口缺少data对象")
	}
	return envelope.Data, nil
}

func (a *App) runPreAudit(ctx context.Context, args []string) (err error) {
	defer func() {
		if err != nil {
			a.logger.Error("pre-audit command stopped")
		}
	}()
	if len(args) == 0 {
		return fmt.Errorf("需要review pre-audit prepare/create/start/resume")
	}
	a.logger.Info("pre-audit command requested", "action", args[0])
	if args[0] == "create" {
		return a.runPreAuditCreate(ctx, args[1:])
	}
	if args[0] == "start" || args[0] == "resume" {
		return a.runPreAuditReview(ctx, args[0], args[1:])
	}
	if args[0] != "prepare" {
		return fmt.Errorf("unknown review pre-audit subcommand %q", args[0])
	}
	parsed, err := parseReviewArgs(args[1:], []string{"--file", "--name"}, []string{"--dry-run", "--print-input"})
	if err != nil {
		return err
	}
	if err := validateReviewFile(parsed.String("--file")); err != nil {
		return err
	}
	if err := validateReviewName(parsed.String("--name")); err != nil {
		return err
	}
	options := parseReviewOptions(parsed)
	if parsed.Bool("--dry-run") || parsed.Bool("--print-input") {
		return a.renderReviewValue(options, map[string]string{"operation": "pre-audit.prepare", "name": parsed.String("--name"), "nextStage": "awaiting_category"})
	}
	run, err := a.preparePreAudit(ctx, options, parsed.String("--file"), parsed.String("--name"))
	return a.finishReviewWorkflow(options, preAuditOutput(run), err)
}

func preAuditOutput(run *preAuditRun) map[string]any {
	result := map[string]any{}
	if run == nil {
		return result
	}
	result["runId"], result["workflowStage"] = run.RunID, run.Stage
	for key, value := range map[string]string{"sourceFileId": run.SourceFileID, "categoryId": run.CategoryID, "preAuditId": run.PreAuditID, "latestFileId": run.LatestFileID, "fileId": run.AIFileID, "fileHash": run.FileHash, "taskId": run.TaskID} {
		if value != "" {
			result[key] = value
		}
	}
	for key, value := range map[string]map[string]any{"upload": run.Upload, "subjects": run.Subjects, "start": run.Start, "final": run.Final} {
		if value != nil {
			result[key] = value
		}
	}
	return result
}

func (a *App) preparePreAudit(ctx context.Context, options commandOptions, path, name string) (*preAuditRun, error) {
	a.logger.Info("pre-audit preparation started")
	runID, err := newPreAuditID()
	if err != nil {
		return nil, err
	}
	run := &preAuditRun{SchemaVersion: 1, RunID: runID, Stage: "upload_unknown", Name: name}
	session, err := a.newPreAuditSession(ctx, options, run)
	if err != nil {
		a.logger.Error("pre-audit account binding failed")
		return nil, err
	}
	store := a.preAuditStore()
	release, err := store.lock(runID)
	if err != nil {
		return nil, err
	}
	defer release()
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	if err := store.save(run); err != nil {
		return run, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	response, err := contractsvc.NewService(session.client).UploadFile(ctx, session.rc, contractsvc.UploadFileInput{FileName: name, FileType: "text", File: file})
	if err != nil {
		a.logger.Error("pre-audit CLM upload failed", "run_id", runID)
		return run, fmt.Errorf("上传结果未确认，请保留runId核实，不自动重传: %w", err)
	}
	data, err := preAuditResponseData(response)
	if err != nil {
		return run, err
	}
	id, ok := data["file_id"].(string)
	if !ok {
		return run, fmt.Errorf("CLM上传缺少字符串file_id")
	}
	if n, err := strconv.ParseInt(id, 10, 64); err != nil || n <= 0 {
		return run, fmt.Errorf("CLM上传返回无效file_id")
	}
	run.SourceFileID, run.Stage = id, "awaiting_category"
	if err := store.save(run); err != nil {
		return run, err
	}
	a.logger.Info("pre-audit awaiting category selection", "run_id", runID)
	return run, nil
}

func preAuditStringID(data map[string]any, key string) (string, error) {
	id, ok := data[key].(string)
	if n, err := strconv.ParseInt(id, 10, 64); !ok || err != nil || n <= 0 || strconv.FormatInt(n, 10) != id {
		return "", fmt.Errorf("预审响应缺少有效字符串%s", key)
	}
	return id, nil
}

func (a *App) withPreAuditRun(ctx context.Context, options commandOptions, runID string, preview bool, operation func(*preAuditRun, commandOptions) error) error {
	store := a.preAuditStore()
	if preview {
		if !validPreAuditRunID(runID) {
			return fmt.Errorf("run-id 必须为32位小写十六进制")
		}
		if err := checkPreAuditPath(store.dir, true); err != nil {
			return err
		}
		run, err := store.load(runID)
		if err != nil {
			return err
		}
		return a.finishReviewWorkflow(options, preAuditOutput(run), operation(run, options))
	}
	release, err := store.lock(runID)
	if err != nil {
		return err
	}
	defer release()
	run, err := store.load(runID)
	if err != nil {
		return err
	}
	session, err := a.newPreAuditSession(ctx, options, run)
	if err != nil {
		a.logger.Error("pre-audit recovery identity rejected", "run_id", runID)
		return a.finishReviewWorkflow(options, preAuditOutput(run), err)
	}
	options.reviewSession = session
	a.logger.Info("pre-audit stage started", "run_id", runID, "stage", run.Stage)
	err = operation(run, options)
	if err != nil {
		a.logger.Error("pre-audit stage stopped", "run_id", runID, "stage", run.Stage)
	} else {
		a.logger.Info("pre-audit stage completed", "run_id", runID, "stage", run.Stage)
	}
	return a.finishReviewWorkflow(options, preAuditOutput(run), err)
}

func (a *App) runPreAuditCreate(ctx context.Context, args []string) error {
	parsed, err := parseReviewArgs(args, []string{"--run-id", "--category-id"}, []string{"--dry-run", "--print-input"})
	if err != nil {
		return err
	}
	category, err := preAuditStringID(map[string]any{"category_id": parsed.String("--category-id")}, "category_id")
	if err != nil {
		return err
	}
	return a.withPreAuditRun(ctx, parseReviewOptions(parsed), parsed.String("--run-id"), parsed.Bool("--dry-run") || parsed.Bool("--print-input"), func(run *preAuditRun, options commandOptions) error {
		if run.CategoryID != "" && run.CategoryID != category {
			return fmt.Errorf("原run已绑定其他分类，不能更换创建参数")
		}
		if parsed.Bool("--dry-run") || parsed.Bool("--print-input") {
			return nil
		}
		if run.Stage == "awaiting_config" {
			return nil
		}
		if run.Stage != "awaiting_category" && run.Stage != "create_pending" && run.Stage != "pre_audit_created" {
			return fmt.Errorf("当前阶段%s不允许重复创建或引用上传，请保留原run核实", run.Stage)
		}
		firstAttempt := run.Stage == "awaiting_category"
		if firstAttempt {
			run.CategoryID = category
			run.RequestID = ""
			run.Stage = "create_pending"
			if err := a.preAuditStore().save(run); err != nil {
				return err
			}
		}
		return a.createPreAudit(ctx, options, run, firstAttempt)
	})
}

func (a *App) createPreAudit(ctx context.Context, options commandOptions, run *preAuditRun, firstAttempt bool) error {
	store := a.preAuditStore()
	if run.Stage == "create_pending" {
		if !firstAttempt {
			a.logger.Warn("pre-audit creation outcome unknown; recovery unsupported", "run_id", run.RunID)
			return fmt.Errorf("创建结果未知，暂不支持重试恢复；请保留原run并人工核实，不要重新发起创建")
		}
		if run.SourceFileID == "" || run.CategoryID == "" {
			return fmt.Errorf("创建记录不完整，已停止")
		}
		body, _ := json.Marshal(map[string]string{"source_file_id": run.SourceFileID, "category_id": run.CategoryID, "pre_audit_name": run.Name})
		response, err := a.doReview(ctx, options, openplatform.Request{Method: http.MethodPost, Path: preAuditPath, Body: body, IdentityPolicy: openplatform.IdentityPolicyUserOnly})
		var data map[string]any
		if err == nil {
			data, err = preAuditResponseData(response)
		}
		if err != nil {
			// 只有首次明确拒绝才能重新选择分类；结果未知时停止核实。
			if firstAttempt && run.PreAuditID == "" && preAuditCreateRejected(response, err) {
				next := *run
				next.Stage, next.CategoryID, next.RequestID = "awaiting_category", "", ""
				if saveErr := store.save(&next); saveErr != nil {
					a.logger.Error("pre-audit category reselection save failed", "run_id", run.RunID)
					return fmt.Errorf("%w；分类重选状态保存失败，原创建参数仍保留: %v", err, saveErr)
				}
				*run = next
				a.logger.Warn("pre-audit creation rejected, awaiting category selection", "run_id", run.RunID)
			} else {
				a.logger.Warn("pre-audit creation binding retained for recovery", "run_id", run.RunID, "status_code", response.StatusCode)
			}
			return err
		}
		if err := bindPreAuditDetail(run, data); err != nil {
			return err
		}
		run.Stage = "pre_audit_created"
		if err := store.save(run); err != nil {
			return err
		}
	}
	request, err := reviewUploadRequest(ctx, nil, "", run.Name, map[string]string{"businessId": run.PreAuditID, "businessFileId": run.LatestFileID, "businessFileVersion": "v1", "sourceLocator": `{"provider":"clm","sourceFileType":0}`, "channelType": "cli"})
	if err != nil {
		return err
	}
	run.Stage = "reference_unknown"
	if err := store.save(run); err != nil {
		return err
	}
	data, err := a.reviewDocument(ctx, options, request)
	if err != nil {
		return fmt.Errorf("引用上传未完成，保留预审及run，不自动重新上传: %w", err)
	}
	id, err := reviewResponseID(data, "fileId")
	if err != nil {
		return err
	}
	run.AIFileID = strconv.FormatInt(id, 10)
	hash, _ := data["fileHash"].(string)
	run.FileHash, err = normalizedReviewHash(hash)
	if err != nil || data["snapshotReady"] != true || (data["businessId"] != nil && data["businessId"] != run.PreAuditID) {
		run.Stage = "reference_not_ready"
		if saveErr := store.save(run); saveErr != nil {
			return saveErr
		}
		return fmt.Errorf("预审引用snapshotReady/fileHash/业务身份未就绪，未启动审查")
	}
	run.Upload = map[string]any{"businessId": run.PreAuditID, "fileId": run.AIFileID, "fileHash": run.FileHash, "snapshotReady": true}
	run.Stage = "awaiting_config"
	return store.save(run)
}

func preAuditCreateRejected(response openplatform.Response, requestErr error) bool {
	var uncertain *openplatform.UncertainWriteError
	if requestErr == nil || errors.As(requestErr, &uncertain) {
		return false
	}
	var rejectedHTTP bool
	switch response.StatusCode {
	case 400, 401, 403, 404, 405, 413, 415, 422, 429:
		rejectedHTTP = true
	default:
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return false
		}
	}
	var envelope struct {
		Code json.Number     `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(response.Body, &envelope) == nil {
		var data map[string]json.RawMessage
		if len(envelope.Data) > 0 {
			if json.Unmarshal(envelope.Data, &data) != nil {
				return false
			}
			if _, hasID := data["pre_audit_id"]; hasID {
				return false
			}
		}
		if envelope.Code != "" {
			// CLM's generic errors and post-commit response failures do not
			// establish that creation rolled back. Keep their original key.
			switch envelope.Code {
			case "110000", "110004", "110301", "110413", "110903", "117008":
				return true
			default:
				return false
			}
		}
	}
	return rejectedHTTP
}

func bindPreAuditDetail(run *preAuditRun, data map[string]any) error {
	values := make(map[string]string)
	for _, key := range []string{"pre_audit_id", "source_file_id", "latest_file_id", "category_id"} {
		id, err := preAuditStringID(data, key)
		if err != nil {
			return err
		}
		values[key] = id
	}
	if data["business_type_code"] != json.Number("24") || values["source_file_id"] != run.SourceFileID || values["category_id"] != run.CategoryID || (run.PreAuditID != "" && run.PreAuditID != values["pre_audit_id"]) || (run.LatestFileID != "" && run.LatestFileID != values["latest_file_id"]) {
		return fmt.Errorf("预审详情与原run的业务/分类/文件不一致，已停止")
	}
	run.PreAuditID, run.LatestFileID = values["pre_audit_id"], values["latest_file_id"]
	return nil
}
