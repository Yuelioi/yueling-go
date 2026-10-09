package db

import (
	"errors"
	"fmt"
	"testing"

	"gorm.io/gorm"
)

func TestKnowledgeCreationQuotaIsAtomic(t *testing.T) {
	initPostgresForTest(t)
	store := NewKnowledgeStore(DB)
	results := make(chan error, 12)
	for i := range 12 {
		go func() {
			_, err := store.CreateEntry(100, 1, fmt.Sprint(i), "content", "", nil, 2)
			results <- err
		}()
	}
	success := 0
	for range 12 {
		err := <-results
		if err == nil {
			success++
		} else if !errors.Is(err, ErrKnowledgeLimit) {
			t.Fatal(err)
		}
	}
	if success != 2 {
		t.Fatalf("created %d entries with quota 2", success)
	}
	if count, err := store.CountGroupKnowledge(100); err != nil || count != 2 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}

func TestKnowledgeShortcutConflictRollsBackWholeEntry(t *testing.T) {
	initPostgresForTest(t)
	store := NewKnowledgeStore(DB)
	if _, err := store.CreateEntry(100, 1, "existing", "content", "", []string{"occupied"}, 100); err != nil {
		t.Fatal(err)
	}
	row, err := store.CreateEntry(100, 1, "rejected", "content", "", []string{"new", "occupied"}, 100)
	if row != nil || !errors.Is(err, ErrKnowledgeShortcutConflict) {
		t.Fatalf("row=%v err=%v", row, err)
	}
	if count, err := store.CountGroupKnowledge(100); err != nil || count != 1 {
		t.Fatalf("partial entry survived: count=%d err=%v", count, err)
	}
	if _, err := store.FindGroupKnowledgeShortcut(100, "new"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("partial shortcut survived: %v", err)
	}
}

func TestKnowledgeStoresKeepTheirDatabase(t *testing.T) {
	initPostgresForTest(t)
	first := NewKnowledgeStore(DB)
	if _, err := first.CreateEntry(100, 1, "first", "content", "", nil, 100); err != nil {
		t.Fatal(err)
	}
	initPostgresForTest(t)
	second := NewKnowledgeStore(DB)
	if _, err := second.CreateEntry(100, 1, "second", "content", "", nil, 100); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		store *KnowledgeStore
		title string
	}{{first, "first"}, {second, "second"}} {
		rows, err := tt.store.ListGroupKnowledge(100)
		if err != nil || len(rows) != 1 || rows[0].Title != tt.title {
			t.Fatalf("database isolation failed: rows=%v err=%v", rows, err)
		}
	}
}

func TestGroupKnowledgeCRUDAndIsolation(t *testing.T) {
	initPostgresForTest(t)

	row, err := NewKnowledgeStore(DB).CreateGroupKnowledge(100, 1, "入群规则", "新成员需要修改群名片", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewKnowledgeStore(DB).CreateGroupKnowledge(200, 2, "其他群", "不可见", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	rows, err := NewKnowledgeStore(DB).ListGroupKnowledge(100)
	if err != nil || len(rows) != 1 || rows[0].ID != row.ID || rows[0].Content != "新成员需要修改群名片" {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	if count, err := NewKnowledgeStore(DB).CountGroupKnowledge(100); err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if err := NewKnowledgeStore(DB).DeleteGroupKnowledge(row.ID, 200); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-group delete err=%v", err)
	}
	if err := NewKnowledgeStore(DB).DeleteGroupKnowledge(row.ID, 100); err != nil {
		t.Fatal(err)
	}
}

func TestKnowledgeShortcutsAreGroupScoped(t *testing.T) {
	initPostgresForTest(t)

	group100, err := NewKnowledgeStore(DB).CreateGroupKnowledge(100, 1, "AE 下载", "https://group100.example/ae", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewKnowledgeStore(DB).SetGroupKnowledgeShortcuts(group100.ID, 100, []string{"ae下载", "AE 安装包"}); err != nil {
		t.Fatal(err)
	}
	group200, err := NewKnowledgeStore(DB).CreateGroupKnowledge(200, 2, "另一个群的 AE", "https://group200.example/ae", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewKnowledgeStore(DB).SetGroupKnowledgeShortcuts(group200.ID, 200, []string{"ae下载"}); err != nil {
		t.Fatal(err)
	}

	got, err := NewKnowledgeStore(DB).FindGroupKnowledgeShortcut(100, "  AE下载 ")
	if err != nil || got.ID != group100.ID {
		t.Fatalf("group100 shortcut=%+v err=%v", got, err)
	}
	got, err = NewKnowledgeStore(DB).FindGroupKnowledgeShortcut(200, "ae下载")
	if err != nil || got.ID != group200.ID {
		t.Fatalf("group200 shortcut=%+v err=%v", got, err)
	}
	if _, err := NewKnowledgeStore(DB).FindGroupKnowledgeShortcut(300, "ae下载"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-group shortcut leaked: err=%v", err)
	}
	rows, err := NewKnowledgeStore(DB).ListGroupKnowledge(100)
	if err != nil || len(rows) != 1 || rows[0].ID != group100.ID || len(rows[0].Shortcuts) != 2 {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	if _, err := NewKnowledgeStore(DB).SetGroupKnowledgeShortcuts(group100.ID, 200, []string{"越权"}); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-group shortcut mutation err=%v", err)
	}
}

func TestSharedKnowledgeIsAvailableAndGroupShortcutOverridesIt(t *testing.T) {
	initPostgresForTest(t)

	shared, err := NewKnowledgeStore(DB).CreateGroupKnowledge(SharedKnowledgeGroupID, 1, "公共下载", "公共地址", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewKnowledgeStore(DB).SetGroupKnowledgeShortcuts(shared.ID, SharedKnowledgeGroupID, []string{"下载"}); err != nil {
		t.Fatal(err)
	}
	private, err := NewKnowledgeStore(DB).CreateGroupKnowledge(100, 2, "本群下载", "本群地址", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewKnowledgeStore(DB).SetGroupKnowledgeShortcuts(private.ID, 100, []string{"下载"}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewKnowledgeStore(DB).CreateGroupKnowledge(200, 3, "其他群秘密", "不可见", ""); err != nil {
		t.Fatal(err)
	}

	got, err := NewKnowledgeStore(DB).FindGroupKnowledgeShortcut(100, "下载")
	if err != nil || got.ID != private.ID {
		t.Fatalf("private override=%+v err=%v", got, err)
	}
	got, err = NewKnowledgeStore(DB).FindGroupKnowledgeShortcut(300, "下载")
	if err != nil || got.ID != shared.ID {
		t.Fatalf("shared fallback=%+v err=%v", got, err)
	}

	rows, err := NewKnowledgeStore(DB).ListAvailableGroupKnowledge(100)
	if err != nil || len(rows) != 2 || rows[0].ID != private.ID || rows[1].ID != shared.ID {
		t.Fatalf("available rows=%+v err=%v", rows, err)
	}
	exactShared, err := NewKnowledgeStore(DB).ListGroupKnowledge(SharedKnowledgeGroupID)
	if err != nil || len(exactShared) != 1 || exactShared[0].ID != shared.ID {
		t.Fatalf("shared scope rows=%+v err=%v", exactShared, err)
	}
}

func TestKnowledgeShortcutConflictRollsBackReplacement(t *testing.T) {
	initPostgresForTest(t)

	first, _ := NewKnowledgeStore(DB).CreateGroupKnowledge(100, 1, "第一条", "第一条内容", "")
	second, _ := NewKnowledgeStore(DB).CreateGroupKnowledge(100, 1, "第二条", "第二条内容", "")
	if _, err := NewKnowledgeStore(DB).SetGroupKnowledgeShortcuts(first.ID, 100, []string{"原触发"}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewKnowledgeStore(DB).SetGroupKnowledgeShortcuts(second.ID, 100, []string{"已占用"}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewKnowledgeStore(DB).SetGroupKnowledgeShortcuts(first.ID, 100, []string{"已占用"}); !errors.Is(err, ErrKnowledgeShortcutConflict) {
		t.Fatalf("conflict err=%v", err)
	}
	got, err := NewKnowledgeStore(DB).FindGroupKnowledgeShortcut(100, "原触发")
	if err != nil || got.ID != first.ID {
		t.Fatalf("old shortcut should survive rollback: got=%+v err=%v", got, err)
	}
}
