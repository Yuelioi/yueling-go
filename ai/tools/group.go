package tools

import (
	"fmt"
	"strconv"

	"github.com/Yuelioi/yueling-go/ai"
)

func init() {
	registerKickTool()
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
