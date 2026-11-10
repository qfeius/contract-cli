package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cn.qfei/contract-cli/internal/openplatform"
	contractsvc "cn.qfei/contract-cli/internal/openplatform/contract"
	"cn.qfei/contract-cli/internal/tracecontext"
)

// Content upload returns only a boolean envelope, never a file or a large resource list.
const maxUploadContentResponseBytes = 1024 * 1024

type contractFileUploadSession struct {
	UploadID      string            `json:"upload_id"`
	UploadURL     string            `json:"upload_url"`
	UploadMethod  string            `json:"upload_method"`
	UploadHeaders map[string]string `json:"upload_headers"`
	ExpiresAt     string            `json:"expires_at"`
	MaxSize       int64             `json:"max_size"`
}

func (a *App) runContractUploadAttachmentAsUser(ctx context.Context, service *contractsvc.Service, requestContext openplatform.RequestContext, options commandOptions, input contractsvc.UploadFileInput, fileSize int64) (resultErr error) {
	phase := "prepare"
	logger := a.commandTraceLogger(ctx)
	logger.Info("user attachment upload started", "profile", requestContext.Profile.Name, "file_type", input.FileType, "size_bytes", fileSize)
	defer func() {
		if resultErr != nil {
			logger.Error("user attachment upload failed", "profile", requestContext.Profile.Name, "phase", phase, "error", resultErr.Error())
		}
	}()
	if fileSize <= 0 {
		return fmt.Errorf("attachment upload requires a non-empty file")
	}
	body, err := marshalJSONObject(map[string]any{"file_name": input.FileName, "file_type": input.FileType})
	if err != nil {
		return err
	}
	response, err := service.PrepareFileUpload(ctx, requestContext, body)
	if err != nil {
		return sanitizeAttachmentUploadError(phase, err)
	}
	if err := validateAttachmentUploadResponse(phase, response.Body); err != nil {
		return err
	}
	session, err := decodeContractFileUploadSession(response.Body, fileSize, a.now())
	if err != nil {
		return err
	}

	phase = "content"
	if err := a.uploadContractAttachmentContent(ctx, session, input); err != nil {
		return err
	}

	phase = "commit"
	logger.Info("user attachment upload committing", "profile", requestContext.Profile.Name)
	body, err = marshalJSONObject(map[string]any{"upload_id": session.UploadID})
	if err != nil {
		return err
	}
	response, err = service.CommitFileUpload(ctx, requestContext, body)
	if err != nil {
		return sanitizeAttachmentUploadError(phase, err)
	}
	if err := validateAttachmentUploadResponse(phase, response.Body); err != nil {
		return err
	}
	var committed struct {
		Data struct {
			FileID string `json:"file_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body, &committed); err != nil {
		return fmt.Errorf("upload commit returned an invalid file_id: %w", &openplatform.UncertainWriteError{})
	}
	fileID, err := strconv.ParseInt(committed.Data.FileID, 10, 64)
	if err != nil || fileID <= 0 {
		return fmt.Errorf("upload commit returned an invalid file_id: %w", &openplatform.UncertainWriteError{})
	}
	logger.Info("user attachment upload completed", "profile", requestContext.Profile.Name, "file_id", committed.Data.FileID)
	return a.renderOpenPlatformResponse(options, response)
}

func decodeContractFileUploadSession(body []byte, fileSize int64, now time.Time) (contractFileUploadSession, error) {
	var envelope struct {
		Data contractFileUploadSession `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return envelope.Data, errors.New("invalid upload session response")
	}
	session := envelope.Data
	uploadURL, err := url.Parse(session.UploadURL)
	if err != nil || uploadURL.Scheme != "https" || uploadURL.Host == "" || uploadURL.User != nil || uploadURL.Fragment != "" {
		return session, errors.New("upload session returned an invalid HTTPS URL")
	}
	// The current application content endpoint is POST with no authentication headers.
	if strings.TrimSpace(session.UploadID) == "" || session.UploadMethod != http.MethodPost || len(session.UploadHeaders) != 0 {
		return session, errors.New("upload session returned unsupported upload parameters")
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, session.ExpiresAt)
	if err != nil || !expiresAt.After(now) {
		return session, errors.New("upload session has an invalid or expired expiry time")
	}
	if session.MaxSize <= 0 || fileSize > session.MaxSize {
		return session, errors.New("upload file exceeds the session size limit")
	}
	return session, nil
}

func (a *App) uploadContractAttachmentContent(ctx context.Context, session contractFileUploadSession, input contractsvc.UploadFileInput) error {
	logger := a.commandTraceLogger(ctx)
	logger.Info("user attachment content upload started")
	body, contentType := contractsvc.UploadSessionContentBody(input.FileName, input.File)
	defer func() { _ = body.Close() }() // Closing the input stream only releases multipart resources.
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, session.UploadURL, body)
	if err != nil {
		return errors.New("build attachment content upload request failed")
	}
	request.Header.Set("Content-Type", contentType)
	requestTrace, err := tracecontext.ForRequest(ctx)
	if err != nil {
		return fmt.Errorf("prepare attachment content trace: %w", err)
	}
	if err := requestTrace.Apply(request.Header); err != nil {
		return fmt.Errorf("apply attachment content trace: %w", err)
	}
	// The session URL is a capability. Do not add user credentials or replay the file on redirects.
	client := *a.httpClient
	client.Jar = nil
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(request)
	if err != nil {
		return sanitizeAttachmentUploadError("content", &openplatform.UncertainWriteError{Cause: err})
	}
	defer func() { _ = response.Body.Close() }() // Read/status errors determine the request result.
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		statusErr := fmt.Errorf("content upload failed with status %d", response.StatusCode)
		if response.StatusCode >= http.StatusInternalServerError {
			return fmt.Errorf("%v: %w", statusErr, &openplatform.UncertainWriteError{})
		}
		return statusErr
	}
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxUploadContentResponseBytes+1))
	if err != nil || len(responseBody) > maxUploadContentResponseBytes {
		return fmt.Errorf("read upload content response failed: %w", &openplatform.UncertainWriteError{})
	}
	if err := validateAttachmentUploadResponse("content", responseBody); err != nil {
		return err
	}
	var envelope struct {
		Data bool `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &envelope); err != nil || !envelope.Data {
		return fmt.Errorf("upload content was not confirmed: %w", &openplatform.UncertainWriteError{})
	}
	logger.Info("user attachment content upload completed")
	return nil
}

func validateAttachmentUploadResponse(phase string, body []byte) error {
	if err := validateContractMCPResponse(body); err == nil {
		return nil
	}
	var envelope struct {
		Code    *int64 `json:"code"`
		Success *bool  `json:"success"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Code != nil && (*envelope.Code != 0 || (envelope.Success != nil && !*envelope.Success)) {
		return fmt.Errorf("%s upload failed with business code %d", phase, *envelope.Code)
	}
	err := fmt.Errorf("invalid %s upload response or missing business code", phase)
	if phase == "prepare" {
		return err
	}
	return fmt.Errorf("%v: %w", err, &openplatform.UncertainWriteError{})
}

func sanitizeAttachmentUploadError(phase string, err error) error {
	// HTTP errors can contain echoed session URLs/IDs. Preserve only safe status and uncertainty.
	var statusErr *openplatform.HTTPStatusError
	message := fmt.Sprintf("%s upload request failed", phase)
	if errors.As(err, &statusErr) {
		message = fmt.Sprintf("%s with status %d", message, statusErr.StatusCode)
	}
	var contextErr error
	if errors.Is(err, context.Canceled) {
		contextErr = context.Canceled
	} else if errors.Is(err, context.DeadlineExceeded) {
		contextErr = context.DeadlineExceeded
	}
	if contextErr != nil {
		message += ": " + contextErr.Error()
	}
	var uncertain *openplatform.UncertainWriteError
	if errors.As(err, &uncertain) {
		return fmt.Errorf("%s: %w", message, &openplatform.UncertainWriteError{Cause: contextErr})
	}
	if contextErr != nil {
		return fmt.Errorf("%s: %w", phase+" upload request failed", contextErr)
	}
	return errors.New(message)
}
