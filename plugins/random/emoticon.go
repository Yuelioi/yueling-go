package random

import (
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/bot/rule"
	"github.com/Yuelioi/yueling-go/services"
)

func RegisterEmoticon(b *bot.Bot) {
	b.OnGroupMessage().When(rule.NoReply, rule.NoAt).Handle(func(ctx *bot.GroupContext) error {
		text := ctx.MsgCtx.Event.Message.Text()

		if strings.HasPrefix(text, "   ") {
			return nil // triple-space: ignore
		}

		if keyword, ok := strings.CutPrefix(text, "  "); ok {
			names, err := services.ListImageNames("表情", keyword)
			if err != nil || len(names) == 0 {
				return ctx.Reply(fmt.Sprintf("没有找到包含「%s」的表情", keyword))
			}
			preview := names
			if len(preview) > 10 {
				preview = preview[:10]
			}
			return ctx.Reply(fmt.Sprintf("共找到%d个:\n%s", len(names), strings.Join(preview, "\n")))
		}

		if keyword, ok := strings.CutPrefix(text, " "); ok {
			path, err := services.GetRandomImage("表情", keyword)
			if err != nil {
				return nil
			}
			return ctx.SendGroupLocalImage(ctx.GroupID(), path)
		}

		return nil
	})
}
