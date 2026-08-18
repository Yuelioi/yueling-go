package bot

import (
	"reflect"
	"testing"
)

func withCommandSyntax(t *testing.T, prefix string, requireArgSpace bool) {
	t.Helper()
	oldPrefix := CmdPrefix
	oldRequireArgSpace := CommandArgSpaceRequired
	CmdPrefix = prefix
	CommandArgSpaceRequired = requireArgSpace
	t.Cleanup(func() {
		CmdPrefix = oldPrefix
		CommandArgSpaceRequired = oldRequireArgSpace
	})
}

func TestCommandMatcherOptionalArgumentSpace(t *testing.T) {
	withCommandSyntax(t, "", false)

	mr := Command("语录").Match(&MsgCtx{Event: testEvent("语录张三")})
	if !mr.Matched || mr.Cmd != "语录" || !reflect.DeepEqual(mr.Args, []string{"张三"}) {
		t.Fatalf("match = %+v, want attached Chinese argument", mr)
	}
}

func TestCommandMatcherRequiredArgumentSpace(t *testing.T) {
	withCommandSyntax(t, "", true)

	if mr := Command("语录").Match(&MsgCtx{Event: testEvent("语录张三")}); mr.Matched {
		t.Fatalf("attached argument unexpectedly matched: %+v", mr)
	}
	mr := Command("语录").Match(&MsgCtx{Event: testEvent("语录 张三")})
	if !mr.Matched || !reflect.DeepEqual(mr.Args, []string{"张三"}) {
		t.Fatalf("spaced argument match = %+v", mr)
	}
}

func TestCommandPrefixIsRequiredWhenConfigured(t *testing.T) {
	withCommandSyntax(t, "/", false)

	if mr := Command("语录").Match(&MsgCtx{Event: testEvent("语录张三")}); mr.Matched {
		t.Fatalf("bare command unexpectedly matched with configured prefix: %+v", mr)
	}
	mr := Command("语录").Match(&MsgCtx{Event: testEvent("/语录张三")})
	if !mr.Matched || !reflect.DeepEqual(mr.Args, []string{"张三"}) {
		t.Fatalf("prefixed command match = %+v", mr)
	}
}

func TestDispatchPrefersLongestAttachedCommand(t *testing.T) {
	withCommandSyntax(t, "", false)

	b := New()
	var called []string
	b.OnCommand("积分").Handle(func(*CommandContext) error {
		called = append(called, "积分")
		return nil
	})
	b.OnCommand("积分排行").Handle(func(*CommandContext) error {
		called = append(called, "积分排行")
		return nil
	})

	b.dispatchGroupMessage(testAPI(), testEvent("积分排行"))
	if !reflect.DeepEqual(called, []string{"积分排行"}) {
		t.Fatalf("called = %v, want only longest command", called)
	}
}
