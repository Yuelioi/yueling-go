package memo

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/scheduler"
)

const maxPerUser = 5

func Register(b *bot.Bot) {
	registerAdd(b)
	registerDelete(b)
	registerList(b)
}

func registerAdd(b *bot.Bot) {
	b.OnCommand("提醒").Handle(func(ctx *bot.CommandContext) error {
		if len(ctx.Args) < 2 {
			return ctx.Reply("用法：\n" +
				"  提醒 HH:MM 内容    — 每天定时提醒\n" +
				"  提醒 N分钟后 内容  — N分钟后提醒一次\n" +
				"  提醒 N小时后 内容  — N小时后提醒一次")
		}

		timeStr := ctx.Args[0]
		message := strings.Join(ctx.Args[1:], " ")

		count, _ := db.CountUserReminders(ctx.UserID(), ctx.GroupID())
		if count >= maxPerUser {
			return ctx.Reply(fmt.Sprintf("最多只能设置 %d 个提醒，请先取消旧的。", maxPerUser))
		}

		// 一次性提醒：N分钟后 / N小时后
		if n, unit, ok := parseRelativeTime(timeStr); ok {
			var desc string
			var cronExpr string
			var err error
			if unit == "分钟" {
				cronExpr, err = scheduler.AfterMinutes(n)
				desc = fmt.Sprintf("%d分钟后", n)
			} else {
				cronExpr, err = scheduler.AfterMinutes(n * 60)
				desc = fmt.Sprintf("%d小时后", n)
			}
			if err != nil {
				return ctx.Reply("设置失败：" + err.Error())
			}
			r, err := scheduler.Add(ctx.BotAPI, ctx.UserID(), ctx.GroupID(), cronExpr, message)
			if err != nil {
				return ctx.Reply("设置提醒失败，请稍后再试。")
			}
			return ctx.Reply(fmt.Sprintf("一次性提醒已设置（ID: %d）\n%s 提醒：%s", r.ID, desc, message))
		}

		// 每日定时：HH:MM
		cronExpr, err := scheduler.ParseTime(timeStr)
		if err != nil {
			return ctx.Reply(err.Error())
		}
		r, err := scheduler.Add(ctx.BotAPI, ctx.UserID(), ctx.GroupID(), cronExpr, message)
		if err != nil {
			return ctx.Reply("设置提醒失败，请稍后再试。")
		}
		return ctx.Reply(fmt.Sprintf("每日提醒已设置（ID: %d）\n每天 %s 提醒：%s", r.ID, timeStr, message))
	})
}

// parseRelativeTime parses "30分钟后" → (30, "分钟", true) or "2小时后" → (2, "小时", true)
func parseRelativeTime(s string) (int, string, bool) {
	for _, unit := range []string{"分钟后", "小时后"} {
		if strings.HasSuffix(s, unit) {
			numStr := strings.TrimSuffix(s, unit)
			n, err := strconv.Atoi(numStr)
			if err == nil && n > 0 {
				return n, strings.TrimSuffix(unit, "后"), true
			}
		}
	}
	return 0, "", false
}

func registerDelete(b *bot.Bot) {
	b.OnCommand("取消提醒").Handle(func(ctx *bot.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("用法：取消提醒 <ID>")
		}
		id, err := strconv.ParseUint(ctx.Args[0], 10, 64)
		if err != nil {
			return ctx.Reply("ID 格式错误。")
		}
		if err := scheduler.Remove(uint(id), ctx.UserID()); err != nil {
			return ctx.Reply("取消失败，请确认 ID 是否正确。")
		}
		return ctx.Reply(fmt.Sprintf("提醒 %d 已取消。", id))
	})
}

func registerList(b *bot.Bot) {
	b.OnCommand("我的提醒").Handle(func(ctx *bot.CommandContext) error {
		rows, err := db.GetUserReminders(ctx.UserID(), ctx.GroupID())
		if err != nil || len(rows) == 0 {
			return ctx.Reply("你还没有设置任何提醒。")
		}
		var sb strings.Builder
		sb.WriteString("你的提醒列表：\n")
		for _, r := range rows {
			sb.WriteString(fmt.Sprintf("ID %d — %s\n", r.ID, r.Message))
		}
		return ctx.Reply(strings.TrimRight(sb.String(), "\n"))
	})
}
