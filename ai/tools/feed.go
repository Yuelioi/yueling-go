package tools

import (
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/services/feed"
)

func init() {
	ai.Register(ai.ToolMeta{
		Name:        "manage_feed_subscription",
		Description: "管理当前群的 RSS/Atom 订阅和更新推送",
		Tags:        []string{"工具", "订阅", "资讯"},
		Triggers:    []string{"RSS订阅", "Atom订阅", "订阅源", "我的订阅", "订阅B站", "订阅X", "订阅静默"},
		Patterns:    []string{`订阅.{0,12}https?://`, `(查看|检查|删除|状态|暂停|恢复).{0,6}订阅`, `订阅.{0,8}(UP主|开播|发视频|发推|静默|暂停|恢复)`},
		Slots:       []string{"RSS", "Atom", "资讯订阅", "更新推送", "B站动态", "B站直播", "X发推", "静默时段"},
		Permission:  ai.PermAdmin,
		Params: []ai.Param{
			{Name: "action", Type: "string", Description: "操作: add/add_platform/list/remove/set_enabled/check/status/set_quiet", Required: true},
			{Name: "url", Type: "string", Description: "RSS/Atom 地址（add 时必填）", Required: false},
			{Name: "name", Type: "string", Description: "自定义订阅名称", Required: false},
			{Name: "platform", Type: "string", Description: "平台类型: bilibili_dynamic/bilibili_video/bilibili_live/x_user", Required: false},
			{Name: "target", Type: "string", Description: "UID、X用户名、主页或直播间链接", Required: false},
			{Name: "subscription_id", Type: "integer", Description: "订阅 ID（remove/set_enabled 时必填）", Required: false},
			{Name: "enabled", Type: "boolean", Description: "是否启用订阅（set_enabled 时必填）", Required: false},
			{Name: "quiet_enabled", Type: "boolean", Description: "是否开启静默时段（set_quiet 时必填）", Required: false},
			{Name: "quiet_start", Type: "string", Description: "静默开始时间 HH:MM", Required: false},
			{Name: "quiet_end", Type: "string", Description: "静默结束时间 HH:MM", Required: false},
		},
		Handler: manageFeedSubscription,
	})
}

func manageFeedSubscription(ctx *ai.ToolContext) (string, error) {
	disabled, err := db.IsGroupPluginDisabled(ctx.GroupID(), catalog.PluginFeedSubscription)
	if err != nil {
		return "读取订阅中心状态失败", nil
	}
	if disabled {
		return "订阅中心在本群已禁用", nil
	}

	switch strings.ToLower(strings.TrimSpace(ctx.String("action"))) {
	case "add":
		row, parsed, err := feed.DefaultManager.Add(
			ctx.GroupID(), ctx.UserID(), ctx.String("url"), ctx.String("name"),
		)
		if err != nil {
			return "添加订阅失败：" + err.Error(), nil
		}
		result := fmt.Sprintf("订阅已添加（ID: %d）：%s，每 10 分钟检查一次", row.ID, row.Name)
		if len(parsed.Items) > 0 {
			result += "；当前最新内容是“" + parsed.Items[0].Title + "”"
		}
		return result, nil

	case "add_platform":
		kind := feed.PlatformKind(strings.ToLower(strings.TrimSpace(ctx.String("platform"))))
		row, parsed, err := feed.DefaultManager.AddPlatform(
			ctx.GroupID(), ctx.UserID(), kind, ctx.String("target"), ctx.String("name"),
		)
		if err != nil {
			return "添加平台订阅失败：" + err.Error(), nil
		}
		result := fmt.Sprintf("%s已添加（ID: %d）：%s", feed.PlatformLabel(kind), row.ID, row.Name)
		if len(parsed.Items) > 0 {
			result += "；当前最新内容是“" + parsed.Items[0].Title + "”"
		}
		return result, nil

	case "list":
		rows, err := feed.DefaultManager.List(ctx.GroupID())
		if err != nil {
			return "读取订阅失败", nil
		}
		if len(rows) == 0 {
			return "本群还没有 RSS/Atom 订阅", nil
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
		return "本群订阅：\n" + strings.Join(lines, "\n"), nil

	case "remove":
		id := ctx.Int("subscription_id")
		if id <= 0 {
			return "请提供要删除的订阅 ID", nil
		}
		if err := feed.DefaultManager.Remove(uint(id), ctx.GroupID()); err != nil {
			return "没有找到本群的这个订阅", nil
		}
		return fmt.Sprintf("订阅 %d 已删除", id), nil

	case "set_enabled":
		id := ctx.Int("subscription_id")
		if id <= 0 {
			return "请提供要暂停或恢复的订阅 ID", nil
		}
		row, err := feed.DefaultManager.SetEnabled(uint(id), ctx.GroupID(), ctx.Bool("enabled"))
		if err != nil {
			return "没有找到本群的这个订阅", nil
		}
		if row.Enabled {
			return fmt.Sprintf("订阅 %d「%s」已恢复，将在下一轮重新检查", row.ID, row.Name), nil
		}
		return fmt.Sprintf("订阅 %d「%s」已暂停，待推送内容已清理", row.ID, row.Name), nil

	case "check":
		result, err := feed.DefaultManager.CheckGroup(ctx.BotAPI(), ctx.GroupID())
		if err != nil {
			return "检查订阅失败：" + err.Error(), nil
		}
		if result.Checked == 0 {
			rows, listErr := feed.DefaultManager.List(ctx.GroupID())
			if listErr == nil && len(rows) > 0 {
				return "当前没有运行中的订阅；请先恢复要检查的订阅", nil
			}
			return "本群还没有订阅", nil
		}
		return fmt.Sprintf("检查完成：检查 %d 个订阅，发现 %d 条，推送 %d 条，队列剩余 %d 条，失败 %d 个",
			result.Checked, result.Items, result.Delivered, result.Queued, result.Failed), nil

	case "status":
		rows, err := feed.DefaultManager.List(ctx.GroupID())
		if err != nil {
			return "读取订阅状态失败", nil
		}
		setting, pending, err := feed.DefaultManager.DeliveryStatus(ctx.GroupID())
		if err != nil {
			return "读取订阅状态失败", nil
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
		return fmt.Sprintf("订阅状态：活跃 %d，暂停 %d，异常 %d，待推送 %d，静默时段 %s", active, len(rows)-active, failing, pending, quiet), nil

	case "set_quiet":
		setting, err := feed.DefaultManager.SetQuietHours(
			ctx.GroupID(), ctx.Bool("quiet_enabled"), ctx.String("quiet_start"), ctx.String("quiet_end"),
		)
		if err != nil {
			return "设置订阅静默失败：" + err.Error(), nil
		}
		if !setting.QuietEnabled {
			return "订阅静默时段已关闭", nil
		}
		return fmt.Sprintf("订阅静默时段已设为 %s–%s，期间更新会在结束后合并推送", setting.QuietStart, setting.QuietEnd), nil
	}
	return "未知操作，支持 add/add_platform/list/remove/set_enabled/check/status/set_quiet", nil
}

func truncateFeedValue(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "…"
}
