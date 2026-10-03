package bilibili

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Contact struct {
	Number        int    `json:"number"`
	UID           int64  `json:"uid"`
	Preview       string `json:"preview"`
	LastMessageAt int64  `json:"last_message_at"`
	Name          string `json:"name"`
}

func (m *Manager) trackContact(ctx context.Context, msg PrivateMessage) *Contact {
	c := m.state.Contacts[msg.SenderID]
	if c == nil {
		m.state.NextContactID++
		c = &Contact{Number: m.state.NextContactID, UID: msg.SenderID, Name: fmt.Sprintf("用户 %d", msg.SenderID)}
		if lookup, ok := m.backend.(interface {
			ContactName(context.Context, int64) (string, error)
		}); ok {
			lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			name, err := lookup.ContactName(lookupCtx, msg.SenderID)
			cancel()
			if err == nil && strings.TrimSpace(name) != "" {
				c.Name = truncate(strings.TrimSpace(name), 80)
			}
		}
		m.state.Contacts[msg.SenderID] = c
	}
	c.Preview, c.LastMessageAt = truncate(msg.Text(), 160), msg.Timestamp
	return c
}

func (m *Manager) Contacts() ([]Contact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.load(); err != nil {
		return nil, err
	}
	result := make([]Contact, 0, len(m.state.Contacts))
	for _, c := range m.state.Contacts {
		result = append(result, *c)
	}
	slices.SortFunc(result, func(a, b Contact) int { return a.Number - b.Number })
	return result, nil
}

func (m *Manager) ResolveReply(body string) (int64, int, string, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return 0, 0, "", fmt.Errorf("用法：B站回复 <内容>；多人时：B站回复 <会话编号> <内容>，用 B站会话 查看编号")
	}
	contacts, err := m.Contacts()
	if err != nil {
		return 0, 0, "", err
	}
	if i := strings.IndexFunc(body, unicode.IsSpace); i > 0 {
		if n, err := strconv.Atoi(body[:i]); err == nil {
			text := strings.TrimSpace(body[i:])
			for _, c := range contacts {
				if c.Number == n {
					return c.UID, c.Number, text, nil
				}
			}
			return 0, 0, "", fmt.Errorf("会话编号 %d 不存在，请用 B站会话 查看编号", n)
		}
	}
	if len(contacts) == 1 {
		return contacts[0].UID, contacts[0].Number, body, nil
	}
	if len(contacts) == 0 {
		return 0, 0, "", fmt.Errorf("暂无收到新私信的联系人；收到转发后可直接回复")
	}
	return 0, 0, "", fmt.Errorf("当前有 %d 个联系人，请用 B站回复 <会话编号> <内容>；B站会话 可查看编号", len(contacts))
}
