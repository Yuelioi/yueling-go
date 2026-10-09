package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
	openai "github.com/sashabaranov/go-openai"
)

func TestStructuredToolOutcomesPreservedAndDeduplicated(t *testing.T) {
	for _, status := range []ToolStatus{ToolSucceeded, ToolEmpty, ToolRejected, ToolFailed} {
		t.Run(string(status), func(t *testing.T) {
			old := testEngine.Registry
			t.Cleanup(func() { testEngine.Registry = old })
			testEngine.Registry = &Registry{tools: map[string]*ToolMeta{}}
			calls := 0
			testEngine.Registry.Register(ToolMeta{Name: "outcome", ResultHandler: func(*ToolContext) ToolResult {
				calls++
				return ToolResult{Status: status, Content: "业务结果", PossibleSideEffects: status == ToolSucceeded}
			}})
			session := newSession(2, 1)
			for i := 0; i < 2; i++ {
				result := testEngine.executeTool(context.Background(), nil, &bot.GroupMessageEvent{GroupID: 1, UserID: 2}, session, PermMember, openai.ToolCall{Function: openai.FunctionCall{Name: "outcome", Arguments: `{}`}}, nil)
				if result.Status != status || !result.Invoked || result.Content != "业务结果" || result.PossibleSideEffects != (status == ToolSucceeded) {
					t.Fatalf("result=%+v", result)
				}
			}
			if calls != 1 {
				t.Fatalf("executed %d times", calls)
			}
		})
	}
}

func TestStructuredToolInvalidOutcomeAndPanicAreUncertain(t *testing.T) {
	for _, panics := range []bool{false, true} {
		old := testEngine.Registry
		testEngine.Registry = &Registry{tools: map[string]*ToolMeta{}}
		calls := 0
		testEngine.Registry.Register(ToolMeta{Name: "broken", ResultHandler: func(*ToolContext) ToolResult {
			calls++
			if panics {
				panic("private diagnostic")
			}
			return ToolResult{Content: "private diagnostic"}
		}})
		session := newSession(2, 1)
		for i := 0; i < 2; i++ {
			result := testEngine.executeTool(context.Background(), nil, &bot.GroupMessageEvent{GroupID: 1, UserID: 2}, session, PermMember, openai.ToolCall{Function: openai.FunctionCall{Name: "broken", Arguments: `{}`}}, nil)
			if result.Status != ToolFailed || !result.Invoked || !result.PossibleSideEffects || strings.Contains(result.Content, "private") {
				t.Errorf("result=%+v", result)
			}
		}
		testEngine.Registry = old
		if calls != 1 {
			t.Fatalf("executed %d times", calls)
		}
	}
}

func TestConfirmedStructuredToolUsesBusinessOutcome(t *testing.T) {
	oldRegistry, oldConfirms, oldSessions := testEngine.Registry, testEngine.Confirms, testEngine.Sessions
	t.Cleanup(func() {
		testEngine.Registry, testEngine.Confirms, testEngine.Sessions = oldRegistry, oldConfirms, oldSessions
	})
	testEngine.Registry = &Registry{tools: map[string]*ToolMeta{}}
	testEngine.Confirms = &ConfirmManager{pending: map[string]*PendingAction{}}
	testEngine.Sessions = &SessionManager{}
	testEngine.Registry.Register(ToolMeta{Name: "confirm_outcome", ConfirmRequired: true, ResultHandler: func(*ToolContext) ToolResult {
		return ToolResult{Status: ToolRejected, Content: "目标已失效，未执行操作"}
	}})
	actionID, code := testEngine.Confirms.Store(42, 100, "confirm_outcome", nil)
	event := &bot.GroupMessageEvent{GroupID: 100, UserID: 42, Message: bot.Msg().Text("确认 " + code + " " + actionID).Build()}
	reply, handled, err := captureConfirmation(context.Background(), &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: event}})
	if err != nil || !handled || reply != "目标已失效，未执行操作" {
		t.Fatalf("reply=%q handled=%v err=%v", reply, handled, err)
	}
}

func TestResetInvalidatesPendingAction(t *testing.T) {
	oldRegistry, oldConfirms, oldSessions := testEngine.Registry, testEngine.Confirms, testEngine.Sessions
	t.Cleanup(func() {
		testEngine.Registry, testEngine.Confirms, testEngine.Sessions = oldRegistry, oldConfirms, oldSessions
	})
	testEngine.Registry = &Registry{tools: map[string]*ToolMeta{}}
	testEngine.Confirms = &ConfirmManager{pending: map[string]*PendingAction{}}
	testEngine.Sessions = &SessionManager{}
	testEngine.Registry.Register(ToolMeta{Name: "confirmed", ConfirmRequired: true, Handler: func(*ToolContext) (string, error) { t.Fatal("executed canceled action"); return "", nil }})
	s := testEngine.Sessions.Get(1, 2)
	result := testEngine.executeTool(context.Background(), nil, &bot.GroupMessageEvent{GroupID: 1, UserID: 2}, s, PermMember, openai.ToolCall{Function: openai.FunctionCall{Name: "confirmed", Arguments: `{}`}}, nil)
	if result.Status != ToolConfirmation {
		t.Fatalf("result=%+v", result)
	}
	testEngine.Sessions.Delete(1, 2)
	for id, pending := range testEngine.Confirms.pending {
		if _, ok := testEngine.Confirms.Verify(id, pending.Code, 2, 1); ok {
			t.Fatal("reset left a usable confirmation")
		}
	}
}
