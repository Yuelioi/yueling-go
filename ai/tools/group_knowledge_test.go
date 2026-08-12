package tools

import (
	"testing"

	"github.com/Yuelioi/yueling-go/ai"
)

func TestGroupKnowledgeToolRoutesNaturalRequest(t *testing.T) {
	tool, ok := ai.GetTool("search_group_knowledge")
	if !ok {
		t.Fatal("search_group_knowledge tool is not registered")
	}
	for _, text := range []string{"根据知识库回答入群规则", "群里规定新人要做什么", "查一下群资料"} {
		if routed := ai.Route(text, []*ai.ToolMeta{tool}); len(routed) != 1 {
			t.Fatalf("Route(%q) = %#v", text, routed)
		}
	}
}
