package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/Yuelioi/bpi-go/message"
)

type Session struct {
	TalkerID    int64           `json:"talker_id"`
	Type        int             `json:"session_type"`
	Timestamp   int64           `json:"session_ts"`
	SystemType  int             `json:"system_msg_type"`
	LastMessage *PrivateMessage `json:"last_msg"`
}

type PrivateMessage struct {
	SenderID     int64  `json:"sender_uid"`
	ReceiverID   int64  `json:"receiver_id"`
	ReceiverType int    `json:"receiver_type"`
	Type         int    `json:"msg_type"`
	Content      string `json:"content"`
	Seq          int64  `json:"msg_seqno"`
	Timestamp    int64  `json:"timestamp"`
	Status       int    `json:"msg_status"`
	Source       int    `json:"msg_source"`
}

func (c *Client) AccountID() int64 {
	return c.accountID
}

func (c *Client) Sessions(ctx context.Context, since int64) ([]Session, error) {
	if since < 0 {
		return nil, fmt.Errorf("私信会话时间游标无效")
	}
	var result []Session
	seen := map[[2]int64]int{}
	api := message.NewClient(c.sdk)
	// All 查询可能忽略 end_ts，分类读取才能可靠翻页。
	for _, kind := range []message.SessionListType{message.SessionUserAndSystem, message.SessionUnfollowed} {
		params, err := message.NewSessionsParams().WithSessionType(kind)
		if err != nil {
			return nil, err
		}
		if since > 0 {
			params, err = params.WithBeginTS(uint64(since))
			if err != nil {
				return nil, err
			}
		}
		var end uint64
		completed := false
		for page := 0; page < 50; page++ {
			body, err := api.Sessions(ctx, params)
			if err != nil {
				return nil, err
			}
			var next uint64
			for _, item := range body.SessionList {
				if item.TalkerID > math.MaxInt64 || item.SessionTS > math.MaxInt64 {
					return nil, fmt.Errorf("私信会话编号或时间超过本地存储范围")
				}
				session := Session{TalkerID: int64(item.TalkerID), Type: int(item.SessionType), Timestamp: int64(item.SessionTS), SystemType: int(item.SystemMsgType)}
				if item.LastMessage != nil {
					last, err := fromSDKMessage(*item.LastMessage)
					if err != nil {
						return nil, err
					}
					session.LastMessage = &last
				}
				key := [2]int64{session.TalkerID, int64(session.Type)}
				if index, exists := seen[key]; exists {
					old := result[index]
					if session.Timestamp > old.Timestamp || (session.Timestamp == old.Timestamp && sessionSeq(session) > sessionSeq(old)) {
						result[index] = session
					}
				} else {
					seen[key] = len(result)
					result = append(result, session)
				}
				if item.SessionTS > 0 && (next == 0 || item.SessionTS < next) {
					next = item.SessionTS
				}
			}
			if body.HasMore == 0 {
				completed = true
				break
			}
			if next == 0 || (end > 0 && next >= end) {
				return nil, fmt.Errorf("私信会话分页未前进")
			}
			end = next
			params, err = params.WithEndTS(end)
			if err != nil {
				return nil, err
			}
		}
		if !completed {
			return nil, fmt.Errorf("私信会话超过单次分页上限，未推进检查点")
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Timestamp == result[j].Timestamp {
			return result[i].TalkerID < result[j].TalkerID
		}
		return result[i].Timestamp > result[j].Timestamp
	})
	return result, nil
}

func (c *Client) Messages(ctx context.Context, talkerID, after int64) ([]PrivateMessage, error) {
	if talkerID <= 0 || after < 0 {
		return nil, fmt.Errorf("私信对象或消息游标无效")
	}
	params, err := message.NewSessionMessagesParams(uint64(talkerID))
	if err != nil {
		return nil, err
	}
	if after > 0 {
		params, err = params.WithBeginSeqno(uint64(after))
		if err != nil {
			return nil, err
		}
	}
	api := message.NewClient(c.sdk)
	var result []PrivateMessage
	var end uint64
	seen := map[int64]bool{}
	for page := 0; page < 50; page++ {
		body, err := api.SessionMessages(ctx, params)
		if err != nil {
			return nil, err
		}
		var next uint64
		boundary := false
		for _, item := range body.Messages {
			msg, err := fromSDKMessage(item)
			if err != nil {
				return nil, err
			}
			if msg.Seq <= after {
				boundary = true
				continue
			}
			if !seen[msg.Seq] {
				result = append(result, msg)
				seen[msg.Seq] = true
			}
			if next == 0 || item.MsgSeqno < next {
				next = item.MsgSeqno
			}
		}
		if body.HasMore == 0 || boundary {
			sort.Slice(result, func(i, j int) bool { return result[i].Seq < result[j].Seq })
			return result, nil
		}
		if next == 0 || (end > 0 && next >= end) {
			return nil, fmt.Errorf("私信消息分页未前进")
		}
		end = next
		params, err = params.WithEndSeqno(end)
		if err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("私信消息超过单次分页上限，未推进检查点")
}

func fromSDKMessage(item message.PrivateMessage) (PrivateMessage, error) {
	if item.SenderUID > math.MaxInt64 || item.ReceiverID > math.MaxInt64 || item.MsgSeqno > math.MaxInt64 || item.Timestamp > math.MaxInt64 {
		return PrivateMessage{}, fmt.Errorf("私信编号或时间超过本地存储范围")
	}
	return PrivateMessage{SenderID: int64(item.SenderUID), ReceiverID: int64(item.ReceiverID), ReceiverType: int(item.ReceiverType), Type: int(item.MsgType), Content: item.Content, Seq: int64(item.MsgSeqno), Timestamp: int64(item.Timestamp), Status: int(item.MsgStatus), Source: int(item.MsgSource)}, nil
}

func sessionSeq(session Session) int64 {
	if session.LastMessage == nil {
		return 0
	}
	return session.LastMessage.Seq
}

func (c *Client) SendText(ctx context.Context, receiverID int64, text string) error {
	if receiverID <= 0 || receiverID == c.AccountID() {
		return fmt.Errorf("回复目标 UID 无效")
	}
	params, err := message.NewTextSendParams(uint64(receiverID), text)
	if err != nil {
		return err
	}
	if _, complete := c.sdk.Account(); c.AccountID() <= 0 || !c.HasLogin() || !complete {
		return fmt.Errorf("发送私信需要 Cookie 中的 DedeUserID、SESSDATA、bili_jct 和 buvid3")
	}
	result, err := message.NewClient(c.sdk).Send(ctx, params)
	if err != nil {
		return err
	}
	if result.MsgKey == nil || *result.MsgKey == 0 {
		return fmt.Errorf("B站未返回有效消息编号，发送结果不确定")
	}
	return nil
}

func (m PrivateMessage) Text() string {
	var content struct {
		Content string `json:"content"`
		URL     string `json:"url"`
		Title   string `json:"title"`
	}
	if json.Unmarshal([]byte(m.Content), &content) != nil {
		return "[无法解析的私信]"
	}
	if m.Type == 1 {
		return content.Content
	}
	if m.Type == 2 {
		return "[图片] " + content.URL
	}
	if content.Title != "" {
		return fmt.Sprintf("[消息类型 %d] %s", m.Type, content.Title)
	}
	return fmt.Sprintf("[消息类型 %d，请在 B站查看]", m.Type)
}
