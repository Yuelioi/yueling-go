package bot

import (
	"github.com/Yuelioi/yueling-go/config"
	"testing"
)

func TestCommandSettingsApplyToExistingMatcher(t *testing.T) {
	old := liveCommandOptions.Load()
	defer liveCommandOptions.Store(old)
	prefix := "!"
	spaces := true
	SetCommandOptionsProvider(func() (string, bool) { return prefix, spaces })
	matcher := Command("hello")
	ctx := &MsgCtx{Event: testEvent("!hello world")}
	if !matcher.Match(ctx).Matched {
		t.Fatal("initial command failed")
	}
	prefix = "/"
	spaces = false
	ctx.Event = testEvent("/helloworld")
	if !matcher.Match(ctx).Matched {
		t.Fatal("updated settings did not reach existing matcher")
	}
	ctx.Event = testEvent("!hello world")
	if matcher.Match(ctx).Matched {
		t.Fatal("old prefix still accepted")
	}
}

func TestGlobalPluginGatePrivateAndGroupLive(t *testing.T) {
	old := config.Current()
	defer config.Publish(old)
	b := New()
	calls := 0
	b.SetPluginGate(func(groupID int64, id int) (bool, error) {
		if groupID == 0 {
			t.Error("private message called group gate")
		}
		return false, nil
	})
	next := old
	next.Plugins.Disabled = []int{29}
	config.Publish(next)
	for _, group := range []int64{0, 100} {
		if !b.pluginDisabled(group, 29) {
			t.Fatal("global gate ignored")
		}
	}
	b.OnFullMatch("hello").Plugin(29).Handle(func(*GroupContext) error { calls++; return nil })
	b.dispatchGroupMessage(testAPI(), testEvent("hello"))
	if calls != 0 {
		t.Fatal("disabled handler called")
	}
	next.Plugins.Disabled = nil
	config.Publish(next)
	if b.pluginDisabled(0, 29) {
		t.Fatal("private stayed disabled")
	}
	b.dispatchGroupMessage(testAPI(), testEvent("hello"))
	if calls != 1 {
		t.Fatal("enable did not take effect")
	}
}

func TestBlacklistStopsAllMessageHandlersAndPokesLive(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	b := New()
	messages, pokes, private := 0, 0, 0
	b.OnPrivateCommand("hello").Handle(func(*CommandContext) error { private++; return nil })
	b.OnGroupMessage().Handle(func(*GroupContext) error { messages++; return nil })
	b.OnNotice("notify").Handle(func(*NoticeContext) error { pokes++; return nil })
	event := testEvent("月灵 你好")
	poke := &NoticeEvent{NoticeType: "notify", SubType: "poke", UserID: event.UserID}
	next := old.Clone()
	next.Bot.Blacklist = []int64{event.UserID}
	config.Publish(next)
	b.dispatchMessage(nil, &GroupMessageEvent{UserID: event.UserID, SelfID: event.SelfID, Message: Msg().Text("hello").Build()}, "private_message")
	b.dispatchGroupMessage(nil, event)
	b.dispatchNotice(nil, poke)
	if messages != 0 || pokes != 0 || private != 0 {
		t.Fatalf("blocked account reached handlers: messages=%d pokes=%d", messages, pokes)
	}
	next.Bot.Blacklist = nil
	config.Publish(next)
	b.dispatchMessage(nil, &GroupMessageEvent{UserID: event.UserID, SelfID: event.SelfID, Message: Msg().Text("hello").Build()}, "private_message")
	b.dispatchGroupMessage(nil, event)
	b.dispatchNotice(nil, poke)
	if messages != 1 || pokes != 1 || private != 1 {
		t.Fatalf("unblock did not apply immediately: messages=%d pokes=%d", messages, pokes)
	}
}
