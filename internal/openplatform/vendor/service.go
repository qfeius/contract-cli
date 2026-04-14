package vendor

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"cn.qfei/contract-cli/internal/openplatform"
)

const collectionPath = "/open-apis/mdm/v1/vendors"

type Service struct {
	client *openplatform.Client
}

func NewService(client *openplatform.Client) *Service {
	return &Service{client: client}
}

func (s *Service) Get(ctx context.Context, requestContext openplatform.RequestContext, vendorID string) (openplatform.Response, error) {
	vendorID = strings.TrimSpace(vendorID)
	if vendorID == "" {
		return openplatform.Response{}, fmt.Errorf("vendor id is required")
	}

	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method: http.MethodGet,
		Path:   collectionPath + "/" + url.PathEscape(vendorID),
	})
}
