package tools

import (
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
)

func TestFormatReminderDelay(t *testing.T) {
	tests := []struct {
		delay time.Duration
		want  string
	}{
		{30 * time.Minute, "30分钟"},
		{2 * time.Hour, "2小时"},
	}
	for _, test := range tests {
		if got := formatReminderDelay(test.delay); got != test.want {
			t.Fatalf("formatReminderDelay(%s) = %q, want %q", test.delay, got, test.want)
		}
	}
}

func TestReminderToolOwnsTimedReminderRoutes(t *testing.T) {
	tool, ok := ai.GetTool("manage_reminder")
	if !ok {
		t.Fatal("manage_reminder tool is not registered")
	}
	for _, text := range []string{"提醒我30分钟后关火", "每天09:30叫我喝水", "我的提醒"} {
		routed := ai.Route(text, []*ai.ToolMeta{tool})
		if len(routed) != 1 {
			t.Fatalf("Route(%q) = %#v", text, routed)
		}
	}
}

func TestReminderDelayRejectsOverflowBeforeConversion(t *testing.T) {
	ctx := &ai.ToolContext{Params: map[string]any{
		"action":  "add_after",
		"content": "测试",
		"amount":  float64(1 << 62),
		"unit":    "hour",
	}}
	result, err := manageReminderHandler(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "一次性提醒最长可设置一年后" {
		t.Fatalf("overflow result = %q", result)
	}
}
