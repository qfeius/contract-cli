package openplatform

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
)

func (c *Client) fileDownloadBody(response *http.Response) (io.Reader, error) {
	// An explicit filename identifies a file, even when its JSON resembles an
	// API error. A bare or malformed disposition does not provide that evidence.
	if hasDownloadFilename(response.Header.Get("Content-Disposition")) {
		return response.Body, nil
	}
	reader := bufio.NewReader(response.Body)
	prefix, err := reader.Peek(512)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("inspect file download response: %w", err)
	}
	mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	trimmed := bytes.TrimSpace(bytes.TrimPrefix(bytes.TrimSpace(prefix), []byte("\xef\xbb\xbf")))
	jsonType := mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
	if !jsonType && (len(trimmed) == 0 || trimmed[0] != '{') {
		return reader, nil
	}
	// Non-file JSON responses use the existing bounded API response reader.
	// Properly identified attachments above retain unrestricted streaming.
	body, err := c.readResponseBody(reader)
	if err != nil {
		return nil, fmt.Errorf("read file download response: %w", err)
	}
	if err := fileDownloadBusinessError(body); err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func hasDownloadFilename(value string) bool {
	disposition, parameters, err := mime.ParseMediaType(value)
	return err == nil && (disposition == "attachment" || disposition == "inline") && strings.TrimSpace(parameters["filename"]) != ""
}

func fileDownloadBusinessError(body []byte) error {
	var envelope struct {
		Code    *int64  `json:"code"`
		Success *bool   `json:"success"`
		Msg     *string `json:"msg"`
		Message *string `json:"message"`
	}
	jsonBody := bytes.TrimPrefix(bytes.TrimSpace(body), []byte("\xef\xbb\xbf"))
	if json.Unmarshal(jsonBody, &envelope) != nil || envelope.Code == nil {
		return nil
	}
	failed := envelope.Success != nil && !*envelope.Success
	withMessage := envelope.Msg != nil || envelope.Message != nil
	if failed || (*envelope.Code != 0 && withMessage) {
		// Keep backend messages and attachment contents out of logs and stderr.
		return fmt.Errorf("file download failed with business code %d", *envelope.Code)
	}
	return nil
}
