package ai

import "testing"

func TestGuardAllowsMemberRevokeAndBanTools(t *testing.T) {
	for _, text := range []string{"禁言刚才说脏话的人", "撤回刚才那条消息"} {
		if got := Guard(text, PermMember); got != GuardAllow {
			t.Fatalf("Guard(%q) = %v for member, want GuardAllow", text, got)
		}
	}
	if got := Guard("踢出这个人", PermMember); got != GuardBlockPerm {
		t.Fatalf("Guard() = %v for member kick, want GuardBlockPerm", got)
	}
	if got := Guard("踢出这个人", PermSuperUser); got != GuardAllow {
		t.Fatalf("Guard() = %v for superuser kick, want GuardAllow", got)
	}
}
