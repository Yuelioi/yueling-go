package db

import (
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

func (s *AffinityStore) ListAIAffinityAdmin(groupID int64, query string, limit int) ([]AIAffinity, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := s.database.Model(&AIAffinity{})
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

func (s *AffinityStore) SetAIAffinityScore(id uint, score, minScore, maxScore int, reason string) (*AIAffinity, error) {
	score = clampScore(score, minScore, maxScore)
	now := time.Now().Unix()
	var row AIAffinity
	err := s.database.Transaction(func(tx *gorm.DB) error {
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

func (s *AffinityStore) AdjustAIAffinityScore(id uint, delta, minScore, maxScore int, reason string) (*AIAffinity, error) {
	if maxScore < minScore {
		maxScore = minScore
	}
	now := time.Now().Unix()
	var row AIAffinity
	err := s.database.Transaction(func(tx *gorm.DB) error {
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

func (s *AffinityStore) ResetAIAffinityScore(id uint, initial, minScore, maxScore int, reason string) (*AIAffinity, error) {
	return s.SetAIAffinityScore(id, initial, minScore, maxScore, reason)
}
