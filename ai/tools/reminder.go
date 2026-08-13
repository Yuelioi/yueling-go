package tools

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/scheduler"
)

const maxAIRemindersPerGroup = 5

func init() { registerManageReminder() }

func registerManageReminder() {
	ai.Register(ai.ToolMeta{
		Name:        "manage_reminder",
		Description: "用绝对时间创建、修改、顺延、查看或取消会在本群真实触发的个人提醒。先根据系统当前时间理解明天、下周、晚上等人类表达，再传 RFC3339 时间",
		Tags:        []string{"工具", "效率", "提醒"},
		Triggers:    []string{"提醒我", "我的提醒", "取消提醒", "改一下提醒", "推迟提醒", "叫我"},
		Patterns:    []string{`(明天|后天|下周|周[一二三四五六日天]|晚上|早上|下午).{0,12}(提醒|叫我)`, `\d+(分钟|小时|天)后`},
		Slots:       []string{"定时提醒", "闹钟", "稍后提醒", "工作日提醒", "每周提醒"},
		PluginID:    catalog.PluginReminder,
		Params: []ai.Param{
			{Name: "action", Type: "string", Description: "操作", Required: true, Enum: []string{"create", "list", "update", "remove", "snooze"}},
			{Name: "content", Type: "string", Description: "提醒内容；create 必填，update 可选", Required: false},
			{Name: "trigger_at", Type: "string", Description: "一次性提醒的绝对时间，必须为带时区的 RFC3339，例如 2026-08-14T09:30:00+08:00", Required: false},
			{Name: "repeat", Type: "string", Description: "重复方式；create/update 重复提醒时使用", Required: false, Enum: []string{"none", "daily", "workdays", "weekly"}},
			{Name: "time", Type: "string", Description: "重复提醒的本地时间 HH:MM", Required: false},
			{Name: "weekdays", Type: "array", ItemsType: "integer", Description: "weekly 的星期列表，0=周日、1=周一……6=周六", Required: false},
			{Name: "reminder_id", Type: "integer", Description: "update/remove/snooze 的提醒 ID", Required: false},
			{Name: "snooze_minutes", Type: "integer", Description: "顺延分钟数，1到10080", Required: false},
		},
		Handler: manageReminderHandler,
	})
}

func manageReminderHandler(ctx *ai.ToolContext) (string, error) {
	action := strings.ToLower(strings.TrimSpace(ctx.String("action")))
	switch action {
	case "create":
		content := strings.TrimSpace(ctx.String("content"))
		if content == "" {
			return "请提供提醒内容", nil
		}
		if utf8Len(content) > 256 {
			return "提醒内容最多256个字符", nil
		}
		if hint := ensureAIReminderCapacity(ctx.UserID(), ctx.GroupID()); hint != "" {
			return hint, nil
		}
		row, err := createReminder(ctx, content)
		if err != nil {
			return "设置提醒失败：" + err.Error(), nil
		}
		return fmt.Sprintf("提醒已设置（ID: %d）· %s · %s", row.ID, scheduler.DescribeReminder(*row), content), nil

	case "list":
		return listReminders(ctx)

	case "update":
		id := ctx.Int("reminder_id")
		if id <= 0 {
			return "请提供要修改的提醒ID", nil
		}
		current, err := db.GetReminder(uint(id), ctx.UserID(), ctx.GroupID())
		if err != nil {
			return "未找到属于你的提醒 ID " + strconv.FormatInt(id, 10), nil
		}
		content := strings.TrimSpace(ctx.String("content"))
		if content == "" {
			content = current.Message
		}
		if utf8Len(content) > 256 {
			return "提醒内容最多256个字符", nil
		}
		cronExpr, runAt, recurring, err := requestedSchedule(ctx, current)
		if err != nil {
			return "修改提醒失败：" + err.Error(), nil
		}
		row, err := scheduler.Update(ctx.BotAPI(), uint(id), ctx.UserID(), ctx.GroupID(), cronExpr, content, runAt, recurring)
		if err != nil {
			return "修改提醒失败：" + err.Error(), nil
		}
		return fmt.Sprintf("提醒 %d 已更新 · %s · %s", row.ID, scheduler.DescribeReminder(*row), row.Message), nil

	case "snooze":
		id, minutes := ctx.Int("reminder_id"), ctx.Int("snooze_minutes")
		if id <= 0 || minutes < 1 || minutes > 7*24*60 {
			return "请提供提醒ID和1到10080的顺延分钟数", nil
		}
		current, err := db.GetReminder(uint(id), ctx.UserID(), ctx.GroupID())
		if err != nil {
			return "未找到属于你的提醒", nil
		}
		if current.Recurring {
			return "重复提醒不能整体顺延；可以直接告诉我要改到几点", nil
		}
		runAt := time.Unix(current.RunAt, 0).Add(time.Duration(minutes) * time.Minute)
		row, err := scheduler.Update(ctx.BotAPI(), current.ID, ctx.UserID(), ctx.GroupID(), "", current.Message, runAt, false)
		if err != nil {
			return "顺延失败：" + err.Error(), nil
		}
		return fmt.Sprintf("提醒 %d 已顺延到 %s", row.ID, scheduler.DescribeReminder(*row)), nil

	case "remove":
		id := ctx.Int("reminder_id")
		if id <= 0 {
			return "请提供要取消的提醒ID", nil
		}
		if err := scheduler.Remove(uint(id), ctx.UserID(), ctx.GroupID()); err != nil {
			return "未找到属于你的提醒 ID " + strconv.FormatInt(id, 10), nil
		}
		return fmt.Sprintf("提醒 %d 已取消", id), nil
	}
	return "未知操作", nil
}

func createReminder(ctx *ai.ToolContext, content string) (*db.Reminder, error) {
	repeat := strings.ToLower(strings.TrimSpace(ctx.String("repeat")))
	if repeat == "" || repeat == "none" {
		runAt, err := parseReminderTime(ctx.String("trigger_at"))
		if err != nil {
			return nil, err
		}
		return scheduler.AddAt(ctx.BotAPI(), ctx.UserID(), ctx.GroupID(), runAt, content)
	}
	cronExpr, err := scheduler.ParseRecurring(strings.TrimSpace(ctx.String("time")), repeat, ctx.IntSlice("weekdays"))
	if err != nil {
		return nil, err
	}
	return scheduler.Add(ctx.BotAPI(), ctx.UserID(), ctx.GroupID(), cronExpr, content)
}

func requestedSchedule(ctx *ai.ToolContext, current *db.Reminder) (string, time.Time, bool, error) {
	_, triggerChanged := ctx.Params["trigger_at"]
	_, repeatChanged := ctx.Params["repeat"]
	_, timeChanged := ctx.Params["time"]
	_, weekdaysChanged := ctx.Params["weekdays"]
	if !triggerChanged && !repeatChanged && !timeChanged && !weekdaysChanged {
		return current.CronExpr, time.Unix(current.RunAt, 0), current.Recurring, nil
	}
	repeat := strings.ToLower(strings.TrimSpace(ctx.String("repeat")))
	if triggerChanged || repeat == "none" {
		runAt, err := parseReminderTime(ctx.String("trigger_at"))
		return "", runAt, false, err
	}
	if repeat == "" && current.Recurring {
		return "", time.Time{}, false, fmt.Errorf("修改重复时间时请同时提供 repeat")
	}
	cronExpr, err := scheduler.ParseRecurring(strings.TrimSpace(ctx.String("time")), repeat, ctx.IntSlice("weekdays"))
	return cronExpr, time.Time{}, true, err
}

func parseReminderTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("请提供具体提醒时间")
	}
	runAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("时间必须包含日期和时区，例如 2026-08-14T09:30:00+08:00")
	}
	return runAt, nil
}

func listReminders(ctx *ai.ToolContext) (string, error) {
	rows, err := db.GetUserReminders(ctx.UserID(), ctx.GroupID())
	if err != nil {
		return "读取提醒失败", nil
	}
	if len(rows) == 0 {
		return "你在本群还没有提醒", nil
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("ID %d · %s · %s", row.ID, scheduler.DescribeReminder(row), row.Message))
	}
	return "你的提醒：\n" + strings.Join(lines, "\n"), nil
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

func utf8Len(value string) int { return len([]rune(value)) }
