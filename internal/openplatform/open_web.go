package openplatform

import (
	"fmt"
	"strings"
)

// OpenWebPathPrefix is the gateway namespace for page-business APIs.
const OpenWebPathPrefix = "/open-web/web/"

// ValidateOpenWebPath keeps generic calls on the gateway's relative Open Web
// namespace and rejects path forms that can be interpreted inconsistently by
// proxies or downstream services.
func ValidateOpenWebPath(path string) error {
	if strings.TrimSpace(path) != path || !strings.HasPrefix(path, OpenWebPathPrefix) {
		return fmt.Errorf("open web path must be a relative %s path", OpenWebPathPrefix)
	}
	if strings.ContainsAny(path, "?#") {
		return fmt.Errorf("open web path must not contain a query or fragment; pass query values with --query")
	}
	if strings.Contains(path, "%") || strings.Contains(path, "\\") || strings.Contains(path, "//") || strings.Contains(path, ";") {
		return fmt.Errorf("open web path contains an unsafe path form")
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("open web path contains an unsafe dot segment")
		}
	}
	return nil
}
