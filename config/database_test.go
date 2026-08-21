package config

import (
	"strings"
	"testing"
)

func TestLoadDatabaseDefaults(t *testing.T) {
	path := writeConfig(t, baseConfig(""))
	if err := Load(path); err != nil {
		t.Fatal(err)
	}
	if C.Database.MaxOpen != 20 || C.Database.MaxIdle != 5 || C.Database.ConnMaxLifetime != 30 {
		t.Fatalf("database defaults = %+v", C.Database)
	}
}

func TestLoadDatabaseDSNFromEnvironment(t *testing.T) {
	t.Setenv("YUELING_DATABASE_DSN", "postgres://env:secret@postgres:5432/envdb?sslmode=disable")
	path := writeConfig(t, baseConfig(""))
	if err := Load(path); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(C.Database.DSN, "envdb") {
		t.Fatalf("database DSN was not overridden: %q", C.Database.DSN)
	}
}

func TestLoadRejectsNonPostgresDatabase(t *testing.T) {
	content := strings.Replace(baseConfig(""),
		"postgres://test:test@127.0.0.1:5432/test?sslmode=disable", "mysql://test:test@127.0.0.1:3306/test", 1)
	path := writeConfig(t, content)
	err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "database.dsn") {
		t.Fatalf("error = %v", err)
	}
}
