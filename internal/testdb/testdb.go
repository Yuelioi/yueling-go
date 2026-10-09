// Package testdb owns isolated PostgreSQL fixtures; runtime code never imports it.
package testdb

import (
	"testing"

	"github.com/Yuelioi/yueling-go/config"
	appdb "github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/pgtest"
	"gorm.io/gorm"
)

var Database *gorm.DB

func Init(t *testing.T) {
	t.Helper()

	previous := Database
	opened, err := appdb.Open(config.DatabaseConfig{
		DSN:             pgtest.NewSchema(t),
		MaxOpen:         5,
		MaxIdle:         1,
		ConnMaxLifetime: 5,
	})
	if err != nil {
		t.Fatalf("initialize PostgreSQL test database: %v", err)
	}
	Database = opened
	t.Cleanup(func() {
		if sqlDB, err := opened.DB(); err == nil {
			_ = sqlDB.Close()
		}
		Database = previous
	})
}
