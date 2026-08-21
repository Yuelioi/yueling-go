package db

import (
	"testing"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/internal/pgtest"
)

func initPostgresForTest(t *testing.T) {
	t.Helper()

	previous := DB
	if err := Init(config.DatabaseConfig{
		DSN:             pgtest.NewSchema(t),
		MaxOpen:         5,
		MaxIdle:         1,
		ConnMaxLifetime: 5,
	}); err != nil {
		t.Fatalf("initialize PostgreSQL test database: %v", err)
	}
	opened := DB
	t.Cleanup(func() {
		if sqlDB, err := opened.DB(); err == nil {
			_ = sqlDB.Close()
		}
		DB = previous
	})
}
