package quotation

import (
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/plugins/image"
	"github.com/Yuelioi/yueling-go/services"
)

var whitelist = []string{"玉米", "甜甜"}

func Register(b *bot.Bot) {
	b.OnCommand("语录").Handle(func(ctx *bot.CommandContext) error {
		nickname := strings.TrimSpace(strings.Join(ctx.Args, " "))
		var keyword string
		switch {
		case nickname == "":
			keyword = fmt.Sprintf("%d_", ctx.GroupID())
		case isWhitelisted(nickname):
			keyword = nickname
		default:
			keyword = fmt.Sprintf("%d_%s", ctx.GroupID(), nickname)
		}
		path, err := services.GetRandomImage("语录", keyword)
		if err != nil {
			return ctx.Reply("尚未添加此人语录")
		}
		return ctx.SendGroupLocalImage(ctx.GroupID(), path)
	})

	b.OnCommand("添加语录").Handle(func(ctx *bot.CommandContext) error {
		return image.Upload(ctx, "语录", nameQuotation)
	})
}

func nameQuotation(hash, arg string, gid int64) string {
	if arg != "" {
		return fmt.Sprintf("%d_%s_%s", gid, arg, hash)
	}
	return fmt.Sprintf("%d_%s", gid, hash)
}

func isWhitelisted(name string) bool {
	for _, w := range whitelist {
		if name == w {
			return true
		}
	}
	return false
}

// HelpCommands 返回语录相关命令（供 help 注册表）。
func HelpCommands() []string { return []string{"语录", "添加语录"} }
