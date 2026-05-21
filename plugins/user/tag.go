package user

import (
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
)

func Register(b *bot.Bot) {
	registerProfile(b)
}

// ── 画像（键值对） ─────────────────────────────────────────────────────────────

func registerProfile(b *bot.Bot) {
	b.OnCommand("我的标签", "我的信息").Handle(func(ctx *bot.CommandContext) error {
		return showProfile(ctx)
	})

	// 添加标签 位置 上海  /  添加标签 爱好 打游戏
	b.OnCommand("添加标签", "设置标签").Handle(func(ctx *bot.CommandContext) error {
		if len(ctx.Args) < 2 {
			return ctx.Reply("用法：添加标签 键 值\n例：添加标签 位置 上海\n   添加标签 爱好 打游戏")
		}
		key := ctx.Args[0]
		value := strings.Join(ctx.Args[1:], " ")
		if err := db.SetUserProfile(ctx.UserID(), key, value); err != nil {
			return ctx.Reply("设置失败")
		}
		return ctx.Reply(fmt.Sprintf("已设置 %s = %s", key, value))
	})

	b.OnCommand("删除标签").Handle(func(ctx *bot.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("用法：删除标签 键\n例：删除标签 位置")
		}
		key := strings.Join(ctx.Args, " ")
		if err := db.DeleteUserProfile(ctx.UserID(), key); err != nil {
			return ctx.Reply("删除失败，请确认键名是否正确。")
		}
		return ctx.Reply("已删除标签：" + key)
	})
}

func showProfile(ctx *bot.CommandContext) error {
	profile, err := db.GetAllUserProfile(ctx.UserID())
	if err != nil || len(profile) == 0 {
		return ctx.Reply("你还没有设置任何标签。\n用法：添加标签 键 值\n例：添加标签 位置 上海")
	}
	var sb strings.Builder
	sb.WriteString("你的标签：\n")
	for k, v := range profile {
		sb.WriteString(fmt.Sprintf("  %s：%s\n", k, v))
	}
	return ctx.Reply(strings.TrimRight(sb.String(), "\n"))
}
