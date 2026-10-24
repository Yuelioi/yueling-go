package wuxia

import (
	"context"
	"reflect"
	"testing"
)

func TestCasinoAllDiceOutcomes(t *testing.T) {
	counts := map[string]int{}
	var payout int64
	for a := 1; a <= 6; a++ {
		for b := 1; b <= 6; b++ {
			for c := 1; c <= 6; c++ {
				r, err := ResolveCasino(100, [3]int{a, b, c})
				if err != nil {
					t.Fatal(err)
				}
				counts[r.Outcome]++
				payout += r.Payout
				if a == b && b == c && r.Payout != int64(a+4)*100 {
					t.Fatal("wrong triple payout")
				}
			}
		}
	}
	if payout != 21300 || !reflect.DeepEqual(counts, map[string]int{"豹子": 6, "顺子": 24, "对子回本": 90, "三奇三偶": 12, "落空": 84}) {
		t.Fatalf("counts=%v payout=%d", counts, payout)
	}
	r, err := ResolveCasino(21, [3]int{6, 2, 4})
	if err != nil || r.Payout != 10 {
		t.Fatal("half payout must round down")
	}
	for _, arg := range []string{"-1", "0", "19", "1.5", "大 50", "100 200", "999999999999999999999"} {
		if _, err := ParseCommand("casino", arg, 0); err == nil {
			t.Fatalf("accepted %q", arg)
		}
	}
	for _, arg := range []string{"", "20", "21", "100", "501"} {
		if _, err := ParseCommand("casino", arg, 0); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range [][3]int{{0, 1, 2}, {1, 7, 2}} {
		if _, err := ResolveCasino(100, d); err == nil {
			t.Fatal("invalid die accepted")
		}
	}
}

func TestCasinoReceiptSettlesOnce(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	req := Request{BotID: 1, GroupID: 880, UserID: 10, MessageID: 1, Name: "骰客", Action: "daily"}
	if _, err := s.Operate(ctx, req); err != nil {
		t.Fatal(err)
	}
	before, _, err := s.Profile(ctx, 880, 10)
	if err != nil {
		t.Fatal(err)
	}
	req.MessageID = 2
	req.Action = "casino"
	req.Argument = "20"
	r, err := s.Operate(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if r.Casino == nil || r.SilverDelta != r.Casino.Payout-20 {
		t.Fatalf("bad result %+v", r)
	}
	again, err := s.Operate(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Replayed || !reflect.DeepEqual(r.Casino, again.Casino) {
		t.Fatal("replay rerolled dice")
	}
	after, _, err := s.Profile(ctx, 880, 10)
	if err != nil {
		t.Fatal(err)
	}
	if after.Character.Silver != before.Character.Silver+r.SilverDelta {
		t.Fatal("double settlement")
	}
	if err := s.database.Model(&Character{}).Where("group_id = ? AND user_id = ?", 880, 10).Update("silver", 0).Error; err != nil {
		t.Fatal(err)
	}
	req.MessageID = 3
	if _, err := s.Operate(ctx, req); err == nil {
		t.Fatal("accepted unfunded bet")
	}
}

func TestCasinoDefaultStakeAndExplicitOverride(t *testing.T) {
	for _, tc := range []struct {
		arg  string
		want int64
	}{{"", 100}, {"20", 20}, {"500", 500}} {
		cmd, err := ParseCommand("casino", tc.arg, 0)
		if err != nil {
			t.Fatal(err)
		}
		casino, ok := cmd.(CasinoCommand)
		if !ok || casino.Stake != tc.want {
			t.Fatalf("%q: got %#v want %d", tc.arg, cmd, tc.want)
		}
	}
}
