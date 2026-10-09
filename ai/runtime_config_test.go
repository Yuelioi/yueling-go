package ai

import (
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

func TestOnlineGlobalPluginSwitchFiltersTools(t *testing.T) {
	old := config.Current()
	defer config.Publish(old)
	next := old
	next.Plugins.Disabled = []int{29}
	config.Publish(next)
	disabled := &ToolMeta{Name: "disabled", PluginID: 29}
	enabled := &ToolMeta{Name: "enabled", PluginID: 30}
	ungated := &ToolMeta{Name: "ungated"}
	got := filterByGroupPlugin(testEngine.Groups, []*ToolMeta{disabled, enabled, ungated}, 0)
	if len(got) != 2 || got[0] != enabled || got[1] != ungated {
		t.Fatalf("testEngine.Registry switch did not filter tools: %v", got)
	}
	if toolEnabledInGroup(testEngine.Groups, disabled, 0) {
		t.Fatal("disabled tool can still execute")
	}
	next.Plugins.Disabled = nil
	config.Publish(next)
	if !toolEnabledInGroup(testEngine.Groups, disabled, 0) {
		t.Fatal("online enable did not reach existing tool")
	}
}
