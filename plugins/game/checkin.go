package game

import (
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
)

func RegisterCheckIn(b *bot.Bot) {
	b.OnCommand("签到").Handle(func(ctx *bot.CommandContext) error {
		gained, streak, _, already, err := db.CheckIn(ctx.UserID(), ctx.GroupID(), ctx.Nickname())
		if err != nil {
			return ctx.Reply("签到失败，请稍后再试。")
		}
		if already {
			return ctx.Reply("你今天已经签到过了，明天再来吧。")
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("签到成功！获得 %d 积分", gained))
		if streak > 1 {
			sb.WriteString(fmt.Sprintf("（连续签到 %d 天）", streak))
		}

		r, _ := db.GetOrCreateGameRecord(ctx.UserID(), ctx.GroupID(), ctx.Nickname())
		sb.WriteString(fmt.Sprintf("，当前积分 %d", r.Score))
		return ctx.Reply(sb.String())
	})
}

func RegisterScore(b *bot.Bot) {
	b.OnCommand("积分", "我的积分").Handle(func(ctx *bot.CommandContext) error {
		r, err := db.GetOrCreateGameRecord(ctx.UserID(), ctx.GroupID(), ctx.Nickname())
		if err != nil {
			return ctx.Reply("查询失败，请稍后再试。")
		}
		return ctx.Reply(fmt.Sprintf(
			"积分：%d\n战绩：%d胜 %d负\n连续签到：%d天",
			r.Score, r.WinCount, r.LoseCount, r.Streak,
		))
	})
}

func RegisterRanking(b *bot.Bot) {
	b.OnCommand("排行", "积分排行").Handle(func(ctx *bot.CommandContext) error {
		rows, err := db.GetTopScores(ctx.GroupID(), 10)
		if err != nil || len(rows) == 0 {
			return ctx.Reply("暂无排行数据。")
		}

		var sb strings.Builder
		sb.WriteString("积分排行榜 TOP 10\n")
		for i, r := range rows {
			name := r.Nickname
			if name == "" {
				name = fmt.Sprintf("QQ:%d", r.UserID)
			}
			sb.WriteString(fmt.Sprintf("%d. %s — %d 分\n", i+1, name, r.Score))
		}
		return ctx.Reply(strings.TrimRight(sb.String(), "\n"))
	})
}
