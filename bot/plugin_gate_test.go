package bot

import (
	"encoding/json"
	"errors"
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

func testAPI() *BotAPI {
	done := make(chan struct{})
	close(done)
	return &BotAPI{done: done}
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

	b.dispatchGroupMessage(testAPI(), testEvent("hello"))
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

	b.dispatchGroupMessage(testAPI(), testEvent("hello"))
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

	b.dispatchGroupMessage(testAPI(), testEvent("hello"))
	if !called {
		t.Fatalf("untagged handler should not be gated")
	}
}

func TestPluginGateFailOpenOnError(t *testing.T) {
	b := New()
	b.SetPluginGate(func(groupID int64, pluginID int) (bool, error) {
		return false, errors.New("gate unavailable")
	})

	called := false
	b.OnFullMatch("hello").Plugin(29).Handle(func(ctx *GroupContext) error {
		called = true
		return nil
	})

	b.dispatchGroupMessage(testAPI(), testEvent("hello"))
	if !called {
		t.Fatalf("handler should run when plugin gate errors")
	}
}

func TestPluginGateMarksCommandMatchedBeforeSkippingDisabledHandler(t *testing.T) {
	b := New()
	b.SetPluginGate(func(groupID int64, pluginID int) (bool, error) {
		return true, nil
	})

	b.OnFullMatch("hello").Priority(20).Plugin(29).Handle(func(ctx *GroupContext) error {
		t.Fatalf("disabled command-like handler was called")
		return nil
	})

	var sawCommandMatched bool
	b.OnGroupMessage().Priority(0).Handle(func(ctx *GroupContext) error {
		sawCommandMatched = ctx.CommandMatched()
		return nil
	})

	b.dispatchGroupMessage(testAPI(), testEvent("hello"))
	if !sawCommandMatched {
		t.Fatalf("passive handler did not see commandMatched from disabled command-like handler")
	}
}
