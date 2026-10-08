package chatsummary

import (
	"context"
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/services/chatinsights"
)

type groupReader struct {
	api       *bot.BotAPI
	groupID   int64
	messageID int32
}

func NewGroupReader(api *bot.BotAPI, groupID int64, messageID int32) Reader {
	return &groupReader{api: api, groupID: groupID, messageID: messageID}
}

func (r *groupReader) Read(ctx context.Context, q Query) (Source, error) {
	if r.groupID == 0 {
		return Source{}, ErrUnavailable
	}
	if q.Period == "recent" {
		if r.api == nil {
			return Source{}, ErrUnavailable
		}
		messages, err := r.api.WithContext(ctx).GetGroupMsgHistory(r.groupID, r.messageID, q.Count)
		if err != nil {
			if ctx.Err() != nil {
				return Source{}, ctx.Err()
			}
			return Source{}, ErrUnavailable
		}
		return Source{Scope: fmt.Sprintf("最近最多 %d 条群聊记录的取样", q.Count), Records: historyRecords(messages, r.messageID)}, nil
	}
	window, ok := chatinsights.ResolvePeriod(q.Period, bot.Now())
	if !ok || db.DB == nil {
		return Source{}, ErrUnavailable
	}
	rows, err := db.GetGroupChatMessagesContext(ctx, r.groupID, 0, window.Start, window.End, q.Count+1)
	if err != nil {
		if ctx.Err() != nil {
			return Source{}, ctx.Err()
		}
		return Source{}, ErrUnavailable
	}
	source := Source{Scope: fmt.Sprintf("%s本地已保存记录按时间顺序的前 %d 条取样，可能不涵盖整个时间段", window.Label, q.Count)}
	for _, row := range rows {
		if row.MessageID == r.messageID || strings.TrimSpace(row.Content) == "" {
			continue
		}
		source.Records = append(source.Records, Record{MessageID: row.MessageID, UserID: row.UserID, Name: row.Nickname, Text: row.Content})
	}
	return source, nil
}

func historyRecords(messages []bot.HistoryMessage, currentMessageID int32) []Record {
	var records []Record
	for _, msg := range messages {
		if msg.MessageID == currentMessageID {
			continue
		}
		name := msg.Sender.Card
		if name == "" {
			name = msg.Sender.Nickname
		}
		if name == "" {
			name = fmt.Sprint(msg.UserID)
		}
		var parts []string
		for _, seg := range msg.Message {
			if seg.Type == "text" && strings.TrimSpace(seg.Data.Text) != "" {
				parts = append(parts, seg.Data.Text)
			}
		}
		if len(parts) > 0 {
			records = append(records, Record{MessageID: msg.MessageID, UserID: msg.UserID, Name: name, Text: strings.Join(parts, " ")})
		}
	}
	return records
}
