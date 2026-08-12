package bot

import (
	"net/http/httptest"
	"testing"
)

func TestReverseWSTokenMatches(t *testing.T) {
	if !reverseWSTokenMatches("Bearer secret", "secret") {
		t.Fatal("valid token did not match")
	}
	for _, value := range []string{"", "secret", "Bearer wrong", "Bearer secret "} {
		if reverseWSTokenMatches(value, "secret") {
			t.Fatalf("invalid authorization %q matched", value)
		}
	}
}

func TestReverseWSRemoteWithoutTokenMustBePrivate(t *testing.T) {
	for _, address := range []string{"127.0.0.1:1234", "[::1]:1234", "172.18.0.2:1234", "192.168.1.2:1234"} {
		if !reverseWSRemoteAllowedWithoutToken(address) {
			t.Fatalf("private address %q was rejected", address)
		}
	}
	for _, address := range []string{"8.8.8.8:1234", "203.0.113.1:1234", "invalid"} {
		if reverseWSRemoteAllowedWithoutToken(address) {
			t.Fatalf("public/invalid address %q was accepted", address)
		}
	}
}

func TestReverseWSOriginPolicy(t *testing.T) {
	req := httptest.NewRequest("GET", "http://bot.example/onebot/v11/ws", nil)
	if !reverseWSOriginAllowed(req) {
		t.Fatal("non-browser client without Origin was rejected")
	}
	req.Header.Set("Origin", "http://bot.example")
	if !reverseWSOriginAllowed(req) {
		t.Fatal("same-origin client was rejected")
	}
	req.Header.Set("Origin", "https://evil.example")
	if reverseWSOriginAllowed(req) {
		t.Fatal("cross-origin browser client was accepted")
	}
}
