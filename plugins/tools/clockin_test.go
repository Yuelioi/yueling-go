package tools

import "testing"

func TestPickClockinEncouragementUsesMonthlyTier(t *testing.T) {
	tests := []struct {
		name    string
		monthly int
		want    string
	}{
		{name: "first", monthly: 1, want: "第 1 次到手，开张啦～"},
		{name: "getting started", monthly: 2, want: "都第 2 次了，节奏开始有了～"},
		{name: "steady", monthly: 7, want: "嚯，都第 7 次了，有点稳啊"},
		{name: "frequent", monthly: 15, want: "第 15 次了，这出勤率有点狠"},
		{name: "near perfect", monthly: 25, want: "第 25 次！这个月的打卡区快被你承包了"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickClockinEncouragement(tt.monthly, func(n int) int {
				if n == 0 {
					t.Fatal("encouragement tier must not be empty")
				}
				return 0
			})
			if got != tt.want {
				t.Fatalf("pickClockinEncouragement(%d) = %q, want %q", tt.monthly, got, tt.want)
			}
		})
	}
}

func TestPickClockinEncouragementCanChooseDifferentReplies(t *testing.T) {
	first := pickClockinEncouragement(10, func(int) int { return 0 })
	second := pickClockinEncouragement(10, func(int) int { return 1 })
	if first == second {
		t.Fatalf("different picks returned the same reply: %q", first)
	}
}
