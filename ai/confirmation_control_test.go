package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestHandleConfirmationExecutesPendingTool(t *testing.T) {
	oldRegistry := global
	oldConfirms := Confirms
	oldSessions := Sessions
	oldName := config.C.Bot.Name
	t.Cleanup(func() {
		global = oldRegistry
		Confirms = oldConfirms
		Sessions = oldSessions
		config.C.Bot.Name = oldName
	})
	global = &registry{tools: map[string]*ToolMeta{}}
	Confirms = &ConfirmManager{pending: map[string]*PendingAction{}}
	Sessions = &SessionManager{sessions: map[string]*Session{}}
	config.C.Bot.Name = "月灵"

	executed := false
	Register(ToolMeta{
		Name:            "confirmed_test",
		ConfirmRequired: true,
		Handler: func(ctx *ToolContext) (string, error) {
			executed = ctx.String("target") == "ok"
			return "操作已执行", nil
		},
	})
	actionID, code := Confirms.Store(42, 100, "confirmed_test", map[string]any{"target": "ok"})
	event := &bot.GroupMessageEvent{
		SelfID: 1, GroupID: 100, UserID: 42,
		Message: bot.Msg().Text("月灵 确认 " + code + " " + actionID).Build(),
		Sender:  bot.Sender{Role: "member"},
	}
	reply, handled := handleConfirmation(&bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: event}})
	if !handled || reply != "操作已执行" || !executed {
		t.Fatalf("reply=%q handled=%v executed=%v", reply, handled, executed)
	}
	if session := Sessions.Get(100, 42); len(session.Messages) != 2 || session.Messages[1].Content != "操作已执行" {
		t.Fatalf("confirmed result was not recorded in session: %#v", session.Messages)
	}
	if _, ok := Confirms.Verify(actionID, code, 42, 100); ok {
		t.Fatal("confirmed action could be reused")
	}
}

func TestNewConfirmationReplacesPreviousMatchingAction(t *testing.T) {
	manager := &ConfirmManager{pending: map[string]*PendingAction{}}
	oldID, oldCode := manager.Store(42, 100, "ban", map[string]any{"user_id": "1"})
	newID, newCode := manager.Store(42, 100, "ban", map[string]any{"user_id": "2"})
	if _, ok := manager.Verify(oldID, oldCode, 42, 100); ok {
		t.Fatal("superseded confirmation remained valid")
	}
	if _, ok := manager.Verify(newID, newCode, 42, 100); !ok {
		t.Fatal("latest confirmation is not valid")
	}
}

func TestExecuteToolCreatesDescriptiveConfirmation(t *testing.T) {
	oldRegistry := global
	oldConfirms := Confirms
	oldName := config.C.Bot.Name
	t.Cleanup(func() {
		global = oldRegistry
		Confirms = oldConfirms
		config.C.Bot.Name = oldName
	})
	global = &registry{tools: map[string]*ToolMeta{}}
	Confirms = &ConfirmManager{pending: map[string]*PendingAction{}}
	config.C.Bot.Name = "月灵"
	Register(ToolMeta{
		Name:            "dangerous_test",
		Description:     "测试危险操作",
		ConfirmRequired: true,
	})

	session := newSession(42, 100)
	event := &bot.GroupMessageEvent{GroupID: 100, UserID: 42}
	reply := executeTool(context.Background(), nil, event, session, PermMember, openai.ToolCall{
		Function: openai.FunctionCall{Name: "dangerous_test", Arguments: `{"target":"ok"}`},
	}, nil)
	if !strings.Contains(reply, "测试危险操作") || !strings.Contains(reply, "月灵 确认") {
		t.Fatalf("confirmation reply = %q", reply)
	}
	if session.canCall("dangerous_test") {
		t.Fatal("high-risk tool could be requested repeatedly in one turn")
	}
}

func TestConfirmationIsBoundToGroup(t *testing.T) {
	manager := &ConfirmManager{pending: map[string]*PendingAction{}}
	actionID, code := manager.Store(42, 100, "test", nil)
	if _, ok := manager.Verify(actionID, code, 42, 200); ok {
		t.Fatal("confirmation succeeded in another group")
	}
	if _, ok := manager.Verify(actionID, code, 42, 100); !ok {
		t.Fatal("valid confirmation was consumed by wrong-group attempt")
	}
}

func TestConfirmationPreservesOriginalMessageContext(t *testing.T) {
	oldRegistry, oldConfirms, oldSessions := global, Confirms, Sessions
	t.Cleanup(func() { global, Confirms, Sessions = oldRegistry, oldConfirms, oldSessions })
	global = &registry{tools: map[string]*ToolMeta{}}
	Confirms = &ConfirmManager{pending: map[string]*PendingAction{}}
	Sessions = &SessionManager{sessions: map[string]*Session{}}

	var mentioned []int64
	Register(ToolMeta{Name: "context_confirmation", Handler: func(ctx *ToolContext) (string, error) {
		mentioned = ctx.MentionedUserIDs()
		value, _ := ctx.GetState("history")
		return value.(string), nil
	}})
	original := &bot.GroupMessageEvent{
		SelfID: 1, GroupID: 100, UserID: 42,
		Message: bot.Msg().At(123).Text("禁言").Build(),
	}
	actionID, code := Confirms.StoreWithContext(42, 100, "context_confirmation", map[string]any{}, original, map[string]any{"history": "kept"})
	confirmation := &bot.GroupMessageEvent{
		SelfID: 1, GroupID: 100, UserID: 42,
		Message: bot.Msg().Text("月灵 确认 " + code + " " + actionID).Build(),
		Sender:  bot.Sender{Role: "member"},
	}
	reply, handled := handleConfirmation(&bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: confirmation}})
	if !handled || reply != "kept" || len(mentioned) != 1 || mentioned[0] != 123 {
		t.Fatalf("reply=%q handled=%v mentioned=%v", reply, handled, mentioned)
	}
}
