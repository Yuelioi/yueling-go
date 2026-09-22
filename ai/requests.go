package ai

import (
	"time"

	"github.com/Yuelioi/yueling-go/bot"
)

type messageKey struct{ bot, group, user, message int64 }
type messageReceipt struct {
	finished bool
	expires  time.Time
}

type claimState int

const (
	claimStarted claimState = iota
	claimDuplicate
	claimFull
)

// claimMessage suppresses duplicates within this process. Failures remain recorded:
// a lost reply cannot prove that an external action did not run.
func (m *SessionManager) claimMessage(event *bot.GroupMessageEvent) (claimState, func()) {
	if event.MessageID == 0 {
		return claimStarted, func() {}
	}
	key := messageKey{event.SelfID, event.GroupID, event.UserID, int64(event.MessageID)}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, receipt := range m.requests {
		if receipt.finished && now.After(receipt.expires) {
			delete(m.requests, k)
		}
	}
	if _, exists := m.requests[key]; exists {
		return claimDuplicate, func() {}
	}
	const maxReceipts = 4096
	if len(m.requests) >= maxReceipts {
		return claimFull, func() {}
	}
	if m.requests == nil {
		m.requests = make(map[messageKey]*messageReceipt)
	}
	receipt := &messageReceipt{}
	m.requests[key] = receipt
	return claimStarted, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		receipt.finished = true
		receipt.expires = time.Now().Add(sessionTTL)
	}
}
