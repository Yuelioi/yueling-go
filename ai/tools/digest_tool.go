package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"gorm.io/gorm"
)

func (t *toolset) registerDigestTools() {
	t.registry.Register(ai.ToolMeta{
		Name:        "manage_daily_digest",
		Description: "设置、查看、关闭或立即生成本群聊天日报，仅管理员可用",
		Tags:        []string{"群聊日报", "群管理"},
		Triggers:    []string{"群聊日报", "聊天日报", "总结今天群聊", "今日群聊总结"},
		Patterns:    []string{`(每天|每日).{0,8}(总结|日报)`, `(立即|现在).{0,8}(日报|总结群聊)`},
		Slots:       []string{"定时群聊总结", "每日摘要"},
		PluginID:    catalog.PluginDailyDigest,
		Permission:  ai.PermAdmin,
		Params: []ai.Param{
			{Name: "action", Type: "string", Description: "操作", Required: true, Enum: []string{"set", "status", "disable", "run_now"}},
			{Name: "time", Type: "string", Description: "set 的每日时间 HH:MM", Required: false},
			{Name: "message_count", Type: "integer", Description: "总结最近消息数，10到100，默认80", Required: false},
		},
		ResultHandler: func(ctx *ai.ToolContext) ai.ToolResult {
			count := int(ctx.Int("message_count"))
			if count == 0 {
				count = 80
			}
			if count < 10 || count > 100 {
				return outcome(ai.ToolRejected, "消息条数应为10到100", false)
			}
			switch strings.TrimSpace(ctx.String("action")) {
			case "set":
				row, err := t.schedules.SetDailyDigest(ctx.BotAPI().Connection(), ctx.GroupID(), ctx.UserID(), ctx.String("time"), count)
				if err != nil {
					return outcome(ai.ToolFailed, "操作未能确认完成，请先核对实际结果，避免重复提交。", true)
				}
				return outcome(ai.ToolSucceeded, fmt.Sprintf("群聊日报已开启：每天 %s，总结最近 %d 条消息", row.SendTime, row.MessageCount), true)
			case "status":
				row, err := t.schedules.Digest(ctx.GroupID())
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return outcome(ai.ToolEmpty, "本群尚未开启群聊日报", false)
				}
				if err != nil {
					return outcome(ai.ToolFailed, "读取日报设置失败，请稍后再试。", false)
				}
				return outcome(ai.ToolSucceeded, fmt.Sprintf("群聊日报：每天 %s，总结最近 %d 条消息", row.SendTime, row.MessageCount), false)
			case "disable":
				if err := t.schedules.RemoveDailyDigest(ctx.GroupID()); err != nil {
					return outcome(ai.ToolFailed, "操作未能确认完成，请先核对实际结果，避免重复提交。", true)
				}
				return outcome(ai.ToolSucceeded, "群聊日报已关闭", true)
			case "run_now":
				runCtx, cancel := context.WithTimeout(ctx.Context(), 45*time.Second)
				defer cancel()
				summary, err := ai.GenerateAndSendGroupDigest(runCtx, ctx.BotAPI(), ctx.GroupID(), count)
				if err != nil {
					return outcome(ai.ToolFailed, "操作未能确认完成，请先核对实际结果，避免重复提交。", true)
				}
				if summary == "" {
					return outcome(ai.ToolEmpty, "最近没有可总结的群聊内容", false)
				}
				return outcome(ai.ToolSucceeded, "群聊日报已发送", true)
			}
			return outcome(ai.ToolRejected, "未知操作", false)
		},
	})
}
