package db

import (
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

func (s *OverviewStore) GetWebUIOverview(blockBelow int) (*WebUIOverview, error) {
	var overview WebUIOverview
	queries := []struct {
		target *int64
		query  *gorm.DB
	}{
		{&overview.AffinityCount, s.database.Model(&AIAffinity{})},
		{&overview.LowAffinityCount, s.database.Model(&AIAffinity{}).Where("score < ?", blockBelow)},
		{&overview.MemoryCount, s.database.Model(&SemanticMemory{})},
		{&overview.MemoryUserCount, s.database.Model(&SemanticMemory{}).Distinct("user_id")},
		{&overview.DigestCount, s.database.Model(&DailyDigest{}).Where("enabled = ?", true)},
		{&overview.FeedCount, s.database.Model(&FeedSubscription{}).Where("enabled = ?", true)},
		{&overview.KnowledgeCount, s.database.Model(&GroupKnowledge{})},
	}
	for _, item := range queries {
		if err := item.query.Count(item.target).Error; err != nil {
			return nil, err
		}
	}
	return &overview, nil
}

func (s *GroupStore) IsGroupPluginDisabled(groupID int64, pluginID int) (bool, error) {
	var count int64
	err := s.database.Model(&GroupPluginDisabled{}).
		Where("group_id = ? AND plugin_id = ?", groupID, pluginID).
		Count(&count).Error
	return count > 0, err
}

func (s *GroupStore) GetDisabledPlugins(groupID int64) (map[int]bool, error) {
	var rows []GroupPluginDisabled
	if err := s.database.Where("group_id = ?", groupID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[int]bool, len(rows))
	for _, row := range rows {
		out[row.PluginID] = true
	}
	return out, nil
}

func (s *GroupStore) SetGroupPluginDisabled(groupID int64, pluginID int, disabled bool) error {
	if disabled {
		return s.database.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&GroupPluginDisabled{GroupID: groupID, PluginID: pluginID}).Error
	}
	return s.database.Where("group_id = ? AND plugin_id = ?", groupID, pluginID).
		Delete(&GroupPluginDisabled{}).Error
}

func (s *GroupStore) SetPluginDisabledForGroups(pluginID int, groupIDs []int64, disabled bool) error {
	return s.database.Transaction(func(tx *gorm.DB) error {
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
