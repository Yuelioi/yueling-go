package system

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
)

func RegisterRules(b *bot.Bot) {
	b.OnCommand("添加群规则").Where(bot.AdminOnly{}).Handle(func(ctx *bot.CommandContext) error {
		rule := strings.Join(ctx.Args, " ")
		if rule == "" {
			return ctx.Reply("用法：/添加群规则 规则内容")
		}
		if err := ai.AddGroupRule(ctx.GroupID(), ctx.UserID(), rule); err != nil {
			return ctx.Reply(err.Error())
		}
		return ctx.Reply("群规则已添加。")
	})

	b.OnCommand("删除群规则").Where(bot.AdminOnly{}).Handle(func(ctx *bot.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("用法：/删除群规则 <ID>")
		}
		id, err := strconv.ParseUint(ctx.Args[0], 10, 64)
		if err != nil {
			return ctx.Reply("ID 格式错误。")
		}
		if err := ai.RemoveGroupRule(ctx.GroupID(), uint(id)); err != nil {
			return ctx.Reply("删除失败。")
		}
		return ctx.Reply(fmt.Sprintf("规则 %d 已删除。", id))
	})

	b.OnCommand("群规则").Handle(func(ctx *bot.CommandContext) error {
		rows := ai.ListGroupRules(ctx.GroupID())
		if len(rows) == 0 {
			return ctx.Reply("本群暂无规则。")
		}
		var sb strings.Builder
		sb.WriteString("本群规则：\n")
		for _, r := range rows {
			sb.WriteString(fmt.Sprintf("ID %d — %s\n", r.ID, r.Rule))
		}
		return ctx.Reply(strings.TrimRight(sb.String(), "\n"))
	})
}
