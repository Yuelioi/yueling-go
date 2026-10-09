package tools

import (
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/services/feed"
)

func FeedTool(feeds *feed.Manager) ai.ToolMeta {
	return ai.ToolMeta{
		Name:        "manage_feed_subscription",
		Description: "管理当前群的 RSS/Atom 订阅和更新推送",
		Tags:        []string{"工具", "订阅", "资讯"},
		Triggers:    []string{"RSS订阅", "Atom订阅", "订阅源", "我的订阅", "订阅B站", "订阅X", "订阅静默"},
		Patterns:    []string{`订阅.{0,12}https?://`, `(查看|检查|删除|状态|暂停|恢复).{0,6}订阅`, `订阅.{0,8}(UP主|开播|发视频|发推|静默|暂停|恢复)`},
		Slots:       []string{"RSS", "Atom", "资讯订阅", "更新推送", "B站动态", "B站直播", "X发推", "静默时段"},
		PluginID:    catalog.PluginFeedSubscription,
		Permission:  ai.PermAdmin,
		Params: []ai.Param{
			{Name: "action", Type: "string", Description: "操作", Required: true, Enum: []string{"add", "add_platform", "list", "remove", "set_enabled", "check", "status", "set_quiet"}},
			{Name: "url", Type: "string", Description: "RSS/Atom 地址（add 时必填）", Required: false},
			{Name: "name", Type: "string", Description: "自定义订阅名称", Required: false},
			{Name: "platform", Type: "string", Description: "平台类型", Required: false, Enum: []string{"bilibili_dynamic", "bilibili_video", "bilibili_live", "x_user"}},
			{Name: "target", Type: "string", Description: "UID、X用户名、主页或直播间链接", Required: false},
			{Name: "subscription_id", Type: "integer", Description: "订阅 ID（remove/set_enabled 时必填）", Required: false},
			{Name: "enabled", Type: "boolean", Description: "是否启用订阅（set_enabled 时必填）", Required: false},
			{Name: "quiet_enabled", Type: "boolean", Description: "是否开启静默时段（set_quiet 时必填）", Required: false},
			{Name: "quiet_start", Type: "string", Description: "静默开始时间 HH:MM", Required: false},
			{Name: "quiet_end", Type: "string", Description: "静默结束时间 HH:MM", Required: false},
		},
		ResultHandler: func(ctx *ai.ToolContext) ai.ToolResult { return manageFeedSubscription(feeds, ctx) },
	}
}

func manageFeedSubscription(feeds *feed.Manager, ctx *ai.ToolContext) ai.ToolResult {
	disabled, err := feeds.GroupDisabled(ctx.GroupID())
	if err != nil {
		return outcome(ai.ToolFailed, "读取订阅数据失败，请稍后再试。", false)
	}
	if disabled {
		return outcome(ai.ToolRejected, "订阅中心在本群已禁用", false)
	}

	switch strings.ToLower(strings.TrimSpace(ctx.String("action"))) {
	case "add":
		row, parsed, err := feeds.Add(
			ctx.GroupID(), ctx.UserID(), ctx.String("url"), ctx.String("name"),
		)
		if err != nil {
			return outcome(ai.ToolFailed, "操作未能确认完成，请先核对实际结果，避免重复提交。", true)
		}
		result := fmt.Sprintf("订阅已添加（ID: %d）：%s，按本群配置的间隔检查", row.ID, row.Name)
		if len(parsed.Items) > 0 {
			result += "；当前最新内容是“" + parsed.Items[0].Title + "”"
		}
		return outcome(ai.ToolSucceeded, result, true)

	case "add_platform":
		kind := feed.PlatformKind(strings.ToLower(strings.TrimSpace(ctx.String("platform"))))
		row, parsed, err := feeds.AddPlatform(
			ctx.GroupID(), ctx.UserID(), kind, ctx.String("target"), ctx.String("name"),
		)
		if err != nil {
			return outcome(ai.ToolFailed, "操作未能确认完成，请先核对实际结果，避免重复提交。", true)
		}
		result := fmt.Sprintf("%s已添加（ID: %d）：%s", feed.PlatformLabel(kind), row.ID, row.Name)
		if len(parsed.Items) > 0 {
			result += "；当前最新内容是“" + parsed.Items[0].Title + "”"
		}
		return outcome(ai.ToolSucceeded, result, true)

	case "list":
		rows, err := feeds.List(ctx.GroupID())
		if err != nil {
			return outcome(ai.ToolFailed, "读取订阅数据失败，请稍后再试。", false)
		}
		if len(rows) == 0 {
			return outcome(ai.ToolEmpty, "本群还没有 RSS/Atom 订阅", false)
		}
		lines := make([]string, 0, len(rows))
		for _, row := range rows {
			health := "已暂停"
			if row.Enabled && row.ConsecutiveFailures > 0 {
				health = fmt.Sprintf("异常%d次", row.ConsecutiveFailures)
			} else if row.Enabled {
				health = "正常"
			}
			lines = append(lines, fmt.Sprintf("ID %d · %s · %s · %s", row.ID, row.Name, health, truncateFeedValue(row.URL, 80)))
		}
		return outcome(ai.ToolSucceeded, "本群订阅：\n"+strings.Join(lines, "\n"), false)

	case "remove":
		id := ctx.Int("subscription_id")
		if id <= 0 {
			return outcome(ai.ToolRejected, "请提供要删除的订阅 ID", false)
		}
		if err := feeds.Remove(uint(id), ctx.GroupID()); err != nil {
			return outcome(ai.ToolFailed, "操作未能确认完成，请先核对实际结果，避免重复提交。", true)
		}
		return outcome(ai.ToolSucceeded, fmt.Sprintf("订阅 %d 已删除", id), true)

	case "set_enabled":
		if _, ok := ctx.Params["enabled"].(bool); !ok {
			return outcome(ai.ToolRejected, "请明确是否启用订阅", false)
		}
		id := ctx.Int("subscription_id")
		if id <= 0 {
			return outcome(ai.ToolRejected, "请提供要暂停或恢复的订阅 ID", false)
		}
		row, err := feeds.SetEnabled(uint(id), ctx.GroupID(), ctx.Bool("enabled"))
		if err != nil {
			return outcome(ai.ToolFailed, "操作未能确认完成，请先核对实际结果，避免重复提交。", true)
		}
		if row.Enabled {
			return outcome(ai.ToolSucceeded, fmt.Sprintf("订阅 %d「%s」已恢复，将在下一轮重新检查", row.ID, row.Name), true)
		}
		return outcome(ai.ToolSucceeded, fmt.Sprintf("订阅 %d「%s」已暂停，待推送内容已清理", row.ID, row.Name), true)

	case "check":
		result, err := feeds.CheckGroup(ctx.BotAPI(), ctx.GroupID())
		if err != nil {
			return outcome(ai.ToolFailed, "操作未能确认完成，请先核对实际结果，避免重复提交。", true)
		}
		if result.Checked == 0 && result.Delivered == 0 && result.Failed == 0 && result.Queued == 0 {
			rows, listErr := feeds.List(ctx.GroupID())
			if listErr != nil {
				return outcome(ai.ToolFailed, "读取订阅失败", false)
			}
			if len(rows) > 0 {
				return outcome(ai.ToolRejected, "当前没有运行中的订阅；请先恢复要检查的订阅", false)
			}
			return outcome(ai.ToolEmpty, "本群还没有订阅", false)
		}
		status := ai.ToolSucceeded
		if result.Failed > 0 {
			status = ai.ToolFailed
		}
		return outcome(status, fmt.Sprintf("检查完成：检查 %d 个订阅，发现 %d 条，推送 %d 条，队列剩余 %d 条，失败 %d 个", result.Checked, result.Items, result.Delivered, result.Queued, result.Failed), true)

	case "status":
		rows, err := feeds.List(ctx.GroupID())
		if err != nil {
			return outcome(ai.ToolFailed, "读取订阅数据失败，请稍后再试。", false)
		}
		setting, pending, err := feeds.DeliveryStatus(ctx.GroupID())
		if err != nil {
			return outcome(ai.ToolFailed, "读取订阅数据失败，请稍后再试。", false)
		}
		failing, active := 0, 0
		for _, row := range rows {
			if row.Enabled {
				active++
			}
			if row.Enabled && row.ConsecutiveFailures > 0 {
				failing++
			}
		}
		quiet := "关闭"
		if setting.QuietEnabled {
			quiet = setting.QuietStart + "–" + setting.QuietEnd
		}
		return outcome(ai.ToolSucceeded, fmt.Sprintf("订阅状态：活跃 %d，暂停 %d，异常 %d，待推送 %d，静默时段 %s", active, len(rows)-active, failing, pending, quiet), false)

	case "set_quiet":
		if _, ok := ctx.Params["quiet_enabled"].(bool); !ok {
			return outcome(ai.ToolRejected, "请明确是否启用静默时段", false)
		}
		setting, err := feeds.SetQuietHours(
			ctx.GroupID(), ctx.Bool("quiet_enabled"), ctx.String("quiet_start"), ctx.String("quiet_end"),
		)
		if err != nil {
			return outcome(ai.ToolFailed, "操作未能确认完成，请先核对实际结果，避免重复提交。", true)
		}
		if !setting.QuietEnabled {
			return outcome(ai.ToolSucceeded, "订阅静默时段已关闭", true)
		}
		return outcome(ai.ToolSucceeded, fmt.Sprintf("订阅静默时段已设为 %s–%s，期间更新会在结束后合并推送", setting.QuietStart, setting.QuietEnd), true)
	}
	return outcome(ai.ToolRejected, "未知操作，支持 add/add_platform/list/remove/set_enabled/check/status/set_quiet", false)
}

func truncateFeedValue(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "…"
}
