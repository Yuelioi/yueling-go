package system

import (
	"os"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"slices"
)

func RegisterReboot(b *bot.Bot, superusers []int64) {
	b.OnCommand("reboot", "重启").Plugin(catalog.PluginSystemTools).Where(bot.CondFn(func(_ *bot.BotAPI, msg *bot.MsgCtx) bool {
		return slices.Contains(config.Current().Bot.SuperUsers, msg.UserID())
	})).Handle(func(ctx *bot.CommandContext) error {
		ctx.Reply("正在重启...")
		os.Exit(0)
		return nil
	})
}
