package system

import (
	"os"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/bot/perm"
)

func RegisterReboot(b *bot.Bot, superusers []int64) {
	b.OnCommand("reboot", "重启").Where(perm.SuperUser(superusers...)).Handle(func(ctx *bot.CommandContext) error {
		ctx.Reply("正在重启...")
		os.Exit(0)
		return nil
	})
}
