package ai

import "testing"

func TestResolveCount(t *testing.T) {
	cases := []struct {
		name           string
		provided, def  int
		min, max, want int
	}{
		{"omitted uses default", 0, 15, 1, 30, 15},
		{"negative uses default", -3, 15, 1, 30, 15},
		{"valid value passes through", 20, 15, 1, 30, 20},
		{"above max clamps to max", 99, 15, 1, 30, 30},
		{"below min clamps to min", 5, 50, 10, 100, 10},
		{"default out of range is clamped too", 0, 999, 10, 100, 100},
		{"boundary min", 10, 50, 10, 100, 10},
		{"boundary max", 100, 50, 10, 100, 100},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ResolveCount(c.provided, c.def, c.min, c.max); got != c.want {
				t.Fatalf("ResolveCount(%d,%d,%d,%d) = %d, want %d",
					c.provided, c.def, c.min, c.max, got, c.want)
			}
		})
	}
}
