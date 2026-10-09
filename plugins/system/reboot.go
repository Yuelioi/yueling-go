package system

import (
	"slices"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
)

func RegisterReboot(b *bot.Bot, requestRestart func()) {
	b.OnCommand("reboot", "重启").Plugin(catalog.PluginSystemTools).Where(bot.CondFn(func(_ *bot.BotAPI, msg *bot.MsgCtx) bool {
		return slices.Contains(config.Current().Bot.SuperUsers, msg.UserID())
	})).Handle(func(ctx *bot.CommandContext) error {
		err := ctx.Reply("正在重启...")
		requestRestart()
		return err
	})
}
