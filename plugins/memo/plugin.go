package memo

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/scheduler"
)

func Register(b *bot.Bot) {
	registerDelete(b)
	registerList(b)
	registerDailyDigest(b)
}

func registerDelete(b *bot.Bot) {
	b.OnCommand("取消提醒").Plugin(catalog.PluginReminder).Handle(func(ctx *bot.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("用法：取消提醒 <ID>")
		}
		id, err := strconv.ParseUint(ctx.Args[0], 10, 64)
		if err != nil {
			return ctx.Reply("ID 格式错误。")
		}
		if err := scheduler.Remove(uint(id), ctx.UserID(), ctx.GroupID()); err != nil {
			return ctx.Reply("取消失败，请确认 ID 是否正确。")
		}
		return ctx.Reply(fmt.Sprintf("提醒 %d 已取消。", id))
	})
}

func registerList(b *bot.Bot) {
	b.OnCommand("我的提醒").Plugin(catalog.PluginReminder).Handle(func(ctx *bot.CommandContext) error {
		rows, err := db.GetUserReminders(ctx.UserID(), ctx.GroupID())
		if err != nil || len(rows) == 0 {
			return ctx.Reply("你还没有设置任何提醒。")
		}
		var sb strings.Builder
		sb.WriteString("你的提醒列表：\n")
		for _, r := range rows {
			sb.WriteString(fmt.Sprintf("ID %d — %s — %s\n", r.ID, scheduler.DescribeReminder(r), r.Message))
		}
		return ctx.Reply(strings.TrimRight(sb.String(), "\n"))
	})
}
