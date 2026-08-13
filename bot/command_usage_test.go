package bot

import (
	"sync/atomic"
	"testing"
	"time"
)

type recordedCommandUsage struct {
	groupID  int64
	userID   int64
	pluginID int
	command  string
}

func TestDispatchRecordsAcceptedCommandUsage(t *testing.T) {
	oldPrefix := CmdPrefix
	CmdPrefix = "/"
	t.Cleanup(func() { CmdPrefix = oldPrefix })

	recorded := make(chan recordedCommandUsage, 1)
	b := New()
	b.SetCommandUsageRecorder(func(groupID, userID int64, pluginID int, command string) error {
		recorded <- recordedCommandUsage{groupID, userID, pluginID, command}
		return nil
	})
	b.OnCommand("ping", "p").Plugin(42).Handle(func(*CommandContext) error { return nil })

	b.dispatchGroupMessage(testAPI(), testEvent("/p hello"))
	select {
	case got := <-recorded:
		if got.groupID != 100 || got.userID != 1 || got.pluginID != 42 || got.command != "p" {
			t.Fatalf("record=%+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("command usage was not recorded")
	}
}

func TestFullMatchReportsMatchedCommand(t *testing.T) {
	mr := FullMatch("签到", "打卡").Match(&MsgCtx{Event: testEvent("打卡")})
	if !mr.Matched || mr.Cmd != "打卡" {
		t.Fatalf("match=%+v", mr)
	}
}

func TestDispatchDoesNotRecordPassiveOrDisabledMatchers(t *testing.T) {
	var calls atomic.Int32
	recorder := func(groupID, userID int64, pluginID int, command string) error {
		calls.Add(1)
		return nil
	}

	passive := New()
	passive.SetCommandUsageRecorder(recorder)
	passive.OnKeyword("hello").Plugin(28).Handle(func(*GroupContext) error { return nil })
	passive.dispatchGroupMessage(testAPI(), testEvent("hello"))

	disabled := New()
	disabled.SetCommandUsageRecorder(recorder)
	disabled.SetPluginGate(func(groupID int64, pluginID int) (bool, error) { return true, nil })
	disabled.OnFullMatch("hello").Plugin(29).Handle(func(*GroupContext) error { return nil })
	disabled.dispatchGroupMessage(testAPI(), testEvent("hello"))

	if got := calls.Load(); got != 0 {
		t.Fatalf("unexpected records=%d", got)
	}
}
