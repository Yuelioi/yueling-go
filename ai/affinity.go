package ai

import (
	"strings"
	"unicode/utf8"

	"github.com/Yuelioi/yueling-go/config"
)

type AffinityEvent struct {
	Message string
}

func ClassifyAffinityDelta(event AffinityEvent) (int, string) {
	message := strings.ToLower(strings.TrimSpace(event.Message))
	if message == "" {
		return 0, ""
	}

	negativeKeywords := []string{
		"涩涩", "黄色", "色情", "色图", "裸照", "开车", "约炮", "操你", "傻逼",
		"去死", "越狱", "jailbreak", "忽略之前", "忽略以上", "system prompt",
	}
	for _, keyword := range negativeKeywords {
		if strings.Contains(message, keyword) {
			return -10, "包含冒犯、骚扰或越界内容"
		}
	}

	positiveKeywords := []string{"请", "谢谢", "麻烦", "帮我", "总结", "部署", "辛苦"}
	for _, keyword := range positiveKeywords {
		if strings.Contains(message, keyword) {
			return 2, "礼貌正常交流"
		}
	}

	if utf8.RuneCountInString(message) >= 8 {
		return 1, "正常交流"
	}
	return 0, ""
}

func NormalizeAffinityConfig(cfg config.AffinityConfig) config.AffinityConfig {
	if cfg.Min == 0 && cfg.Max == 0 {
		cfg.Min = 0
		cfg.Max = 100
	}
	if cfg.Initial == 0 {
		cfg.Initial = 50
	}
	if cfg.BlockBelow == 0 {
		cfg.BlockBelow = 10
	}
	if cfg.Max < cfg.Min {
		cfg.Max = cfg.Min
	}
	if cfg.Initial < cfg.Min {
		cfg.Initial = cfg.Min
	}
	if cfg.Initial > cfg.Max {
		cfg.Initial = cfg.Max
	}
	if cfg.BlockBelow < cfg.Min {
		cfg.BlockBelow = cfg.Min
	}
	if cfg.BlockBelow > cfg.Max {
		cfg.BlockBelow = cfg.Max
	}
	return cfg
}

func ApplyAffinityDelta(current, delta int, cfg config.AffinityConfig) int {
	cfg = NormalizeAffinityConfig(cfg)
	next := current + delta
	if next < cfg.Min {
		return cfg.Min
	}
	if next > cfg.Max {
		return cfg.Max
	}
	return next
}

func AffinityPrompt(score int, cfg config.AffinityConfig) string {
	cfg = NormalizeAffinityConfig(cfg)
	switch {
	case score < cfg.BlockBelow:
		return "当前关系状态：疏远。回复要保持克制、简短，避免主动亲近。"
	case score < cfg.Initial:
		return "当前关系状态：普通。回复保持礼貌自然。"
	case score >= cfg.Max:
		return "当前关系状态：亲近。回复可以更熟悉，但仍需遵守边界。"
	default:
		return "当前关系状态：友好。回复自然、温和。"
	}
}
