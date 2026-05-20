package game

import (
	"fmt"
	"math/rand"
	"strconv"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
)

var pkActions = []string{
	"使出了降龙十八掌",
	"祭出了葵花宝典",
	"施展了凌波微步",
	"掏出了倚天剑",
	"献上了独孤九剑",
	"亮出了打狗棒法",
	"祭起了九阳神功",
	"抖出了乾坤大挪移",
}

func RegisterPK(b *bot.Bot) {
	b.OnCommand("pk", "PK").Handle(func(ctx *bot.CommandContext) error {
		targets := ctx.Message().AtTargets()
		if len(targets) == 0 {
			return ctx.Reply("用法：/pk @某人")
		}

		targetID, err := strconv.ParseInt(targets[0], 10, 64)
		if err != nil {
			return ctx.Reply("无效的目标用户。")
		}
		if targetID == ctx.UserID() {
			return ctx.Reply("不能和自己 PK。")
		}
		if targetID == ctx.MsgCtx.SelfID() {
			return ctx.Reply("你打不过我的。")
		}

		challengerNick := ctx.Nickname()
		targetSender, _ := ctx.GetGroupMemberInfo(ctx.GroupID(), targetID)
		targetNick := fmt.Sprintf("QQ:%d", targetID)
		if targetSender != nil {
			if targetSender.Card != "" {
				targetNick = targetSender.Card
			} else if targetSender.Nickname != "" {
				targetNick = targetSender.Nickname
			}
		}

		challengerWins := rand.Intn(2) == 0

		var winnerID, loserID int64
		var winnerNick, loserNick string
		if challengerWins {
			winnerID, winnerNick = ctx.UserID(), challengerNick
			loserID, loserNick = targetID, targetNick
		} else {
			winnerID, winnerNick = targetID, targetNick
			loserID, loserNick = ctx.UserID(), challengerNick
		}

		action := pkActions[rand.Intn(len(pkActions))]
		winnerScore, loserScore, err := db.UpdatePKResult(winnerID, loserID, ctx.GroupID(), winnerNick, loserNick)
		if err != nil {
			return ctx.Reply("PK 结算失败，请稍后再试。")
		}

		result := fmt.Sprintf(
			"[PK] %s vs %s\n%s %s，%s 落败！\n%s +5分（共%d）  %s -2分（共%d）",
			challengerNick, targetNick,
			winnerNick, action, loserNick,
			winnerNick, winnerScore, loserNick, loserScore,
		)
		return ctx.Reply(result)
	})
}
