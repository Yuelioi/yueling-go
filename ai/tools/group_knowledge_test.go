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

func TestLegacyAutoReplyToolWasMergedIntoKnowledgeAdmin(t *testing.T) {
	if _, ok := ai.GetTool("manage_auto_reply"); ok {
		t.Fatal("legacy manage_auto_reply tool should be removed")
	}
	tool, ok := ai.GetTool("manage_group_knowledge")
	if !ok {
		t.Fatal("manage_group_knowledge tool is not registered")
	}
	if routed := ai.Route("给知识12设置快捷词ae下载", []*ai.ToolMeta{tool}); len(routed) != 1 {
		t.Fatalf("shortcut request not routed: %#v", routed)
	}
}
