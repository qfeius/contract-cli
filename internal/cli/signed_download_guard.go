package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
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

type publicFileTransport struct {
	next     http.RoundTripper
	standard *http.Transport
}

func (transport publicFileTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if _, err := validateSignedDownloadURL(request.URL.String()); err != nil {
		return nil, err
	}
	if transport.standard != nil {
		if _, err := publicFileIPs(request.Context(), request.URL.Hostname(), net.DefaultResolver.LookupIPAddr); err != nil {
			return nil, err
		}
		if transport.standard.Proxy != nil {
			proxy, err := transport.standard.Proxy(request)
			if err != nil {
				return nil, err
			}
			if proxy != nil {
				// An explicitly configured proxy is trusted transport, not a file
				// target. Keep enterprise proxies working without allowing private URLs.
				request = request.WithContext(context.WithValue(request.Context(), configuredFileProxyKey{}, proxy.Hostname()))
			}
		}
	}
	return transport.next.RoundTrip(request)
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
	dial := standard.DialContext
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	guarded.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if proxyHost, _ := ctx.Value(configuredFileProxyKey{}).(string); proxyHost != "" && strings.EqualFold(proxyHost, host) {
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
	return publicFileTransport{next: guarded, standard: guarded}
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
