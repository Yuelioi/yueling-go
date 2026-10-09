package db

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestTodoConcurrentQuotaAndStableOwnership(t *testing.T) {
	initPostgresForTest(t)
	store := NewTodoStore(DB)
	results := make(chan error, 30)
	for range 30 {
		go func() { results <- store.Add(1, 100, "task") }()
	}
	accepted := 0
	for range 30 {
		err := <-results
		if err == nil {
			accepted++
		} else if !errors.Is(err, ErrTodoLimit) {
			t.Fatal(err)
		}
	}
	if accepted != 20 {
		t.Fatalf("accepted=%d want20", accepted)
	}
	rows, err := store.List(1)
	if err != nil || len(rows) != 20 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	first, second := rows[0], rows[1]
	if _, err := store.Complete(2, first.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("other user completed task: %v", err)
	}
	if _, err := store.Delete(2, first.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("other user deleted task: %v", err)
	}
	if _, err := store.Delete(1, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Complete(1, first.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("stale ID selected another task: %v", err)
	}
	if row, err := store.Complete(1, second.ID); err != nil || row.ID != second.ID {
		t.Fatalf("stable ID failed: %v", err)
	}
	if err := store.Add(1, 100, "replacement"); err != nil {
		t.Fatal(err)
	}
}
