package group

import (
	"reflect"
	"testing"
)

func TestDecideJoin(t *testing.T) {
	allow := []string{"交流", "学习"}
	deny := []string{"广告"}
	cases := []struct {
		name    string
		comment string
		allow   []string
		deny    []string
		want    joinDecision
	}{
		{"命中通过词", "我想来交流技术", allow, deny, decisionApprove},
		{"命中拒绝词", "代理广告招商", allow, deny, decisionReject},
		{"拒绝优先", "交流广告", allow, deny, decisionReject},
		{"通配任意非空", "随便写点", []string{"*"}, nil, decisionApprove},
		{"空comment留人工", "", []string{"*"}, deny, decisionNone},
		{"都不命中留人工", "你好啊", allow, deny, decisionNone},
		{"无配置留人工", "交流", nil, nil, decisionNone},
	}
	for _, c := range cases {
		if got := decideJoin(c.comment, c.allow, c.deny); got != c.want {
			t.Errorf("%s: decideJoin=%d want %d", c.name, got, c.want)
		}
	}
}

func TestParseKeywordArg(t *testing.T) {
	cases := []struct {
		raw     string
		wantAdd bool
		wantKws []string
		wantOK  bool
	}{
		{"+交流", true, []string{"交流"}, true},
		{"-广告", false, []string{"广告"}, true},
		{"+交流,学习", true, []string{"交流", "学习"}, true},
		{"+交流，学习", true, []string{"交流", "学习"}, true},
		{"+大写ABC", true, []string{"大写abc"}, true},
		{"+*", true, []string{"*"}, true},
		{"交流", false, nil, false},
		{"+", false, nil, false},
		{"+ , ", false, nil, false},
		{"", false, nil, false},
	}
	for _, c := range cases {
		add, kws, ok := parseKeywordArg(c.raw)
		if ok != c.wantOK || add != c.wantAdd || !reflect.DeepEqual(kws, c.wantKws) {
			t.Errorf("%q: got (add=%v kws=%v ok=%v) want (add=%v kws=%v ok=%v)",
				c.raw, add, kws, ok, c.wantAdd, c.wantKws, c.wantOK)
		}
	}
}
