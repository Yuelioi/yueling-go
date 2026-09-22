package tools

import (
	"testing"

	"github.com/Yuelioi/yueling-go/ai"
)

func TestSummaryRoutesGroupTasksOnly(t *testing.T) {
	tool, ok := ai.GetTool("summarize_chat")
	if !ok {
		t.Fatal("missing tool")
	}
	for _, text := range []string{"总结群聊", "总结昨天的讨论", "提取群聊待办", "整理讨论结论", "群里在聊什么", "列出未解决问题"} {
		if len(ai.Route(text, []*ai.ToolMeta{tool})) != 1 {
			t.Errorf("not routed: %s", text)
		}
	}
	if len(ai.Route("帮我总结这段文章", []*ai.ToolMeta{tool})) != 0 {
		t.Fatal("unrelated supplied text routed to group history")
	}
}
