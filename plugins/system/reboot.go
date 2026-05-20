package system

import (
	"os"

	"github.com/Yuelioi/yueling-go/bot"
)

func RegisterReboot(b *bot.Bot, superusers []int64) {
	b.OnCommand("reboot", "重启").Where(bot.SuperUserOnly{IDs: superusers}).Handle(func(ctx *bot.CommandContext) error {
		ctx.Reply("正在重启...")
		os.Exit(0)
		return nil
	})
}
