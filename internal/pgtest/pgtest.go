// Package pgtest creates isolated PostgreSQL schemas for database tests.
package pgtest

import (
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const DSNEnv = "YUELING_TEST_DATABASE_DSN"

// NewSchema creates a disposable schema and returns a DSN whose search_path
// points at it. The schema is dropped after callers close their test database.
func NewSchema(t *testing.T) string {
	t.Helper()

	rawDSN := strings.TrimSpace(os.Getenv(DSNEnv))
	if rawDSN == "" {
		t.Skip("set " + DSNEnv + " to run PostgreSQL database tests")
	}
	parsed, err := url.Parse(rawDSN)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Hostname() == "" {
		t.Fatalf("%s must be a valid postgres/postgresql URL", DSNEnv)
	}

	admin, err := gorm.Open(postgres.Open(rawDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open PostgreSQL test database: %v", err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatalf("get PostgreSQL test handle: %v", err)
	}
	if err := admin.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", int64(917_202_606_181)).Error; err != nil {
			return err
		}
		for _, extension := range []string{"pg_trgm", "zhparser"} {
			if err := tx.Exec("CREATE EXTENSION IF NOT EXISTS " + extension + " WITH SCHEMA public").Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		_ = adminSQL.Close()
		t.Fatalf("prepare PostgreSQL test extensions: %v", err)
	}

	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		_ = adminSQL.Close()
		t.Fatalf("generate PostgreSQL test schema name: %v", err)
	}
	schema := "yueling_test_" + hex.EncodeToString(random[:])
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		_ = adminSQL.Close()
		t.Fatalf("create PostgreSQL test schema: %v", err)
	}

	t.Cleanup(func() {
		if err := admin.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error; err != nil {
			t.Errorf("drop PostgreSQL test schema %s: %v", schema, err)
		}
		_ = adminSQL.Close()
	})

	query := parsed.Query()
	query.Set("search_path", schema+",public")
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
