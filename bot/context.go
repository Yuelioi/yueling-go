package bot

import "sync"

// GroupContext is passed to every group-message handler.
// It embeds *BotAPI (API calls) and *MsgCtx (event data + match cache).
type GroupContext struct {
	*BotAPI
	*MsgCtx
}

// Reply is a convenience wrapper for the common reply pattern.
func (c *GroupContext) Reply(text string) error {
	return c.ReplyGroup(c.MsgCtx.Event, text)
}

// CommandContext extends GroupContext with parsed command and arguments.
type CommandContext struct {
	*GroupContext
	Cmd  string
	Args []string
}

// NoticeContext is passed to notice handlers.
type NoticeContext struct {
	*BotAPI
	Event *NoticeEvent
}

func (c *NoticeContext) Reply(text string) error {
	return c.SendGroupText(c.Event.GroupID, text)
}

// RequestContext is passed to request (friend/group-join) handlers.
type RequestContext struct {
	*BotAPI
	Event *RequestEvent
}

// ---- MsgCtx ----

// MsgCtx wraps a GroupMessageEvent with a per-event match cache.
// Matchers store their results here to avoid re-running on the same event.
type MsgCtx struct {
	Event      *GroupMessageEvent
	matchCache sync.Map // string → MatchResult
}

func (m *MsgCtx) UserID() int64    { return m.Event.UserID }
func (m *MsgCtx) GroupID() int64   { return m.Event.GroupID }
func (m *MsgCtx) Text() string     { return m.Event.Message.Text() }
func (m *MsgCtx) Message() Message { return m.Event.Message }
func (m *MsgCtx) Role() string     { return m.Event.Sender.Role }
func (m *MsgCtx) SelfID() int64 { return m.Event.SelfID }
func (m *MsgCtx) Nickname() string {
	if m.Event.Sender.Card != "" {
		return m.Event.Sender.Card
	}
	return m.Event.Sender.Nickname
}
