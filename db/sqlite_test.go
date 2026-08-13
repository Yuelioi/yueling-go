package db

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func initSQLiteForTest(path string) error {
	var err error
	DB, err = gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return err
	}
	models := []any{
		&UserGameRecord{}, &AIAffinity{}, &Reminder{},
		&SemanticMemory{}, &ProceduralMemory{},
		&TodoItem{}, &UserProfile{}, &GroupJoinRule{},
		&GroupPluginDisabled{}, &DailyDigest{}, &FeedSubscription{}, &FeedGroupSetting{}, &FeedPendingItem{},
		&GroupKnowledge{}, &GroupKnowledgeShortcut{}, &GroupChatMessage{}, &GroupAISetting{},
	}
	if err := DB.AutoMigrate(models...); err != nil {
		return fmt.Errorf("migrate sqlite test database: %w", err)
	}
	return nil
}
