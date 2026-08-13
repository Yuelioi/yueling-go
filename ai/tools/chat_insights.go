package tools

import (
	"fmt"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
)

func init() {
	ai.Register(ai.ToolMeta{
		Name:        "query_chat_insights",
		Description: "查询当前群已落库聊天的消息量、活跃成员、高频词、个人口头禅或谁最爱说某个词，不生成图片",
		Tags:        []string{"群聊统计", "日常"},
		Triggers:    []string{"谁话最多", "谁最活跃", "废话榜", "口头禅", "谁最爱说", "群里最近聊什么", "高频词"},
		Patterns:    []string{`(今天|昨天|本周|最近).{0,8}(聊了|说了|活跃|消息)`, `谁.{0,5}(说得多|话最多)`},
		Slots:       []string{"群聊数据", "聊天统计", "活跃榜", "关键词统计"},
		PluginID:    catalog.PluginChatStats,
		Params: []ai.Param{
			{Name: "action", Type: "string", Description: "查询类型", Required: true, Enum: []string{"summary", "leaderboard", "top_words", "user_phrases", "phrase_users"}},
			{Name: "period", Type: "string", Description: "时间范围", Required: false, Enum: []string{"today", "yesterday", "week", "7days"}},
			{Name: "user_id", Type: "integer", Description: "user_phrases 的用户ID；省略时查自己", Required: false},
			{Name: "keyword", Type: "string", Description: "phrase_users 要统计的词，最多24字", Required: false},
		},
		Handler: queryChatInsights,
	})
}

func queryChatInsights(ctx *ai.ToolContext) (string, error) {
	label, start, end := chatInsightRange(ctx.String("period"), bot.Now())
	summary, err := db.GetGroupChatSummary(ctx.GroupID(), 0, start, end)
	if err != nil {
		return "读取群聊统计失败", nil
	}
	if summary.Total == 0 {
		return label + "还没有可统计的群聊记录", nil
	}
	switch ctx.String("action") {
	case "summary":
		return fmt.Sprintf("%s共 %d 条消息，其中 %d 条文字消息，%d 人参与", label, summary.Total, summary.TextTotal, summary.Participants), nil
	case "leaderboard":
		rows, err := db.GetGroupChatUserCounts(ctx.GroupID(), start, end, 10)
		if err != nil {
			return "读取活跃榜失败", nil
		}
		lines := make([]string, 0, len(rows))
		for i, row := range rows {
			lines = append(lines, fmt.Sprintf("%d. %s · %d 条", i+1, chatInsightName(row), row.Count))
		}
		return label + "活跃榜：\n" + strings.Join(lines, "\n"), nil
	case "top_words":
		rows, err := db.GetGroupChatTopWords(ctx.GroupID(), 0, start, end, 20)
		if err != nil {
			return "读取高频词失败", nil
		}
		return formatInsightWords(label+"高频词", rows), nil
	case "user_phrases":
		userID := ctx.Int("user_id")
		if userID == 0 {
			userID = ctx.UserID()
		}
		rows, err := db.GetGroupChatTopWords(ctx.GroupID(), userID, start, end, 12)
		if err != nil {
			return "读取口头禅失败", nil
		}
		name, _ := db.GetLatestGroupChatNickname(ctx.GroupID(), userID, start, end)
		if name == "" {
			name = fmt.Sprintf("用户%d", userID)
		}
		return formatInsightWords(name+label+"的口头禅", rows), nil
	case "phrase_users":
		keyword := strings.TrimSpace(ctx.String("keyword"))
		if keyword == "" || utf8Len(keyword) > 24 {
			return "关键词不能为空且最多24个字", nil
		}
		rows, err := db.FindGroupChatUsersSaying(ctx.GroupID(), start, end, keyword, 8)
		if err != nil {
			return "读取关键词统计失败", nil
		}
		if len(rows) == 0 {
			return label + "没人说过「" + keyword + "」", nil
		}
		lines := make([]string, 0, len(rows))
		for i, row := range rows {
			lines = append(lines, fmt.Sprintf("%d. %s · %d 条", i+1, chatInsightName(row), row.Count))
		}
		return label + "最爱说「" + keyword + "」：\n" + strings.Join(lines, "\n"), nil
	}
	return "未知查询类型", nil
}

func chatInsightRange(period string, now time.Time) (string, time.Time, time.Time) {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch period {
	case "yesterday":
		return "昨天", day.AddDate(0, 0, -1), day
	case "week":
		daysSinceMonday := (int(now.Weekday()) + 6) % 7
		return "本周", day.AddDate(0, 0, -daysSinceMonday), now.Add(time.Second)
	case "7days":
		return "最近七天", day.AddDate(0, 0, -6), now.Add(time.Second)
	default:
		return "今天", day, now.Add(time.Second)
	}
}

func chatInsightName(row db.GroupChatUserCount) string {
	if strings.TrimSpace(row.Nickname) != "" {
		return row.Nickname
	}
	return fmt.Sprintf("用户%d", row.UserID)
}

func formatInsightWords(title string, rows []db.GroupChatWordCount) string {
	if len(rows) == 0 {
		return title + "：文字记录还不够"
	}
	lines := make([]string, 0, len(rows))
	for i, row := range rows {
		lines = append(lines, fmt.Sprintf("%d. %s · %d 条", i+1, row.Text, row.Count))
	}
	return title + "：\n" + strings.Join(lines, "\n")
}
