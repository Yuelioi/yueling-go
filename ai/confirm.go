package ai

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
)

const confirmTTL = 30 * time.Second

// PendingAction holds a high-risk operation awaiting user confirmation.
type PendingAction struct {
	Code      string
	UserID    int64
	GroupID   int64
	ToolName  string
	Params    map[string]any
	event     *bot.GroupMessageEvent
	toolState map[string]any
	expiresAt time.Time
}

// ConfirmManager stores pending confirmations.
type ConfirmManager struct {
	mu      sync.Mutex
	pending map[string]*PendingAction // actionID → action
}

var Confirms = &ConfirmManager{pending: map[string]*PendingAction{}}

// Store registers a pending action and returns (actionID, 4-digit code).
func (c *ConfirmManager) Store(userID, groupID int64, toolName string, params map[string]any) (actionID, code string) {
	return c.store(userID, groupID, toolName, params, nil, nil)
}

func (c *ConfirmManager) StoreWithContext(userID, groupID int64, toolName string, params map[string]any, event *bot.GroupMessageEvent, toolState map[string]any) (actionID, code string) {
	return c.store(userID, groupID, toolName, params, event, toolState)
}

func (c *ConfirmManager) store(userID, groupID int64, toolName string, params map[string]any, event *bot.GroupMessageEvent, toolState map[string]any) (actionID, code string) {
	actionID = randHex(4)
	code = fmt.Sprintf("%04d", randN(10000))
	var eventCopy *bot.GroupMessageEvent
	if event != nil {
		copied := *event
		copied.Message = append(bot.Message(nil), event.Message...)
		eventCopy = &copied
	}
	stateCopy := make(map[string]any, len(toolState))
	for key, value := range toolState {
		stateCopy[key] = value
	}

	c.mu.Lock()
	now := time.Now()
	for id, pending := range c.pending {
		if now.After(pending.expiresAt) ||
			(pending.UserID == userID && pending.GroupID == groupID && pending.ToolName == toolName) {
			delete(c.pending, id)
		}
	}
	c.pending[actionID] = &PendingAction{
		Code:      code,
		UserID:    userID,
		GroupID:   groupID,
		ToolName:  toolName,
		Params:    params,
		event:     eventCopy,
		toolState: stateCopy,
		expiresAt: now.Add(confirmTTL),
	}
	c.mu.Unlock()
	return actionID, code
}

// Verify checks the code and, on success, removes and returns the action.
func (c *ConfirmManager) Verify(actionID, code string, userID, groupID int64) (*PendingAction, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	p, ok := c.pending[actionID]
	if !ok {
		return nil, false
	}
	if time.Now().After(p.expiresAt) {
		delete(c.pending, actionID)
		return nil, false
	}
	if p.UserID != userID || p.GroupID != groupID || p.Code != code {
		return nil, false
	}
	delete(c.pending, actionID)
	return p, true
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func randN(max int64) int64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(max))
	return n.Int64()
}
