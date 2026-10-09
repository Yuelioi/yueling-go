package ai

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/Yuelioi/yueling-go/internal/testdb"
	openai "github.com/sashabaranov/go-openai"
	"gorm.io/gorm"
)

func TestMemoryQuotaDedupAndRollback(t *testing.T) {
	testdb.Init(t)
	m := NewMemoryManager(testdb.Database)
	// Different instances exercise database serialization as well as local locks.
	other := NewMemoryManager(testdb.Database)
	var wg sync.WaitGroup
	for i := range 110 {
		wg.Go(func() {
			owner := m
			if i%2 == 0 {
				owner = other
			}
			if err := owner.WriteSemantic(1, fmt.Sprintf("preference %d", i), "general"); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	rows, err := m.ListSemanticMemoryRecords(1, 100)
	if err != nil || len(rows) != 100 {
		t.Fatalf("quota: %d, %v", len(rows), err)
	}
	before := rows[0]
	if err := m.WriteSemanticDetailed(1, before.Content, "explicit-category", "explicit", 1, 1.5); err != nil {
		t.Fatal(err)
	}
	if err := other.WriteSemantic(1, before.Content, "inferred-category"); err != nil {
		t.Fatal(err)
	}
	row, err := m.GetSemanticMemoryAdmin(before.ID)
	if err != nil || row.Category != "explicit-category" || row.Source != "explicit" {
		t.Fatalf("explicit memory overwritten: %+v, %v", row, err)
	}
	if err := testdb.Database.Callback().Create().Before("gorm:create").Register("fail_memory_create", func(tx *gorm.DB) {
		if tx.Statement.Table == "semantic_memories" {
			tx.AddError(errors.New("injected failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := m.WriteSemantic(1, "new failing record", "general"); err == nil {
		t.Fatal("write unexpectedly succeeded")
	}
	after, err := m.ListSemanticMemoryRecords(1, 100)
	if err != nil || len(after) != 100 {
		t.Fatalf("eviction was not rolled back: %d %v", len(after), err)
	}
	for i := range rows {
		if rows[i].ID != after[i].ID {
			t.Fatal("failed insert evicted existing memory")
		}
	}
}

func TestMemoryClearInvalidatesInFlightExtraction(t *testing.T) {
	testdb.Init(t)
	m := NewMemoryManager(testdb.Database)
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	m.extractText = func(context.Context, openai.ChatCompletionRequest) (string, error) {
		close(started)
		<-release
		return `[{"content":"用户喜欢咖啡","category":"food"}]`, nil
	}
	go func() {
		defer close(done)
		m.SmartWriteSemantic(context.Background(), 1, "我喜欢咖啡", "记住了")
	}()
	<-started
	_, err := m.ClearSemanticMemories(1)
	close(release)
	<-done
	if err != nil {
		t.Fatal(err)
	}
	rows, err := m.ListSemanticMemoryRecords(1, 100)
	if err != nil || len(rows) != 0 {
		t.Fatalf("old extraction restored cleared data: %+v %v", rows, err)
	}
}

func TestRulesQuotaAndProfileConcurrentUpsert(t *testing.T) {
	testdb.Init(t)
	m := NewMemoryManager(testdb.Database)
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			_ = m.AddGroupRule(1, 1, fmt.Sprintf("rule %d", i))
			if err := m.SetUserProfile(1, "位置", fmt.Sprintf("city %d", i)); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	rows, err := m.ReadGroupRules(1)
	if err != nil || len(rows) != 10 {
		t.Fatalf("rules quota: %d %v", len(rows), err)
	}
	profile, err := m.GetAllUserProfile(1)
	if err != nil || len(profile) != 1 {
		t.Fatalf("profile upsert: %+v %v", profile, err)
	}
	if err := m.SetUserProfile(1, "位置", ""); err != nil {
		t.Fatal(err)
	}
	if value, ok := m.GetUserProfile(1, "位置"); !ok || value != "" {
		t.Fatalf("empty update: %q %v", value, ok)
	}
}
