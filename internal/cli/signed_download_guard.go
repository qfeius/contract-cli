package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
)

const maxSignedFileBytes int64 = 1 << 30

var errSignedFileTooLarge = errors.New("signed file exceeds size limit")

// Non-public special-purpose ranges not covered by net.IP.IsPrivate.
// https://www.iana.org/assignments/iana-ipv4-special-registry
var nonPublicFileRanges = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func publicFileHost(host string) error {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.Contains(host, "%") {
		return errors.New("file address must not target a local or private host")
	}
	if ip := net.ParseIP(host); ip != nil && !publicFileIP(ip) {
		return errors.New("file address must not target a local or private IP")
	}
	return nil
}

func publicFileIP(ip net.IP) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	address = address.Unmap()
	for _, prefix := range nonPublicFileRanges {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

func publicFileIPs(ctx context.Context, host string, lookup func(context.Context, string) ([]net.IPAddr, error)) ([]net.IPAddr, error) {
	if err := publicFileHost(host); err != nil {
		return nil, err
	}
	ips, err := lookup(ctx, host)
	if err != nil {
		return nil, errors.New("could not resolve public file host")
	}
	if len(ips) == 0 {
		return nil, errors.New("file host has no public address")
	}
	for _, ip := range ips {
		if !publicFileIP(ip.IP) {
			return nil, errors.New("file host resolves to a local or private address")
		}
	}
	return ips, nil
}

type configuredFileProxyKey struct{}

type configuredFileProxy struct {
	url     *url.URL
	address string
}

type publicFileTransport struct {
	next          http.RoundTripper
	proxy         func(*http.Request) (*url.URL, error)
	customTLSDial bool
}

func (transport publicFileTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if _, err := validateSignedDownloadURL(request.URL.String()); err != nil {
		return nil, err
	}
	selected := configuredFileProxy{}
	if transport.proxy != nil {
		proxy, err := transport.proxy(request)
		if err != nil {
			return nil, err
		}
		selected.url = proxy
		if proxy != nil {
			// Explicit proxies own target DNS and destination access policy. Only
			// the configured proxy socket may bypass direct public-IP validation.
			selected.address = fileProxyAddress(proxy)
		}
		request = request.WithContext(context.WithValue(request.Context(), configuredFileProxyKey{}, selected))
	}
	// Custom TLS dialers bypass Transport.DialContext. Retain their existing
	// direct-target DNS check without imposing local target DNS on proxies.
	if selected.url == nil && transport.customTLSDial {
		if _, err := publicFileIPs(request.Context(), request.URL.Hostname(), net.DefaultResolver.LookupIPAddr); err != nil {
			return nil, err
		}
	}
	return transport.next.RoundTrip(request)
}

func fileProxyAddress(proxy *url.URL) string {
	port := proxy.Port()
	if port == "" {
		switch proxy.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		case "socks5", "socks5h":
			port = "1080"
		default:
			return ""
		}
	}
	return net.JoinHostPort(proxy.Hostname(), port)
}

func publicSignedFileTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	standard, ok := base.(*http.Transport)
	if !ok {
		return publicFileTransport{next: base}
	}
	guarded := standard.Clone()
	if standard.Proxy != nil {
		// Resolve a possibly dynamic Proxy function once per request, and use
		// that exact selection for both Transport routing and dial validation.
		guarded.Proxy = func(request *http.Request) (*url.URL, error) {
			selected, _ := request.Context().Value(configuredFileProxyKey{}).(configuredFileProxy)
			return selected.url, nil
		}
	}
	dial := standard.DialContext
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	guarded.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if proxy, _ := ctx.Value(configuredFileProxyKey{}).(configuredFileProxy); proxy.address != "" && strings.EqualFold(proxy.address, address) {
			return dial(ctx, network, address)
		}
		ips, err := publicFileIPs(ctx, host, net.DefaultResolver.LookupIPAddr)
		if err != nil {
			return nil, err
		}
		var last error
		for _, ip := range ips {
			connection, err := dial(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if err == nil {
				return connection, nil
			}
			last = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
		}
		return nil, last
	}
	// Honor caller-supplied legacy TLS hooks without removing their DNS guard.
	customTLSDial := standard.DialTLSContext != nil || standard.DialTLS != nil //nolint:staticcheck // Compatibility with existing http.Transport callers.
	return publicFileTransport{next: guarded, proxy: standard.Proxy, customTLSDial: customTLSDial}
}

func copySignedFile(writer io.Writer, reader io.Reader, limit int64) (int64, error) {
	count, err := io.Copy(writer, io.LimitReader(reader, limit+1))
	if err != nil {
		return count, err
	}
	if count > limit {
		return count, fmt.Errorf("%w (%d bytes)", errSignedFileTooLarge, limit)
	}
	return count, nil
}
