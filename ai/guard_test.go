package ai

import "testing"

func TestGuardLeavesToolPermissionsToRegistry(t *testing.T) {
	for _, text := range []string{"禁言刚才说脏话的人", "撤回刚才那条消息"} {
		if got := Guard(text, PermMember); got != GuardAllow {
			t.Fatalf("Guard(%q) = %v for member, want GuardAllow", text, got)
		}
	}
}
