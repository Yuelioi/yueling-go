package db

import (
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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
		if userID, err := strconv.ParseInt(query, 10, 64); err == nil {
			q = q.Where("user_id = ?", userID)
		} else {
			q = q.Where("nickname LIKE ?", "%"+query+"%")
		}
	}
	var rows []AIAffinity
	err := q.Order("updated_at desc").Limit(limit).Find(&rows).Error
	return rows, err
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
	var current AIAffinity
	if err := DB.Where("id = ?", id).First(&current).Error; err != nil {
		return nil, err
	}
	return SetAIAffinityScore(id, current.Score+delta, minScore, maxScore, reason)
}

func ResetAIAffinityScore(id uint, initial, minScore, maxScore int, reason string) (*AIAffinity, error) {
	return SetAIAffinityScore(id, initial, minScore, maxScore, reason)
}
