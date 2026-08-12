package httpclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxPublicRedirects = 5

var blockedPublicNetworks = mustNetworks(
	"0.0.0.0/8",
	"100.64.0.0/10",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"240.0.0.0/4",
	"2001:db8::/32",
)

func mustNetworks(values ...string) []*net.IPNet {
	networks := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			panic(err)
		}
		networks = append(networks, network)
	}
	return networks
}

func validatePublicURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("无效 URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("只支持 http/https URL")
	}
	if parsed.Hostname() == "" {
		return fmt.Errorf("URL 缺少主机名")
	}
	if parsed.User != nil {
		return fmt.Errorf("URL 不允许包含用户信息")
	}
	return nil
}

func blockedPublicIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, network := range blockedPublicNetworks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func publicDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("无效目标地址: %w", err)
	}

	var addresses []net.IPAddr
	if literal := net.ParseIP(strings.Trim(host, "[]")); literal != nil {
		addresses = []net.IPAddr{{IP: literal}}
	} else {
		addresses, err = net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("主机名没有可用地址")
	}
	for _, address := range addresses {
		if blockedPublicIP(address.IP) {
			return nil, fmt.Errorf("拒绝访问非公网地址")
		}
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var lastErr error
	for _, address := range addresses {
		if network == "tcp4" && address.IP.To4() == nil {
			continue
		}
		if network == "tcp6" && address.IP.To4() != nil {
			continue
		}
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("主机名没有匹配的网络地址")
	}
	return nil, lastErr
}

func newPublicClient() *Client {
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           publicDialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          50,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxPublicRedirects {
				return fmt.Errorf("重定向次数过多")
			}
			return validatePublicURL(req.URL.String())
		},
	}
	return &Client{Client: client}
}

// Public fetches untrusted URLs without allowing loopback, private, link-local,
// metadata, documentation, or other non-public network targets.
var Public = newPublicClient()

// GetPublicBytesLimit downloads an untrusted public URL with a strict body cap.
func GetPublicBytesLimit(rawURL string, limit int64, headers ...string) ([]byte, error) {
	if err := validatePublicURL(rawURL); err != nil {
		return nil, err
	}
	return Public.GetBytesLimit(rawURL, limit, headers...)
}
