package db

import (
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GroupChatMessage keeps a long-term, group-scoped copy of chat text for local
// statistics and future member-profile analysis. Rows are not expired
// automatically; explicit maintenance tools may still remove them if needed.
type GroupChatMessage struct {
	ID           uint   `gorm:"primarykey;autoIncrement"`
	GroupID      int64  `gorm:"not null;uniqueIndex:idx_group_chat_message;index:idx_group_chat_time,priority:1;index:idx_group_chat_user_time,priority:1"`
	MessageID    int32  `gorm:"not null;uniqueIndex:idx_group_chat_message"`
	UserID       int64  `gorm:"not null;index:idx_group_chat_user_time,priority:2"`
	Nickname     string `gorm:"size:64"`
	Content      string `gorm:"type:text"`
	StatExcluded bool   `gorm:"not null;default:false"`
	CreatedAt    int64  `gorm:"not null;index:idx_group_chat_time,priority:2;index:idx_group_chat_user_time,priority:3"`
}

type GroupChatSummary struct {
	Total        int `gorm:"column:total" json:"total"`
	TextTotal    int `gorm:"column:text_total" json:"text_total"`
	Participants int `gorm:"column:participants" json:"participants"`
}

type GroupChatWordCount struct {
	Text  string `gorm:"column:text" json:"text"`
	Count int    `gorm:"column:count" json:"count"`
}

type GroupChatUserCount struct {
	UserID    int64  `gorm:"column:user_id" json:"user_id"`
	Nickname  string `gorm:"column:nickname" json:"nickname"`
	Count     int    `gorm:"column:count" json:"count"`
	TextTotal int    `gorm:"column:text_total" json:"-"`
}

type GroupChatPhraseCount struct {
	Text  string `gorm:"column:text" json:"text"`
	Count int    `gorm:"column:count" json:"count"`
}

type GroupChatHistoryStats struct {
	Total    int64 `gorm:"column:total" json:"total"`
	Matched  int64 `gorm:"column:matched" json:"matched"`
	OldestAt int64 `gorm:"column:oldest_at" json:"oldest_at"`
	NewestAt int64 `gorm:"column:newest_at" json:"newest_at"`
}

type groupChatHistoryWatermark struct {
	GroupID         int64 `gorm:"primaryKey"`
	DeletedBeforeAt int64 `gorm:"not null"`
	UpdatedAt       int64 `gorm:"not null"`
}

func (groupChatHistoryWatermark) TableName() string {
	return "group_chat_history_watermarks"
}

var groupChatStopWords = map[string]struct{}{
	"一个": {}, "一下": {}, "一些": {}, "已经": {}, "还是": {}, "不是": {}, "就是": {}, "但是": {},
	"因为": {}, "所以": {}, "然后": {}, "如果": {}, "而且": {}, "或者": {}, "可以": {}, "可能": {},
	"应该": {}, "感觉": {}, "觉得": {}, "这个": {}, "那个": {}, "这些": {}, "那些": {}, "这里": {},
	"那里": {}, "这么": {}, "那么": {}, "什么": {}, "怎么": {}, "为什么": {}, "我们": {}, "你们": {},
	"他们": {}, "自己": {}, "现在": {}, "时候": {}, "这样": {}, "知道": {}, "没有": {}, "还有": {},
	"然后呢": {}, "真的吗": {}, "是不是": {}, "怎么样": {}, "怎么办": {}, "有没有": {},
}

// SaveGroupChatMessages inserts messages idempotently. History backfills and
// live events can overlap, so the (group_id, message_id) key ignores duplicates.
func SaveGroupChatMessages(rows []GroupChatMessage) error {
	if len(rows) == 0 {
		return nil
	}
	return DB.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 200).Error
}

func SaveGroupChatMessage(row GroupChatMessage) error {
	return SaveGroupChatMessages([]GroupChatMessage{row})
}

func SaveGroupChatBackfill(rows []GroupChatMessage) error {
	if len(rows) == 0 {
		return nil
	}
	groupIDs := uniqueSortedGroupIDs(rows)
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := lockGroupChatHistory(tx, groupIDs); err != nil {
			return err
		}
		var watermarks []groupChatHistoryWatermark
		if err := tx.Where("group_id IN ?", groupIDs).Find(&watermarks).Error; err != nil {
			return err
		}
		cutoffs := make(map[int64]int64, len(watermarks))
		for _, watermark := range watermarks {
			cutoffs[watermark.GroupID] = watermark.DeletedBeforeAt
		}
		filtered := make([]GroupChatMessage, 0, len(rows))
		for _, row := range rows {
			if row.CreatedAt >= cutoffs[row.GroupID] {
				filtered = append(filtered, row)
			}
		}
		if len(filtered) == 0 {
			return nil
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(filtered, 200).Error
	})
}

func uniqueSortedGroupIDs(rows []GroupChatMessage) []int64 {
	seen := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		seen[row.GroupID] = struct{}{}
	}
	groupIDs := make([]int64, 0, len(seen))
	for groupID := range seen {
		groupIDs = append(groupIDs, groupID)
	}
	sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i] < groupIDs[j] })
	return groupIDs
}

func lockGroupChatHistory(tx *gorm.DB, groupIDs []int64) error {
	for _, groupID := range groupIDs {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", groupID).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetGroupChatMessages returns at most limit messages in chronological order.
// userID=0 includes the whole group.
func GetGroupChatMessages(groupID, userID int64, start, end time.Time, limit int) ([]GroupChatMessage, error) {
	if limit <= 0 || limit > 20_000 {
		limit = 20_000
	}
	query := DB.Where("group_id = ? AND created_at >= ? AND created_at < ?", groupID, start.Unix(), end.Unix())
	if userID != 0 {
		query = query.Where("user_id = ?", userID)
	}
	var rows []GroupChatMessage
	err := query.Order("created_at ASC, id ASC").Limit(limit).Find(&rows).Error
	return rows, err
}

func GetGroupChatHistoryStats(groupID, beforeAt int64) (GroupChatHistoryStats, error) {
	var stats GroupChatHistoryStats
	err := DB.Model(&GroupChatMessage{}).
		Where("group_id = ?", groupID).
		Select(`COUNT(*) AS total,
			COALESCE(MIN(created_at), 0) AS oldest_at,
			COALESCE(MAX(created_at), 0) AS newest_at`).
		Scan(&stats).Error
	if err != nil {
		return stats, err
	}
	if beforeAt <= 0 {
		stats.Matched = stats.Total
		return stats, nil
	}
	return stats, DB.Model(&GroupChatMessage{}).
		Where("group_id = ? AND created_at < ?", groupID, beforeAt).
		Count(&stats.Matched).Error
}

func DeleteGroupChatMessagesForGroupBefore(groupID, beforeAt int64) (int64, error) {
	return deleteGroupChatMessages(groupID, beforeAt, false, time.Now())
}

func DeleteAllGroupChatMessagesForGroup(groupID int64, deletedAt time.Time) (int64, error) {
	return deleteGroupChatMessages(groupID, deletedAt.Unix()+1, true, deletedAt)
}

func deleteGroupChatMessages(groupID, watermark int64, all bool, updatedAt time.Time) (int64, error) {
	var deleted int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockGroupChatHistory(tx, []int64{groupID}); err != nil {
			return err
		}
		if err := tx.Exec(`
			INSERT INTO group_chat_history_watermarks (group_id, deleted_before_at, updated_at)
			VALUES (?, ?, ?)
			ON CONFLICT (group_id) DO UPDATE SET
				deleted_before_at = GREATEST(group_chat_history_watermarks.deleted_before_at, EXCLUDED.deleted_before_at),
				updated_at = EXCLUDED.updated_at`, groupID, watermark, updatedAt.Unix()).Error; err != nil {
			return err
		}
		query := tx.Where("group_id = ?", groupID)
		if !all {
			query = query.Where("created_at < ?", watermark)
		}
		result := query.Delete(&GroupChatMessage{})
		deleted = result.RowsAffected
		return result.Error
	})
	return deleted, err
}

func groupChatRange(groupID, userID int64, start, end time.Time) *gorm.DB {
	query := DB.Model(&GroupChatMessage{}).
		Where("group_id = ? AND created_at >= ? AND created_at < ? AND stat_excluded = false", groupID, start.Unix(), end.Unix())
	if userID != 0 {
		query = query.Where("user_id = ?", userID)
	}
	return query
}

func GetGroupChatSummary(groupID, userID int64, start, end time.Time) (GroupChatSummary, error) {
	var summary GroupChatSummary
	err := groupChatRange(groupID, userID, start, end).
		Select(`COUNT(*) AS total,
			COUNT(*) FILTER (WHERE TRIM(content) <> '') AS text_total,
			COUNT(DISTINCT user_id) AS participants`).
		Scan(&summary).Error
	return summary, err
}

// GetGroupChatTopWords counts one occurrence per message. search_vector is
// generated by PostgreSQL with zhparser, so no message text is segmented in Go.
func GetGroupChatTopWords(groupID, userID int64, start, end time.Time, limit int) ([]GroupChatWordCount, error) {
	if limit <= 0 || limit > 512 {
		limit = 256
	}
	var rows []GroupChatWordCount
	query := `
		SELECT lexeme AS text, COUNT(*)::integer AS count
		FROM group_chat_messages AS message
		CROSS JOIN LATERAL unnest(tsvector_to_array(message.search_vector)) AS lexeme
		WHERE message.group_id = ?
		  AND message.created_at >= ? AND message.created_at < ?
		  AND message.stat_excluded = false
		  AND char_length(lexeme) >= 2`
	args := []any{groupID, start.Unix(), end.Unix()}
	if userID != 0 {
		query += " AND message.user_id = ?"
		args = append(args, userID)
	}
	query += `
		GROUP BY lexeme
		ORDER BY count DESC, char_length(lexeme) DESC, lexeme ASC
		LIMIT ?`
	args = append(args, limit)
	err := DB.Raw(query, args...).Scan(&rows).Error
	return rows, err
}

// SelectGroupChatWords ranks zhparser lexemes and removes generic or redundant
// terms. It does not segment text in Go; production tokenisation remains fully
// delegated to PostgreSQL's generated search_vector.
func SelectGroupChatWords(rows []GroupChatWordCount, messageCount, limit int) []GroupChatWordCount {
	if limit <= 0 {
		return nil
	}
	minCount := 1
	if messageCount >= 12 {
		minCount = 2
	}
	words := make([]GroupChatWordCount, 0, len(rows))
	for _, row := range rows {
		text := strings.ToLower(strings.TrimSpace(row.Text))
		if row.Count < minCount {
			continue
		}
		if _, stopped := groupChatStopWords[text]; !stopped {
			words = append(words, GroupChatWordCount{Text: text, Count: row.Count})
		}
	}
	sort.Slice(words, func(i, j int) bool {
		si := float64(words[i].Count) * (1 + 0.12*float64(utf8.RuneCountInString(words[i].Text)-2))
		sj := float64(words[j].Count) * (1 + 0.12*float64(utf8.RuneCountInString(words[j].Text)-2))
		if si != sj {
			return si > sj
		}
		if utf8.RuneCountInString(words[i].Text) != utf8.RuneCountInString(words[j].Text) {
			return utf8.RuneCountInString(words[i].Text) > utf8.RuneCountInString(words[j].Text)
		}
		return words[i].Text < words[j].Text
	})

	selected := make([]GroupChatWordCount, 0, limit)
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

// GetGroupChatTopPhrases finds exact short messages that recur in a group or
// for one member. Unlike top words, these are real message snippets such as a
// catchphrase or a frequently repeated reaction.
func GetGroupChatTopPhrases(groupID, userID int64, start, end time.Time, limit int) ([]GroupChatPhraseCount, error) {
	if limit <= 0 || limit > 20 {
		limit = 6
	}
	query := groupChatRange(groupID, userID, start, end).
		Where("TRIM(content) <> '' AND LENGTH(TRIM(content)) BETWEEN 2 AND 48").
		Where("TRIM(content) NOT LIKE 'http%'")
	var rows []GroupChatPhraseCount
	err := query.
		Select("TRIM(content) AS text, COUNT(*) AS count").
		Group("TRIM(content)").
		Having("COUNT(*) >= 2").
		Order("count DESC, LENGTH(TRIM(content)) DESC, text ASC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

type groupChatUserTextCount struct {
	UserID int64  `gorm:"column:user_id"`
	Text   string `gorm:"column:text"`
	Count  int    `gorm:"column:count"`
}

// GetGroupChatTopWordsForUsers returns each requested member's leading
// lexemes with one database query. Empty results are represented as empty
// slices so callers can serialize them consistently.
func GetGroupChatTopWordsForUsers(groupID int64, userIDs []int64, start, end time.Time, limit int) (map[int64][]GroupChatWordCount, error) {
	result := make(map[int64][]GroupChatWordCount, len(userIDs))
	for _, userID := range userIDs {
		result[userID] = []GroupChatWordCount{}
	}
	if len(userIDs) == 0 {
		return result, nil
	}
	if limit <= 0 || limit > 512 {
		limit = 96
	}

	var rows []groupChatUserTextCount
	err := DB.Raw(`
		WITH word_counts AS (
			SELECT message.user_id, lexeme AS text, COUNT(*)::integer AS count
			FROM group_chat_messages AS message
			CROSS JOIN LATERAL unnest(tsvector_to_array(message.search_vector)) AS lexeme
			WHERE message.group_id = ?
			  AND message.created_at >= ? AND message.created_at < ?
			  AND message.stat_excluded = false
			  AND message.user_id IN ?
			  AND char_length(lexeme) >= 2
			GROUP BY message.user_id, lexeme
		), ranked_words AS (
			SELECT user_id, text, count,
				ROW_NUMBER() OVER (
					PARTITION BY user_id
					ORDER BY count DESC, char_length(text) DESC, text ASC
				) AS word_rank
			FROM word_counts
		)
		SELECT user_id, text, count
		FROM ranked_words
		WHERE word_rank <= ?
		ORDER BY user_id, word_rank`, groupID, start.Unix(), end.Unix(), userIDs, limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.UserID] = append(result[row.UserID], GroupChatWordCount{Text: row.Text, Count: row.Count})
	}
	return result, nil
}

func GetGroupChatTopPhrasesForUsers(groupID int64, userIDs []int64, start, end time.Time, limit int) (map[int64][]GroupChatPhraseCount, error) {
	result := make(map[int64][]GroupChatPhraseCount, len(userIDs))
	for _, userID := range userIDs {
		result[userID] = []GroupChatPhraseCount{}
	}
	if len(userIDs) == 0 {
		return result, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 4
	}

	var rows []groupChatUserTextCount
	err := DB.Raw(`
		WITH phrase_counts AS (
			SELECT user_id, TRIM(content) AS text, COUNT(*)::integer AS count
			FROM group_chat_messages
			WHERE group_id = ?
			  AND created_at >= ? AND created_at < ?
			  AND stat_excluded = false
			  AND user_id IN ?
			  AND TRIM(content) <> ''
			  AND LENGTH(TRIM(content)) BETWEEN 2 AND 48
			  AND TRIM(content) NOT LIKE 'http%'
			GROUP BY user_id, TRIM(content)
			HAVING COUNT(*) >= 2
		), ranked_phrases AS (
			SELECT user_id, text, count,
				ROW_NUMBER() OVER (
					PARTITION BY user_id
					ORDER BY count DESC, LENGTH(text) DESC, text ASC
				) AS phrase_rank
			FROM phrase_counts
		)
		SELECT user_id, text, count
		FROM ranked_phrases
		WHERE phrase_rank <= ?
		ORDER BY user_id, phrase_rank`, groupID, start.Unix(), end.Unix(), userIDs, limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.UserID] = append(result[row.UserID], GroupChatPhraseCount{Text: row.Text, Count: row.Count})
	}
	return result, nil
}

func GetGroupChatUserCounts(groupID int64, start, end time.Time, limit int) ([]GroupChatUserCount, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	var rows []GroupChatUserCount
	err := DB.Raw(`
			SELECT user_id,
			       COALESCE((array_remove(array_agg(NULLIF(nickname, '') ORDER BY created_at DESC, id DESC), NULL))[1], '') AS nickname,
			       COUNT(*)::integer AS count,
			       COUNT(*) FILTER (WHERE TRIM(content) <> '')::integer AS text_total
		FROM group_chat_messages
		WHERE group_id = ? AND created_at >= ? AND created_at < ? AND stat_excluded = false
		GROUP BY user_id
		ORDER BY count DESC, user_id ASC
		LIMIT ?`, groupID, start.Unix(), end.Unix(), limit).Scan(&rows).Error
	return rows, err
}

func FindGroupChatUsersSaying(groupID int64, start, end time.Time, keyword string, limit int) ([]GroupChatUserCount, error) {
	if limit <= 0 || limit > 100 {
		limit = 8
	}
	pattern := "%" + escapeLike(keyword) + "%"
	var rows []GroupChatUserCount
	err := DB.Raw(`
		SELECT user_id,
		       COALESCE((array_remove(array_agg(NULLIF(nickname, '') ORDER BY created_at DESC, id DESC), NULL))[1], '') AS nickname,
		       COUNT(*)::integer AS count
		FROM group_chat_messages
		WHERE group_id = ? AND created_at >= ? AND created_at < ?
		  AND stat_excluded = false AND content ILIKE ? ESCAPE '\'
		GROUP BY user_id
		ORDER BY count DESC, user_id ASC
		LIMIT ?`, groupID, start.Unix(), end.Unix(), pattern, limit).Scan(&rows).Error
	return rows, err
}

func GetLatestGroupChatNickname(groupID, userID int64, start, end time.Time) (string, error) {
	var nickname string
	err := groupChatRange(groupID, userID, start, end).
		Where("nickname <> ''").Order("created_at DESC, id DESC").Limit(1).
		Pluck("nickname", &nickname).Error
	return nickname, err
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}
