package emoticon

import (
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/bot/rule"
	"github.com/Yuelioi/yueling-go/plugins/image"
	"github.com/Yuelioi/yueling-go/services"
)

func Register(b *bot.Bot) {
	b.OnCommand("添加表情").Handle(func(ctx *bot.CommandContext) error {
		return image.Upload(ctx, "表情", nameEmoticon)
	})

	b.OnGroupMessage().When(rule.NoReply, rule.NoAt).Handle(func(ctx *bot.GroupContext) error {
		text := ctx.MsgCtx.Event.Message.Text()

		if strings.HasPrefix(text, "   ") {
			return nil
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

func nameEmoticon(hash, arg string, _ int64) string {
	if arg != "" {
		return arg + "_" + hash
	}
	return hash
}

// HelpCommands 返回表情相关命令（供 help 注册表）。
func HelpCommands() []string { return []string{"添加表情"} }
