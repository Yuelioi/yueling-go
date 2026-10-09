package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

type BilibiliConfig struct {
	Enabled              bool              `toml:"enabled" json:"enabled"`
	Cookie               string            `toml:"cookie" json:"cookie"`
	UID                  int64             `toml:"uid" json:"uid"`
	RecentCount          int               `toml:"recent_count" json:"recent_count"`
	NotifyUsers          []int64           `toml:"notify_users" json:"notify_users"`
	PollSeconds          int               `toml:"poll_seconds" json:"poll_seconds"`
	ReplyMode            string            `toml:"reply_mode" json:"reply_mode"`
	ReplyCooldownSeconds int               `toml:"reply_cooldown_seconds" json:"reply_cooldown_seconds"`
	ReplyPerMinute       int               `toml:"reply_per_minute" json:"reply_per_minute"`
	AIPrompt             string            `toml:"ai_prompt" json:"ai_prompt"`
	Keywords             []BilibiliKeyword `toml:"keywords" json:"keywords"`
}

type BilibiliKeyword struct {
	Keyword string `toml:"keyword" json:"keyword"`
	Reply   string `toml:"reply" json:"reply"`
	Exact   bool   `toml:"exact" json:"exact"`
}

func (c BilibiliConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.UID < 0 {
		return fmt.Errorf("bilibili.uid must not be negative")
	}
	if c.RecentCount < 1 || c.RecentCount > 10 {
		return fmt.Errorf("bilibili.recent_count must be between 1 and 10")
	}
	if strings.ContainsAny(c.Cookie, "\r\n") {
		return fmt.Errorf("bilibili.cookie must be a single HTTP Cookie header value")
	}
	if c.PollSeconds < 20 || c.PollSeconds > 3600 {
		return fmt.Errorf("bilibili.poll_seconds must be between 20 and 3600")
	}
	if c.ReplyCooldownSeconds < 0 || c.ReplyCooldownSeconds > 86400 || c.ReplyPerMinute < 1 || c.ReplyPerMinute > 60 {
		return fmt.Errorf("invalid bilibili reply rate limits")
	}
	if !ValidBilibiliReplyMode(c.ReplyMode) {
		return fmt.Errorf("bilibili.reply_mode must be off, keyword, ai or hybrid")
	}
	for _, uid := range c.NotifyUsers {
		if uid <= 0 {
			return fmt.Errorf("bilibili.notify_users must contain positive QQ numbers")
		}
	}
	if len(c.Keywords) > 50 {
		return fmt.Errorf("bilibili.keywords supports at most 50 rules")
	}
	for _, rule := range c.Keywords {
		if err := ValidateBilibiliKeyword(rule); err != nil {
			return err
		}
	}
	return nil
}

func ValidateBilibiliKeyword(rule BilibiliKeyword) error {
	if strings.TrimSpace(rule.Keyword) == "" || strings.TrimSpace(rule.Reply) == "" {
		return fmt.Errorf("bilibili.keywords requires non-empty keyword and reply")
	}
	if len([]rune(rule.Keyword)) > 100 {
		return fmt.Errorf("bilibili keyword cannot exceed 100 characters")
	}
	content, _ := json.Marshal(map[string]string{"content": rule.Reply})
	if len(content) > 2000 {
		return fmt.Errorf("bilibili keyword reply cannot exceed 2000 encoded bytes")
	}
	return nil
}

func ValidBilibiliReplyMode(mode string) bool {
	return mode == "off" || mode == "keyword" || mode == "ai" || mode == "hybrid"
}
