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

func TestFormatZssmResponse(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{`{"output":"地球是圆的","keyword":["地球"],"block":false}`, "关键词：地球\n\n地球是圆的"},
		{"```json\n{\"output\":\"x\",\"keyword\":[],\"block\":false}\n```", "x"},
		{`{"output":"","keyword":[],"block":true}`, "（抱歉, 我现在还不会这个）"},
	}
	for _, c := range cases {
		got, err := formatZssmResponse(c.raw)
		if err != nil {
			t.Fatalf("raw=%q err=%v", c.raw, err)
		}
		if got != c.want {
			t.Fatalf("raw=%q got=%q want=%q", c.raw, got, c.want)
		}
	}
	if _, err := formatZssmResponse("not json"); err == nil {
		t.Fatalf("非 JSON 应当报错")
	}
}
