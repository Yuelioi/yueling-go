package db

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func runMigrations(database *gorm.DB) error {
	if err := database.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version bigint PRIMARY KEY,
			name text NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`).Error; err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		version, err := migrationVersion(entry.Name())
		if err != nil {
			return err
		}
		body, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		if err := database.Transaction(func(tx *gorm.DB) error {
			// Serialise startup migrations when two bot instances race.
			if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", int64(917_202_606_180)).Error; err != nil {
				return err
			}
			var applied bool
			if err := tx.Raw("SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = ?)", version).Scan(&applied).Error; err != nil {
				return err
			}
			if applied {
				return nil
			}
			if err := tx.Exec(string(body)).Error; err != nil {
				return err
			}
			return tx.Exec("INSERT INTO schema_migrations (version, name) VALUES (?, ?)", version, entry.Name()).Error
		}); err != nil {
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func migrationVersion(name string) (int64, error) {
	prefix, _, ok := strings.Cut(name, "_")
	if !ok {
		return 0, fmt.Errorf("migration %q has no numeric prefix", name)
	}
	version, err := strconv.ParseInt(prefix, 10, 64)
	if err != nil || version <= 0 {
		return 0, fmt.Errorf("migration %q has invalid version", name)
	}
	return version, nil
}
