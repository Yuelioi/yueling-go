package bot

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPrivateCommandsAreIsolatedAndRespectConditions(t *testing.T) {
	withCommandSyntax(t, "", false)
	b := New()
	var hits []string
	b.OnCommand("B站回复").Handle(func(*CommandContext) error { t.Fatal("group command received private event"); return nil })
	b.OnPrivateCommand("B站").Handle(func(*CommandContext) error { t.Fatal("shorter private command received event"); return nil })
	b.OnPrivateCommand("B站回复").Where(CondFn(func(_ *BotAPI, msg *MsgCtx) bool { return msg.UserID() == 99 })).Handle(func(ctx *CommandContext) error {
		if ctx.GroupID() != 0 || ctx.Cmd != "B站回复" || len(ctx.Args) != 2 {
			t.Fatalf("private context=%+v", ctx)
		}
		hits = append(hits, ctx.Args[1])
		return nil
	})
	for _, uid := range []int64{98, 99} {
		raw, err := json.Marshal(GroupMessageEvent{PostType: "message", MessageType: "private", SelfID: 10, UserID: uid, GroupID: 123, Message: Msg().Text("B站回复20 你好").Build()})
		if err != nil {
			t.Fatal(err)
		}
		b.dispatch(nil, raw)
	}
	if len(hits) != 1 || hits[0] != "你好" {
		t.Fatalf("private dispatch hits=%v", hits)
	}
}

func TestPrivateContextReplyUsesPrivateDestination(t *testing.T) {
	send := make(chan []byte, 1)
	api := &BotAPI{sendCh: send, done: make(chan struct{})}
	ctx := &GroupContext{BotAPI: api, MsgCtx: &MsgCtx{Event: &GroupMessageEvent{MessageType: "private", UserID: 99, MessageID: 12}}}
	finished := make(chan error, 1)
	go func() { finished <- ctx.Reply("回复内容") }()
	var payload struct {
		Action string `json:"action"`
		Params struct {
			UserID  int64   `json:"user_id"`
			GroupID int64   `json:"group_id"`
			Message Message `json:"message"`
		} `json:"params"`
		Echo string `json:"echo"`
	}
	select {
	case raw := <-send:
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("private reply was not queued")
	}
	if payload.Action != "send_private_msg" || payload.Params.UserID != 99 || payload.Params.GroupID != 0 || payload.Params.Message.Text() != "回复内容" {
		t.Fatalf("private reply payload=%+v", payload)
	}
	api.deliver(payload.Echo, json.RawMessage(`{"message_id":42}`))
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}
