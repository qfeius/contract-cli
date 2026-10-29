package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"cn.qfei/contract-cli/internal/openplatform"
	contractsvc "cn.qfei/contract-cli/internal/openplatform/contract"
	"cn.qfei/contract-cli/internal/output"
)

const maxSignedDownloadRedirects = 10

type contractFileDownloadEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
	Success *bool  `json:"success"`
	Data    struct {
		FileID      string `json:"file_id"`
		FileName    string `json:"file_name"`
		FileSize    *int64 `json:"file_size"`
		DownloadURL string `json:"download_url"`
	} `json:"data"`
}

func (a *App) runContractDownloadFileAsUser(ctx context.Context, service *contractsvc.Service, requestContext openplatform.RequestContext, contractID string, fileID string, outputFile string, raw bool, force bool, format output.Format) error {
	logger := a.commandTraceLogger(ctx)
	logger.Info("resolve user contract file download", "contract_id", contractID, "file_id", fileID)
	outputPath, err := a.resolveContractDownloadPath(ctx, fileID, outputFile, raw, force)
	if err != nil {
		return err
	}
	response, err := service.GetFileDownloadMetadata(ctx, requestContext, contractID, fileID)
	if err != nil {
		logger.Error("resolve user contract file download failed", "contract_id", contractID, "file_id", fileID, "error", err.Error())
		return err
	}
	metadata, err := decodeContractFileDownloadMetadata(response.Body)
	if err != nil {
		logger.Error("decode user contract file download metadata failed", "contract_id", contractID, "file_id", fileID, "error", err.Error())
		return err
	}
	if raw {
		return a.downloadSignedContractFile(ctx, metadata.Data.DownloadURL, metadata.Data.FileSize, a.stdout)
	}
	if err := a.downloadSignedContractFileToPath(ctx, metadata.Data.DownloadURL, metadata.Data.FileSize, outputPath, force); err != nil {
		return err
	}
	return a.renderDownloadedFile(format, contractID, fileID, metadata.Data.FileName, outputPath)
}

func (a *App) renderDownloadedFile(format output.Format, contractID, fileID, fileName, outputPath string) error {
	if format == "" {
		_, err := fmt.Fprintf(a.stdout, "Downloaded file to %s\n", outputPath)
		return err
	}
	absolutePath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve downloaded file path: %w", err)
	}
	info, err := os.Stat(absolutePath)
	if err != nil {
		return fmt.Errorf("inspect downloaded file: %w", err)
	}
	if fileName == "" {
		fileName = filepath.Base(absolutePath)
	}
	result := map[string]any{
		"file_id": fileID, "file_name": fileName, "output_path": absolutePath,
		"file_size": info.Size(),
	}
	if contractID != "" {
		result["contract_id"] = contractID
	}
	return output.NewRenderer(a.stdout).WithNotice(a.updateNotice).Render(format, result)
}

func decodeContractFileDownloadMetadata(body []byte) (contractFileDownloadEnvelope, error) {
	var envelope contractFileDownloadEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return envelope, fmt.Errorf("decode contract file download metadata: %w", err)
	}
	if envelope.Code != 0 || (envelope.Success != nil && !*envelope.Success) {
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message = "request was not successful"
		}
		return envelope, fmt.Errorf("contract file download metadata failed with code %d: %s", envelope.Code, message)
	}
	if _, err := validateSignedDownloadURL(envelope.Data.DownloadURL); err != nil {
		return envelope, err
	}
	if envelope.Data.FileSize != nil && *envelope.Data.FileSize < 0 {
		return envelope, fmt.Errorf("contract file metadata returned an invalid file size")
	}
	return envelope, nil
}

func (a *App) resolveContractDownloadPath(ctx context.Context, fileID string, outputFile string, raw bool, force bool) (string, error) {
	if raw {
		return "", nil
	}
	outputPath := strings.TrimSpace(outputFile)
	if outputPath == "" {
		selectedPath, err := a.saveFileDialog(ctx, fileID)
		if err != nil {
			return "", fmt.Errorf("select save path failed; pass --output-file <path> in non-GUI environments: %w", err)
		}
		outputPath = strings.TrimSpace(selectedPath)
		if outputPath == "" {
			return "", fmt.Errorf("select save path failed; pass --output-file <path> in non-GUI environments")
		}
	}
	if _, err := os.Lstat(outputPath); err == nil && !force {
		return "", fmt.Errorf("download output file %q already exists; pass --force to overwrite", outputPath)
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect download output file: %w", err)
	}
	return outputPath, nil
}

func (a *App) downloadSignedContractFileToPath(ctx context.Context, rawURL string, expectedSize *int64, outputPath string, force bool) error {
	temporary, err := os.CreateTemp(filepath.Dir(outputPath), filepath.Base(outputPath)+".part-*")
	if err != nil {
		return fmt.Errorf("create temporary download file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }() // Best-effort removal, including after rename.

	downloadErr := a.downloadSignedContractFile(ctx, rawURL, expectedSize, temporary)
	closeErr := temporary.Close()
	if downloadErr != nil {
		return downloadErr
	}
	if closeErr != nil {
		return fmt.Errorf("close temporary download file: %w", closeErr)
	}
	return commitDownloadedFile(temporaryPath, outputPath, force)
}

func commitDownloadedFile(temporaryPath string, outputPath string, force bool) error {
	return commitDownloadedFileWithLink(temporaryPath, outputPath, force, os.Link)
}

func commitDownloadedFileWithLink(temporaryPath string, outputPath string, force bool, linkFile func(string, string) error) error {
	if force {
		if err := os.Rename(temporaryPath, outputPath); err != nil {
			return fmt.Errorf("replace download output file: %w", err)
		}
		return nil
	}
	if err := linkFile(temporaryPath, outputPath); err == nil {
		return nil
	} else if os.IsExist(err) {
		return fmt.Errorf("download output file %q already exists; pass --force to overwrite", outputPath)
	}
	return copyDownloadedFileNoReplace(temporaryPath, outputPath)
}

func copyDownloadedFileNoReplace(temporaryPath string, outputPath string) error {
	source, err := os.Open(temporaryPath)
	if err != nil {
		return fmt.Errorf("open completed download file: %w", err)
	}
	defer func() { _ = source.Close() }() // Read-only source; copy and target close errors are checked below.

	target, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("download output file %q already exists; pass --force to overwrite", outputPath)
		}
		return fmt.Errorf("create download output file: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		_ = target.Close()
		_ = os.Remove(outputPath)
	}()

	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("copy completed download to output file: %w", err)
	}
	if err := target.Sync(); err != nil {
		return fmt.Errorf("sync download output file: %w", err)
	}
	if err := target.Close(); err != nil {
		return fmt.Errorf("close download output file: %w", err)
	}
	committed = true
	return nil
}

func (a *App) downloadSignedContractFile(ctx context.Context, rawURL string, expectedSize *int64, writer io.Writer) error {
	logger := a.commandTraceLogger(ctx)
	parsedURL, err := validateSignedDownloadURL(rawURL)
	if err != nil {
		return err
	}
	logger.Info("signed contract file download started", "host", parsedURL.Hostname())
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return fmt.Errorf("build signed file download request: %w", err)
	}
	// Signed downloads carry no CLI credentials and retain their separate HTTPS
	// redirect policy, including cross-host CDN redirects and header stripping.
	downloadClient := signedDownloadHTTPClient(a.fileHTTPClient)
	response, err := downloadClient.Do(request)
	if err != nil {
		logger.Error("signed contract file download failed", "host", parsedURL.Hostname(), "error_type", fmt.Sprintf("%T", err))
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("perform signed file download: %w", ctxErr)
		}
		return fmt.Errorf("perform signed file download: request failed")
	}
	defer func() { _ = response.Body.Close() }() // Response read errors are checked by the download path.
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		logger.Error("signed contract file download failed", "host", parsedURL.Hostname(), "status_code", response.StatusCode)
		return fmt.Errorf("signed file download failed with status %d", response.StatusCode)
	}
	written, err := io.Copy(writer, response.Body)
	if err != nil {
		logger.Error("copy signed contract file download failed", "host", parsedURL.Hostname(), "error_type", fmt.Sprintf("%T", err))
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("copy signed file download: %w", ctxErr)
		}
		return fmt.Errorf("copy signed file download: response stream failed")
	}
	// Prefer the actual storage response length; stored FileInfo sizes can be stale.
	if response.ContentLength >= 0 {
		expectedSize = &response.ContentLength
	}
	if expectedSize != nil && written != *expectedSize {
		logger.Error("signed contract file size mismatch", "host", parsedURL.Hostname(), "expected_size", *expectedSize, "actual_size", written)
		return fmt.Errorf("signed file download size mismatch: expected %d bytes, got %d", *expectedSize, written)
	}
	logger.Info("signed contract file download completed", "host", parsedURL.Hostname(), "size", written)
	return nil
}

func signedDownloadHTTPClient(base *http.Client) *http.Client {
	client := *base
	client.Jar = nil
	previousCheckRedirect := client.CheckRedirect
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if _, err := validateSignedDownloadURL(request.URL.String()); err != nil {
			return fmt.Errorf("reject unsafe signed file download redirect")
		}
		stripSignedDownloadRedirectHeaders(request.Header)
		if previousCheckRedirect != nil {
			if err := previousCheckRedirect(request, via); err != nil {
				return err
			}
		} else if len(via) >= maxSignedDownloadRedirects {
			return fmt.Errorf("stopped after %d redirects", maxSignedDownloadRedirects)
		}
		stripSignedDownloadRedirectHeaders(request.Header)
		return nil
	}
	return &client
}

func stripSignedDownloadRedirectHeaders(headers http.Header) {
	headers.Del("Authorization")
	headers.Del("Cookie")
	headers.Del("Referer")
}

func validateSignedDownloadURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return nil, fmt.Errorf("contract file metadata returned an invalid HTTPS download URL")
	}
	return parsed, nil
}
