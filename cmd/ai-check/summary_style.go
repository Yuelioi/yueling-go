package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/chatsummary"
	"github.com/Yuelioi/yueling-go/services/llm"
)

type styleSource []chatsummary.Record

func (s styleSource) Read(context.Context, chatsummary.Query) (chatsummary.Source, error) {
	return chatsummary.Source{Scope: "虚构测试：本地前50条取样", Records: s}, nil
}

// This probe judges concrete facts and information leakage, not subjective style.
// Replies are printed so a human can also review how natural they sound.
func probeSummaryStyle(ctx context.Context, client *llm.Client, settings config.AIConfig) {
	cases := []struct {
		name, mode      string
		lines           [][2]string
		want, forbidden []string
	}{
		{"casual", "summary", [][2]string{{"阿岚", "第一次喝威士忌太冲了，有没有好入口的喝法？"}, {"小周", "加冰慢慢喝，别和可乐兑太多，不然像在喝糖水。"}, {"阿岚", "今晚试试加冰。"}, {"小雨", "杭州又下雨了，衣服已经晾三天。"}, {"小周", "阳台快变蘑菇种植基地了。"}, {"阿岚", "刚看个短剧，主角失忆三次了还在结婚。"}, {"小周", "编剧把重启当修复方案了。"}}, []string{"阿岚", "小周", "加冰", "衣服", "失忆"}, []string{"决策", "待办", "负责人", "未明确"}},
		{"actions", "actions", [][2]string{{"阿岚", "我周四修复登录失败的问题。"}, {"小周", "我周五负责回归测试。"}, {"小雨", "消息推送也得检查一下，谁有空接一下？"}}, []string{"阿岚", "周四", "小周", "周五", "消息推送"}, nil},
		{"unread_link", "summary", [][2]string{{"阿岚", "周末想看部电影，有推荐吗？"}, {"小雨", "我没想好。"}, {"小周", "https://example.invalid/video/42"}}, []string{"电影"}, []string{"搞笑视频", "感人", "剧情讲述"}},
		{"missing_name", "summary", [][2]string{{"", "雨停了，终于能晒衣服了。"}, {"", "我也把被子拿出去。"}}, []string{"衣服", "被子"}, nil},
	}
	failed := false
	for _, c := range cases {
		var records styleSource
		for i, line := range c.lines {
			records = append(records, chatsummary.Record{MessageID: int32(900000001 + i), UserID: int64(810000001 + i), Name: line[0], Text: line[1]})
		}
		material, err := chatsummary.Read(ctx, records, chatsummary.Query{Count: 50, Mode: c.mode})
		if err != nil {
			fmt.Fprintln(os.Stderr, "style fixture failed")
			os.Exit(1)
		}
		reply, err := client.Text(ctx, ai.SummaryRequest(material, settings.MaxTokens, settings.ReplyMaxChars))
		issues := []string{}
		if err != nil {
			issues = append(issues, "model_error")
		}
		for _, want := range c.want {
			if !strings.Contains(reply, want) {
				issues = append(issues, "missing:"+want)
			}
		}
		for _, bad := range append([]string{"9000000", "8100000", "message_id", "user_id", "取样", "来源：", "来源:", "本地已保存"}, c.forbidden...) {
			if strings.Contains(reply, bad) {
				issues = append(issues, "unexpected:"+bad)
			}
		}
		passed := len(issues) == 0
		failed = failed || !passed
		json.NewEncoder(os.Stdout).Encode(map[string]any{"case": c.name, "model": settings.Bindings["conversation"], "passed": passed, "issues": issues, "reply": reply})
	}
	if failed {
		os.Exit(1)
	}
}
