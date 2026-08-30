package chatinsights

import (
	"fmt"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/db"
)

type Period string

const (
	PeriodToday      Period = "today"
	PeriodYesterday  Period = "yesterday"
	PeriodWeek       Period = "week"
	PeriodSevenDays  Period = "7days"
	PeriodThirtyDays Period = "30days"
)

type Window struct {
	Period Period
	Label  string
	Start  time.Time
	End    time.Time
}

func ResolvePeriod(value string, now time.Time) (Window, bool) {
	period := Period(strings.TrimSpace(value))
	if period == "" {
		period = PeriodToday
	}
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	window := Window{Period: period, End: now.Add(time.Second)}
	switch period {
	case PeriodToday:
		window.Label = "今日"
		window.Start = day
		window.End = day.AddDate(0, 0, 1)
	case PeriodYesterday:
		window.Label = "昨日"
		window.Start = day.AddDate(0, 0, -1)
		window.End = day
	case PeriodWeek:
		window.Label = "本周"
		daysSinceMonday := (int(now.Weekday()) + 6) % 7
		window.Start = day.AddDate(0, 0, -daysSinceMonday)
		window.End = day.AddDate(0, 0, 1)
	case PeriodSevenDays:
		window.Label = "近 7 天"
		window.Start = now.AddDate(0, 0, -7)
	case PeriodThirtyDays:
		window.Label = "近 30 天"
		window.Start = now.AddDate(0, 0, -30)
	default:
		return Window{}, false
	}
	return window, true
}

type User struct {
	UserID   int64                     `json:"user_id"`
	Nickname string                    `json:"nickname"`
	Count    int                       `json:"count"`
	Phrases  []db.GroupChatPhraseCount `json:"phrases"`
	Words    []db.GroupChatWordCount   `json:"words"`
}

type Group struct {
	Summary db.GroupChatSummary
	Words   []db.GroupChatWordCount
	Users   []User
}

func QueryGroup(groupID int64, window Window) (Group, error) {
	result := Group{Words: []db.GroupChatWordCount{}, Users: []User{}}
	summary, err := db.GetGroupChatSummary(groupID, 0, window.Start, window.End)
	if err != nil {
		return result, fmt.Errorf("summary: %w", err)
	}
	result.Summary = summary
	if summary.Total == 0 {
		return result, nil
	}

	rawWords, err := db.GetGroupChatTopWords(groupID, 0, window.Start, window.End, 256)
	if err != nil {
		return result, fmt.Errorf("group words: %w", err)
	}
	result.Words = db.SelectGroupChatWords(rawWords, summary.TextTotal, 36)

	userRows, err := db.GetGroupChatUserCounts(groupID, window.Start, window.End, 8)
	if err != nil {
		return result, fmt.Errorf("active users: %w", err)
	}
	userIDs := make([]int64, 0, len(userRows))
	for _, row := range userRows {
		userIDs = append(userIDs, row.UserID)
	}
	phrasesByUser, err := db.GetGroupChatTopPhrasesForUsers(groupID, userIDs, window.Start, window.End, 4)
	if err != nil {
		return result, fmt.Errorf("user phrases: %w", err)
	}
	wordsByUser, err := db.GetGroupChatTopWordsForUsers(groupID, userIDs, window.Start, window.End, 96)
	if err != nil {
		return result, fmt.Errorf("user words: %w", err)
	}
	result.Users = make([]User, 0, len(userRows))
	for _, row := range userRows {
		result.Users = append(result.Users, User{
			UserID: row.UserID, Nickname: row.Nickname, Count: row.Count,
			Phrases: phrasesByUser[row.UserID],
			Words:   db.SelectGroupChatWords(wordsByUser[row.UserID], row.TextTotal, 5),
		})
	}
	return result, nil
}

type UserExpressions struct {
	Nickname string
	Phrases  []db.GroupChatPhraseCount
	Words    []db.GroupChatWordCount
}

func QueryUserExpressions(groupID, userID int64, window Window) (UserExpressions, error) {
	var result UserExpressions
	phrases, err := db.GetGroupChatTopPhrases(groupID, userID, window.Start, window.End, 5)
	if err != nil {
		return result, fmt.Errorf("phrases: %w", err)
	}
	rawWords, err := db.GetGroupChatTopWords(groupID, userID, window.Start, window.End, 256)
	if err != nil {
		return result, fmt.Errorf("words: %w", err)
	}
	summary, err := db.GetGroupChatSummary(groupID, userID, window.Start, window.End)
	if err != nil {
		return result, fmt.Errorf("summary: %w", err)
	}
	nickname, err := db.GetLatestGroupChatNickname(groupID, userID, window.Start, window.End)
	if err != nil {
		return result, fmt.Errorf("nickname: %w", err)
	}
	result.Nickname = nickname
	result.Phrases = phrases
	result.Words = db.SelectGroupChatWords(rawWords, summary.TextTotal, 8)
	return result, nil
}
