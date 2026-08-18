package db

import (
	"time"

	"gorm.io/gorm"
)

// AIAffinityTier is one administrator-defined relationship level. The tier
// applies from MinScore (inclusive) until the next tier starts.
type AIAffinityTier struct {
	ID        uint   `gorm:"primarykey;autoIncrement" json:"-"`
	MinScore  int    `gorm:"not null;uniqueIndex" json:"min_score"`
	Name      string `gorm:"size:32;not null" json:"name"`
	Prompt    string `gorm:"type:text;not null" json:"prompt"`
	UpdatedAt int64  `gorm:"not null" json:"updated_at"`
}

func ListAIAffinityTiers() ([]AIAffinityTier, error) {
	var rows []AIAffinityTier
	err := DB.Order("min_score ASC").Find(&rows).Error
	return rows, err
}

// ReplaceAIAffinityTiers atomically replaces the global tier configuration.
// An empty slice removes the override and restores the built-in defaults.
func ReplaceAIAffinityTiers(tiers []AIAffinityTier) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AIAffinityTier{}).Error; err != nil {
			return err
		}
		if len(tiers) == 0 {
			return nil
		}
		now := time.Now().Unix()
		rows := make([]AIAffinityTier, len(tiers))
		for i, tier := range tiers {
			rows[i] = AIAffinityTier{
				MinScore:  tier.MinScore,
				Name:      tier.Name,
				Prompt:    tier.Prompt,
				UpdatedAt: now,
			}
		}
		return tx.Create(&rows).Error
	})
}
