package bot

import (
	"encoding/json"
	"testing"
)

func TestMessageSummary(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{`[{"type":"reply","data":{"id":"123"}},{"type":"text","data":{"text":"zssm "}},{"type":"image","data":{"file":"x.jpg"}}]`, "[回复]zssm [图片]"},
		{`[{"type":"text","data":{"text":"zssm https://wasmer.io/posts/edgejs"}}]`, "zssm https://wasmer.io/posts/edgejs"},
		{`[{"type":"at","data":{"qq":"all"}},{"type":"text","data":{"text":"hi"}}]`, "[@全体]hi"},
		{`[{"type":"at","data":{"qq":"10001"}},{"type":"face","data":{"id":"1"}}]`, "[@10001][表情]"},
		{`[{"type":"forward","data":{}}]`, "[合并转发]"},
	}
	for _, c := range cases {
		var m Message
		if err := json.Unmarshal([]byte(c.raw), &m); err != nil {
			t.Fatalf("unmarshal %q: %v", c.raw, err)
		}
		if got := m.Summary(); got != c.want {
			t.Fatalf("raw=%q got=%q want=%q", c.raw, got, c.want)
		}
	}
}
