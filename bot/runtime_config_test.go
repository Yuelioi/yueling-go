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
