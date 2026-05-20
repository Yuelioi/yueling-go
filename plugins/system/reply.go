package system

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/group"
)

func RegisterReply(b *bot.Bot) {
	b.OnCommand("添加回复").Handle(func(ctx *bot.CommandContext) error {
		if len(ctx.Args) < 2 {
			return ctx.Reply("用法: /添加回复 关键词 内容")
		}
		kw := ctx.Args[0]
		content := strings.Join(ctx.Args[1:], " ")
		content = strings.ReplaceAll(content, `\n`, "\n")
		if err := db.AddReply(ctx.UserID(), kw, content, ""); err != nil {
			return ctx.Reply("添加失败: " + err.Error())
		}
		group.ReloadCache()
		return ctx.Reply("添加成功")
	})

	b.OnCommand("删除回复").Handle(func(ctx *bot.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("用法: /删除回复 ID")
		}
		id, err := strconv.ParseUint(ctx.Args[0], 10, 64)
		if err != nil {
			return ctx.Reply("ID 格式不对")
		}
		if err := db.DeleteReply(uint(id)); err != nil {
			return ctx.Reply("删除失败: " + err.Error())
		}
		group.ReloadCache()
		return ctx.Reply("删除成功")
	})

	b.OnCommand("更新回复").Handle(func(ctx *bot.CommandContext) error {
		group.ReloadCache()
		return ctx.Reply("回复表已重新加载")
	})

	b.OnCommand("查看回复").Handle(func(ctx *bot.CommandContext) error {
		rows, err := db.GetAllReplies()
		if err != nil {
			return ctx.Reply("查询失败")
		}
		if len(rows) == 0 {
			return ctx.Reply("暂无回复规则")
		}
		var sb strings.Builder
		for _, r := range rows {
			sb.WriteString(fmt.Sprintf("[%d] %s → %s\n", r.ID, r.Keyword, r.Reply))
		}
		return ctx.Reply(strings.TrimSpace(sb.String()))
	})
}
