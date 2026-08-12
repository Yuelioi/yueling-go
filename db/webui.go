package db

import (
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WebUIOverview struct {
	AffinityCount    int64 `json:"affinity_count"`
	LowAffinityCount int64 `json:"low_affinity_count"`
	MemoryCount      int64 `json:"memory_count"`
	MemoryUserCount  int64 `json:"memory_user_count"`
	DigestCount      int64 `json:"digest_count"`
	FeedCount        int64 `json:"feed_count"`
	KnowledgeCount   int64 `json:"knowledge_count"`
}

func GetWebUIOverview(blockBelow int) (*WebUIOverview, error) {
	var overview WebUIOverview
	queries := []struct {
		target *int64
		query  *gorm.DB
	}{
		{&overview.AffinityCount, DB.Model(&AIAffinity{})},
		{&overview.LowAffinityCount, DB.Model(&AIAffinity{}).Where("score < ?", blockBelow)},
		{&overview.MemoryCount, DB.Model(&SemanticMemory{})},
		{&overview.MemoryUserCount, DB.Model(&SemanticMemory{}).Distinct("user_id")},
		{&overview.DigestCount, DB.Model(&DailyDigest{}).Where("enabled = ?", true)},
		{&overview.FeedCount, DB.Model(&FeedSubscription{}).Where("enabled = ?", true)},
		{&overview.KnowledgeCount, DB.Model(&GroupKnowledge{})},
	}
	for _, item := range queries {
		if err := item.query.Count(item.target).Error; err != nil {
			return nil, err
		}
	}
	return &overview, nil
}

func IsGroupPluginDisabled(groupID int64, pluginID int) (bool, error) {
	var count int64
	err := DB.Model(&GroupPluginDisabled{}).
		Where("group_id = ? AND plugin_id = ?", groupID, pluginID).
		Count(&count).Error
	return count > 0, err
}

func GetDisabledPlugins(groupID int64) (map[int]bool, error) {
	var rows []GroupPluginDisabled
	if err := DB.Where("group_id = ?", groupID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[int]bool, len(rows))
	for _, row := range rows {
		out[row.PluginID] = true
	}
	return out, nil
}

func SetGroupPluginDisabled(groupID int64, pluginID int, disabled bool) error {
	if disabled {
		return DB.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&GroupPluginDisabled{GroupID: groupID, PluginID: pluginID}).Error
	}
	return DB.Where("group_id = ? AND plugin_id = ?", groupID, pluginID).
		Delete(&GroupPluginDisabled{}).Error
}

func SetPluginDisabledForGroups(pluginID int, groupIDs []int64, disabled bool) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		for _, groupID := range groupIDs {
			if disabled {
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
					Create(&GroupPluginDisabled{GroupID: groupID, PluginID: pluginID}).Error; err != nil {
					return err
				}
				continue
			}
			if err := tx.Where("group_id = ? AND plugin_id = ?", groupID, pluginID).
				Delete(&GroupPluginDisabled{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func ListAIAffinityAdmin(groupID int64, query string, limit int) ([]AIAffinity, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := DB.Model(&AIAffinity{})
	if groupID != 0 {
		q = q.Where("group_id = ?", groupID)
	}
	query = strings.TrimSpace(query)
	if query != "" {
		nicknameLike := "%" + query + "%"
		if userID, err := strconv.ParseInt(query, 10, 64); err == nil {
			q = q.Where("user_id = ? OR nickname LIKE ?", userID, nicknameLike)
		} else {
			q = q.Where("nickname LIKE ?", nicknameLike)
		}
	}
	var rows []AIAffinity
	err := q.Order("updated_at desc").Limit(limit).Find(&rows).Error
	return rows, err
}

func ListSemanticMemoriesAdmin(query string, limit int) ([]SemanticMemory, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	q := DB.Model(&SemanticMemory{})
	query = strings.TrimSpace(query)
	if query != "" {
		like := "%" + query + "%"
		if userID, err := strconv.ParseInt(query, 10, 64); err == nil {
			q = q.Where("user_id = ? OR content LIKE ? OR category LIKE ?", userID, like, like)
		} else {
			q = q.Where("content LIKE ? OR category LIKE ?", like, like)
		}
	}
	var rows []SemanticMemory
	err := q.Order("created_at desc, id desc").Limit(limit).Find(&rows).Error
	return rows, err
}

func GetSemanticMemoryAdmin(id uint) (*SemanticMemory, error) {
	var row SemanticMemory
	if err := DB.Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func clampScore(score, minScore, maxScore int) int {
	if maxScore < minScore {
		maxScore = minScore
	}
	if score < minScore {
		return minScore
	}
	if score > maxScore {
		return maxScore
	}
	return score
}

func SetAIAffinityScore(id uint, score, minScore, maxScore int, reason string) (*AIAffinity, error) {
	score = clampScore(score, minScore, maxScore)
	now := time.Now().Unix()
	var row AIAffinity
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&AIAffinity{}).Where("id = ?", id).Updates(map[string]any{
			"score":       score,
			"last_reason": reason,
			"updated_at":  now,
		}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).First(&row).Error
	})
	return &row, err
}

func AdjustAIAffinityScore(id uint, delta, minScore, maxScore int, reason string) (*AIAffinity, error) {
	if maxScore < minScore {
		maxScore = minScore
	}
	now := time.Now().Unix()
	var row AIAffinity
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&AIAffinity{}).Where("id = ?", id).Updates(map[string]any{
			"score":       clampedScoreExpr(tx, maxScore, minScore, delta),
			"last_reason": reason,
			"updated_at":  now,
		}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).First(&row).Error
	})
	return &row, err
}

func ResetAIAffinityScore(id uint, initial, minScore, maxScore int, reason string) (*AIAffinity, error) {
	return SetAIAffinityScore(id, initial, minScore, maxScore, reason)
}
