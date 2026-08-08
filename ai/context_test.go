package ai

import (
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
)

func TestToolContextExtractsTrustedQQEventContext(t *testing.T) {
	event := &bot.GroupMessageEvent{
		SelfID: 42,
		Message: bot.Msg().
			At(42).
			At(100).
			At(100).
			At(200).
			Reply(300).
			Build(),
	}
	ctx := newToolCtx(nil, event, newSession(1, 2), PermAdmin, nil)

	mentions := ctx.MentionedUserIDs()
	if len(mentions) != 2 || mentions[0] != 100 || mentions[1] != 200 {
		t.Fatalf("MentionedUserIDs() = %v, want [100 200]", mentions)
	}
	if replyID, ok := ctx.ReplyMessageID(); !ok || replyID != 300 {
		t.Fatalf("ReplyMessageID() = (%d, %v), want (300, true)", replyID, ok)
	}
	if ctx.Permission() != PermAdmin {
		t.Fatalf("Permission() = %v, want PermAdmin", ctx.Permission())
	}
	if ctx.SelfID() != 42 {
		t.Fatalf("SelfID() = %d, want 42", ctx.SelfID())
	}
}

func TestToolContextRejectsInvalidReplyMessageID(t *testing.T) {
	event := &bot.GroupMessageEvent{
		Message: bot.Message{{Type: "reply", Data: []byte(`{"id":"not-a-number"}`)}},
	}
	ctx := newToolCtx(nil, event, newSession(1, 2), PermMember, nil)

	if id, ok := ctx.ReplyMessageID(); ok || id != 0 {
		t.Fatalf("ReplyMessageID() = (%d, %v), want (0, false)", id, ok)
	}
}
