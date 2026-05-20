package ai

import (
	"github.com/Yuelioi/yueling-go/bot"
)

// ToolContext is passed to every AI tool handler.
type ToolContext struct {
	api    *bot.BotAPI
	event  *bot.GroupMessageEvent
	session *Session
	Params map[string]any
}

func newToolCtx(api *bot.BotAPI, e *bot.GroupMessageEvent, s *Session, params map[string]any) *ToolContext {
	return &ToolContext{api: api, event: e, session: s, Params: params}
}

// BotAPI exposes the raw API for tools that need calls not covered by helper methods.
func (c *ToolContext) BotAPI() *bot.BotAPI { return c.api }

func (c *ToolContext) UserID() int64    { return c.event.UserID }
func (c *ToolContext) GroupID() int64   { return c.event.GroupID }
func (c *ToolContext) MessageID() int32 { return c.event.MessageID }
func (c *ToolContext) Role() string   { return c.event.Sender.Role }
func (c *ToolContext) Nickname() string {
	if c.event.Sender.Card != "" {
		return c.event.Sender.Card
	}
	return c.event.Sender.Nickname
}

func (c *ToolContext) Reply(text string) error {
	return c.api.ReplyGroup(c.event, text)
}

func (c *ToolContext) SendText(text string) error {
	return c.api.SendGroupText(c.event.GroupID, text)
}

// String returns a param as string, empty if missing or wrong type.
func (c *ToolContext) String(key string) string {
	v, ok := c.Params[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// Int returns a param as int64 (JSON numbers come in as float64).
func (c *ToolContext) Int(key string) int64 {
	v, ok := c.Params[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	}
	return 0
}

// Float returns a param as float64.
func (c *ToolContext) Float(key string) float64 {
	v, ok := c.Params[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	}
	return 0
}

// Bool returns a param as bool.
func (c *ToolContext) Bool(key string) bool {
	v, ok := c.Params[key]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// SetState stores a value in the session's tool state (not sent to LLM).
func (c *ToolContext) SetState(key string, val any) {
	c.session.ToolState[key] = val
}

// GetState retrieves a value from the session's tool state.
func (c *ToolContext) GetState(key string) (any, bool) {
	v, ok := c.session.ToolState[key]
	return v, ok
}
