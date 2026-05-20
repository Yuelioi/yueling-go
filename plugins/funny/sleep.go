package funny

import (
	"fmt"
	"math/rand"

	"github.com/Yuelioi/yueling-go/bot"
)

var sleepWords = []string{
	"被梦魇抓走了",
	"被僵尸吃掉了脑子",
	"被外星人抓走做实验了",
	"去梦里拯救世界了",
	"被睡神选中强制下线了",
	"掉进了睡眠黑洞",
}

func RegisterSleep(b *bot.Bot) {
	b.OnKeyword("我要睡觉").Handle(func(ctx *bot.GroupContext) error {
		hours := 5 + rand.Intn(4) // 5~8 小时
		seconds := hours * 3600
		nick := ctx.Nickname()
		word := sleepWords[rand.Intn(len(sleepWords))]

		ctx.SetGroupBan(ctx.GroupID(), ctx.UserID(), seconds)
		return ctx.Reply(fmt.Sprintf("%s %s，%d 小时后见！", nick, word, hours))
	})
}
