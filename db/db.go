package db

import (
	"fmt"
	"time"

	"github.com/Yuelioi/yueling-go/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type SemanticMemory struct {
	ID           uint    `gorm:"primarykey;autoIncrement"`
	UserID       int64   `gorm:"index"`
	Content      string  `gorm:"type:text"`
	Category     string  `gorm:"size:32"`
	Source       string  `gorm:"size:16;default:auto"`
	Confidence   float64 `gorm:"default:0.8"`
	Importance   float64 `gorm:"default:1.0"`
	Score        float64 `gorm:"default:1.0"`
	CreatedAt    float64
	UpdatedAt    float64
	LastAccessed float64
}

type ProceduralMemory struct {
	ID        uint   `gorm:"primarykey;autoIncrement"`
	GroupID   int64  `gorm:"index"`
	Rule      string `gorm:"type:text"`
	Priority  int    `gorm:"default:0"`
	CreatedBy int64
	CreatedAt float64
}

type GroupPluginDisabled struct {
	ID       uint  `gorm:"primarykey;autoIncrement"`
	GroupID  int64 `gorm:"uniqueIndex:idx_group_plugin_disabled"`
	PluginID int   `gorm:"uniqueIndex:idx_group_plugin_disabled"`
}

const (
	JoinActionAllow = "allow"
	JoinActionDeny  = "deny"
)

// Open returns an owned database handle and applies pending schema
// migrations. PostgreSQL is intentionally the sole production dialect.
func Open(cfg config.DatabaseConfig) (*gorm.DB, error) {
	database, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  cfg.DSN,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		return nil, fmt.Errorf("postgres handle: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Minute)
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	if err := runMigrations(database); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return database, nil
}
