package ai

import (
	"testing"
	"time"
)

func TestAILimiterUserLimit(t *testing.T) {
	l := newAILimiter(2, 0, time.Minute) // 2/min per user, group unlimited
	const u, g = int64(1), int64(100)

	for i := 0; i < 2; i++ {
		if ok, _ := l.Allow(u, g); !ok {
			t.Fatalf("call %d should be allowed", i+1)
		}
	}
	if ok, hint := l.Allow(u, g); ok || hint == "" {
		t.Fatalf("3rd call should be blocked with a hint, got ok=%v hint=%q", ok, hint)
	}
	// A different user is unaffected.
	if ok, _ := l.Allow(2, g); !ok {
		t.Fatalf("other user should be allowed")
	}
}

func TestAILimiterGroupLimit(t *testing.T) {
	l := newAILimiter(0, 2, time.Minute) // user unlimited, 2/min per group
	const g = int64(100)

	if ok, _ := l.Allow(1, g); !ok {
		t.Fatalf("user1 call1 should be allowed")
	}
	if ok, _ := l.Allow(2, g); !ok {
		t.Fatalf("user2 call1 should be allowed")
	}
	// 3rd call in the group, even from a fresh user, is blocked.
	if ok, hint := l.Allow(3, g); ok || hint == "" {
		t.Fatalf("group 3rd call should be blocked, got ok=%v hint=%q", ok, hint)
	}
	// A different group is unaffected.
	if ok, _ := l.Allow(3, 200); !ok {
		t.Fatalf("other group should be allowed")
	}
}

func TestAILimiterZeroMeansUnlimited(t *testing.T) {
	l := newAILimiter(0, 0, time.Minute)
	for i := 0; i < 50; i++ {
		if ok, _ := l.Allow(1, 100); !ok {
			t.Fatalf("call %d should be allowed when both limits are 0", i+1)
		}
	}
}

func TestAILimiterBlockedCallDoesNotConsumeOtherWindow(t *testing.T) {
	// user limit 1, group limit large. The user's 2nd call is blocked; it must
	// NOT consume a group slot, so a different user can still use the group.
	l := newAILimiter(1, 10, time.Minute)
	const g = int64(100)

	if ok, _ := l.Allow(1, g); !ok {
		t.Fatalf("user1 call1 should be allowed")
	}
	if ok, _ := l.Allow(1, g); ok {
		t.Fatalf("user1 call2 should be blocked by user limit")
	}
	// Group should have recorded exactly 1 call so far. Fill the remaining 9.
	for i := 0; i < 9; i++ {
		if ok, _ := l.Allow(int64(10+i), g); !ok {
			t.Fatalf("group fill call %d should be allowed (blocked user call must not have consumed a group slot)", i+1)
		}
	}
	if ok, _ := l.Allow(99, g); ok {
		t.Fatalf("group should now be full (10 real calls)")
	}
}

func TestAILimiterPrivateSkipsGroupWindow(t *testing.T) {
	// groupID <= 0 (private chat) only goes through the user window.
	l := newAILimiter(0, 1, time.Minute) // group limit 1, user unlimited
	for i := 0; i < 5; i++ {
		if ok, _ := l.Allow(1, 0); !ok {
			t.Fatalf("private call %d should be allowed (group window skipped)", i+1)
		}
	}
}
