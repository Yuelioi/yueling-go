package db

import (
	"errors"
	"path/filepath"
	"testing"

	"gorm.io/gorm"
)

func TestGroupKnowledgeCRUDAndIsolation(t *testing.T) {
	oldDB := DB
	if err := initSQLiteForTest(filepath.Join(t.TempDir(), "knowledge.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { DB = oldDB })

	row, err := CreateGroupKnowledge(100, 1, "入群规则", "新成员需要修改群名片", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateGroupKnowledge(200, 2, "其他群", "不可见", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	rows, err := ListGroupKnowledge(100)
	if err != nil || len(rows) != 1 || rows[0].ID != row.ID || rows[0].Content != "新成员需要修改群名片" {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	if count, err := CountGroupKnowledge(100); err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if err := DeleteGroupKnowledge(row.ID, 200); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-group delete err=%v", err)
	}
	if err := DeleteGroupKnowledge(row.ID, 100); err != nil {
		t.Fatal(err)
	}
}
