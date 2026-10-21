package main

import (
	"context"
	"encoding/json"
	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
	"os"
	"strings"
	"time"
)

func probeProactiveStyle(settings config.AIConfig) {
	c := config.Current()
	c.AI = settings
	config.Publish(c)
	cases := []struct {
		name, old string
		latest    []string
		forbidden []string
	}{
		{"topic_switch", "显卡价格太贵，显存也不够", []string{"不聊显卡了，晚上吃火锅吧"}, []string{"显卡", "显存", "GPU", "游戏"}},
		{"recent_reference", "杭州下雨，衣服还没干", []string{"晚上吃火锅吗？", "就鸳鸯锅吧，我不太能吃辣", "那就这个"}, []string{"下雨", "衣服", "晾", "天气"}},
		{"unclear_latest", "显卡价格太贵，显存也不够", []string{"换个话题", "https://example.invalid/video"}, []string{"显卡", "显存", "GPU"}},
	}
	failed := false
	for _, test := range cases {
		lines := make([]string, 20)
		for i := range lines {
			lines[i] = test.old
		}
		lines = append(lines, test.latest...)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		reply, err := ai.EvaluateProactive(ctx, lines, nil)
		cancel()
		issues := []string{}
		if err != nil {
			issues = append(issues, "model_error")
		}
		for _, bad := range test.forbidden {
			if strings.Contains(reply, bad) {
				issues = append(issues, "old_topic:"+bad)
			}
		}
		if test.name == "unclear_latest" && strings.TrimSpace(reply) != "NO_REPLY" {
			issues = append(issues, "should_stay_silent")
		}
		if test.name != "unclear_latest" && (strings.TrimSpace(reply) == "NO_REPLY" || strings.TrimSpace(reply) == "") {
			issues = append(issues, "missed_clear_latest_topic")
		}
		failed = failed || len(issues) > 0
		json.NewEncoder(os.Stdout).Encode(map[string]any{"case": test.name, "reply": reply, "passed": len(issues) == 0, "issues": issues})
	}
	if failed {
		os.Exit(1)
	}
}
