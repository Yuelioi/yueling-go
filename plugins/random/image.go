package random

import (
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services"
)

var localImageCmds = []struct {
	cmds   []string
	folder string
}{
	{[]string{"龙图", "龙图攻击"}, "龙图"},
	{[]string{"福瑞", "来点福瑞"}, "福瑞"},
	{[]string{"我老公呢", "老公"}, "老公"},
	{[]string{"我老婆呢", "老婆"}, "老婆"},
	{[]string{"沙雕图"}, "沙雕图"},
	{[]string{"杂鱼"}, "杂鱼"},
	{[]string{"美少女"}, "美少女"},
}

func RegisterImage(b *bot.Bot) {
	// cat — external URL, no local files needed
	b.OnCommand("随机猫猫", "来点猫猫").Handle(func(ctx *bot.CommandContext) error {
		_, err := ctx.SendGroupMsg(ctx.GroupID(), bot.Msg().Image("http://edgecats.net/").Build())
		return err
	})

	// local image folders
	for _, entry := range localImageCmds {
		folder := entry.folder
		cmds := entry.cmds
		b.OnCommand(cmds[0], cmds[1:]...).Handle(func(ctx *bot.CommandContext) error {
			path, err := services.GetRandomImage(folder, "")
			if err != nil {
				return ctx.Reply("图片不存在，请先放入素材")
			}
			return ctx.SendGroupLocalImage(ctx.GroupID(), path)
		})
	}
}
