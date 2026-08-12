package funny

import (
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/services/logx"
)

const (
	chatStatsRetention = 35 * 24 * time.Hour
	chatStatsBackfill  = 500
)

var chatStatsLastCleanup atomic.Int64

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

// RegisterChatStats records a short rolling window of group chat and exposes
// chat-native statistics. It performs no AI calls and is fully group-scoped.
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
			label, start, end := chatStatsPeriod(ctx.Cmd, bot.Now())
			return sendChatWordCloud(ctx, label, start, end, 0)
		})

	b.OnCommand("我的词云").
		Plugin(catalog.PluginChatStats).
		Handle(func(ctx *bot.CommandContext) error {
			_, start, end := chatStatsPeriod("今日词云", bot.Now())
			return sendChatWordCloud(ctx, "我的今日", start, end, ctx.UserID())
		})

	b.OnCommand("废话榜", "今日废话榜", "昨日废话榜", "本周废话榜", "今日龙王", "本周龙王").
		Plugin(catalog.PluginChatStats).
		Handle(func(ctx *bot.CommandContext) error {
			label, start, end := chatStatsPeriod(ctx.Cmd, bot.Now())
			backfillRecentChat(ctx, start)
			summary, err := db.GetGroupChatSummary(ctx.GroupID(), 0, start, end)
			if err != nil {
				return ctx.Reply("读取群聊统计失败。")
			}
			if summary.Total == 0 {
				return ctx.Reply("还没有可统计的聊天记录，先聊一会儿再来看看吧。")
			}
			users, err := db.GetGroupChatUserCounts(ctx.GroupID(), start, end, 10)
			if err != nil {
				return ctx.Reply("读取群聊统计失败。")
			}
			analysis := chatAnalysisFromDatabase(label, summary, nil, users)
			return ctx.Reply(formatChatLeaderboard(analysis))
		})

	b.OnCommand("口头禅", "我的口头禅").
		Plugin(catalog.PluginChatStats).
		Handle(func(ctx *bot.CommandContext) error {
			now := bot.Now()
			start := dayStart(now).AddDate(0, 0, -6)
			target := ctx.UserID()
			if targets := ctx.Message().AtTargets(); len(targets) > 0 {
				fmt.Sscan(targets[0], &target)
			}
			backfillRecentChat(ctx, start)
			end := now.Add(time.Second)
			wordRows, err := db.GetGroupChatTopWords(ctx.GroupID(), target, start, end, 256)
			if err != nil {
				return ctx.Reply("读取口头禅失败。")
			}
			summary, err := db.GetGroupChatSummary(ctx.GroupID(), target, start, end)
			if err != nil {
				return ctx.Reply("读取口头禅失败。")
			}
			words := selectChatWords(wordRows, summary.TextTotal, 8)
			if len(words) == 0 {
				return ctx.Reply("最近七天还没有足够的文字记录来分析口头禅。")
			}
			name, err := db.GetLatestGroupChatNickname(ctx.GroupID(), target, start, end)
			if err != nil || name == "" {
				name = fmt.Sprintf("用户%d", target)
			}
			var sb strings.Builder
			fmt.Fprintf(&sb, "%s最近七天的口头禅\n", name)
			for i, word := range words {
				fmt.Fprintf(&sb, "%d. %s（%d 条消息）\n", i+1, word.Text, word.Count)
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
			now := bot.Now()
			start := dayStart(now).AddDate(0, 0, -6)
			backfillRecentChat(ctx, start)
			rows, err := db.FindGroupChatUsersSaying(ctx.GroupID(), start, now.Add(time.Second), keyword, 8)
			if err != nil {
				return ctx.Reply("读取群聊统计失败。")
			}
			if len(rows) == 0 {
				return ctx.Reply("最近七天没人说过「" + keyword + "」。")
			}
			var sb strings.Builder
			fmt.Fprintf(&sb, "最近七天谁最爱说「%s」\n", keyword)
			for i, user := range rows {
				fmt.Fprintf(&sb, "%d. %s · %d 条\n", i+1, displayChatName(chatUserCount(user)), user.Count)
			}
			return ctx.Reply(strings.TrimSpace(sb.String()))
		})
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
	last := chatStatsLastCleanup.Load()
	if now.Unix()-last >= int64(24*time.Hour/time.Second) && chatStatsLastCleanup.CompareAndSwap(last, now.Unix()) {
		go func() {
			if err := db.DeleteGroupChatMessagesBefore(now.Add(-chatStatsRetention)); err != nil {
				logx.Warnf("[chatstats] retention cleanup: %v", err)
			}
		}()
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
	if err := db.SaveGroupChatMessages(rows); err != nil {
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

func dayStart(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func chatStatsPeriod(command string, now time.Time) (string, time.Time, time.Time) {
	today := dayStart(now)
	switch command {
	case "昨日词云", "昨日废话榜":
		return "昨日", today.AddDate(0, 0, -1), today
	case "本周词云", "周词云", "本周废话榜", "本周龙王":
		daysSinceMonday := (int(now.Weekday()) + 6) % 7
		return "本周", today.AddDate(0, 0, -daysSinceMonday), today.AddDate(0, 0, 1)
	default:
		return "今日", today, today.AddDate(0, 0, 1)
	}
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
		analysis.Users = append(analysis.Users, chatUserCount(user))
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

var chatStopWords = map[string]struct{}{
	"一个": {}, "一下": {}, "一些": {}, "已经": {}, "还是": {}, "不是": {}, "就是": {}, "但是": {},
	"因为": {}, "所以": {}, "然后": {}, "如果": {}, "而且": {}, "或者": {}, "可以": {}, "可能": {},
	"应该": {}, "感觉": {}, "觉得": {}, "这个": {}, "那个": {}, "这些": {}, "那些": {}, "这里": {},
	"那里": {}, "这么": {}, "那么": {}, "什么": {}, "怎么": {}, "为什么": {}, "我们": {}, "你们": {},
	"他们": {}, "自己": {}, "现在": {}, "时候": {}, "这样": {}, "知道": {}, "没有": {}, "还有": {},
	"然后呢": {}, "真的吗": {}, "是不是": {}, "怎么样": {}, "怎么办": {}, "有没有": {},
}

func selectChatWords(rows []db.GroupChatWordCount, messageCount, limit int) []chatWord {
	minCount := 1
	if messageCount >= 12 {
		minCount = 2
	}
	words := make([]chatWord, 0, len(rows))
	for _, row := range rows {
		text := strings.ToLower(strings.TrimSpace(row.Text))
		if row.Count < minCount {
			continue
		}
		if _, stopped := chatStopWords[text]; !stopped {
			words = append(words, chatWord{Text: text, Count: row.Count})
		}
	}
	sort.Slice(words, func(i, j int) bool {
		si := float64(words[i].Count) * (1 + 0.12*float64(utf8.RuneCountInString(words[i].Text)-2))
		sj := float64(words[j].Count) * (1 + 0.12*float64(utf8.RuneCountInString(words[j].Text)-2))
		if si != sj {
			return si > sj
		}
		if len([]rune(words[i].Text)) != len([]rune(words[j].Text)) {
			return len([]rune(words[i].Text)) > len([]rune(words[j].Text))
		}
		return words[i].Text < words[j].Text
	})

	selected := make([]chatWord, 0, limit)
	for _, candidate := range words {
		redundant := false
		for _, existing := range selected {
			if strings.Contains(existing.Text, candidate.Text) && existing.Count*10 >= candidate.Count*7 {
				redundant = true
				break
			}
		}
		if redundant {
			continue
		}
		selected = append(selected, candidate)
		if len(selected) >= limit {
			break
		}
	}
	return selected
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
