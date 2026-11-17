package cli

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSignedDownloadRejectsPrivateTargetsBeforeHTTP(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.100.100.200", "198.18.0.1", "0.0.0.0", "[::1]", "[::ffff:127.0.0.1]", "localhost", "LOCALHOST."} {
		t.Run(host, func(t *testing.T) {
			app, _ := preAuditOutcomeFixture(t)
			app.fileHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("private target reached HTTP"); return nil, nil })}
			if err := app.downloadSignedContractFile(context.Background(), "https://"+host+"/file", nil, io.Discard); err == nil {
				t.Fatal("private target accepted")
			}
		})
	}
}

func TestSignedDownloadUnknownLengthHasBudget(t *testing.T) {
	app, _ := preAuditOutcomeFixture(t)
	app.fileHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) > 11*time.Minute {
			t.Fatal("signed download has no deadline")
		}
		return &http.Response{StatusCode: 200, ContentLength: -1, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("file"))}, nil
	})}
	if err := app.downloadSignedContractFile(context.Background(), "https://cdn.example.test/file", nil, io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestSignedDownloadDNSRejectsMixedPrivateResults(t *testing.T) {
	lookup := func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP("127.0.0.1")}}, nil
	}
	if _, err := publicFileIPs(context.Background(), "cdn.example.test", lookup); err == nil {
		t.Fatal("mixed DNS result accepted")
	}
}

func TestSignedDownloadBodyLimitRejectsOverflow(t *testing.T) {
	if _, err := copySignedFile(io.Discard, strings.NewReader("12345"), 4); err == nil {
		t.Fatal("oversized stream accepted")
	}
	if count, err := copySignedFile(io.Discard, strings.NewReader("1234"), 4); err != nil || count != 4 {
		t.Fatalf("exact-size stream rejected: %d %v", count, err)
	}
}

func TestPreAuditLinkRejectsInsecureAndPrivateAddresses(t *testing.T) {
	for _, base := range []string{"http://contract.qfei.cn", "https://127.0.0.1", "https://localhost"} {
		if err := validatePreAuditPageURL(base+"/pre-audit/detail?preAuditId=22&from=cli", "22"); err == nil {
			t.Fatalf("unsafe link accepted: %s", base)
		}
	}
}

func TestSignedDownloadStandardTransportConnectsToValidatedIP(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "file") }))
	defer server.Close()
	base := server.Client().Transport.(*http.Transport).Clone()
	base.TLSClientConfig = base.TLSClientConfig.Clone()
	base.TLSClientConfig.ServerName = "127.0.0.1"
	address := ""
	base.DialContext = func(ctx context.Context, network, remote string) (net.Conn, error) {
		address = remote
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	client := signedDownloadHTTPClient(&http.Client{Transport: base, Timeout: 5 * time.Second})
	defer client.CloseIdleConnections()
	response, err := client.Get("https://8.8.8.8/file")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	body, err := io.ReadAll(response.Body)
	if err != nil || string(body) != "file" || address != "8.8.8.8:443" {
		t.Fatalf("public transport failed: address=%s body=%s err=%v", address, body, err)
	}
}

func TestSignedDownloadKeepsExplicitProxyWithoutAllowingPrivateTargets(t *testing.T) {
	probe := errors.New("proxy dial probe")
	address := ""
	base := &http.Transport{Proxy: func(*http.Request) (*url.URL, error) { return url.Parse("http://127.0.0.1:4567") }, DialContext: func(_ context.Context, _ string, remote string) (net.Conn, error) {
		address = remote
		return nil, probe
	}}
	client := signedDownloadHTTPClient(&http.Client{Transport: base, Timeout: time.Second})
	if _, err := client.Get("https://8.8.8.8/file"); err == nil || address != "127.0.0.1:4567" {
		t.Fatalf("explicit proxy was blocked: %s %v", address, err)
	}
	address = ""
	if _, err := client.Get("https://127.0.0.1/file"); err == nil || address != "" {
		t.Fatal("proxy allowed a private file target")
	}
}

func TestSignedDownloadProxyDoesNotResolveTargetLocally(t *testing.T) {
	original := net.DefaultResolver
	var lookups atomic.Int32
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(context.Context, string, string) (net.Conn, error) {
		lookups.Add(1)
		return nil, errors.New("local DNS unavailable")
	}}
	t.Cleanup(func() { net.DefaultResolver = original })
	for _, proxyURL := range []string{"http://127.0.0.1:4567", "http://127.0.0.1", "https://127.0.0.1", "socks5://127.0.0.1"} {
		t.Run(proxyURL, func(t *testing.T) {
			proxy, err := url.Parse(proxyURL)
			if err != nil {
				t.Fatal(err)
			}
			probe := errors.New("configured proxy reached")
			address, selections := "", 0
			base := &http.Transport{Proxy: func(*http.Request) (*url.URL, error) {
				selections++
				return proxy, nil
			}, DialContext: func(_ context.Context, _ string, remote string) (net.Conn, error) {
				address = remote
				return nil, probe
			}}
			client := signedDownloadHTTPClient(&http.Client{Transport: base, Timeout: time.Second})
			defer client.CloseIdleConnections()
			_, err = client.Get("https://proxy-only.example.invalid/file")
			if !errors.Is(err, probe) || address == "" || lookups.Load() != 0 || selections != 1 {
				t.Fatalf("proxy dispatch failed: dial=%q DNS=%d selections=%d error=%v", address, lookups.Load(), selections, err)
			}
		})
	}
}

func TestSignedDownloadProxyPermissionBindsExactSocket(t *testing.T) {
	probe := errors.New("dial reached")
	base := &http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) { return nil, probe }}
	guard := publicSignedFileTransport(base).(publicFileTransport).next.(*http.Transport)
	defer guard.CloseIdleConnections()
	ctx := context.WithValue(context.Background(), configuredFileProxyKey{}, configuredFileProxy{address: "127.0.0.1:4567"})
	for _, address := range []string{"127.0.0.1:4567", "127.0.0.1:4568", "127.0.0.2:4567"} {
		_, err := guard.DialContext(ctx, "tcp", address)
		if errors.Is(err, probe) != (address == "127.0.0.1:4567") {
			t.Fatalf("proxy permission escaped exact socket: %s %v", address, err)
		}
	}
}

func TestSignedDownloadProxySelectionFailureAndBypass(t *testing.T) {
	for _, selectionError := range []bool{false, true} {
		probe := errors.New("proxy selection unavailable")
		base := &http.Transport{Proxy: func(*http.Request) (*url.URL, error) {
			if selectionError {
				return nil, probe
			}
			return nil, nil
		}, DialContext: func(_ context.Context, _ string, address string) (net.Conn, error) {
			if selectionError || address != "8.8.8.8:443" {
				t.Errorf("unexpected direct dial: %s", address)
			}
			return nil, errors.New("direct dial probe")
		}}
		client := signedDownloadHTTPClient(&http.Client{Transport: base})
		_, err := client.Get("https://8.8.8.8/file")
		client.CloseIdleConnections()
		if err == nil || errors.Is(err, probe) != selectionError {
			t.Fatalf("proxy selection error changed: %v", err)
		}
	}
}

func TestSignedDownloadDirectCustomTLSStillValidatesDNS(t *testing.T) {
	original := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("offline DNS")
	}}
	t.Cleanup(func() { net.DefaultResolver = original })
	for _, legacy := range []bool{false, true} {
		var calls atomic.Int32
		base := &http.Transport{}
		if legacy {
			base.DialTLS = func(string, string) (net.Conn, error) { //nolint:staticcheck // Verify the legacy caller hook cannot lose its existing DNS guard.
				calls.Add(1)
				return nil, errors.New("unexpected custom TLS dial")
			}
		} else {
			base.DialTLSContext = func(context.Context, string, string) (net.Conn, error) {
				calls.Add(1)
				return nil, errors.New("unexpected custom TLS dial")
			}
		}
		client := signedDownloadHTTPClient(&http.Client{Transport: base})
		_, err := client.Get("https://offline.example.invalid/file")
		client.CloseIdleConnections()
		if err == nil || calls.Load() != 0 {
			t.Fatalf("custom TLS bypassed DNS validation: calls=%d error=%v", calls.Load(), err)
		}
	}
}
