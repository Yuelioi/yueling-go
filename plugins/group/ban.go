package group

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
)

func RegisterBan(b *bot.Bot) {
	b.OnCommand("ban", "禁言").
		Where(bot.AdminOnly{}).
		Handle(func(ctx *bot.CommandContext) error {
			target, ok := parseTarget(ctx)
			if !ok {
				return ctx.Reply("用法：/ban @用户 [时长]\n时长示例：10m 1h 600（秒）")
			}
			dur := parseDuration(durationArg(ctx))
			if err := ctx.SetGroupBan(ctx.GroupID(), target, dur); err != nil {
				return ctx.Reply("禁言失败：" + err.Error())
			}
			if dur == 0 {
				return ctx.Reply(fmt.Sprintf("已解除 %d 的禁言", target))
			}
			return ctx.Reply(fmt.Sprintf("已禁言 %d，时长 %s", target, formatDur(dur)))
		})

	b.OnCommand("unban", "解禁").
		Where(bot.AdminOnly{}).
		Handle(func(ctx *bot.CommandContext) error {
			target, ok := parseTarget(ctx)
			if !ok {
				return ctx.Reply("用法：/unban @用户")
			}
			if err := ctx.SetGroupBan(ctx.GroupID(), target, 0); err != nil {
				return ctx.Reply("解禁失败：" + err.Error())
			}
			return ctx.Reply(fmt.Sprintf("已解除 %d 的禁言", target))
		})

	b.OnCommand("kick", "踢出").
		Where(bot.AdminOnly{}).
		Handle(func(ctx *bot.CommandContext) error {
			target, ok := parseTarget(ctx)
			if !ok {
				return ctx.Reply("用法：/kick @用户")
			}
			if err := ctx.SetGroupKick(ctx.GroupID(), target, false); err != nil {
				return ctx.Reply("踢出失败：" + err.Error())
			}
			return ctx.Reply(fmt.Sprintf("已踢出 %d", target))
		})
}

// parseTarget extracts the target QQ from @ mentions or the first arg.
func parseTarget(ctx *bot.CommandContext) (int64, bool) {
	for _, qq := range ctx.Message().AtTargets() {
		if id, err := strconv.ParseInt(qq, 10, 64); err == nil {
			return id, true
		}
	}
	if len(ctx.Args) > 0 {
		if id, err := strconv.ParseInt(ctx.Args[0], 10, 64); err == nil {
			return id, true
		}
	}
	return 0, false
}

// durationArg picks the duration token from args (skips a bare QQ number).
func durationArg(ctx *bot.CommandContext) string {
	for _, arg := range ctx.Args {
		if _, err := strconv.ParseInt(arg, 10, 64); err != nil {
			return arg // not a plain number → it's a duration string
		}
	}
	if len(ctx.Args) >= 2 {
		return ctx.Args[1]
	}
	return ""
}

// parseDuration converts "10m", "1h", "600" → seconds. Default 600s.
func parseDuration(s string) int {
	if s == "" {
		return 600
	}
	if s == "0" {
		return 0
	}
	if d, err := time.ParseDuration(s); err == nil {
		return int(d.Seconds())
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n * 60 // bare number treated as minutes
	}
	return 600
}

func formatDur(secs int) string {
	d := time.Duration(secs) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := secs % 60
	switch {
	case h > 0:
		return fmt.Sprintf("%d小时%d分", h, m)
	case m > 0:
		return fmt.Sprintf("%d分钟", m)
	default:
		return fmt.Sprintf("%d秒", s)
	}
}

// ---- Revoke ----

func RegisterRevoke(b *bot.Bot) {
	b.OnCommand("revoke", "撤回").
		Where(bot.AdminOnly{}).
		Handle(func(ctx *bot.CommandContext) error {
			replyID, ok := ctx.Message().ReplyID()
			if !ok {
				return ctx.Reply("请回复要撤回的消息后使用 /revoke")
			}
			msgID64, err := strconv.ParseInt(replyID, 10, 32)
			if err != nil {
				return ctx.Reply("无法解析消息ID")
			}
			if err := ctx.DeleteMsg(int32(msgID64)); err != nil {
				return ctx.Reply("撤回失败：" + err.Error())
			}
			return nil
		})
}

// ---- Mute all ----

func RegisterMuteAll(b *bot.Bot) {
	b.OnCommand("muteall", "全员禁言").
		Where(bot.AdminOnly{}).
		Handle(func(ctx *bot.CommandContext) error {
			enable := true
			if len(ctx.Args) > 0 && strings.ToLower(ctx.Args[0]) == "off" {
				enable = false
			}
			if err := ctx.SetGroupWholeBan(ctx.GroupID(), enable); err != nil {
				return ctx.Reply("操作失败：" + err.Error())
			}
			if enable {
				return ctx.Reply("已开启全员禁言")
			}
			return ctx.Reply("已关闭全员禁言")
		})
}
