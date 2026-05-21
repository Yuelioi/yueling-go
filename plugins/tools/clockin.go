package tools

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
)

var trailingNum = regexp.MustCompile(`^(.+?)(\d+)$`)

func RegisterClockin(b *bot.Bot) {
	b.OnCommand("打卡").Handle(func(ctx *bot.CommandContext) error {
		_, _, monthly, already, err := db.CheckIn(ctx.UserID(), ctx.GroupID(), ctx.Nickname())
		if err != nil {
			return ctx.Reply("打卡失败，请稍后再试。")
		}
		if already {
			return ctx.Reply("今天打过卡了，明天再来吧~")
		}

		// 尝试把群名片末尾数字 +1
		info, err := ctx.GetGroupMemberInfo(ctx.GroupID(), ctx.UserID())
		if err == nil {
			card := info.Card
			if card == "" {
				card = info.Nickname
			}
			if m := trailingNum.FindStringSubmatch(card); m != nil {
				n, _ := strconv.Atoi(m[2])
				newCard := m[1] + strconv.Itoa(n+1)
				ctx.SetGroupCard(ctx.GroupID(), ctx.UserID(), newCard)
				return ctx.Reply(fmt.Sprintf("打卡成功！%s → %s\n本月已打卡 %d 次", card, newCard, monthly))
			}
		}
		return ctx.Reply(fmt.Sprintf("打卡成功！本月已打卡 %d 次", monthly))
	})
}
