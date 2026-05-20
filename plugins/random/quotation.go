package random

import (
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services"
)

var quotationWhitelist = []string{"玉米", "甜甜"}

func RegisterQuotation(b *bot.Bot) {
	b.OnCommand("语录").Handle(func(ctx *bot.CommandContext) error {
		nickname := strings.TrimSpace(strings.Join(ctx.Args, " "))

		var keyword string
		if nickname == "" {
			keyword = fmt.Sprintf("%d_", ctx.GroupID())
		} else {
			isWhitelisted := false
			for _, w := range quotationWhitelist {
				if nickname == w {
					isWhitelisted = true
					break
				}
			}
			if isWhitelisted {
				keyword = nickname
			} else {
				keyword = fmt.Sprintf("%d_%s", ctx.GroupID(), nickname)
			}
		}

		path, err := services.GetRandomImage("语录", keyword)
		if err != nil {
			return ctx.Reply("尚未添加此人语录")
		}
		return ctx.SendGroupLocalImage(ctx.GroupID(), path)
	})
}
