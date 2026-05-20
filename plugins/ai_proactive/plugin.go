package ai_proactive

import (
	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
)

// Register wires the proactive speech handler onto the bot.
// It runs on every group message with lowest priority so specific handlers fire first.
func Register(b *bot.Bot) {
	b.OnGroupMessage().Priority(0).Handle(func(ctx *bot.GroupContext) error {
		ai.Proactive.Feed(ctx.BotAPI, ctx.MsgCtx.Event)
		return nil
	})
}
