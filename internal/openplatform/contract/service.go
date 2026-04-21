package contract

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
)

type Service struct {
	client *openplatform.Client
}

type TextInput struct {
	FullText bool
	Offset   int
	Limit    int
}

type SearchInput struct {
	Body []byte
}

type ListTemplatesInput struct {
	CategoryNumber string
	PageSize       int
	PageToken      string
}

type UploadFileInput struct {
	FileName string
	FileType string
	File     io.Reader
}

func NewService(client *openplatform.Client) *Service {
	return &Service{client: client}
}

func (s *Service) Search(ctx context.Context, requestContext openplatform.RequestContext, input SearchInput) (openplatform.Response, error) {
	switch requestContext.Identity {
	case config.IdentityUser:
		return s.do(ctx, requestContext, "search-contracts", nil, nil, input.Body)
	case config.IdentityBot:
		return s.client.Do(ctx, requestContext, openplatform.Request{
			Method:         http.MethodPost,
			Path:           "/open-apis/contract/v1/contracts/search",
			Body:           input.Body,
			IdentityPolicy: openplatform.IdentityPolicyAny,
		})
	default:
		return openplatform.Response{}, fmt.Errorf("unsupported identity %q for contract search", requestContext.Identity)
	}
}

func (s *Service) Get(ctx context.Context, requestContext openplatform.RequestContext, contractID string) (openplatform.Response, error) {
	contractID = strings.TrimSpace(contractID)
	if contractID == "" {
		return openplatform.Response{}, fmt.Errorf("contract id is required")
	}
	switch requestContext.Identity {
	case config.IdentityUser:
		return s.do(ctx, requestContext, "get-contract-detail", map[string]string{"{contractId}": url.PathEscape(contractID)}, nil, nil)
	case config.IdentityBot:
		return s.client.Do(ctx, requestContext, openplatform.Request{
			Method:         http.MethodGet,
			Path:           "/open-apis/contract/v1/contracts/" + url.PathEscape(contractID),
			IdentityPolicy: openplatform.IdentityPolicyAny,
		})
	default:
		return openplatform.Response{}, fmt.Errorf("unsupported identity %q for contract get", requestContext.Identity)
	}
}

func (s *Service) SyncUserGroups(ctx context.Context, requestContext openplatform.RequestContext) (openplatform.Response, error) {
	switch requestContext.Identity {
	case config.IdentityUser:
		return s.do(ctx, requestContext, "sync-user-groups", nil, nil, nil)
	case config.IdentityBot:
		return s.client.Do(ctx, requestContext, openplatform.Request{
			Method:         http.MethodPost,
			Path:           "/open-apis/contract/v1/contracts/user-groups/sync",
			IdentityPolicy: openplatform.IdentityPolicyAny,
		})
	default:
		return openplatform.Response{}, fmt.Errorf("unsupported identity %q for contract sync-user-groups", requestContext.Identity)
	}
}

func (s *Service) GetText(ctx context.Context, requestContext openplatform.RequestContext, contractID string, input TextInput) (openplatform.Response, error) {
	contractID = strings.TrimSpace(contractID)
	if contractID == "" {
		return openplatform.Response{}, fmt.Errorf("contract id is required")
	}
	query := url.Values{
		"full_text": {strconv.FormatBool(input.FullText)},
	}
	if input.Offset > 0 {
		query.Set("offset", strconv.Itoa(input.Offset))
	}
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	switch requestContext.Identity {
	case config.IdentityUser:
		return s.do(ctx, requestContext, "get-contract-text", map[string]string{"{contractId}": url.PathEscape(contractID)}, query, nil)
	case config.IdentityBot:
		return s.client.Do(ctx, requestContext, openplatform.Request{
			Method:         http.MethodPost,
			Path:           "/open-apis/contract/v1/contracts/" + url.PathEscape(contractID) + "/text",
			Query:          query,
			IdentityPolicy: openplatform.IdentityPolicyAny,
		})
	default:
		return openplatform.Response{}, fmt.Errorf("unsupported identity %q for contract text", requestContext.Identity)
	}
}

func (s *Service) ListCategories(ctx context.Context, requestContext openplatform.RequestContext, lang string) (openplatform.Response, error) {
	query := url.Values{}
	if strings.TrimSpace(lang) != "" {
		query.Set("lang", strings.TrimSpace(lang))
	}
	switch requestContext.Identity {
	case config.IdentityUser:
		return s.do(ctx, requestContext, "contract_category.list", nil, query, nil)
	case config.IdentityBot:
		return s.client.Do(ctx, requestContext, openplatform.Request{
			Method:         http.MethodGet,
			Path:           "/open-apis/contract/v1/contract_categorys",
			Query:          query,
			IdentityPolicy: openplatform.IdentityPolicyAny,
		})
	default:
		return openplatform.Response{}, fmt.Errorf("unsupported identity %q for contract category list", requestContext.Identity)
	}
}

func (s *Service) Create(ctx context.Context, requestContext openplatform.RequestContext, body []byte) (openplatform.Response, error) {
	switch requestContext.Identity {
	case config.IdentityUser:
		return s.do(ctx, requestContext, "create-contracts", nil, nil, body)
	case config.IdentityBot:
		return s.client.Do(ctx, requestContext, openplatform.Request{
			Method:         http.MethodPost,
			Path:           "/open-apis/contract/v1/contracts",
			Body:           body,
			IdentityPolicy: openplatform.IdentityPolicyAny,
		})
	default:
		return openplatform.Response{}, fmt.Errorf("unsupported identity %q for contract create", requestContext.Identity)
	}
}

func (s *Service) ListTemplates(ctx context.Context, requestContext openplatform.RequestContext, input ListTemplatesInput) (openplatform.Response, error) {
	query := url.Values{}
	if strings.TrimSpace(input.CategoryNumber) != "" {
		query.Set("category_number", strings.TrimSpace(input.CategoryNumber))
	}
	if input.PageSize > 0 {
		query.Set("page_size", strconv.Itoa(input.PageSize))
	}
	if strings.TrimSpace(input.PageToken) != "" {
		query.Set("page_token", strings.TrimSpace(input.PageToken))
	}
	switch requestContext.Identity {
	case config.IdentityUser:
		return s.do(ctx, requestContext, "list-templates", nil, query, nil)
	case config.IdentityBot:
		return s.client.Do(ctx, requestContext, openplatform.Request{
			Method:         http.MethodGet,
			Path:           "/open-apis/contract/v1/templates",
			Query:          query,
			IdentityPolicy: openplatform.IdentityPolicyAny,
		})
	default:
		return openplatform.Response{}, fmt.Errorf("unsupported identity %q for contract template list", requestContext.Identity)
	}
}

func (s *Service) GetTemplate(ctx context.Context, requestContext openplatform.RequestContext, templateID string) (openplatform.Response, error) {
	templateID = strings.TrimSpace(templateID)
	if templateID == "" {
		return openplatform.Response{}, fmt.Errorf("template id is required")
	}
	switch requestContext.Identity {
	case config.IdentityUser:
		return s.do(ctx, requestContext, "get-template-detail", map[string]string{"{template_id}": url.PathEscape(templateID)}, nil, nil)
	case config.IdentityBot:
		return s.client.Do(ctx, requestContext, openplatform.Request{
			Method:         http.MethodGet,
			Path:           "/open-apis/contract/v1/templates/" + url.PathEscape(templateID),
			IdentityPolicy: openplatform.IdentityPolicyAny,
		})
	default:
		return openplatform.Response{}, fmt.Errorf("unsupported identity %q for contract template get", requestContext.Identity)
	}
}

func (s *Service) InstantiateTemplate(ctx context.Context, requestContext openplatform.RequestContext, body []byte) (openplatform.Response, error) {
	switch requestContext.Identity {
	case config.IdentityUser:
		return s.do(ctx, requestContext, "create-template-instance", nil, nil, body)
	case config.IdentityBot:
		return s.client.Do(ctx, requestContext, openplatform.Request{
			Method:         http.MethodPost,
			Path:           "/open-apis/contract/v1/template_instances",
			Body:           body,
			IdentityPolicy: openplatform.IdentityPolicyAny,
		})
	default:
		return openplatform.Response{}, fmt.Errorf("unsupported identity %q for contract template instantiate", requestContext.Identity)
	}
}

func (s *Service) ListEnums(ctx context.Context, requestContext openplatform.RequestContext, enumType string) (openplatform.Response, error) {
	enumType = strings.TrimSpace(enumType)
	if enumType == "" {
		return openplatform.Response{}, fmt.Errorf("enum type is required")
	}
	query := url.Values{"enum_type": {enumType}}
	return s.do(ctx, requestContext, "get-enum-values", nil, query, nil)
}

func (s *Service) UploadFile(ctx context.Context, requestContext openplatform.RequestContext, input UploadFileInput) (openplatform.Response, error) {
	fileName := strings.TrimSpace(input.FileName)
	if fileName == "" {
		return openplatform.Response{}, fmt.Errorf("file name is required")
	}
	fileType := strings.TrimSpace(input.FileType)
	if fileType == "" {
		return openplatform.Response{}, fmt.Errorf("file type is required")
	}
	if input.File == nil {
		return openplatform.Response{}, fmt.Errorf("file reader is required")
	}

	bodyReader, contentType := multipartUploadBody(UploadFileInput{
		FileName: fileName,
		FileType: fileType,
		File:     input.File,
	})
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:     http.MethodPost,
		Path:       "/open-apis/contract/v1/files/upload",
		BodyReader: bodyReader,
		Headers: http.Header{
			"Content-Type": {contentType},
		},
		IdentityPolicy: openplatform.IdentityPolicyBotOnly,
	})
}

func (s *Service) do(ctx context.Context, requestContext openplatform.RequestContext, toolName string, replacements map[string]string, query url.Values, body []byte) (openplatform.Response, error) {
	spec, ok := openplatform.ContractMCPToolSpec(toolName)
	if !ok {
		return openplatform.Response{}, fmt.Errorf("contract tool spec %q is not configured", toolName)
	}

	path := spec.Path
	for old, newValue := range replacements {
		path = strings.ReplaceAll(path, old, newValue)
	}

	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         spec.Method,
		Path:           path,
		Query:          spec.Query(query),
		Body:           body,
		IdentityPolicy: spec.IdentityPolicy,
	})
}

func multipartUploadBody(input UploadFileInput) (io.Reader, string) {
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	return &lazyMultipartUploadReader{
		reader:          reader,
		writer:          writer,
		multipartWriter: multipartWriter,
		input:           input,
	}, multipartWriter.FormDataContentType()
}

type lazyMultipartUploadReader struct {
	once            sync.Once
	reader          *io.PipeReader
	writer          *io.PipeWriter
	multipartWriter *multipart.Writer
	input           UploadFileInput
}

func (r *lazyMultipartUploadReader) Read(p []byte) (int, error) {
	r.once.Do(func() {
		go func() {
			err := writeUploadMultipart(r.multipartWriter, r.input)
			if closeErr := r.multipartWriter.Close(); err == nil {
				err = closeErr
			}
			if err != nil {
				_ = r.writer.CloseWithError(err)
				return
			}
			_ = r.writer.Close()
		}()
	})
	return r.reader.Read(p)
}

func (r *lazyMultipartUploadReader) Close() error {
	_ = r.writer.Close()
	return r.reader.Close()
}

func writeUploadMultipart(writer *multipart.Writer, input UploadFileInput) error {
	if err := writer.WriteField("file_name", input.FileName); err != nil {
		return fmt.Errorf("write file_name field: %w", err)
	}
	if err := writer.WriteField("file_type", input.FileType); err != nil {
		return fmt.Errorf("write file_type field: %w", err)
	}
	part, err := writer.CreateFormFile("file", input.FileName)
	if err != nil {
		return fmt.Errorf("create file form field: %w", err)
	}
	if _, err := io.Copy(part, input.File); err != nil {
		return fmt.Errorf("copy upload file content: %w", err)
	}
	return nil
}
