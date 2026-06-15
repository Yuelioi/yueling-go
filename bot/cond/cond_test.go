package cond

import (
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
)

func mkMsg(userID int64, role string) *bot.MsgCtx {
	return &bot.MsgCtx{Event: &bot.GroupMessageEvent{
		UserID: userID,
		Sender: bot.Sender{UserID: userID, Role: role},
	}}
}

func TestAdminOwnerSuperUser(t *testing.T) {
	config.C.Bot.SuperUsers = []int64{999}

	cases := []struct {
		name      string
		userID    int64
		role      string
		wantAdmin bool
		wantOwner bool
	}{
		{"群管", 1, "admin", true, false},
		{"群主", 2, "owner", true, true},
		{"超管普通成员", 999, "member", true, true},
		{"普通成员", 3, "member", false, false},
	}
	for _, c := range cases {
		msg := mkMsg(c.userID, c.role)
		if got := Admin.Check(nil, msg); got != c.wantAdmin {
			t.Errorf("%s: Admin = %v, want %v", c.name, got, c.wantAdmin)
		}
		if got := Owner.Check(nil, msg); got != c.wantOwner {
			t.Errorf("%s: Owner = %v, want %v", c.name, got, c.wantOwner)
		}
	}
}
