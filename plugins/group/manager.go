package group

import (
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
)

// RegisterManager auto-approves group join requests whose comment contains
// a configured keyword. Config key: bot.join_keywords ([]string).
func RegisterManager(b *bot.Bot) {
	b.OnRequest("group").Handle(func(ctx *bot.RequestContext) error {
		e := ctx.Event
		if e.SubType != "add" {
			return nil
		}
		comment := strings.ToLower(e.Comment)
		if comment == "" {
			return nil
		}
		for _, kw := range config.C.Bot.JoinKeywords {
			if strings.Contains(comment, strings.ToLower(kw)) {
				return ctx.BotAPI.SetGroupAddRequest(e.Flag, e.SubType, true, "")
			}
		}
		return nil
	})
}
