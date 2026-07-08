package bot

import (
	"encoding/json"
	"testing"
)

func textMessage(text string) Message {
	return Message{{Type: "text", Data: json.RawMessage(`{"text":` + strconvQuote(text) + `}`)}}
}

func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func testEvent(text string) *GroupMessageEvent {
	return &GroupMessageEvent{
		SelfID:  999,
		GroupID: 100,
		UserID:  1,
		Message: textMessage(text),
		Sender:  Sender{Nickname: "alice", Role: "member"},
	}
}

func TestPluginGateSkipsDisabledHandlerSilently(t *testing.T) {
	b := New()
	b.SetPluginGate(func(groupID int64, pluginID int) (bool, error) {
		return groupID == 100 && pluginID == 29, nil
	})

	called := false
	b.OnFullMatch("hello").Plugin(29).Handle(func(ctx *GroupContext) error {
		called = true
		return nil
	})

	b.dispatchGroupMessage(&BotAPI{}, testEvent("hello"))
	if called {
		t.Fatalf("disabled plugin handler was called")
	}
}

func TestPluginGateAllowsEnabledHandler(t *testing.T) {
	b := New()
	b.SetPluginGate(func(groupID int64, pluginID int) (bool, error) {
		return false, nil
	})

	called := false
	b.OnFullMatch("hello").Plugin(29).Handle(func(ctx *GroupContext) error {
		called = true
		return nil
	})

	b.dispatchGroupMessage(&BotAPI{}, testEvent("hello"))
	if !called {
		t.Fatalf("enabled plugin handler was not called")
	}
}

func TestPluginGateIgnoresUntaggedHandlers(t *testing.T) {
	b := New()
	b.SetPluginGate(func(groupID int64, pluginID int) (bool, error) {
		return true, nil
	})

	called := false
	b.OnFullMatch("hello").Handle(func(ctx *GroupContext) error {
		called = true
		return nil
	})

	b.dispatchGroupMessage(&BotAPI{}, testEvent("hello"))
	if !called {
		t.Fatalf("untagged handler should not be gated")
	}
}
