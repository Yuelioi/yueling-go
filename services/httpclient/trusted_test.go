package httpclient

import (
	"net/url"
	"testing"
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
