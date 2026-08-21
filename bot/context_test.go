package bot

import (
	"encoding/json"
	"testing"
)

func TestGroupContextTextWithReplyContext(t *testing.T) {
	sendCh := make(chan []byte, 1)
	done := make(chan struct{})
	api := &BotAPI{sendCh: sendCh, done: done}

	go func() {
		payload := <-sendCh
		var request struct {
			Action string `json:"action"`
			Params struct {
				MessageID int32 `json:"message_id"`
			} `json:"params"`
			Echo string `json:"echo"`
		}
		if err := json.Unmarshal(payload, &request); err != nil {
			t.Errorf("decode get_msg request: %v", err)
			return
		}
		if request.Action != "get_msg" || request.Params.MessageID != 5 {
			t.Errorf("request = %+v, want get_msg for message 5", request)
			return
		}
		api.deliver(request.Echo, json.RawMessage(`{
			"message": [
				{"type":"text","data":{"text":"We've investigated the Codex usage limits."}},
				{"type":"image","data":{"file":"screenshot.jpg"}}
			]
		}`))
	}()

	ctx := &GroupContext{
		BotAPI: api,
		MsgCtx: &MsgCtx{Event: &GroupMessageEvent{
			Message: Msg().Reply(5).At(42).Text("翻译").Build(),
		}},
	}

	got := ctx.TextWithReplyContext()
	want := "【引用消息】\nWe've investigated the Codex usage limits.[图片]\n\n【当前消息】\n翻译"
	if got != want {
		t.Fatalf("TextWithReplyContext() = %q, want %q", got, want)
	}
}

func TestGroupContextTextWithReplyContextFallsBackToCurrentText(t *testing.T) {
	tests := []struct {
		name    string
		message Message
	}{
		{name: "no reply", message: Msg().Text("总结一下").Build()},
		{name: "reply cannot be resolved", message: Msg().Reply(99).Text("总结一下").Build()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &GroupContext{MsgCtx: &MsgCtx{Event: &GroupMessageEvent{Message: tt.message}}}
			if got := ctx.TextWithReplyContext(); got != "总结一下" {
				t.Fatalf("TextWithReplyContext() = %q, want current text", got)
			}
		})
	}
}
