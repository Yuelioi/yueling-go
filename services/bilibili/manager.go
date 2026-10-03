package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
)

type PrivateBackend interface {
	AccountID() int64
	Sessions(context.Context, int64) ([]Session, error)
	Messages(context.Context, int64, int64) ([]PrivateMessage, error)
	SendText(context.Context, int64, string) error
}

type CheckpointStore interface {
	Load(int64) ([]byte, error)
	Save(int64, []byte) error
}

type PostgresStore struct{}

func (PostgresStore) Load(id int64) ([]byte, error)    { return db.LoadBilibiliCheckpoint(id) }
func (PostgresStore) Save(id int64, data []byte) error { return db.SaveBilibiliCheckpoint(id, data) }

type cursor struct {
	Seq         int64   `json:"seq"`
	DeliverySeq int64   `json:"delivery_seq,omitempty"`
	DeliveredTo []int64 `json:"delivered_to,omitempty"`
	AutoSeq     int64   `json:"auto_seq,omitempty"`
	LastReply   int64   `json:"last_reply,omitempty"`
}

type checkpoint struct {
	Since         int64                    `json:"since"`
	StartedAt     int64                    `json:"started_at"`
	Mode          string                   `json:"mode"`
	Rules         []config.BilibiliKeyword `json:"rules"`
	Cursors       map[int64]*cursor        `json:"cursors"`
	ReplyTimes    []int64                  `json:"reply_times,omitempty"`
	Settings      *Settings                `json:"settings,omitempty"`
	Contacts      map[int64]*Contact       `json:"contacts,omitempty"`
	NextContactID int                      `json:"next_contact_id,omitempty"`
}

type Manager struct {
	mu            sync.Mutex
	backend       PrivateBackend
	store         CheckpointStore
	cfg           config.BilibiliConfig
	notifyUsers   []int64
	fallbackUsers []int64
	changed       chan struct{}
	state         checkpoint
	loaded        bool
	aiReply       func(context.Context, string) (string, error)
	now           func() time.Time
	lastCheck     time.Time
	lastError     string
}

func NewManager(backend PrivateBackend, store CheckpointStore, cfg config.BilibiliConfig, notifyUsers []int64, aiReply func(context.Context, string) (string, error)) *Manager {
	return &Manager{backend: backend, store: store, cfg: cfg, notifyUsers: slices.Clone(notifyUsers), fallbackUsers: slices.Clone(notifyUsers), changed: make(chan struct{}, 1), aiReply: aiReply, now: time.Now}
}

func (m *Manager) load() error {
	if m.loaded {
		return nil
	}
	if m.backend.AccountID() <= 0 {
		return fmt.Errorf("私信功能需要 Cookie 中的 DedeUserID")
	}
	data, err := m.store.Load(m.backend.AccountID())
	if err != nil {
		return err
	}
	state := checkpoint{Mode: m.cfg.ReplyMode, Rules: slices.Clone(m.cfg.Keywords), Cursors: map[int64]*cursor{}}
	if state.Mode == "" {
		state.Mode = "off"
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &state); err != nil {
			return fmt.Errorf("B站检查点无法解析: %w", err)
		}
	}
	if !config.ValidBilibiliReplyMode(state.Mode) {
		return fmt.Errorf("B站检查点包含无效回复模式")
	}
	if state.Cursors == nil {
		state.Cursors = map[int64]*cursor{}
	}
	if state.Contacts == nil {
		state.Contacts = map[int64]*Contact{}
	}
	if state.Settings != nil {
		if err := state.Settings.Validate(); err != nil {
			return fmt.Errorf("B站已保存设置无效：%w", err)
		}
		state.Settings.apply(&m.cfg)
		m.updateRecipients()
	}
	m.state = state
	m.loaded = true
	return nil
}

func (m *Manager) save() error {
	data, err := json.Marshal(m.state)
	if err != nil {
		return err
	}
	if err := m.store.Save(m.backend.AccountID(), data); err != nil {
		// 持久化失败后从存储重新加载，避免只在内存里推进而丢失未完成的转发。
		m.loaded = false
		return err
	}
	return nil
}

func (m *Manager) Poll(ctx context.Context, notify func(int64, string) error) (processed int, err error) {
	return m.poll(ctx, notify, false)
}

func (m *Manager) poll(ctx context.Context, notify func(int64, string) error, automatic bool) (processed int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	defer func() {
		m.lastCheck = m.now()
		m.lastError = ""
		if err != nil {
			m.lastError = err.Error()
		}
	}()
	if err = m.load(); err != nil {
		return 0, err
	}
	if automatic && (!m.cfg.Enabled || !config.PluginEnabled(44)) {
		return 0, nil
	}
	if len(m.notifyUsers) == 0 {
		return 0, fmt.Errorf("请在后台配置 QQ 私信接收人")
	}
	started := m.now()
	since := m.state.Since
	if since > 0 {
		since -= int64(time.Second / time.Microsecond)
	}
	sessions, err := m.backend.Sessions(ctx, since)
	if err != nil {
		return 0, err
	}
	if m.state.StartedAt == 0 {
		for _, session := range sessions {
			if session.Type == 1 && session.LastMessage != nil {
				m.state.Cursors[session.TalkerID] = &cursor{Seq: session.LastMessage.Seq}
			}
		}
		m.state.StartedAt = started.Unix()
		m.state.Since = started.UnixMicro()
		return 0, m.save()
	}
	var failures []error
	seen := map[int64]bool{}
	for _, session := range sessions {
		if session.Type != 1 || session.SystemType != 0 || session.TalkerID <= 0 || seen[session.TalkerID] {
			continue
		}
		seen[session.TalkerID] = true
		cur := m.state.Cursors[session.TalkerID]
		if cur == nil {
			cur = &cursor{}
			m.state.Cursors[session.TalkerID] = cur
		}
		if session.LastMessage != nil && session.LastMessage.Seq <= cur.Seq {
			continue
		}
		messages, fetchErr := m.backend.Messages(ctx, session.TalkerID, cur.Seq)
		if fetchErr != nil {
			failures = append(failures, fetchErr)
			continue
		}
		for _, msg := range messages {
			if msg.Seq <= cur.Seq {
				continue
			}
			if msg.Timestamp < m.state.StartedAt || msg.SenderID != session.TalkerID || msg.ReceiverID != m.backend.AccountID() || msg.ReceiverType != 1 || msg.Status != 0 || msg.Type == 5 || msg.Source == 5 || msg.Source == 6 || msg.Source == 8 || msg.Source == 9 {
				cur.Seq = msg.Seq
				if err = m.save(); err != nil {
					return processed, err
				}
				continue
			}
			if cur.DeliverySeq != msg.Seq {
				cur.DeliverySeq = msg.Seq
				cur.DeliveredTo = nil
			}
			contact := m.trackContact(ctx, msg)
			if err = m.save(); err != nil {
				return processed, err
			}
			forward := fmt.Sprintf("[B站私信 · 会话 %d]\n%s\n%s\n%s\n回复：%sB站回复 %d <内容>", contact.Number, contact.Name, time.Unix(msg.Timestamp, 0).Format("01-02 15:04:05"), truncate(msg.Text(), 1200), config.Current().Bot.CmdPrefix, contact.Number)
			if len(m.state.Contacts) == 1 {
				forward += fmt.Sprintf("\n当前只有一个联系人，也可直接：%sB站回复 <内容>", config.Current().Bot.CmdPrefix)
			}
			var deliveryErr error
			for _, qq := range m.notifyUsers {
				if slices.Contains(cur.DeliveredTo, qq) {
					continue
				}
				if deliveryErr = notify(qq, forward); deliveryErr != nil {
					break
				}
				cur.DeliveredTo = append(cur.DeliveredTo, qq)
				if err = m.save(); err != nil {
					return processed, err
				}
			}
			if deliveryErr != nil {
				failures = append(failures, deliveryErr)
				break
			}
			previousAutoSeq := cur.AutoSeq
			if replyErr := m.autoReply(ctx, cur, msg); replyErr != nil {
				failures = append(failures, replyErr)
				for _, qq := range m.notifyUsers {
					_ = notify(qq, fmt.Sprintf("[B站自动回复失败] UID %d：%v\n为避免重复发送，未自动重试；可用 B站回复 指令处理。", msg.SenderID, replyErr))
				}
			} else if cur.AutoSeq > previousAutoSeq {
				for _, qq := range m.notifyUsers {
					_ = notify(qq, fmt.Sprintf("[B站自动回复已发送] UID %d · 模式 %s", msg.SenderID, m.state.Mode))
				}
			}
			if !m.loaded {
				return processed, fmt.Errorf("检查点保存失败，等待下次重新加载")
			}
			cur.Seq = msg.Seq
			cur.DeliveredTo = nil
			if err = m.save(); err != nil {
				return processed, err
			}
			processed++
		}
	}
	if len(failures) > 0 {
		return processed, fmt.Errorf("私信同步有 %d 项失败：%w", len(failures), failures[0])
	}
	m.state.Since = started.UnixMicro()
	return processed, m.save()
}

func (m *Manager) autoReply(ctx context.Context, cur *cursor, msg PrivateMessage) error {
	if m.state.Mode == "off" || msg.Type != 1 || msg.Seq <= cur.AutoSeq {
		return nil
	}
	now := m.now().Unix()
	if now-cur.LastReply < int64(m.cfg.ReplyCooldownSeconds) {
		return nil
	}
	m.state.ReplyTimes = slices.DeleteFunc(m.state.ReplyTimes, func(at int64) bool { return at <= now-60 })
	if len(m.state.ReplyTimes) >= m.cfg.ReplyPerMinute {
		return nil
	}
	text := ""
	if m.state.Mode == "keyword" || m.state.Mode == "hybrid" {
		text = MatchKeyword(m.state.Rules, msg.Text())
	}
	if text == "" && (m.state.Mode == "ai" || m.state.Mode == "hybrid") {
		var err error
		if m.aiReply == nil {
			text, err = AIReplyWithPrompt(ctx, msg.Text(), m.cfg.AIPrompt)
		} else {
			text, err = m.aiReply(ctx, msg.Text())
		}
		if err != nil {
			return err
		}
	}
	if strings.TrimSpace(text) == "" {
		return nil
	}
	// 发送前记录尝试；超时或进程退出可能已被上游接收，不能自动重发。
	cur.AutoSeq = msg.Seq
	cur.LastReply = now
	m.state.ReplyTimes = append(m.state.ReplyTimes, now)
	if err := m.save(); err != nil {
		return err
	}
	return m.backend.SendText(ctx, msg.SenderID, text)
}

func (m *Manager) Reply(ctx context.Context, uid int64, text string) error {
	if uid <= 0 || uid == m.backend.AccountID() || strings.TrimSpace(text) == "" {
		return fmt.Errorf("回复目标或内容无效")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.load(); err != nil {
		return err
	}
	cur := m.state.Cursors[uid]
	if cur == nil {
		cur = &cursor{}
		m.state.Cursors[uid] = cur
	}
	cur.LastReply = m.now().Unix()
	if err := m.save(); err != nil {
		return err
	}
	return m.backend.SendText(ctx, uid, text)
}

func (m *Manager) SetMode(mode string) error {
	if !config.ValidBilibiliReplyMode(mode) {
		return fmt.Errorf("回复模式：off / keyword / ai / hybrid")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.load(); err != nil {
		return err
	}
	m.state.Mode = mode
	return m.save()
}

func (m *Manager) Rules() ([]config.BilibiliKeyword, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.load(); err != nil {
		return nil, err
	}
	return slices.Clone(m.state.Rules), nil
}

func (m *Manager) SetRule(rule config.BilibiliKeyword) error {
	rule.Keyword = strings.TrimSpace(rule.Keyword)
	rule.Reply = strings.TrimSpace(rule.Reply)
	if err := config.ValidateBilibiliKeyword(rule); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.load(); err != nil {
		return err
	}
	i := slices.IndexFunc(m.state.Rules, func(r config.BilibiliKeyword) bool { return r.Keyword == rule.Keyword })
	if i < 0 {
		if len(m.state.Rules) >= 50 {
			return fmt.Errorf("最多保存 50 条关键词规则")
		}
		m.state.Rules = append(m.state.Rules, rule)
	} else {
		m.state.Rules[i] = rule
	}
	return m.save()
}

func (m *Manager) DeleteRule(keyword string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.load(); err != nil {
		return err
	}
	m.state.Rules = slices.DeleteFunc(m.state.Rules, func(r config.BilibiliKeyword) bool { return r.Keyword == keyword })
	return m.save()
}

func (m *Manager) Status() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.load(); err != nil {
		return err.Error()
	}
	checked := "尚未检查"
	if !m.lastCheck.IsZero() {
		checked = m.lastCheck.Format("01-02 15:04:05")
	}
	lastError := m.lastError
	if lastError == "" {
		lastError = "无"
	}
	monitor := "已暂停"
	if m.cfg.Enabled {
		monitor = "已开启"
	}
	return fmt.Sprintf("自动轮询：%s · 每 %d 秒\n自动回复：%s\n关键词规则：%d\n上次检查：%s\n最近错误：%s", monitor, m.cfg.PollSeconds, m.state.Mode, len(m.state.Rules), checked, lastError)
}

func MatchKeyword(rules []config.BilibiliKeyword, text string) string {
	text = strings.TrimSpace(text)
	for _, rule := range rules {
		if (rule.Exact && text == rule.Keyword) || (!rule.Exact && strings.Contains(strings.ToLower(text), strings.ToLower(rule.Keyword))) {
			return rule.Reply
		}
	}
	return ""
}

func truncate(text string, limit int) string {
	chars := []rune(text)
	if len(chars) > limit {
		return string(chars[:limit]) + "…"
	}
	return text
}
