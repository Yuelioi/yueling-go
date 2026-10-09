package bilibili

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
)

type Settings struct {
	Enabled              bool                     `json:"enabled"`
	PollSeconds          int                      `json:"poll_seconds"`
	NotifyUsers          []int64                  `json:"notify_users"`
	ReplyMode            string                   `json:"reply_mode"`
	Keywords             []config.BilibiliKeyword `json:"keywords"`
	ReplyCooldownSeconds int                      `json:"reply_cooldown_seconds"`
	ReplyPerMinute       int                      `json:"reply_per_minute"`
	AIPrompt             string                   `json:"ai_prompt"`
	UID                  int64                    `json:"uid"`
	RecentCount          int                      `json:"recent_count"`
}

func (m *Manager) AccountID() int64 { return m.backend.AccountID() }

func settingsFromConfig(c config.BilibiliConfig) Settings {
	return Settings{c.Enabled, c.PollSeconds, slices.Clone(c.NotifyUsers), c.ReplyMode,
		slices.Clone(c.Keywords), c.ReplyCooldownSeconds, c.ReplyPerMinute, c.AIPrompt, c.UID, c.RecentCount}
}

func (s Settings) apply(c *config.BilibiliConfig) {
	c.Enabled, c.PollSeconds, c.NotifyUsers = s.Enabled, s.PollSeconds, slices.Clone(s.NotifyUsers)
	c.ReplyMode, c.Keywords = s.ReplyMode, slices.Clone(s.Keywords)
	c.ReplyCooldownSeconds, c.ReplyPerMinute, c.AIPrompt = s.ReplyCooldownSeconds, s.ReplyPerMinute, s.AIPrompt
	c.UID, c.RecentCount = s.UID, s.RecentCount
}

func (s Settings) Validate() error {
	if len([]rune(s.AIPrompt)) > 4000 {
		return fmt.Errorf("AI 提示词不能超过 4000 个字符")
	}
	seen := map[string]bool{}
	for _, rule := range s.Keywords {
		key := strings.TrimSpace(rule.Keyword)
		if seen[key] {
			return fmt.Errorf("关键词不能重复：%s", key)
		}
		seen[key] = true
	}
	c := config.BilibiliConfig{}
	s.apply(&c)
	c.Enabled = true
	return c.Validate()
}

func (m *Manager) Settings() (Settings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.backend.AccountID() > 0 {
		if err := m.load(); err != nil {
			return Settings{}, err
		}
	}
	s := settingsFromConfig(m.cfg)
	if m.loaded {
		s.ReplyMode, s.Keywords = m.state.Mode, slices.Clone(m.state.Rules)
	}
	if s.PollSeconds < 20 {
		s.PollSeconds = 60
	}
	if s.RecentCount == 0 {
		s.RecentCount = 5
	}
	if s.NotifyUsers == nil {
		s.NotifyUsers = []int64{}
	}
	if s.Keywords == nil {
		s.Keywords = []config.BilibiliKeyword{}
	}
	return s, nil
}

func (m *Manager) SaveSettings(s Settings) error {
	s.NotifyUsers = slices.Clone(s.NotifyUsers)
	s.Keywords = slices.Clone(s.Keywords)
	if err := s.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.load(); err != nil {
		return err
	}
	m.state.Settings = &s
	m.state.Mode, m.state.Rules = s.ReplyMode, slices.Clone(s.Keywords)
	if err := m.save(); err != nil {
		return fmt.Errorf("B站设置保存失败：%w", err)
	}
	s.apply(&m.cfg)
	m.updateRecipients()
	select {
	case m.changed <- struct{}{}:
	default:
	}
	return nil
}

func (m *Manager) SetFallbackRecipients(ids []int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fallbackUsers = slices.Clone(ids)
	m.updateRecipients()
}

func (m *Manager) updateRecipients() {
	ids := m.cfg.NotifyUsers
	if len(ids) == 0 {
		ids = m.fallbackUsers
	}
	m.notifyUsers = nil
	for _, id := range ids {
		if id > 0 && !slices.Contains(m.notifyUsers, id) {
			m.notifyUsers = append(m.notifyUsers, id)
		}
	}
}

func (m *Manager) CanManage(qq int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.backend.AccountID() > 0 {
		if err := m.load(); err != nil {
			return false
		}
	}
	return slices.Contains(m.notifyUsers, qq)
}

func (m *Manager) Run(ctx context.Context, connected func() bool, notify func(int64, string) error, onError func(error)) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.changed:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-timer.C:
			s, err := m.Settings()
			if err != nil {
				onError(err)
			} else if s.Enabled && config.PluginEnabled(catalog.PluginBilibili) && connected() {
				pollCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				_, err = m.poll(pollCtx, notify, true)
				cancel()
				if err != nil && ctx.Err() == nil {
					onError(err)
				}
			}
		}
		s, err := m.Settings()
		interval := time.Minute
		if err == nil {
			interval = time.Duration(s.PollSeconds) * time.Second
		}
		timer.Reset(interval)
	}
}
