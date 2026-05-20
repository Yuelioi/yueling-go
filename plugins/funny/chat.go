package funny

import (
	"fmt"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
)

func RegisterChat(b *bot.Bot) {
	b.OnCommand("查看好感度", "查询好感度").Handle(func(ctx *bot.CommandContext) error {
		record, err := db.GetOrCreateGameRecord(ctx.UserID(), ctx.GroupID(), ctx.Nickname())
		if err != nil {
			return ctx.Reply("查询失败")
		}
		score := record.Score
		var rel, attitude string
		switch {
		case score >= 300:
			rel, attitude = "挚友", "亲密撒娇，温柔可爱，像好朋友"
		case score >= 150:
			rel, attitude = "好朋友", "友好温和，偶尔撒娇"
		case score >= 50:
			rel, attitude = "普通朋友", "正常聊天，不冷不热"
		default:
			rel, attitude = "陌生人", "有点冷淡，回复简短"
		}
		return ctx.Reply(fmt.Sprintf("好感度: %d 积分\n关系: %s\n态度: %s", score, rel, attitude))
	})
}
