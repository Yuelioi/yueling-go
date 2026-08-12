package ai

import (
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

func TestResetTurnClearsTransientToolState(t *testing.T) {
	session := newSession(1, 2)
	session.ToolState["history"] = "stale"
	session.UsedTools["tool"] = 1
	session.StepCount = 2

	session.resetTurn()

	if len(session.ToolState) != 0 {
		t.Fatalf("ToolState = %#v, want empty", session.ToolState)
	}
	if len(session.UsedTools) != 0 || session.StepCount != 0 {
		t.Fatalf("per-turn counters not reset: tools=%v steps=%d", session.UsedTools, session.StepCount)
	}
}

func TestLocalConversationReset(t *testing.T) {
	previousSessions := Sessions
	previousName := config.C.Bot.Name
	t.Cleanup(func() {
		Sessions = previousSessions
		config.C.Bot.Name = previousName
	})
	Sessions = &SessionManager{sessions: map[string]*Session{}}
	config.C.Bot.Name = "月灵"
	Sessions.Get(100, 200)

	reply, handled := handleLocalControl(100, 200, "月灵，新对话")
	if !handled || reply != "好的，我们从这里重新开始。" {
		t.Fatalf("handleLocalControl() = %q, %v", reply, handled)
	}
	if Sessions.Delete(100, 200) {
		t.Fatal("session still exists after reset")
	}
}

func TestLocalMemoryControls(t *testing.T) {
	cleanupAIConfigAndDB(t)
	initAffinityTestDB(t)
	previousSessions := Sessions
	t.Cleanup(func() { Sessions = previousSessions })
	Sessions = &SessionManager{sessions: map[string]*Session{}}

	if err := WriteSemantic(42, "用户喜欢无糖咖啡", "food"); err != nil {
		t.Fatal(err)
	}
	reply, handled := handleLocalControl(7, 42, "你记得我什么")
	if !handled || !strings.Contains(reply, "用户喜欢无糖咖啡") {
		t.Fatalf("list reply = %q, handled=%v", reply, handled)
	}

	reply, handled = handleLocalControl(7, 42, "忘记第 1 条记忆")
	if !handled || !strings.Contains(reply, "已忘记") {
		t.Fatalf("delete reply = %q, handled=%v", reply, handled)
	}
	items, err := ListSemanticMemoryRecords(42, 20)
	if err != nil || len(items) != 0 {
		t.Fatalf("memories after delete = %#v, err=%v", items, err)
	}

	if err := WriteSemantic(42, "用户喜欢夜跑", "hobby"); err != nil {
		t.Fatal(err)
	}
	reply, handled = handleLocalControl(7, 42, "清空我的长期记忆")
	if !handled || !strings.Contains(reply, "已清空 1 条") {
		t.Fatalf("clear reply = %q, handled=%v", reply, handled)
	}
}
