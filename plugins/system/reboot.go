package system

import (
	"os"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/bot/perm"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
)

func RegisterReboot(b *bot.Bot, superusers []int64) {
	b.OnCommand("reboot", "重启").Plugin(catalog.PluginSystemTools).Where(perm.SuperUser(superusers...)).Handle(func(ctx *bot.CommandContext) error {
		ctx.Reply("正在重启...")
		os.Exit(0)
		return nil
	})
}
