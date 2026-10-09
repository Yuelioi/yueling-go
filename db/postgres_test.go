package db

import (
	"testing"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/internal/pgtest"
	"gorm.io/gorm"
)

var DB *gorm.DB

func initPostgresForTest(t testing.TB) {
	t.Helper()

	previous := DB
	opened, err := Open(config.DatabaseConfig{
		DSN:             pgtest.NewSchema(t),
		MaxOpen:         5,
		MaxIdle:         1,
		ConnMaxLifetime: 5,
	})
	if err != nil {
		t.Fatalf("initialize PostgreSQL test database: %v", err)
	}
	DB = opened
	t.Cleanup(func() {
		if sqlDB, err := opened.DB(); err == nil {
			_ = sqlDB.Close()
		}
		DB = previous
	})
}
