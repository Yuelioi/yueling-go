package funny

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/services/chatinsights"
	"github.com/Yuelioi/yueling-go/services/logx"
)

const chatStatsBackfill = 500

type chatWord struct {
	Text  string
	Count int
}

type chatUserCount struct {
	UserID   int64
	Nickname string
	Count    int
}

type chatAnalysis struct {
	Label        string
	Total        int
	TextTotal    int
	Participants int
	Words        []chatWord
	Users        []chatUserCount
}

func RegisterChatStats(b *bot.Bot) {
	// Record before command handlers run so blocked handlers cannot create gaps.
	b.OnGroupMessage().
		Plugin(catalog.PluginChatStats).
		Priority(100).
		Handle(func(ctx *bot.GroupContext) error {
			recordLiveChatMessage(ctx)
			return nil
		})

	b.OnCommand("词云", "今日词云", "昨日词云", "本周词云", "周词云").
		Plugin(catalog.PluginChatStats).
		Handle(func(ctx *bot.CommandContext) error {
			window, _ := chatinsights.ResolvePeriod(string(chatPeriodForCommand(ctx.Cmd)), bot.Now())
			return sendChatWordCloud(ctx, window.Label, window.Start, window.End, 0)
		})

	b.OnCommand("我的词云").
		Plugin(catalog.PluginChatStats).
		Handle(func(ctx *bot.CommandContext) error {
			window, _ := chatinsights.ResolvePeriod(string(chatinsights.PeriodToday), bot.Now())
			return sendChatWordCloud(ctx, "我的今日", window.Start, window.End, ctx.UserID())
		})

	b.OnCommand("废话榜", "今日废话榜", "昨日废话榜", "本周废话榜", "今日龙王", "本周龙王").
		Plugin(catalog.PluginChatStats).
		Handle(func(ctx *bot.CommandContext) error {
			window, _ := chatinsights.ResolvePeriod(string(chatPeriodForCommand(ctx.Cmd)), bot.Now())
			backfillRecentChat(ctx, window.Start)
			summary, err := db.GetGroupChatSummary(ctx.GroupID(), 0, window.Start, window.End)
			if err != nil {
				return ctx.Reply("读取群聊统计失败。")
			}
			if summary.Total == 0 {
				return ctx.Reply("还没有可统计的聊天记录，先聊一会儿再来看看吧。")
			}
			users, err := db.GetGroupChatUserCounts(ctx.GroupID(), window.Start, window.End, 10)
			if err != nil {
				return ctx.Reply("读取群聊统计失败。")
			}
			analysis := chatAnalysisFromDatabase(window.Label, summary, nil, users)
			return ctx.Reply(formatChatLeaderboard(analysis))
		})

	b.OnCommand("口头禅", "我的口头禅").
		Plugin(catalog.PluginChatStats).
		Handle(func(ctx *bot.CommandContext) error {
			window, _ := chatinsights.ResolvePeriod(string(chatinsights.PeriodSevenDays), bot.Now())
			target, valid := resolveChatStatsTarget(ctx.Cmd, ctx.UserID(), ctx.MsgCtx.SelfID(), ctx.Args, ctx.Message().AtTargets())
			if !valid {
				return ctx.Reply("用法：口头禅 @群友（也支持填写 QQ 号）")
			}
			backfillRecentChat(ctx, window.Start)
			expressions, err := chatinsights.QueryUserExpressions(ctx.GroupID(), target, window)
			if err != nil {
				return ctx.Reply("读取口头禅失败。")
			}
			if len(expressions.Phrases) == 0 && len(expressions.Words) == 0 {
				return ctx.Reply("最近七天还没有足够的文字记录来分析口头禅。")
			}
			name := expressions.Nickname
			if name == "" {
				name = fmt.Sprintf("用户%d", target)
			}
			var sb strings.Builder
			fmt.Fprintf(&sb, "%s最近七天的口头禅\n", name)
			if len(expressions.Phrases) > 0 {
				sb.WriteString("常说原句\n")
				for i, phrase := range expressions.Phrases {
					fmt.Fprintf(&sb, "%d. “%s” ×%d\n", i+1, phrase.Text, phrase.Count)
				}
			}
			if len(expressions.Words) > 0 {
				sb.WriteString("高频词\n")
				for i, word := range expressions.Words {
					fmt.Fprintf(&sb, "%d. %s（%d 条消息）\n", i+1, word.Text, word.Count)
				}
			}
			return ctx.Reply(strings.TrimSpace(sb.String()))
		})

	b.OnCommand("谁最爱说").
		Plugin(catalog.PluginChatStats).
		Handle(func(ctx *bot.CommandContext) error {
			keyword := strings.TrimSpace(strings.Join(ctx.Args, " "))
			if keyword == "" {
				return ctx.Reply("用法：谁最爱说 <关键词>")
			}
			if utf8.RuneCountInString(keyword) > 24 {
				return ctx.Reply("关键词太长了，最多 24 个字。")
			}
			window, _ := chatinsights.ResolvePeriod(string(chatinsights.PeriodSevenDays), bot.Now())
			backfillRecentChat(ctx, window.Start)
			rows, err := db.FindGroupChatUsersSaying(ctx.GroupID(), window.Start, window.End, keyword, 8)
			if err != nil {
				return ctx.Reply("读取群聊统计失败。")
			}
			if len(rows) == 0 {
				return ctx.Reply("最近七天没人说过「" + keyword + "」。")
			}
			var sb strings.Builder
			fmt.Fprintf(&sb, "最近七天谁最爱说「%s」\n", keyword)
			for i, user := range rows {
				fmt.Fprintf(&sb, "%d. %s · %d 条\n", i+1, displayChatName(chatUserCount{
					UserID: user.UserID, Nickname: user.Nickname, Count: user.Count,
				}), user.Count)
			}
			return ctx.Reply(strings.TrimSpace(sb.String()))
		})
}

func resolveChatStatsTarget(command string, currentUserID, selfID int64, args, atTargets []string) (int64, bool) {
	if command != "口头禅" {
		return currentUserID, true
	}
	for _, raw := range atTargets {
		target, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && target > 0 && target != selfID {
			return target, true
		}
	}
	if len(args) == 0 {
		return currentUserID, true
	}
	raw := strings.TrimPrefix(args[0], "@")
	target, err := strconv.ParseInt(raw, 10, 64)
	return target, err == nil && target > 0
}

func chatPeriodForCommand(command string) chatinsights.Period {
	switch command {
	case "昨日词云", "昨日废话榜":
		return chatinsights.PeriodYesterday
	case "本周词云", "周词云", "本周废话榜", "本周龙王":
		return chatinsights.PeriodWeek
	default:
		return chatinsights.PeriodToday
	}
}

func recordLiveChatMessage(ctx *bot.GroupContext) {
	now := bot.Now()
	createdAt := ctx.Event.Time
	if createdAt <= 0 {
		createdAt = now.Unix()
	}
	if ctx.MessageID() == 0 {
		return
	}
	row := db.GroupChatMessage{
		GroupID:      ctx.GroupID(),
		MessageID:    ctx.MessageID(),
		UserID:       ctx.UserID(),
		Nickname:     cleanChatNickname(ctx.Nickname()),
		Content:      truncateChatText(ctx.Text(), 2_000),
		StatExcluded: isChatStatsCommand(ctx.Text()),
		CreatedAt:    createdAt,
	}
	if err := db.SaveGroupChatMessage(row); err != nil {
		logx.Warnf("[chatstats] save group=%d message=%d: %v", row.GroupID, row.MessageID, err)
	}
}

func backfillRecentChat(ctx *bot.CommandContext, start time.Time) {
	history, err := ctx.GetGroupMsgHistory(ctx.GroupID(), 0, chatStatsBackfill)
	if err != nil {
		logx.Warnf("[chatstats] history backfill group=%d: %v", ctx.GroupID(), err)
		return
	}
	rows := make([]db.GroupChatMessage, 0, len(history))
	for _, message := range history {
		if message.MessageID == 0 || message.Time == 0 || message.Time < start.Unix() {
			continue
		}
		name := message.Sender.Card
		if name == "" {
			name = message.Sender.Nickname
		}
		content := truncateChatText(historyChatText(message), 2_000)
		rows = append(rows, db.GroupChatMessage{
			GroupID:      ctx.GroupID(),
			MessageID:    message.MessageID,
			UserID:       message.UserID,
			Nickname:     cleanChatNickname(name),
			Content:      content,
			StatExcluded: isChatStatsCommand(content),
			CreatedAt:    message.Time,
		})
	}
	if err := db.SaveGroupChatBackfill(rows); err != nil {
		logx.Warnf("[chatstats] save history group=%d: %v", ctx.GroupID(), err)
	}
}

func historyChatText(message bot.HistoryMessage) string {
	var parts []string
	for _, segment := range message.Message {
		if segment.Type == "text" {
			if text := strings.TrimSpace(segment.Data.Text); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, " ")
}

func sendChatWordCloud(ctx *bot.CommandContext, label string, start, end time.Time, userID int64) error {
	backfillRecentChat(ctx, start)
	summary, err := db.GetGroupChatSummary(ctx.GroupID(), userID, start, end)
	if err != nil {
		return ctx.Reply("读取群聊词云失败。")
	}
	wordRows, err := db.GetGroupChatTopWords(ctx.GroupID(), userID, start, end, 256)
	if err != nil {
		return ctx.Reply("读取群聊词云失败。")
	}
	analysis := chatAnalysisFromDatabase(label, summary, wordRows, nil)
	if analysis.TextTotal < 8 || len(analysis.Words) < 3 {
		return ctx.Reply("文字记录还不够生成词云，先聊一会儿再来看看吧。")
	}
	data, err := renderChatWordCloud(analysis)
	if err != nil {
		logx.Warnf("[chatstats] render: %v", err)
		return ctx.Reply("词云生成失败：" + err.Error())
	}
	return ctx.SendMsg(bot.Msg().ImageBytes(data).Build())
}

func chatAnalysisFromDatabase(label string, summary db.GroupChatSummary, words []db.GroupChatWordCount, users []db.GroupChatUserCount) chatAnalysis {
	analysis := chatAnalysis{
		Label:        label,
		Total:        summary.Total,
		TextTotal:    summary.TextTotal,
		Participants: summary.Participants,
		Words:        selectChatWords(words, summary.TextTotal, 32),
	}
	for _, user := range users {
		analysis.Users = append(analysis.Users, chatUserCount{
			UserID: user.UserID, Nickname: user.Nickname, Count: user.Count,
		})
	}
	return analysis
}

func formatChatLeaderboard(analysis chatAnalysis) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s废话榜 · %d 条消息 / %d 人\n", analysis.Label, analysis.Total, analysis.Participants)
	for i, user := range analysis.Users {
		if i >= 10 {
			break
		}
		fmt.Fprintf(&sb, "%d. %s · %d 条\n", i+1, displayChatName(user), user.Count)
	}
	return strings.TrimSpace(sb.String())
}

func selectChatWords(rows []db.GroupChatWordCount, messageCount, limit int) []chatWord {
	selected := db.SelectGroupChatWords(rows, messageCount, limit)
	words := make([]chatWord, 0, len(selected))
	for _, word := range selected {
		words = append(words, chatWord{Text: word.Text, Count: word.Count})
	}
	return words
}

func isChatStatsCommand(text string) bool {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "/") {
		text = strings.TrimSpace(strings.TrimPrefix(text, "/"))
	}
	for _, prefix := range []string{"词云", "今日词云", "昨日词云", "本周词云", "周词云", "我的词云", "废话榜", "今日废话榜", "昨日废话榜", "本周废话榜", "今日龙王", "本周龙王", "口头禅", "我的口头禅", "谁最爱说"} {
		if text == prefix || strings.HasPrefix(text, prefix+" ") {
			return true
		}
	}
	return false
}

func displayChatName(user chatUserCount) string {
	if user.Nickname != "" {
		return user.Nickname
	}
	return fmt.Sprintf("用户%d", user.UserID)
}

func cleanChatNickname(name string) string {
	return truncateChatText(strings.TrimSpace(name), 32)
}

func truncateChatText(text string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes])
}
