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

func TestParseKeywords(t *testing.T) {
	cases := []struct {
		raw  string
		want []string
	}{
		{"交流", []string{"交流"}},
		{"交流,学习", []string{"交流", "学习"}},
		{"交流，学习", []string{"交流", "学习"}},
		{"大写ABC", []string{"大写abc"}},
		{"*", []string{"*"}},
		{" 交流 , 学习 ", []string{"交流", "学习"}},
		{"", nil},
		{" , ", nil},
	}
	for _, c := range cases {
		if got := parseKeywords(c.raw); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: parseKeywords=%v want %v", c.raw, got, c.want)
		}
	}
}
