package tools

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
)

func init() {
	registerBanTool()
	registerKickTool()
	registerRevokeTool()
}

func registerBanTool() {
	ai.Register(ai.ToolMeta{
		Name:            "ban_member",
		Description:     "禁言群成员，需要管理员权限",
		Tags:            []string{"群管"},
		Triggers:        []string{"禁言"},
		Slots:           []string{"禁言", "闭嘴", "ban"},
		Permission:      ai.PermAdmin,
		Risk:            ai.RiskHigh,
		ConfirmRequired: true,
		Params: []ai.Param{
			{Name: "user_id", Type: "string", Description: "被禁言的QQ号", Required: true},
			{Name: "duration", Type: "integer", Description: "禁言时长（秒），0表示解除禁言，默认600", Required: false},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			uidStr := ctx.String("user_id")
			uid, err := strconv.ParseInt(uidStr, 10, 64)
			if err != nil {
				return "无效的QQ号：" + uidStr, nil
			}
			dur := int(ctx.Int("duration"))
			if dur == 0 {
				dur = 600
			}
			if err := ctx.BotAPI().SetGroupBan(ctx.GroupID(), uid, dur); err != nil {
				return "禁言失败：" + err.Error(), nil
			}
			if dur == 0 {
				return fmt.Sprintf("已解除 %d 的禁言", uid), nil
			}
			d := time.Duration(dur) * time.Second
			return fmt.Sprintf("已禁言 %d，时长 %.0f 分钟", uid, d.Minutes()), nil
		},
	})
}

func registerKickTool() {
	ai.Register(ai.ToolMeta{
		Name:            "kick_member",
		Description:     "将成员踢出群聊，需要管理员权限",
		Tags:            []string{"群管"},
		Triggers:        []string{"踢出", "踢人"},
		Slots:           []string{"踢", "移除", "kick"},
		Permission:      ai.PermAdmin,
		Risk:            ai.RiskHigh,
		ConfirmRequired: true,
		Params: []ai.Param{
			{Name: "user_id", Type: "string", Description: "被踢出的QQ号", Required: true},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			uidStr := ctx.String("user_id")
			uid, err := strconv.ParseInt(uidStr, 10, 64)
			if err != nil {
				return "无效的QQ号：" + uidStr, nil
			}
			if err := ctx.BotAPI().SetGroupKick(ctx.GroupID(), uid, false); err != nil {
				return "踢出失败：" + err.Error(), nil
			}
			return fmt.Sprintf("已将 %d 踢出群聊", uid), nil
		},
	})
}

func registerRevokeTool() {
	ai.Register(ai.ToolMeta{
		Name:        "revoke_message",
		Description: "撤回指定消息，需要管理员权限",
		Tags:        []string{"群管"},
		Triggers:    []string{"撤回"},
		Slots:       []string{"撤回", "删除消息"},
		Permission:  ai.PermAdmin,
		Risk:        ai.RiskMedium,
		Params: []ai.Param{
			{Name: "message_id", Type: "integer", Description: "要撤回的消息ID", Required: true},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			msgID := int32(ctx.Int("message_id"))
			if err := ctx.BotAPI().DeleteMsg(msgID); err != nil {
				return "撤回失败：" + err.Error(), nil
			}
			return "已撤回该消息", nil
		},
	})
}
