package game

import (
	"testing"

	"github.com/Yuelioi/yueling-go/db"
)

func TestFormatScoreReplyOmitsBattleRecord(t *testing.T) {
	reply := formatScoreReply(&db.UserGameRecord{
		Score:  42,
		Streak: 5,
	})

	if want := "积分：42\n连续签到：5天"; reply != want {
		t.Fatalf("formatScoreReply() = %q, want %q", reply, want)
	}
}
