// Package testdb provides isolated SQLite databases for fast unit tests.
// Production binaries do not import this package.
package testdb

import (
	"fmt"

	appdb "github.com/Yuelioi/yueling-go/db"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Init(path string) error {
	database, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return err
	}
	models := []any{
		&appdb.UserGameRecord{}, &appdb.AIAffinity{}, &appdb.Reminder{},
		&appdb.SemanticMemory{}, &appdb.ProceduralMemory{},
		&appdb.TodoItem{}, &appdb.UserProfile{}, &appdb.GroupJoinRule{},
		&appdb.GroupPluginDisabled{}, &appdb.DailyDigest{}, &appdb.FeedSubscription{}, &appdb.FeedGroupSetting{}, &appdb.FeedPendingItem{},
		&appdb.GroupKnowledge{}, &appdb.GroupKnowledgeShortcut{}, &appdb.GroupChatMessage{}, &appdb.GroupAISetting{},
	}
	if err := database.AutoMigrate(models...); err != nil {
		return fmt.Errorf("migrate sqlite test database: %w", err)
	}
	appdb.DB = database
	return nil
}
