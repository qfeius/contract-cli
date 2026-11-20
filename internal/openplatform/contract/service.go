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
	FullText    bool
	FullTextSet bool
	Offset      int
	OffsetSet   bool
	Limit       int
	LimitSet    bool
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

type ProcessInstanceInput struct {
	NoticeFilter       string
	TaskInstanceFilter string
}

type CreateProcessCommentInput struct {
	MentionIDType string
	Body          []byte
}

func NewService(client *openplatform.Client) *Service {
	return &Service{client: client}
}

func (s *Service) Search(ctx context.Context, requestContext openplatform.RequestContext, input SearchInput) (openplatform.Response, error) {
	switch requestContext.Identity {
	case config.IdentityUser:
		return s.do(ctx, requestContext, "search-contracts", nil, nil, input.Body)
	case config.IdentityApp:
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
	case config.IdentityApp:
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
	case config.IdentityApp:
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
		"full_text": {strconv.FormatBool(resolveTextFullText(input))},
	}
	if input.OffsetSet {
		query.Set("offset", strconv.Itoa(input.Offset))
	}
	if input.LimitSet {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	switch requestContext.Identity {
	case config.IdentityUser:
		return s.do(ctx, requestContext, "get-contract-text", map[string]string{"{contractId}": url.PathEscape(contractID)}, query, nil)
	case config.IdentityApp:
		return s.client.Do(ctx, requestContext, openplatform.Request{
			Method:         http.MethodGet,
			Path:           "/open-apis/contract/v1/contracts/" + url.PathEscape(contractID) + "/text",
			Query:          query,
			IdentityPolicy: openplatform.IdentityPolicyAny,
		})
	default:
		return openplatform.Response{}, fmt.Errorf("unsupported identity %q for contract text", requestContext.Identity)
	}
}

func resolveTextFullText(input TextInput) bool {
	if input.FullTextSet {
		return input.FullText
	}
	return !input.OffsetSet && !input.LimitSet
}

func (s *Service) ListCategories(ctx context.Context, requestContext openplatform.RequestContext, lang string) (openplatform.Response, error) {
	query := url.Values{}
	if strings.TrimSpace(lang) != "" {
		query.Set("lang", strings.TrimSpace(lang))
	}
	switch requestContext.Identity {
	case config.IdentityUser:
		return s.do(ctx, requestContext, "contract_category.list", nil, query, nil)
	case config.IdentityApp:
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
	case config.IdentityApp:
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

func (s *Service) SearchProjects(ctx context.Context, requestContext openplatform.RequestContext, body []byte) (openplatform.Response, error) {
	var path string
	var policy openplatform.IdentityPolicy
	switch requestContext.Identity {
	case config.IdentityUser:
		path = "/open-apis/contract/v1/mcp/projects/search"
		policy = openplatform.IdentityPolicyUserOnly
	case config.IdentityApp:
		path = "/open-apis/contract/v1/projects/search"
		policy = openplatform.IdentityPolicyAppOnly
	default:
		return openplatform.Response{}, fmt.Errorf("unsupported identity %q for contract project search", requestContext.Identity)
	}
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodPost,
		Path:           path,
		Body:           body,
		IdentityPolicy: policy,
		OperationKind:  openplatform.OperationRead,
	})
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
	case config.IdentityApp:
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
	case config.IdentityApp:
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
	case config.IdentityApp:
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
		IdentityPolicy: openplatform.IdentityPolicyAny,
		OperationKind:  openplatform.OperationWrite,
	})
}

func (s *Service) Submit(ctx context.Context, requestContext openplatform.RequestContext, contractID string, body []byte) (openplatform.Response, error) {
	contractID = strings.TrimSpace(contractID)
	if contractID == "" {
		return openplatform.Response{}, fmt.Errorf("contract id is required")
	}
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/contracts/" + url.PathEscape(contractID) + "/submit",
		Body:           body,
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (s *Service) Resubmit(ctx context.Context, requestContext openplatform.RequestContext, contractID string, body []byte) (openplatform.Response, error) {
	contractID = strings.TrimSpace(contractID)
	if contractID == "" {
		return openplatform.Response{}, fmt.Errorf("contract id is required")
	}
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/contracts/" + url.PathEscape(contractID) + "/resubmit",
		Body:           body,
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (s *Service) Patch(ctx context.Context, requestContext openplatform.RequestContext, contractID string, body []byte) (openplatform.Response, error) {
	contractID = strings.TrimSpace(contractID)
	if contractID == "" {
		return openplatform.Response{}, fmt.Errorf("contract id is required")
	}
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodPatch,
		Path:           "/open-apis/contract/v1/contracts/" + url.PathEscape(contractID),
		Body:           body,
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (s *Service) DownloadFile(ctx context.Context, requestContext openplatform.RequestContext, fileID string, writer io.Writer) (openplatform.Response, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return openplatform.Response{}, fmt.Errorf("file id is required")
	}
	if writer == nil {
		return openplatform.Response{}, fmt.Errorf("download writer is required")
	}
	return s.client.DoStream(ctx, requestContext, openplatform.Request{
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/files/" + url.PathEscape(fileID),
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	}, writer)
}

func (s *Service) Delete(ctx context.Context, requestContext openplatform.RequestContext, contractID string) (openplatform.Response, error) {
	contractID = strings.TrimSpace(contractID)
	if contractID == "" {
		return openplatform.Response{}, fmt.Errorf("contract id is required")
	}
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodDelete,
		Path:           "/open-apis/contract/v1/contracts/" + url.PathEscape(contractID),
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (s *Service) PrintFile(ctx context.Context, requestContext openplatform.RequestContext, body []byte) (openplatform.Response, error) {
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/files",
		Body:           body,
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (s *Service) GetShareRecords(ctx context.Context, requestContext openplatform.RequestContext, contractID string) (openplatform.Response, error) {
	contractID = strings.TrimSpace(contractID)
	if contractID == "" {
		return openplatform.Response{}, fmt.Errorf("contract id is required")
	}
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/contracts/" + url.PathEscape(contractID) + "/share_records",
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (s *Service) GetCooperationLink(ctx context.Context, requestContext openplatform.RequestContext, contractID string) (openplatform.Response, error) {
	contractID = strings.TrimSpace(contractID)
	if contractID == "" {
		return openplatform.Response{}, fmt.Errorf("contract id is required")
	}
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/contracts/" + url.PathEscape(contractID) + "/cooperation_link",
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (s *Service) GetCooperationRecordInfo(ctx context.Context, requestContext openplatform.RequestContext, contractID string) (openplatform.Response, error) {
	contractID = strings.TrimSpace(contractID)
	if contractID == "" {
		return openplatform.Response{}, fmt.Errorf("contract id is required")
	}
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/contracts/" + url.PathEscape(contractID) + "/cooperation_record_info",
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (s *Service) StartApproval(ctx context.Context, requestContext openplatform.RequestContext, processInstanceID string, body []byte) (openplatform.Response, error) {
	processInstanceID = strings.TrimSpace(processInstanceID)
	if processInstanceID == "" {
		return openplatform.Response{}, fmt.Errorf("process instance id is required")
	}
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/process_instances/" + url.PathEscape(processInstanceID) + "/task_approval",
		Body:           body,
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (s *Service) GetProcessInstance(ctx context.Context, requestContext openplatform.RequestContext, processInstanceID string, input ProcessInstanceInput) (openplatform.Response, error) {
	processInstanceID = strings.TrimSpace(processInstanceID)
	if processInstanceID == "" {
		return openplatform.Response{}, fmt.Errorf("process instance id is required")
	}
	query := url.Values{}
	if strings.TrimSpace(input.NoticeFilter) != "" {
		query.Set("notice_filter", strings.TrimSpace(input.NoticeFilter))
	}
	if strings.TrimSpace(input.TaskInstanceFilter) != "" {
		query.Set("task_instance_filter", strings.TrimSpace(input.TaskInstanceFilter))
	}
	switch requestContext.Identity {
	case config.IdentityUser:
		return s.doForCurrentUser(ctx, requestContext, "get-process-instance", map[string]string{
			"{process_instance_id}": url.PathEscape(processInstanceID),
		}, query, nil)
	case config.IdentityApp:
		return s.client.Do(ctx, requestContext, openplatform.Request{
			Method:         http.MethodGet,
			Path:           "/open-apis/contract/v1/process_instances/" + url.PathEscape(processInstanceID),
			Query:          query,
			IdentityPolicy: openplatform.IdentityPolicyAppOnly,
		})
	default:
		return openplatform.Response{}, fmt.Errorf("unsupported identity %q for contract approval get", requestContext.Identity)
	}
}

func (s *Service) ListProcessComments(ctx context.Context, requestContext openplatform.RequestContext, processInstanceID string) (openplatform.Response, error) {
	processInstanceID = strings.TrimSpace(processInstanceID)
	if processInstanceID == "" {
		return openplatform.Response{}, fmt.Errorf("process instance id is required")
	}
	return s.doForCurrentUser(ctx, requestContext, "list-process-comments", map[string]string{
		"{process_instance_id}": url.PathEscape(processInstanceID),
	}, nil, nil)
}

func (s *Service) CreateProcessComment(ctx context.Context, requestContext openplatform.RequestContext, processInstanceID string, input CreateProcessCommentInput) (openplatform.Response, error) {
	processInstanceID = strings.TrimSpace(processInstanceID)
	if processInstanceID == "" {
		return openplatform.Response{}, fmt.Errorf("process instance id is required")
	}
	query := url.Values{}
	if mentionIDType := strings.TrimSpace(input.MentionIDType); mentionIDType != "" {
		query.Set("user_id_type", mentionIDType)
	}
	return s.doForCurrentUser(ctx, requestContext, "create-process-comment", map[string]string{
		"{process_instance_id}": url.PathEscape(processInstanceID),
	}, query, input.Body)
}

func (s *Service) ListPersonalTasks(ctx context.Context, requestContext openplatform.RequestContext, body []byte) (openplatform.Response, error) {
	return s.doForCurrentUser(ctx, requestContext, "list-personal-tasks", nil, nil, body)
}

func (s *Service) ProcessApprovalTask(ctx context.Context, requestContext openplatform.RequestContext, taskInstanceID string, body []byte) (openplatform.Response, error) {
	taskInstanceID = strings.TrimSpace(taskInstanceID)
	if taskInstanceID == "" {
		return openplatform.Response{}, fmt.Errorf("task instance id is required")
	}
	return s.doForCurrentUser(ctx, requestContext, "process-approval-task", map[string]string{
		"{task_instance_id}": url.PathEscape(taskInstanceID),
	}, nil, body)
}

func (s *Service) GetFileDownloadMetadata(ctx context.Context, requestContext openplatform.RequestContext, contractID string, fileID string) (openplatform.Response, error) {
	contractID = strings.TrimSpace(contractID)
	if contractID == "" {
		return openplatform.Response{}, fmt.Errorf("contract id is required")
	}
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return openplatform.Response{}, fmt.Errorf("file id is required")
	}
	return s.doForCurrentUser(ctx, requestContext, "download-contract-file", map[string]string{
		"{contract_id}": url.PathEscape(contractID),
		"{file_id}":     url.PathEscape(fileID),
	}, nil, nil)
}

func (s *Service) doForCurrentUser(ctx context.Context, requestContext openplatform.RequestContext, toolName string, replacements map[string]string, query url.Values, body []byte) (openplatform.Response, error) {
	requestContext.CommonQuery = nil
	return s.do(ctx, requestContext, toolName, replacements, query, body)
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
		OperationKind:  spec.OperationKind,
	})
}

func (s *Service) PrepareFileUpload(ctx context.Context, requestContext openplatform.RequestContext, body []byte) (openplatform.Response, error) {
	requestContext.CommonQuery = nil
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/mcp/files/upload_sessions/prepare",
		Body:           body,
		IdentityPolicy: openplatform.IdentityPolicyUserOnly,
		OperationKind:  openplatform.OperationWrite,
	})
}

func (s *Service) CommitFileUpload(ctx context.Context, requestContext openplatform.RequestContext, body []byte) (openplatform.Response, error) {
	requestContext.CommonQuery = nil
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/mcp/files/upload_sessions/commit",
		Body:           body,
		IdentityPolicy: openplatform.IdentityPolicyUserOnly,
		OperationKind:  openplatform.OperationWrite,
	})
}

// UploadSessionContentBody streams the file part; prepare already supplied its metadata.
func UploadSessionContentBody(fileName string, file io.Reader) (io.ReadCloser, string) {
	return multipartUploadBody(UploadFileInput{FileName: fileName, File: file})
}

func multipartUploadBody(input UploadFileInput) (io.ReadCloser, string) {
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
	if input.FileType != "" {
		if err := writer.WriteField("file_name", input.FileName); err != nil {
			return fmt.Errorf("write file_name field: %w", err)
		}
		if err := writer.WriteField("file_type", input.FileType); err != nil {
			return fmt.Errorf("write file_type field: %w", err)
		}
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
