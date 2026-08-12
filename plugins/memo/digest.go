package memo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/bot/perm"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/scheduler"
	"gorm.io/gorm"
)

func registerDailyDigest(b *bot.Bot) {
	b.OnCommand("群聊日报").
		Plugin(catalog.PluginDailyDigest).
		Where(perm.Admin).
		Handle(func(ctx *bot.CommandContext) error {
			if len(ctx.Args) == 0 {
				row, err := db.GetDailyDigest(ctx.GroupID())
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ctx.Reply("群聊日报未开启。\n用法：群聊日报 HH:MM [消息条数]")
				}
				if err != nil {
					return ctx.Reply("读取日报设置失败。")
				}
				return ctx.Reply(fmt.Sprintf("群聊日报已开启：每天 %s，总结最近 %d 条消息。", row.SendTime, row.MessageCount))
			}

			if strings.EqualFold(ctx.Args[0], "off") || ctx.Args[0] == "关闭" {
				if err := scheduler.RemoveDailyDigest(ctx.GroupID()); err != nil {
					return ctx.Reply("关闭群聊日报失败。")
				}
				return ctx.Reply("群聊日报已关闭。")
			}

			if ctx.Args[0] == "现在" || ctx.Args[0] == "立即" || strings.EqualFold(ctx.Args[0], "now") {
				count := 80
				if len(ctx.Args) > 1 {
					parsed, err := strconv.Atoi(ctx.Args[1])
					if err != nil || parsed < 10 || parsed > 100 {
						return ctx.Reply("消息条数应为 10 到 100。")
					}
					count = parsed
				}
				ctx.React(bot.EmojiProcessing)
				generateCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				defer cancel()
				summary, err := ai.GenerateAndSendGroupDigest(generateCtx, ctx.BotAPI, ctx.GroupID(), count)
				if err != nil {
					return ctx.Reply("生成群聊日报失败：" + err.Error())
				}
				if summary == "" {
					return ctx.Reply("最近没有可总结的群聊内容。")
				}
				return nil
			}

			count := 80
			if len(ctx.Args) > 1 {
				parsed, err := strconv.Atoi(ctx.Args[1])
				if err != nil || parsed < 10 || parsed > 100 {
					return ctx.Reply("消息条数应为 10 到 100。")
				}
				count = parsed
			}
			row, err := scheduler.SetDailyDigest(ctx.BotAPI, ctx.GroupID(), ctx.UserID(), ctx.Args[0], count)
			if err != nil {
				return ctx.Reply("设置失败：" + err.Error())
			}
			return ctx.Reply(fmt.Sprintf("群聊日报已开启：每天 %s，总结最近 %d 条消息。", row.SendTime, row.MessageCount))
		})
}
