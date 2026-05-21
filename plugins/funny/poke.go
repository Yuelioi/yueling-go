package funny

import (
	"math/rand"

	"github.com/Yuelioi/yueling-go/bot"
)

var pokeReplies = []string{
	"干嘛戳我！",
	"别戳了别戳了！",
	"再戳就禁言你！",
	"戳一下五块钱，戳完了记得付款。",
	"你好，你已成功激怒月灵。",
	"我在睡觉呢！",
	"( ` ω ´ )",
	"有事说事，别总戳人家！",
}

func RegisterPoke(b *bot.Bot) {
	b.OnNotice("notify"). // NapCat 的戳一戳 notice_type 是 "notify"，sub_type 是 "poke"
				Handle(func(ctx *bot.NoticeContext) error {
			e := ctx.Event
			// 只响应戳机器人自己的事件
			if e.SubType != "poke" || e.TargetID != ctx.SelfID {
				return nil
			}
			reply := pokeReplies[rand.Intn(len(pokeReplies))]
			return ctx.Reply(reply)
		})
}
