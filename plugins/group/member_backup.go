package group

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services"
)

func RegisterMemberBackup(b *bot.Bot) {
	b.OnCommand("群友备份", "备份群友").Where(bot.AdminOnly{}).Handle(func(ctx *bot.CommandContext) error {
		members, err := ctx.GetGroupMemberList(ctx.GroupID())
		if err != nil {
			return ctx.Reply("获取成员列表失败：" + err.Error())
		}

		type memberEntry struct {
			UserID   int64  `json:"user_id"`
			Nickname string `json:"nickname"`
			Card     string `json:"card"`
			Role     string `json:"role"`
		}
		entries := make([]memberEntry, 0, len(members))
		for _, m := range members {
			entries = append(entries, memberEntry{
				UserID:   m.UserID,
				Nickname: m.Nickname,
				Card:     m.Card,
				Role:     m.Role,
			})
		}

		backupDir := services.DataPath("backup")
		if err := os.MkdirAll(backupDir, 0o755); err != nil {
			return ctx.Reply("创建备份目录失败")
		}
		ts := bot.Now().Format("20060102_150405")
		fname := filepath.Join(backupDir, fmt.Sprintf("members_%d_%s.json", ctx.GroupID(), ts))
		data, _ := json.MarshalIndent(entries, "", "  ")
		if err := os.WriteFile(filepath.Clean(fname), data, 0o644); err != nil {
			return ctx.Reply("写入文件失败：" + err.Error())
		}
		return ctx.Reply(fmt.Sprintf("备份完成，共 %d 名群友，已保存到 %s", len(entries), fname))
	})
}
