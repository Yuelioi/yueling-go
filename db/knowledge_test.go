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

func TestKnowledgeShortcutsAreGroupScoped(t *testing.T) {
	oldDB := DB
	if err := initSQLiteForTest(filepath.Join(t.TempDir(), "knowledge-shortcuts.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { DB = oldDB })

	group100, err := CreateGroupKnowledge(100, 1, "AE 下载", "https://group100.example/ae", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SetGroupKnowledgeShortcuts(group100.ID, 100, []string{"ae下载", "AE 安装包"}); err != nil {
		t.Fatal(err)
	}
	group200, err := CreateGroupKnowledge(200, 2, "另一个群的 AE", "https://group200.example/ae", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SetGroupKnowledgeShortcuts(group200.ID, 200, []string{"ae下载"}); err != nil {
		t.Fatal(err)
	}

	got, err := FindGroupKnowledgeShortcut(100, "  AE下载 ")
	if err != nil || got.ID != group100.ID {
		t.Fatalf("group100 shortcut=%+v err=%v", got, err)
	}
	got, err = FindGroupKnowledgeShortcut(200, "ae下载")
	if err != nil || got.ID != group200.ID {
		t.Fatalf("group200 shortcut=%+v err=%v", got, err)
	}
	if _, err := FindGroupKnowledgeShortcut(300, "ae下载"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-group shortcut leaked: err=%v", err)
	}
	rows, err := ListGroupKnowledge(100)
	if err != nil || len(rows) != 1 || rows[0].ID != group100.ID || len(rows[0].Shortcuts) != 2 {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	if _, err := SetGroupKnowledgeShortcuts(group100.ID, 200, []string{"越权"}); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-group shortcut mutation err=%v", err)
	}
}

func TestSharedKnowledgeIsAvailableAndGroupShortcutOverridesIt(t *testing.T) {
	oldDB := DB
	if err := initSQLiteForTest(filepath.Join(t.TempDir(), "shared-knowledge.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { DB = oldDB })

	shared, err := CreateGroupKnowledge(SharedKnowledgeGroupID, 1, "公共下载", "公共地址", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SetGroupKnowledgeShortcuts(shared.ID, SharedKnowledgeGroupID, []string{"下载"}); err != nil {
		t.Fatal(err)
	}
	private, err := CreateGroupKnowledge(100, 2, "本群下载", "本群地址", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SetGroupKnowledgeShortcuts(private.ID, 100, []string{"下载"}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateGroupKnowledge(200, 3, "其他群秘密", "不可见", ""); err != nil {
		t.Fatal(err)
	}

	got, err := FindGroupKnowledgeShortcut(100, "下载")
	if err != nil || got.ID != private.ID {
		t.Fatalf("private override=%+v err=%v", got, err)
	}
	got, err = FindGroupKnowledgeShortcut(300, "下载")
	if err != nil || got.ID != shared.ID {
		t.Fatalf("shared fallback=%+v err=%v", got, err)
	}

	rows, err := ListAvailableGroupKnowledge(100)
	if err != nil || len(rows) != 2 || rows[0].ID != private.ID || rows[1].ID != shared.ID {
		t.Fatalf("available rows=%+v err=%v", rows, err)
	}
	exactShared, err := ListGroupKnowledge(SharedKnowledgeGroupID)
	if err != nil || len(exactShared) != 1 || exactShared[0].ID != shared.ID {
		t.Fatalf("shared scope rows=%+v err=%v", exactShared, err)
	}
}

func TestKnowledgeShortcutConflictRollsBackReplacement(t *testing.T) {
	oldDB := DB
	if err := initSQLiteForTest(filepath.Join(t.TempDir(), "knowledge-shortcut-conflict.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { DB = oldDB })

	first, _ := CreateGroupKnowledge(100, 1, "第一条", "第一条内容", "")
	second, _ := CreateGroupKnowledge(100, 1, "第二条", "第二条内容", "")
	if _, err := SetGroupKnowledgeShortcuts(first.ID, 100, []string{"原触发"}); err != nil {
		t.Fatal(err)
	}
	if _, err := SetGroupKnowledgeShortcuts(second.ID, 100, []string{"已占用"}); err != nil {
		t.Fatal(err)
	}
	if _, err := SetGroupKnowledgeShortcuts(first.ID, 100, []string{"已占用"}); !errors.Is(err, ErrKnowledgeShortcutConflict) {
		t.Fatalf("conflict err=%v", err)
	}
	got, err := FindGroupKnowledgeShortcut(100, "原触发")
	if err != nil || got.ID != first.ID {
		t.Fatalf("old shortcut should survive rollback: got=%+v err=%v", got, err)
	}
}
