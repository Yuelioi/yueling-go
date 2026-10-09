package runtimeconfig

import (
	"errors"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/internal/testdb"
)

func TestPostgresConfigConflictPreservesPublishedSnapshot(t *testing.T) {
	fixture(t)
	testdb.Init(t)
	first, second := New(NewPostgresStore(testdb.Database), nil), New(NewPostgresStore(testdb.Database), nil)
	for _, m := range []*Manager{first, second} {
		if err := m.Initialize(); err != nil {
			t.Fatal(err)
		}
	}
	revision := second.View().Revision
	if _, err := update(first, "bot", "bot.name", "first"); err != nil {
		t.Fatal(err)
	}
	if _, err := update(second, "bot", "bot.name", "stale"); !errors.Is(err, ErrConflict) {
		t.Fatalf("error=%v", err)
	}
	if second.View().Revision != revision || config.Current().Bot.Name != "first" {
		t.Fatal("conflict changed published state")
	}
	if err := second.Initialize(); err != nil {
		t.Fatal(err)
	}
	if second.View().Revision != first.View().Revision {
		t.Fatal("reload did not recover current revision")
	}
}

func TestPostgresStoresKeepTheirDatabase(t *testing.T) {
	testdb.Init(t)
	first := NewPostgresStore(testdb.Database)
	if err := first.Seed([]byte(`{"bot":{"name":"first"}}`)); err != nil {
		t.Fatal(err)
	}
	testdb.Init(t)
	second := NewPostgresStore(testdb.Database)
	if err := second.Seed([]byte(`{"bot":{"name":"second"}}`)); err != nil {
		t.Fatal(err)
	}
	firstData, _, err := first.Load()
	if err != nil {
		t.Fatal(err)
	}
	secondData, _, err := second.Load()
	if err != nil {
		t.Fatal(err)
	}
	if string(firstData) == string(secondData) {
		t.Fatal("stores shared a database")
	}
}
