package ai

import (
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
)

func TestDigestHistoryTextCapsInputAndKeepsUsefulSegments(t *testing.T) {
	message := bot.HistoryMessage{UserID: 42, Sender: bot.Sender{Nickname: "alice"}}
	message.Message = append(message.Message, struct {
		Type string `json:"type"`
		Data struct {
			Text string `json:"text"`
			QQ   string `json:"qq"`
		} `json:"data"`
	}{Type: "text", Data: struct {
		Text string `json:"text"`
		QQ   string `json:"qq"`
	}{Text: strings.Repeat("很长的聊天内容", 100)}})

	got := digestHistoryText([]bot.HistoryMessage{message})
	if !strings.HasPrefix(got, "alice: ") || !strings.HasSuffix(got, "…") {
		t.Fatalf("digest text = %q", got)
	}
	if len([]rune(got)) > maxDigestLineRunes+1 {
		t.Fatalf("digest line runes = %d, want capped", len([]rune(got)))
	}
}
