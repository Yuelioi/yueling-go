package subscription

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/bot/perm"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/services/feed"
	"gorm.io/gorm"
)

func Register(b *bot.Bot) {
	b.OnConnect(func(api *bot.BotAPI) {
		feed.DefaultManager.Start(api)
	})

	b.OnCommand("订阅添加").
		Plugin(catalog.PluginFeedSubscription).
		Where(perm.Admin).
		Handle(handleAdd)

	b.OnCommand("订阅列表").
		Plugin(catalog.PluginFeedSubscription).
		Handle(handleList)

	b.OnCommand("订阅删除").
		Plugin(catalog.PluginFeedSubscription).
		Where(perm.Admin).
		Handle(handleDelete)

	b.OnCommand("订阅暂停").
		Plugin(catalog.PluginFeedSubscription).
		Where(perm.Admin).
		Handle(func(ctx *bot.CommandContext) error { return handleSetEnabled(ctx, false) })

	b.OnCommand("订阅恢复").
		Plugin(catalog.PluginFeedSubscription).
		Where(perm.Admin).
		Handle(func(ctx *bot.CommandContext) error { return handleSetEnabled(ctx, true) })

	b.OnCommand("订阅检查").
		Plugin(catalog.PluginFeedSubscription).
		Where(perm.Admin).
		Handle(handleCheck)

	b.OnCommand("订阅状态").
		Plugin(catalog.PluginFeedSubscription).
		Handle(handleStatus)

	b.OnCommand("订阅静默").
		Plugin(catalog.PluginFeedSubscription).
		Where(perm.Admin).
		Handle(handleQuietHours)

	platformCommands := []struct {
		command string
		kind    feed.PlatformKind
	}{
		{"订阅B站动态", feed.PlatformBilibiliDynamic},
		{"订阅B站视频", feed.PlatformBilibiliVideo},
		{"订阅B站直播", feed.PlatformBilibiliLive},
		{"订阅X", feed.PlatformXUser},
	}
	for _, entry := range platformCommands {
		kind := entry.kind
		b.OnCommand(entry.command).
			Plugin(catalog.PluginFeedSubscription).
			Where(perm.Admin).
			Handle(func(ctx *bot.CommandContext) error { return handlePlatformAdd(ctx, kind) })
	}
}

func handleAdd(ctx *bot.CommandContext) error {
	if len(ctx.Args) == 0 {
		return ctx.Reply("用法：订阅添加 <RSS/Atom URL> [名称]")
	}
	ctx.React(bot.EmojiProcessing)
	name := ""
	if len(ctx.Args) > 1 {
		name = strings.Join(ctx.Args[1:], " ")
	}
	row, parsed, err := feed.DefaultManager.Add(ctx.GroupID(), ctx.UserID(), ctx.Args[0], name)
	if err != nil {
		return ctx.Reply("添加订阅失败：" + friendlyFeedError(err))
	}
	reply := fmt.Sprintf("订阅已添加（ID: %d）\n%s\n每 10 分钟检查一次更新。", row.ID, row.Name)
	if len(parsed.Items) > 0 {
		reply += "\n当前最新：" + parsed.Items[0].Title
	}
	return ctx.Reply(reply)
}

func handleList(ctx *bot.CommandContext) error {
	rows, err := feed.DefaultManager.List(ctx.GroupID())
	if err != nil {
		return ctx.Reply("读取订阅失败。")
	}
	if len(rows) == 0 {
		return ctx.Reply("本群还没有 RSS/Atom 订阅。")
	}
	return ctx.Reply(formatSubscriptionList(rows))
}

func handlePlatformAdd(ctx *bot.CommandContext, kind feed.PlatformKind) error {
	if len(ctx.Args) == 0 {
		return ctx.Reply("用法：" + ctx.Cmd + " <UID/用户名/主页或直播间链接> [名称]")
	}
	ctx.React(bot.EmojiProcessing)
	name := ""
	if len(ctx.Args) > 1 {
		name = strings.Join(ctx.Args[1:], " ")
	}
	row, parsed, err := feed.DefaultManager.AddPlatform(ctx.GroupID(), ctx.UserID(), kind, ctx.Args[0], name)
	if err != nil {
		return ctx.Reply("添加订阅失败：" + friendlyFeedError(err))
	}
	reply := fmt.Sprintf("%s已添加（ID: %d）\n%s\n每 10 分钟检查一次。", feed.PlatformLabel(kind), row.ID, row.Name)
	if len(parsed.Items) > 0 {
		reply += "\n当前最新：" + parsed.Items[0].Title
	}
	return ctx.Reply(reply)
}

func handleDelete(ctx *bot.CommandContext) error {
	if len(ctx.Args) == 0 {
		return ctx.Reply("用法：订阅删除 <ID>")
	}
	id, err := strconv.ParseUint(ctx.Args[0], 10, 64)
	if err != nil || id == 0 {
		return ctx.Reply("订阅 ID 格式错误。")
	}
	if err := feed.DefaultManager.Remove(uint(id), ctx.GroupID()); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Reply("没有找到本群的这个订阅。")
		}
		return ctx.Reply("删除订阅失败。")
	}
	return ctx.Reply(fmt.Sprintf("订阅 %d 已删除。", id))
}

func handleSetEnabled(ctx *bot.CommandContext, enabled bool) error {
	if len(ctx.Args) == 0 {
		return ctx.Reply("用法：" + ctx.Cmd + " <ID>")
	}
	id, err := strconv.ParseUint(ctx.Args[0], 10, 64)
	if err != nil || id == 0 {
		return ctx.Reply("订阅 ID 格式错误。")
	}
	row, err := feed.DefaultManager.SetEnabled(uint(id), ctx.GroupID(), enabled)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Reply("没有找到本群的这个订阅。")
		}
		return ctx.Reply("修改订阅状态失败。")
	}
	if enabled {
		return ctx.Reply(fmt.Sprintf("订阅 %d「%s」已恢复，将在下一轮重新检查。", row.ID, row.Name))
	}
	return ctx.Reply(fmt.Sprintf("订阅 %d「%s」已暂停，尚未发送的队列内容已清理。", row.ID, row.Name))
}

func handleCheck(ctx *bot.CommandContext) error {
	ctx.React(bot.EmojiProcessing)
	result, err := feed.DefaultManager.CheckGroup(ctx.BotAPI, ctx.GroupID())
	if err != nil {
		return ctx.Reply("检查订阅失败：" + err.Error())
	}
	if result.Checked == 0 {
		rows, listErr := feed.DefaultManager.List(ctx.GroupID())
		if listErr == nil && len(rows) > 0 {
			return ctx.Reply("本群订阅均已暂停；请先使用“订阅恢复 <ID>”。")
		}
		return ctx.Reply("本群还没有 RSS/Atom 订阅。")
	}
	return ctx.Reply(fmt.Sprintf(
		"订阅检查完成：检查 %d 个，发现 %d 条，推送 %d 条，队列剩余 %d 条，失败 %d 个。",
		result.Checked, result.Items, result.Delivered, result.Queued, result.Failed,
	))
}

func handleStatus(ctx *bot.CommandContext) error {
	rows, err := feed.DefaultManager.List(ctx.GroupID())
	if err != nil {
		return ctx.Reply("读取订阅状态失败。")
	}
	setting, pending, err := feed.DefaultManager.DeliveryStatus(ctx.GroupID())
	if err != nil {
		return ctx.Reply("读取订阅状态失败。")
	}
	failing := 0
	active := 0
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
	var lines []string
	lines = append(lines, fmt.Sprintf("本群订阅状态\n活跃：%d · 暂停：%d · 异常：%d · 待推送：%d\n静默时段：%s",
		active, len(rows)-active, failing, pending, quiet))
	for _, row := range rows {
		lines = append(lines, formatFeedHealthLine(row, bot.Now().Location()))
	}
	return ctx.Reply(strings.Join(lines, "\n"))
}

func handleQuietHours(ctx *bot.CommandContext) error {
	if len(ctx.Args) == 0 {
		setting, pending, err := feed.DefaultManager.DeliveryStatus(ctx.GroupID())
		if err != nil {
			return ctx.Reply("读取订阅静默设置失败。")
		}
		if !setting.QuietEnabled {
			return ctx.Reply(fmt.Sprintf("订阅静默时段未开启，当前有 %d 条等待推送。\n用法：订阅静默 23:00-08:00", pending))
		}
		return ctx.Reply(fmt.Sprintf("订阅静默时段：%s–%s，当前有 %d 条等待推送。", setting.QuietStart, setting.QuietEnd, pending))
	}
	raw := strings.TrimSpace(strings.Join(ctx.Args, ""))
	if raw == "关闭" || strings.EqualFold(raw, "off") {
		if _, err := feed.DefaultManager.SetQuietHours(ctx.GroupID(), false, "", ""); err != nil {
			return ctx.Reply("关闭订阅静默失败。")
		}
		return ctx.Reply("订阅静默时段已关闭，队列中的更新会在下一轮检查时合并推送。")
	}
	for _, separator := range []string{"—", "–", "~", "至"} {
		raw = strings.ReplaceAll(raw, separator, "-")
	}
	parts := strings.SplitN(raw, "-", 2)
	if len(parts) != 2 {
		return ctx.Reply("用法：订阅静默 23:00-08:00；关闭请用：订阅静默 off")
	}
	setting, err := feed.DefaultManager.SetQuietHours(ctx.GroupID(), true, parts[0], parts[1])
	if err != nil {
		return ctx.Reply("设置失败：" + err.Error())
	}
	return ctx.Reply(fmt.Sprintf("订阅静默时段已设为 %s–%s；期间照常抓取，更新会在结束后合并推送。", setting.QuietStart, setting.QuietEnd))
}

func formatSubscriptionList(rows []db.FeedSubscription) string {
	var lines []string
	for _, row := range rows {
		rawURL := row.URL
		if len([]rune(rawURL)) > 80 {
			rawURL = string([]rune(rawURL)[:80]) + "…"
		}
		health := feedHealthLabel(row)
		lines = append(lines, fmt.Sprintf("ID %d · %s · %s\n%s", row.ID, row.Name, health, rawURL))
	}
	return fmt.Sprintf("本群订阅（%d/%d）：\n%s", len(rows), feed.MaxSubscriptionsPerGroup, strings.Join(lines, "\n\n"))
}

func feedHealthLabel(row db.FeedSubscription) string {
	if !row.Enabled {
		return "已暂停"
	}
	if row.ConsecutiveFailures > 0 {
		return fmt.Sprintf("异常 %d 次", row.ConsecutiveFailures)
	}
	return "正常"
}

func formatFeedHealthLine(row db.FeedSubscription, location *time.Location) string {
	line := fmt.Sprintf("ID %d · %s · %s", row.ID, row.Name, feedHealthLabel(row))
	if row.Enabled && row.ConsecutiveFailures > 0 {
		if row.NextCheckAt > 0 {
			line += " · 重试 " + time.Unix(row.NextCheckAt, 0).In(location).Format("01-02 15:04")
		}
		if message := truncateFeedStatus(row.LastError, 60); message != "" {
			line += "\n  " + message
		}
	}
	return line
}

func truncateFeedStatus(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes-1]) + "…"
}

func friendlyFeedError(err error) string {
	switch {
	case errors.Is(err, feed.ErrSubscriptionLimit), errors.Is(err, feed.ErrDuplicateSubscription):
		return err.Error()
	default:
		return err.Error()
	}
}
