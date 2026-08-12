package tools

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/scheduler"
)

const maxAIRemindersPerGroup = 5

func init() {
	registerManageReminder()
}

func registerManageReminder() {
	ai.Register(ai.ToolMeta{
		Name:        "manage_reminder",
		Description: "管理会在群里真实触发的个人提醒；有明确时间的“提醒我”请求必须使用本工具，不要当成待办",
		Tags:        []string{"工具", "效率", "提醒"},
		Triggers:    []string{"提醒我", "我的提醒", "取消提醒", "叫我"},
		Patterns:    []string{`\d+(分钟|小时)后`, `(每天|每日).{0,8}\d{1,2}:\d{2}`},
		Slots:       []string{"定时提醒", "闹钟", "稍后提醒"},
		Params: []ai.Param{
			{Name: "action", Type: "string", Description: "操作: add_after/add_daily/list/remove", Required: true},
			{Name: "content", Type: "string", Description: "提醒内容（新增时必填）", Required: false},
			{Name: "amount", Type: "integer", Description: "延迟数量（add_after时必填）", Required: false},
			{Name: "unit", Type: "string", Description: "延迟单位: minute/hour（add_after时必填）", Required: false},
			{Name: "time", Type: "string", Description: "每日时间 HH:MM（add_daily时必填）", Required: false},
			{Name: "reminder_id", Type: "integer", Description: "提醒ID（remove时必填）", Required: false},
		},
		Handler: manageReminderHandler,
	})
}

func manageReminderHandler(ctx *ai.ToolContext) (string, error) {
	action := strings.ToLower(strings.TrimSpace(ctx.String("action")))
	switch action {
	case "add_after":
		content := strings.TrimSpace(ctx.String("content"))
		amount := ctx.Int("amount")
		unit := strings.ToLower(strings.TrimSpace(ctx.String("unit")))
		if content == "" || amount <= 0 {
			return "请提供提醒内容和大于0的延迟时间", nil
		}
		if utf8Len(content) > 256 {
			return "提醒内容最多256个字符", nil
		}
		var delay time.Duration
		switch unit {
		case "minute", "minutes", "分钟":
			if amount > 365*24*60 {
				return "一次性提醒最长可设置一年后", nil
			}
			delay = time.Duration(amount) * time.Minute
		case "hour", "hours", "小时":
			if amount > 365*24 {
				return "一次性提醒最长可设置一年后", nil
			}
			delay = time.Duration(amount) * time.Hour
		default:
			return "时间单位只支持 minute 或 hour", nil
		}
		if hint := ensureAIReminderCapacity(ctx.UserID(), ctx.GroupID()); hint != "" {
			return hint, nil
		}
		row, err := scheduler.AddAfter(ctx.BotAPI(), ctx.UserID(), ctx.GroupID(), delay, content)
		if err != nil {
			return "设置提醒失败: " + err.Error(), nil
		}
		return fmt.Sprintf("一次性提醒已设置（ID: %d），%s后提醒你：%s", row.ID, formatReminderDelay(delay), content), nil

	case "add_daily":
		content := strings.TrimSpace(ctx.String("content"))
		hhmm := strings.TrimSpace(ctx.String("time"))
		if content == "" || hhmm == "" {
			return "请提供每日提醒时间和内容", nil
		}
		if utf8Len(content) > 256 {
			return "提醒内容最多256个字符", nil
		}
		if hint := ensureAIReminderCapacity(ctx.UserID(), ctx.GroupID()); hint != "" {
			return hint, nil
		}
		cronExpr, err := scheduler.ParseTime(hhmm)
		if err != nil {
			return err.Error(), nil
		}
		row, err := scheduler.Add(ctx.BotAPI(), ctx.UserID(), ctx.GroupID(), cronExpr, content)
		if err != nil {
			return "设置提醒失败: " + err.Error(), nil
		}
		return fmt.Sprintf("每日提醒已设置（ID: %d），每天 %s 提醒你：%s", row.ID, hhmm, content), nil

	case "list":
		rows, err := db.GetUserReminders(ctx.UserID(), ctx.GroupID())
		if err != nil {
			return "读取提醒失败", nil
		}
		if len(rows) == 0 {
			return "你在本群还没有提醒", nil
		}
		var lines []string
		for _, row := range rows {
			lines = append(lines, fmt.Sprintf("ID %d · %s · %s", row.ID, scheduler.DescribeReminder(row), row.Message))
		}
		return "你的提醒：\n" + strings.Join(lines, "\n"), nil

	case "remove":
		id := ctx.Int("reminder_id")
		if id <= 0 {
			return "请提供要取消的提醒ID", nil
		}
		rows, err := db.GetUserReminders(ctx.UserID(), ctx.GroupID())
		if err != nil {
			return "读取提醒失败", nil
		}
		owned := false
		for _, row := range rows {
			if row.ID == uint(id) {
				owned = true
				break
			}
		}
		if !owned {
			return "未找到属于你的提醒 ID " + strconv.FormatInt(id, 10), nil
		}
		if err := scheduler.Remove(uint(id), ctx.UserID(), ctx.GroupID()); err != nil {
			return "取消提醒失败", nil
		}
		return fmt.Sprintf("提醒 %d 已取消", id), nil
	}
	return "未知操作，支持 add_after/add_daily/list/remove", nil
}

func ensureAIReminderCapacity(userID, groupID int64) string {
	count, err := db.CountUserReminders(userID, groupID)
	if err != nil {
		return "读取提醒数量失败"
	}
	if count >= maxAIRemindersPerGroup {
		return fmt.Sprintf("本群最多设置 %d 个提醒，请先取消旧提醒", maxAIRemindersPerGroup)
	}
	return ""
}

func formatReminderDelay(delay time.Duration) string {
	if delay%time.Hour == 0 {
		return fmt.Sprintf("%d小时", int(delay/time.Hour))
	}
	return fmt.Sprintf("%d分钟", int(delay/time.Minute))
}

func utf8Len(value string) int {
	return len([]rune(value))
}
