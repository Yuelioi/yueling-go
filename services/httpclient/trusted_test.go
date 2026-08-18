package httpclient

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

func TestSameOrigin(t *testing.T) {
	tests := []struct {
		left  string
		right string
		want  bool
	}{
		{"http://rsshub:1200", "http://rsshub:1200/bilibili/user/video/1", true},
		{"https://rsshub.app", "https://RSSHUB.APP/twitter/user/test", true},
		{"https://rsshub.app", "http://rsshub.app/twitter/user/test", false},
		{"https://rsshub.app", "https://evil.example/twitter/user/test", false},
	}
	for _, test := range tests {
		left, _ := url.Parse(test.left)
		right, _ := url.Parse(test.right)
		if got := sameOrigin(left, right); got != test.want {
			t.Fatalf("sameOrigin(%q, %q) = %v", test.left, test.right, got)
		}
	}
}

func TestTrustedBaseFetchUsesConfiguredProxy(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Host != "rsshub.invalid" || r.URL.Path != "/feed" {
			t.Fatalf("proxy request URL = %s, want http://rsshub.invalid/feed", r.URL.String())
		}
		_, _ = io.WriteString(w, "proxied feed")
	}))
	defer proxy.Close()

	oldConfig := config.C
	oldProxy := Proxy
	config.C.Tools.Proxy = proxy.URL
	InitProxy()
	t.Cleanup(func() {
		config.C = oldConfig
		Proxy = oldProxy
	})

	body, err := GetTrustedBaseBytesLimit(
		"http://rsshub.invalid/feed",
		"http://rsshub.invalid",
		1024,
	)
	if err != nil {
		t.Fatalf("GetTrustedBaseBytesLimit() error = %v", err)
	}
	if strings.TrimSpace(string(body)) != "proxied feed" {
		t.Fatalf("body = %q, want proxied feed", body)
	}
}
