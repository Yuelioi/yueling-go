package tools

import (
	"encoding/json"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
)

func TestFormatChatHistoryIncludesMessageAndUserIDs(t *testing.T) {
	var messages []bot.HistoryMessage
	if err := json.Unmarshal([]byte(`[
		{
			"message_id": 101,
			"user_id": 202,
			"sender": {"nickname": "Alice"},
			"message": [{"type": "text", "data": {"text": "刚才的话"}}]
		}
	]`), &messages); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	got := formatChatHistory(messages)
	want := "[消息ID:101 用户ID:202] Alice: 刚才的话"
	if got != want {
		t.Fatalf("formatChatHistory() = %q, want %q", got, want)
	}
	refs := historyReferences(messages)
	if len(refs) != 1 || refs[0].messageID != 101 || refs[0].userID != 202 {
		t.Fatalf("historyReferences() = %+v, want message 101 user 202", refs)
	}
}
