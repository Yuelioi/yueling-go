package tools

import (
	"testing"

	"github.com/Yuelioi/yueling-go/ai"
)

func TestFeedSubscriptionToolRoutesNaturalRequests(t *testing.T) {
	tool, ok := ai.GetTool("manage_feed_subscription")
	if !ok {
		t.Fatal("manage_feed_subscription tool is not registered")
	}
	for _, text := range []string{
		"把 https://example.com/feed.xml 加到RSS订阅",
		"查看本群订阅源",
		"检查一下订阅更新",
	} {
		if routed := ai.Route(text, []*ai.ToolMeta{tool}); len(routed) != 1 {
			t.Fatalf("Route(%q) = %#v", text, routed)
		}
	}
}
