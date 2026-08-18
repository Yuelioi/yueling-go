package httpclient

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GetTrustedBaseBytesLimit fetches a URL only when it stays on an explicitly
// configured origin. Unlike Public it may reach a private Docker/LAN service,
// so callers must only pass operator-controlled base URLs.
func GetTrustedBaseBytesLimit(rawURL, baseURL string, limit int64, headers ...string) ([]byte, error) {
	base, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || base.Hostname() == "" || (base.Scheme != "http" && base.Scheme != "https") || base.User != nil {
		return nil, fmt.Errorf("无效的可信服务地址")
	}
	target, err := url.Parse(rawURL)
	if err != nil || !sameOrigin(base, target) || target.User != nil {
		return nil, fmt.Errorf("目标不属于配置的可信服务")
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if Proxy != nil && Proxy.Client != nil {
		if configured, ok := Proxy.Transport.(*http.Transport); ok {
			transport = configured.Clone()
		}
	}
	if isPrivateServiceHost(base.Hostname()) {
		// A configured outbound proxy cannot resolve Docker service names and
		// should never be used for loopback/LAN operator-controlled services.
		transport.Proxy = nil
	}
	transport.ForceAttemptHTTP2 = true
	transport.MaxIdleConns = 10
	transport.IdleConnTimeout = 30 * time.Second
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.ResponseHeaderTimeout = 40 * time.Second
	transport.ExpectContinueTimeout = time.Second

	client := &Client{Client: &http.Client{
		Transport: transport,
		Timeout:   45 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxPublicRedirects {
				return fmt.Errorf("重定向次数过多")
			}
			if !sameOrigin(base, req.URL) || req.URL.User != nil {
				return fmt.Errorf("可信服务重定向到了其他地址")
			}
			return nil
		},
	}}
	return client.GetBytesLimit(rawURL, limit, headers...)
}

func isPrivateServiceHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".local") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
	}
	return host != "" && !strings.Contains(host, ".")
}

func sameOrigin(left, right *url.URL) bool {
	return strings.EqualFold(left.Scheme, right.Scheme) && strings.EqualFold(left.Host, right.Host)
}
