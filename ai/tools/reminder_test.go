package tools

import (
	"testing"

	"github.com/Yuelioi/yueling-go/ai"
)

func TestParseReminderTimeRequiresRFC3339Timezone(t *testing.T) {
	if _, err := parseReminderTime("明天上午九点"); err == nil {
		t.Fatal("parseReminderTime should reject non-RFC3339 values passed by the model")
	}
	got, err := parseReminderTime("2026-08-14T09:00:00+08:00")
	if err != nil || got.Hour() != 9 {
		t.Fatalf("parseReminderTime() = %v, %v", got, err)
	}
}

func TestReminderToolOwnsNaturalReminderRoutes(t *testing.T) {
	tool, ok := ai.GetTool("manage_reminder")
	if !ok {
		t.Fatal("manage_reminder tool is not registered")
	}
	for _, text := range []string{
		"提醒我30分钟后关火",
		"明天早上叫我喝水",
		"工作日提醒我签到",
		"我的提醒",
		"推迟提醒到下午三点",
	} {
		if routed := ai.Route(text, []*ai.ToolMeta{tool}); len(routed) != 1 {
			t.Fatalf("Route(%q) = %#v", text, routed)
		}
	}
}
