package httpclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

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

func TestTrustedPrivateBaseBypassesConfiguredProxy(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "direct feed")
	}))
	defer target.Close()

	proxyCalls := 0
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		proxyCalls++
		http.Error(w, "private RSSHub must not use proxy", http.StatusBadGateway)
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

	body, err := GetTrustedBaseBytesLimit(target.URL+"/feed", target.URL, 1024)
	if err != nil {
		t.Fatalf("GetTrustedBaseBytesLimit() error = %v", err)
	}
	if strings.TrimSpace(string(body)) != "direct feed" || proxyCalls != 0 {
		t.Fatalf("body=%q proxyCalls=%d, want direct fetch", body, proxyCalls)
	}
}

func TestTrustedFetchContextCancelsInFlightRequest(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := GetTrustedBaseBytesLimitContext(ctx, server.URL, server.URL, 1024); done <- err }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request not started")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("request not canceled")
	}
}
