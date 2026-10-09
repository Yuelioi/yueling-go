package db

import (
	"fmt"

	"gorm.io/gorm"
)

type GroupJoinRule struct {
	ID      uint   `gorm:"primarykey;autoIncrement"`
	GroupID int64  `gorm:"index"`
	Action  string `gorm:"size:8"`
	Keyword string `gorm:"size:128"`
}

func (s *GroupStore) AddGroupJoinRule(groupID int64, action, keyword string) (bool, error) {
	added := false
	err := s.database.Transaction(func(tx *gorm.DB) error {
		if err := lockGroupJoin(tx, groupID); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&GroupJoinRule{}).Where("group_id = ? AND action = ? AND keyword = ?", groupID, action, keyword).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		if err := tx.Create(&GroupJoinRule{GroupID: groupID, Action: action, Keyword: keyword}).Error; err != nil {
			return err
		}
		added = true
		return nil
	})
	return added, err
}

func (s *GroupStore) DeleteGroupJoinRule(groupID int64, action, keyword string) (bool, error) {
	removed := false
	err := s.database.Transaction(func(tx *gorm.DB) error {
		if err := lockGroupJoin(tx, groupID); err != nil {
			return err
		}
		result := tx.Where("group_id = ? AND action = ? AND keyword = ?", groupID, action, keyword).Delete(&GroupJoinRule{})
		removed = result.RowsAffected > 0
		return result.Error
	})
	return removed, err
}

// SetGroupJoinRules overwrites all keywords of one action for a group in a single
// transaction; an empty keywords slice clears them.
func (s *GroupStore) SetGroupJoinRules(groupID int64, action string, keywords []string) error {
	if groupID <= 0 || (action != JoinActionAllow && action != JoinActionDeny) {
		return fmt.Errorf("无效的群或名单类型")
	}
	keywords, err := NormalizeJoinKeywords(keywords)
	if err != nil {
		return err
	}
	return s.database.Transaction(func(tx *gorm.DB) error {
		if err := lockGroupJoin(tx, groupID); err != nil {
			return err
		}
		if err := setJoinMode(tx, groupID, JoinModeOverride); err != nil {
			return err
		}
		if err := tx.Where("group_id = ? AND action = ?", groupID, action).
			Delete(&GroupJoinRule{}).Error; err != nil {
			return err
		}
		for _, kw := range keywords {
			if kw == "" {
				continue
			}
			if err := tx.Create(&GroupJoinRule{GroupID: groupID, Action: action, Keyword: kw}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *GroupStore) GetAllGroupJoinRules() ([]GroupJoinRule, error) {
	var rows []GroupJoinRule
	err := s.database.Find(&rows).Error
	return rows, err
}

func lockGroupJoin(tx *gorm.DB, groupID int64) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", fmt.Sprintf("join:%d", groupID)).Error
}
