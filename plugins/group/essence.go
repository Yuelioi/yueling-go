package group

import (
	"strconv"

	"github.com/Yuelioi/yueling-go/bot"
)

func RegisterEssence(b *bot.Bot) {
	b.OnCommand("设精", "加精").
		Handle(func(ctx *bot.CommandContext) error {
			replyID, ok := ctx.Message().ReplyID()
			if !ok {
				return ctx.Reply("请回复要设精的消息后使用 /设精")
			}
			msgID64, err := strconv.ParseInt(replyID, 10, 32)
			if err != nil {
				return ctx.Reply("无法解析消息ID")
			}
			if err := ctx.SetEssenceMsg(int32(msgID64)); err != nil {
				return ctx.Reply("设精失败：" + err.Error())
			}
			return nil
		})
}
