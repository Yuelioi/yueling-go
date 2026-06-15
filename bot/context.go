package bot

import (
	"fmt"
	"sync"
)

// GroupContext is passed to every group-message handler.
// It embeds *BotAPI (API calls) and *MsgCtx (event data + match cache).
type GroupContext struct {
	*BotAPI
	*MsgCtx
}

// Reply sends text as a quoted reply to the triggering message.
func (c *GroupContext) Reply(text string) error {
	return c.ReplyGroup(c.MsgCtx.Event, text)
}

// Send sends plain text to the group without quoting.
func (c *GroupContext) Send(text string) error {
	return c.SendGroupText(c.GroupID(), text)
}

// SendMsg sends an arbitrary message to the group without quoting.
func (c *GroupContext) SendMsg(msg Message) error {
	_, err := c.SendGroupMsg(c.GroupID(), msg)
	return err
}

// RepliedMessage returns the message this one replies to (fetched via get_msg),
// or ok=false when there is no reply segment or it can't be resolved.
func (c *GroupContext) RepliedMessage() (Message, bool) {
	replyIDStr, ok := c.Message().ReplyID()
	if !ok {
		return nil, false
	}
	var msgID int32
	fmt.Sscan(replyIDStr, &msgID)
	if msgID == 0 {
		return nil, false
	}
	replied, err := c.GetMsg(msgID)
	if err != nil {
		return nil, false
	}
	return replied, true
}

// CollectImageURLs returns image URLs from the current message plus any images
// in the replied-to message.
func (c *GroupContext) CollectImageURLs() []string {
	urls := c.Message().ImageURLs()
	if replied, ok := c.RepliedMessage(); ok {
		urls = append(urls, replied.ImageURLs()...)
	}
	return urls
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
func (m *MsgCtx) SelfID() int64    { return m.Event.SelfID }
func (m *MsgCtx) Nickname() string {
	if m.Event.Sender.Card != "" {
		return m.Event.Sender.Card
	}
	return m.Event.Sender.Nickname
}
