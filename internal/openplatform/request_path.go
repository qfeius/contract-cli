package openplatform

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

func validateRequestPath(path string) (*url.URL, error) {
	if strings.Contains(path, "://") {
		return nil, fmt.Errorf("open platform path must be relative")
	}
	if strings.HasPrefix(path, OpenWebPathPrefix) {
		if err := ValidateOpenWebPath(path); err != nil {
			return nil, err
		}
	} else if !strings.HasPrefix(path, "/open-apis/") {
		return nil, fmt.Errorf("open platform path must be a relative /open-apis/ or %s path", OpenWebPathPrefix)
	}
	if strings.TrimSpace(path) != path || strings.ContainsAny(path, "?#\\;") || strings.Contains(path, "//") {
		return nil, fmt.Errorf("open platform path contains an unsafe path form; pass query values separately")
	}
	parsed, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("parse open platform path: %w", err)
	}
	for _, segment := range strings.Split(parsed.EscapedPath(), "/") {
		if unsafePathSegment(segment) {
			return nil, fmt.Errorf("open platform path contains an unsafe segment")
		}
	}
	return parsed, nil
}

func unsafePathSegment(segment string) bool {
	// Inspect nested escapes without rewriting the original URL or opaque data.
	for {
		if segment == "." || segment == ".." || strings.ContainsAny(segment, "/\\;?#") || strings.ContainsFunc(segment, unicode.IsControl) {
			return true
		}
		decoded, err := url.PathUnescape(segment)
		if err != nil {
			decoded = unescapeValidPathTokens(segment)
		}
		if decoded == segment {
			return false
		}
		segment = decoded
	}
}

// A literal percent must not hide a later, valid escape from nested checks.
// This decoded value is only inspected, never used to rewrite the request URL.
func unescapeValidPathTokens(segment string) string {
	var decoded strings.Builder
	decoded.Grow(len(segment))
	for i := 0; i < len(segment); i++ {
		if segment[i] == '%' && i+2 < len(segment) {
			if value, err := url.PathUnescape(segment[i : i+3]); err == nil {
				decoded.WriteString(value)
				i += 2
				continue
			}
		}
		decoded.WriteByte(segment[i])
	}
	return decoded.String()
}

func prepareRequestTarget(rc RequestContext, request Request) (*url.URL, error) {
	fullURL, err := buildURL(rc.BaseURL, request.Path, nil)
	if err != nil {
		return nil, err
	}
	target, err := url.Parse(fullURL)
	if err != nil {
		return nil, err
	}
	routePolicy := IdentityPolicyForPath(target.Path)
	policy := request.IdentityPolicy
	if policy == "" {
		policy = routePolicy
	}
	if err := validateIdentityPolicy(rc.Identity, routePolicy, target.Path); err != nil {
		return nil, err
	}
	if err := validateIdentityPolicy(rc.Identity, policy, target.Path); err != nil {
		return nil, err
	}
	if routePolicy == IdentityPolicyUserOnly {
		policy = routePolicy
	}
	target.RawQuery = mergeQuery(policy, request.Query, rc.CommonQuery).Encode()
	return target, nil
}
