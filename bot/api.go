package bot

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var echoSeq uint64

// BotAPI wraps the active WebSocket connection and exposes OneBot v11 API calls.
// It is created once per NapCat connection and embedded in every Context type.
type BotAPI struct {
	SelfID  int64
	sendCh  chan<- []byte
	done    <-chan struct{} // closed when this connection's recvLoop exits
	pending sync.Map        // echo → chan json.RawMessage
}

// ---- Group message ----

func (a *BotAPI) SendGroupMsg(groupID int64, msg Message) (int32, error) {
	var resp struct {
		MessageID int32 `json:"message_id"`
	}
	raw, err := a.call("send_group_msg", map[string]any{
		"group_id": groupID,
		"message":  msg,
	})
	if err != nil {
		return 0, err
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, err
	}
	return resp.MessageID, nil
}

func (a *BotAPI) SendGroupText(groupID int64, text string) error {
	_, err := a.SendGroupMsg(groupID, Msg().Text(text).Build())
	return err
}

// SendGroupLocalImage sends an image from a local file path.
func (a *BotAPI) SendGroupLocalImage(groupID int64, path string) error {
	_, err := a.SendGroupMsg(groupID, Msg().LocalImage(path).Build())
	return err
}

func (a *BotAPI) ReplyGroup(e *GroupMessageEvent, text string) error {
	_, err := a.SendGroupMsg(e.GroupID, Msg().Reply(e.MessageID).Text(text).Build())
	return err
}

// ---- Private message ----

func (a *BotAPI) SendPrivateMsg(userID int64, msg Message) (int32, error) {
	var resp struct {
		MessageID int32 `json:"message_id"`
	}
	raw, err := a.call("send_private_msg", map[string]any{
		"user_id": userID,
		"message": msg,
	})
	if err != nil {
		return 0, err
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, err
	}
	return resp.MessageID, nil
}

// ---- Group management ----

func (a *BotAPI) SetGroupBan(groupID, userID int64, seconds int) error {
	_, err := a.call("set_group_ban", map[string]any{
		"group_id": groupID,
		"user_id":  userID,
		"duration": seconds,
	})
	return err
}

func (a *BotAPI) SetGroupWholeBan(groupID int64, enable bool) error {
	_, err := a.call("set_group_whole_ban", map[string]any{
		"group_id": groupID,
		"enable":   enable,
	})
	return err
}

func (a *BotAPI) SetGroupCard(groupID, userID int64, card string) error {
	_, err := a.call("set_group_card", map[string]any{
		"group_id": groupID,
		"user_id":  userID,
		"card":     card,
	})
	return err
}

func (a *BotAPI) SetGroupKick(groupID, userID int64, rejectFuture bool) error {
	_, err := a.call("set_group_kick", map[string]any{
		"group_id":           groupID,
		"user_id":            userID,
		"reject_add_request": rejectFuture,
	})
	return err
}

func (a *BotAPI) SetGroupAdmin(groupID, userID int64, enable bool) error {
	_, err := a.call("set_group_admin", map[string]any{
		"group_id": groupID,
		"user_id":  userID,
		"enable":   enable,
	})
	return err
}

func (a *BotAPI) SetGroupAddRequest(flag, subType string, approve bool, reason string) error {
	_, err := a.call("set_group_add_request", map[string]any{
		"flag":     flag,
		"sub_type": subType,
		"approve":  approve,
		"reason":   reason,
	})
	return err
}

// ---- Message operations ----

func (a *BotAPI) DeleteMsg(msgID int32) error {
	_, err := a.call("delete_msg", map[string]any{"message_id": msgID})
	return err
}

func (a *BotAPI) SetEssenceMsg(msgID int32) error {
	_, err := a.call("set_essence_msg", map[string]any{"message_id": msgID})
	return err
}

// SetMsgEmojiLike 给一条消息贴/撤表情回应（set_msg_emoji_like）。
// emojiID 为 QQ 表情 id 或 emoji 码点字符串；set=true 贴，false 撤。
func (a *BotAPI) SetMsgEmojiLike(messageID int32, emojiID string, set bool) error {
	_, err := a.call("set_msg_emoji_like", map[string]any{
		"message_id": messageID,
		"emoji_id":   emojiID,
		"set":        set,
	})
	return err
}

func (a *BotAPI) GetMsg(msgID int32) (Message, error) {
	raw, err := a.call("get_msg", map[string]any{"message_id": msgID})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Message Message `json:"message"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	return resp.Message, nil
}

func parseForwardMsg(raw json.RawMessage) []Message {
	var resp struct {
		Messages []struct {
			Message Message `json:"message"`
			Content Message `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil
	}
	out := make([]Message, 0, len(resp.Messages))
	for _, m := range resp.Messages {
		switch {
		case len(m.Message) > 0:
			out = append(out, m.Message)
		case len(m.Content) > 0:
			out = append(out, m.Content)
		}
	}
	return out
}

func (a *BotAPI) GetForwardMsg(id string) ([]Message, error) {
	raw, err := a.call("get_forward_msg", map[string]any{"message_id": id})
	if err != nil {
		return nil, err
	}
	return parseForwardMsg(raw), nil
}

// ---- Info queries ----

func (a *BotAPI) GetGroupMemberInfo(groupID, userID int64) (*Sender, error) {
	raw, err := a.call("get_group_member_info", map[string]any{
		"group_id": groupID,
		"user_id":  userID,
	})
	if err != nil {
		return nil, err
	}
	var s Sender
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (a *BotAPI) GetGroupMemberList(groupID int64) ([]Sender, error) {
	raw, err := a.call("get_group_member_list", map[string]any{
		"group_id": groupID,
	})
	if err != nil {
		return nil, err
	}
	var list []Sender
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	return list, nil
}

type GroupInfo struct {
	GroupID   int64  `json:"group_id"`
	GroupName string `json:"group_name"`
}

func (a *BotAPI) GetGroupInfo(groupID int64) (*GroupInfo, error) {
	raw, err := a.call("get_group_info", map[string]any{"group_id": groupID})
	if err != nil {
		return nil, err
	}
	var gi GroupInfo
	if err := json.Unmarshal(raw, &gi); err != nil {
		return nil, err
	}
	return &gi, nil
}

var groupNameCache sync.Map

// GroupName returns the group's display name, cached after the first lookup.
// Falls back to an empty string when the lookup fails.
func (a *BotAPI) GroupName(groupID int64) string {
	if v, ok := groupNameCache.Load(groupID); ok {
		return v.(string)
	}
	gi, err := a.GetGroupInfo(groupID)
	if err != nil || gi.GroupName == "" {
		return ""
	}
	groupNameCache.Store(groupID, gi.GroupName)
	return gi.GroupName
}

// ---- History ----

// HistoryMessage is one entry returned by get_group_msg_history.
type HistoryMessage struct {
	UserID  int64  `json:"user_id"`
	Sender  Sender `json:"sender"`
	Message []struct {
		Type string `json:"type"`
		Data struct {
			Text string `json:"text"`
			QQ   string `json:"qq"`
		} `json:"data"`
	} `json:"message"`
}

func (a *BotAPI) GetGroupMsgHistory(groupID int64, messageID int32, count int) ([]HistoryMessage, error) {
	raw, err := a.call("get_group_msg_history", map[string]any{
		"group_id":   groupID,
		"message_id": messageID,
		"count":      count,
	})
	if err != nil {
		return nil, err
	}
	var result struct {
		Messages []HistoryMessage `json:"messages"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result.Messages, nil
}

// ---- Internal ----

func (a *BotAPI) call(action string, params any) (json.RawMessage, error) {
	echo := fmt.Sprintf("%s_%d", action, atomic.AddUint64(&echoSeq, 1))
	ch := make(chan json.RawMessage, 1)
	a.pending.Store(echo, ch)
	defer a.pending.Delete(echo)

	payload, err := json.Marshal(map[string]any{
		"action": action,
		"params": params,
		"echo":   echo,
	})
	if err != nil {
		return nil, err
	}

	select {
	case a.sendCh <- payload:
	case <-a.done:
		return nil, fmt.Errorf("connection closed: %s", action)
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("send timeout: %s", action)
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-a.done:
		return nil, fmt.Errorf("connection closed: %s", action)
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("response timeout: %s", action)
	}
}

func (a *BotAPI) deliver(echo string, data json.RawMessage) {
	if v, ok := a.pending.Load(echo); ok {
		select {
		case v.(chan json.RawMessage) <- data:
		default:
		}
	}
}
