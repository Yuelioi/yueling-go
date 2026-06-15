package bot

import "testing"

func TestParseForwardMsg(t *testing.T) {
	raw := []byte(`{"messages":[
		{"message":[{"type":"text","data":{"text":"hi"}},{"type":"image","data":{"url":"http://a/1.jpg"}}]},
		{"message":[{"type":"forward","data":{"id":"999"}}]},
		{"content":[{"type":"image","data":{"url":"http://a/2.jpg"}}]}
	]}`)
	msgs := parseForwardMsg(raw)
	if len(msgs) != 3 {
		t.Fatalf("want 3 messages, got %d", len(msgs))
	}
	if got := msgs[0].ImageURLs(); len(got) != 1 || got[0] != "http://a/1.jpg" {
		t.Fatalf("msg0 images = %v", got)
	}
	if msgs[1][0].Type != "forward" {
		t.Fatalf("msg1 seg0 type = %q", msgs[1][0].Type)
	}
	if got := msgs[2].ImageURLs(); len(got) != 1 || got[0] != "http://a/2.jpg" {
		t.Fatalf("msg2 images (content fallback) = %v", got)
	}
	if parseForwardMsg([]byte(`not json`)) != nil {
		t.Fatalf("非法 JSON 应返回 nil")
	}
}
