package httpclient

import (
	"context"
	"net"
	"testing"
)

func TestValidatePublicURL(t *testing.T) {
	for _, rawURL := range []string{
		"https://example.com/path",
		"http://8.8.8.8/image.png",
	} {
		if err := validatePublicURL(rawURL); err != nil {
			t.Fatalf("validatePublicURL(%q) = %v", rawURL, err)
		}
	}
	for _, rawURL := range []string{
		"file:///etc/passwd",
		"http:///missing-host",
		"http://user:pass@example.com/",
	} {
		if err := validatePublicURL(rawURL); err == nil {
			t.Fatalf("validatePublicURL(%q) unexpectedly succeeded", rawURL)
		}
	}
}

func TestBlockedPublicIP(t *testing.T) {
	for _, value := range []string{
		"127.0.0.1",
		"10.0.0.1",
		"169.254.169.254",
		"100.64.0.1",
		"192.0.2.1",
		"::1",
		"fc00::1",
	} {
		if !blockedPublicIP(net.ParseIP(value)) {
			t.Fatalf("blockedPublicIP(%s) = false", value)
		}
	}
	if blockedPublicIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public address was blocked")
	}
}

func TestPublicDialRejectsPrivateLiteralBeforeConnecting(t *testing.T) {
	if _, err := publicDialContext(context.Background(), "tcp", "127.0.0.1:80"); err == nil {
		t.Fatal("private target was allowed")
	}
}
