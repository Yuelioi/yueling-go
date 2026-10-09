package bilibili

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	bservice "github.com/Yuelioi/yueling-go/services/bilibili"
	"github.com/Yuelioi/yueling-go/services/logx"
)

func Register(b *bot.Bot, store bservice.CheckpointStore) (*bservice.Manager, func()) {
	client, err := bservice.New(config.Current().Bilibili)
	if err != nil {
		logx.Warnf("[bilibili] 初始化失败: %v", err)
		return nil, func() {}
	}
	notifyUsers := recipients(config.Current().Bilibili.NotifyUsers, config.Current().Bot.SuperUsers)
	manager := bservice.NewManager(client, store, config.Current().Bilibili, notifyUsers, nil)
	manager.SetFallbackRecipients(config.Current().Bot.SuperUsers)
	runCtx, stop := context.WithCancel(b.Context())
	var current atomic.Pointer[bot.BotAPI]
	var startOnce sync.Once
	notify := func(qq int64, text string) error {
		api := current.Load()
		if api == nil {
			return fmt.Errorf("QQ 尚未连接")
		}
		id, err := api.SendPrivateMsg(qq, bot.Msg().Text(text).Build())
		if err == nil && id == 0 {
			return fmt.Errorf("QQ 私信发送未返回消息编号")
		}
		return err
	}
	poll := func(parent context.Context) (int, error) {
		ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
		defer cancel()
		return manager.Poll(ctx, notify)
	}
	if client.HasLogin() && client.AccountID() > 0 {
		b.OnConnect(func(api *bot.BotAPI) {
			current.Store(api)
			startOnce.Do(func() {
				b.Go(func() {
					manager.Run(runCtx, func() bool { return current.Load().Connected() }, notify, func(err error) {
						logx.Warnf("[bilibili] 私信同步失败: %v", err)
					})
				})
			})
		})
	}
	for _, command := range []struct {
		name    string
		handler func(*bot.CommandContext) error
	}{
		{"B站视频", videoHandler(client)},
		{"B站最近视频", recentHandler(client, manager)},
	} {
		b.OnCommand(command.name).Plugin(catalog.PluginBilibili).Block().Handle(command.handler)
		b.OnPrivateCommand(command.name).Plugin(catalog.PluginBilibili).Block().Handle(command.handler)
	}
	owner := bot.CondFn(func(_ *bot.BotAPI, msg *bot.MsgCtx) bool {
		return slices.Contains(config.Current().Bot.SuperUsers, msg.UserID()) || manager.CanManage(msg.UserID())
	})
	b.OnPrivateCommand("B站未读").Plugin(catalog.PluginBilibili).Where(owner).Block().Handle(func(ctx *bot.CommandContext) error {
		if len(ctx.Args) != 0 {
			return ctx.Reply("用法：B站未读")
		}
		requestCtx, cancel := context.WithTimeout(ctx.BotAPI.Context(), 15*time.Second)
		defer cancel()
		unread, err := client.Unread(requestCtx)
		if err != nil {
			return ctx.Reply("B站未读查询失败：" + err.Error())
		}
		return ctx.Reply(fmt.Sprintf("B站未读私信\n已关注：%d\n未关注：%d\n拦截消息：%d", unread.FollowUnread, unread.UnfollowUnread, unread.DustbinUnread))
	})
	b.OnPrivateCommand("B站状态").Plugin(catalog.PluginBilibili).Where(owner).Block().Handle(func(ctx *bot.CommandContext) error {
		if len(ctx.Args) != 0 {
			return ctx.Reply("用法：B站状态")
		}
		credential := "未配置登录 Cookie"
		if client.HasLogin() {
			credential = "已配置登录 Cookie（有效性以查询结果为准）"
		}
		return ctx.Reply(fmt.Sprintf("B站私信监控\n%s\n%s", credential, manager.Status()))
	})
	for _, command := range []struct {
		name    string
		handler func(*bot.CommandContext) error
	}{
		{"B站检查", func(ctx *bot.CommandContext) error {
			if len(ctx.Args) != 0 {
				return ctx.Reply("用法：B站检查")
			}
			current.Store(ctx.BotAPI)
			count, err := poll(runCtx)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("已转发 %d 条；检查失败：%v", count, err))
			}
			return ctx.Reply(fmt.Sprintf("私信检查完成，新增转发 %d 条（首次检查仅建立基线）", count))
		}},
		{"B站会话", func(ctx *bot.CommandContext) error {
			contacts, err := manager.Contacts()
			if err != nil {
				return ctx.Reply(err.Error())
			}
			if len(contacts) == 0 {
				return ctx.Reply("暂无收到新私信的联系人；新私信会自动转发，不需要手动检查。")
			}
			var lines []string
			for _, contact := range contacts {
				lines = append(lines, fmt.Sprintf("%d · %s：%s", contact.Number, contact.Name, shortText(contact.Preview, 80)))
			}
			return ctx.Reply("B站会话（编号固定）\n" + strings.Join(lines, "\n") + "\n回复：B站回复 <编号> <内容>；只有一个联系人时可省略编号")
		}},
		{"B站回复", func(ctx *bot.CommandContext) error {
			uid, number, text, err := manager.ResolveReply(replyBody(ctx.Text(), ctx.Cmd))
			if err != nil {
				return ctx.Reply(err.Error())
			}
			requestCtx, cancel := context.WithTimeout(runCtx, 25*time.Second)
			defer cancel()
			if err := manager.Reply(requestCtx, uid, text); err != nil {
				return ctx.Reply("B站回复未确认成功：" + err.Error() + "；发送超时可能已送达，请先在 B站核对再重试。")
			}
			return ctx.Reply(fmt.Sprintf("已回复 B站会话 %d", number))
		}},
		{"B站自动回复", func(ctx *bot.CommandContext) error {
			if len(ctx.Args) != 1 {
				return ctx.Reply("用法：B站自动回复 <关闭 / 关键词 / AI / 混合>")
			}
			mode := normalizeMode(ctx.Args[0])
			if err := manager.SetMode(mode); err != nil {
				return ctx.Reply(err.Error())
			}
			return ctx.Reply("B站自动回复模式已设为 " + mode)
		}},
		{"B站关键词", func(ctx *bot.CommandContext) error {
			if len(ctx.Args) != 0 {
				return ctx.Reply("用法：B站关键词；添加：B站添加关键词 <关键词> <回复>")
			}
			rules, err := manager.Rules()
			if err != nil {
				return ctx.Reply(err.Error())
			}
			if len(rules) == 0 {
				return ctx.Reply("暂无 B站关键词规则")
			}
			var lines []string
			for _, rule := range rules {
				kind := "包含"
				if rule.Exact {
					kind = "精确"
				}
				lines = append(lines, fmt.Sprintf("[%s] %s → %s", kind, rule.Keyword, shortText(rule.Reply, 100)))
			}
			return ctx.Reply(strings.Join(lines, "\n"))
		}},
		{"B站添加关键词", func(ctx *bot.CommandContext) error {
			if len(ctx.Args) < 2 {
				return ctx.Reply("用法：B站添加关键词 <关键词> <回复>（同词覆盖，包含匹配）")
			}
			if err := manager.SetRule(config.BilibiliKeyword{Keyword: ctx.Args[0], Reply: commandBody(ctx.Text(), ctx.Cmd)}); err != nil {
				return ctx.Reply(err.Error())
			}
			return ctx.Reply("B站关键词已保存")
		}},
		{"B站删除关键词", func(ctx *bot.CommandContext) error {
			if len(ctx.Args) != 1 {
				return ctx.Reply("用法：B站删除关键词 <关键词>")
			}
			if err := manager.DeleteRule(ctx.Args[0]); err != nil {
				return ctx.Reply(err.Error())
			}
			return ctx.Reply("B站关键词已删除")
		}},
	} {
		b.OnPrivateCommand(command.name).Plugin(catalog.PluginBilibili).Where(owner).Block().Handle(command.handler)
	}
	return manager, stop
}

func commandBody(text, command string) string {
	text = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), bot.CurrentCommandPrefix()))
	text = strings.TrimSpace(strings.TrimPrefix(text, command))
	index := strings.IndexFunc(text, unicode.IsSpace)
	if index < 0 {
		return ""
	}
	return strings.TrimSpace(text[index:])
}

func shortText(text string, limit int) string {
	chars := []rune(text)
	if len(chars) > limit {
		return string(chars[:limit]) + "…"
	}
	return text
}

func recipients(configured, superusers []int64) []int64 {
	if len(configured) == 0 {
		configured = superusers
	}
	var result []int64
	for _, id := range configured {
		if id > 0 && !slices.Contains(result, id) {
			result = append(result, id)
		}
	}
	return result
}

func normalizeMode(mode string) string {
	switch strings.ToLower(mode) {
	case "关闭", "off":
		return "off"
	case "关键词", "keyword":
		return "keyword"
	case "ai":
		return "ai"
	case "混合", "hybrid":
		return "hybrid"
	}
	return mode
}

func replyBody(text, command string) string {
	text = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), bot.CurrentCommandPrefix()))
	return strings.TrimSpace(strings.TrimPrefix(text, command))
}

func videoHandler(client *bservice.Client) func(*bot.CommandContext) error {
	return func(ctx *bot.CommandContext) error {
		if len(ctx.Args) < 1 || len(ctx.Args) > 2 {
			return ctx.Reply("用法：B站视频 <BV号 / av号 / 视频链接> [分P序号]")
		}
		part := 1
		if len(ctx.Args) == 2 {
			var err error
			part, err = strconv.Atoi(ctx.Args[1])
			if err != nil || part < 1 {
				return ctx.Reply("分P序号必须是正整数")
			}
		}
		requestCtx, cancel := context.WithTimeout(ctx.BotAPI.Context(), 25*time.Second)
		defer cancel()
		stats, err := client.Video(requestCtx, ctx.Args[0], part)
		if err != nil {
			return ctx.Reply("B站视频查询失败：" + err.Error())
		}
		return ctx.Reply(formatStats(stats))
	}
}

func recentHandler(client *bservice.Client, manager *bservice.Manager) func(*bot.CommandContext) error {
	return func(ctx *bot.CommandContext) error {
		uid, count, err := parseRecentArgs(ctx.Args)
		if err != nil {
			return ctx.Reply(err.Error())
		}
		settings, err := manager.Settings()
		if err != nil {
			return ctx.Reply(err.Error())
		}
		if uid == 0 {
			uid = settings.UID
		}
		if count == 0 {
			count = settings.RecentCount
		}
		requestCtx, cancel := context.WithTimeout(ctx.BotAPI.Context(), 90*time.Second)
		defer cancel()
		items, err := client.Recent(requestCtx, uid, count)
		if err != nil {
			return ctx.Reply("B站最近视频查询失败：" + err.Error())
		}
		if len(items) == 0 {
			return ctx.Reply("该 UP主暂无可查询的投稿")
		}
		var lines []string
		for _, item := range items {
			if item.Error != nil {
				lines = append(lines, fmt.Sprintf("%s\n%s\n统计暂不可用：%v", item.Title, item.BVID, item.Error))
			} else {
				lines = append(lines, formatStats(item.Stats))
			}
		}
		return ctx.Reply("最近投稿（按发布时间）\n\n" + strings.Join(lines, "\n\n"))
	}
}

func parseRecentArgs(args []string) (int64, int, error) {
	if len(args) > 2 {
		return 0, 0, fmt.Errorf("用法：B站最近视频 [UP主UID] [数量1-10]")
	}
	var uid int64
	var count int
	var err error
	if len(args) > 0 {
		uid, err = strconv.ParseInt(args[0], 10, 64)
		if err != nil || uid <= 0 {
			return 0, 0, fmt.Errorf("UP主 UID 必须是正整数")
		}
	}
	if len(args) > 1 {
		count, err = strconv.Atoi(args[1])
		if err != nil || count < 1 || count > 10 {
			return 0, 0, fmt.Errorf("数量必须在 1 到 10 之间")
		}
	}
	return uid, count, nil
}

func formatStats(stats bservice.VideoStats) string {
	view := stats.View
	favorite := "未知"
	if view.Stat.Favorite != nil {
		favorite = strconv.FormatUint(*view.Stat.Favorite, 10)
	} else if view.Stat.Fav != nil {
		favorite = strconv.FormatUint(*view.Stat.Fav, 10)
	}
	online := "暂不可用"
	if stats.OnlineError == nil && stats.Online.ShowSwitch.Total && stats.Online.Total != "" {
		online = stats.Online.Total
	}
	if stats.OnlineError == nil && stats.Online.ShowSwitch.Count && stats.Online.Count != "" {
		online += "（网页端 " + stats.Online.Count + "）"
	}
	return fmt.Sprintf("%s\nhttps://www.bilibili.com/video/%s\n当前在线（P%d）：%s\n播放 %d · 点赞 %d · 投币 %d\n收藏 %s · 分享 %d · 弹幕 %d · 评论 %d", view.Title, view.BVID, stats.Part, online, view.Stat.View, view.Stat.Like, view.Stat.Coin, favorite, view.Stat.Share, view.Stat.Danmaku, view.Stat.Reply)
}
