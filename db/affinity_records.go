package db

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AIAffinity struct {
	ID         uint   `gorm:"primarykey;autoIncrement"`
	UserID     int64  `gorm:"uniqueIndex:idx_ai_affinity"`
	GroupID    int64  `gorm:"uniqueIndex:idx_ai_affinity"`
	Nickname   string `gorm:"size:64"`
	Score      int    `gorm:"default:50"`
	LastReason string `gorm:"size:64"`
	UpdatedAt  int64
}

func (s *AffinityStore) GetAIAffinity(userID, groupID int64) (*AIAffinity, error) {
	var row AIAffinity
	err := s.database.Where("user_id = ? AND group_id = ?", userID, groupID).First(&row).Error
	return &row, err
}

func (s *AffinityStore) UpdateAIAffinity(userID, groupID int64, nickname string, initial, delta, minScore, maxScore int, reason string) (*AIAffinity, error) {
	var row AIAffinity
	score := initial + delta
	if score < minScore {
		score = minScore
	}
	if score > maxScore {
		score = maxScore
	}
	now := time.Now().Unix()

	err := s.database.Transaction(func(tx *gorm.DB) error {
		create := map[string]any{
			"user_id":     userID,
			"group_id":    groupID,
			"nickname":    nickname,
			"score":       score,
			"last_reason": reason,
			"updated_at":  now,
		}
		updates := map[string]any{
			"score":      clampedScoreExpr(tx, maxScore, minScore, delta),
			"updated_at": now,
		}
		if reason != "" {
			updates["last_reason"] = reason
		}
		if nickname != "" {
			updates["nickname"] = nickname
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "group_id"}},
			DoUpdates: clause.Assignments(updates),
		}).Model(&AIAffinity{}).Create(create).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ? AND group_id = ?", userID, groupID).First(&row).Error
	})
	return &row, err
}

type AffinityStore struct{ database *gorm.DB }

func NewAffinityStore(database *gorm.DB) *AffinityStore { return &AffinityStore{database: database} }
func (s *AffinityStore) Available() bool                { return s != nil && s.database != nil }

func clampedScoreExpr(database *gorm.DB, maxScore, minScore, delta int) clause.Expr {
	if database.Dialector.Name() == "postgres" {
		// ON CONFLICT exposes both the existing row and EXCLUDED. Qualify score
		// explicitly or PostgreSQL rejects the expression as ambiguous.
		return gorm.Expr("LEAST(?, GREATEST(?, ai_affinities.score + ?))", maxScore, minScore, delta)
	}
	return gorm.Expr("MIN(?, MAX(?, ai_affinities.score + ?))", maxScore, minScore, delta)
}
