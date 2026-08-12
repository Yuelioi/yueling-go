package tools

import "testing"

func TestURLHostIsRejectsHostnamesHiddenInPathOrSuffix(t *testing.T) {
	tests := []struct {
		url    string
		domain string
		want   bool
	}{
		{"https://blog.csdn.net/user/article/details/1", "blog.csdn.net", true},
		{"https://www.behance.net/gallery/1", "behance.net", true},
		{"http://127.0.0.1/blog.csdn.net/article", "blog.csdn.net", false},
		{"https://behance.net.evil.example/gallery/1", "behance.net", false},
		{"file://blog.csdn.net/etc/passwd", "blog.csdn.net", false},
	}
	for _, test := range tests {
		if got := urlHostIs(test.url, test.domain); got != test.want {
			t.Fatalf("urlHostIs(%q, %q) = %v, want %v", test.url, test.domain, got, test.want)
		}
	}
}
