package wuxia

import "testing"

func TestNextStepFollowsAvailableActions(t *testing.T) {
	v := View{Level: 3, PracticeCost: 50, Character: Character{Silver: 80, Mastery: 1}}
	for _, tc := range []struct {
		name    string
		day     Daily
		silver  int64
		mastery int
		command string
	}{
		{"legacy unsigned already finished", Daily{Runs: 3}, 80, 1, "切磋 @群友"},
		{"remaining runs", Daily{Signed: true, Runs: 2}, 80, 1, "江湖"},
		{"can train", Daily{Signed: true, Runs: 3}, 50, 1, "切磋 @群友"},
		{"needs silver", Daily{Signed: true, Runs: 3}, 49, 1, "切磋 @群友"},
		{"mastery capped", Daily{Signed: true, Runs: 3}, 80, 3, "切磋 @群友"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v.Character.Silver, v.Character.Mastery = tc.silver, tc.mastery
			if got := v.NextStep(tc.day, 3); got.Command != tc.command {
				t.Fatalf("suggestion=%+v want %s", got, tc.command)
			}
		})
	}
}
