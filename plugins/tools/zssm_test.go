package tools

import (
	"strings"
	"testing"
)

func TestExtractVisibleText(t *testing.T) {
	htmlDoc := `<html><head><title>T</title>
<style>.x{color:red}</style><script>var a=1;</script></head>
<body><h1>标题</h1><p>正文内容</p></body></html>`
	got := extractVisibleText([]byte(htmlDoc))
	if !strings.Contains(got, "标题") || !strings.Contains(got, "正文内容") {
		t.Fatalf("缺正文: %q", got)
	}
	if strings.Contains(got, "color:red") || strings.Contains(got, "var a=1") {
		t.Fatalf("未去除 script/style: %q", got)
	}
}
