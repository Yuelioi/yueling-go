package db

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestGroupJoinRuleCRUD(t *testing.T) {
	initPostgresForTest(t)
	const g = int64(123)

	added, err := NewGroupStore(DB).AddGroupJoinRule(g, JoinActionAllow, "交流")
	if err != nil || !added {
		t.Fatalf("add1: added=%v err=%v", added, err)
	}
	added, err = NewGroupStore(DB).AddGroupJoinRule(g, JoinActionAllow, "交流")
	if err != nil || added {
		t.Fatalf("add dup: added=%v err=%v", added, err)
	}
	if _, err := NewGroupStore(DB).AddGroupJoinRule(g, JoinActionDeny, "广告"); err != nil {
		t.Fatalf("add deny: %v", err)
	}
	if _, err := NewGroupStore(DB).AddGroupJoinRule(456, JoinActionAllow, "你好"); err != nil {
		t.Fatalf("add other group: %v", err)
	}

	rows, err := NewGroupStore(DB).GetAllGroupJoinRules()
	if err != nil || len(rows) != 3 {
		t.Fatalf("getall: n=%d err=%v", len(rows), err)
	}

	removed, err := NewGroupStore(DB).DeleteGroupJoinRule(g, JoinActionAllow, "交流")
	if err != nil || !removed {
		t.Fatalf("del: removed=%v err=%v", removed, err)
	}
	removed, err = NewGroupStore(DB).DeleteGroupJoinRule(g, JoinActionAllow, "交流")
	if err != nil || removed {
		t.Fatalf("del again should be false: removed=%v err=%v", removed, err)
	}
}

func TestJoinRuleConcurrentInsertIsIdempotent(t *testing.T) {
	initPostgresForTest(t)
	store := NewGroupStore(DB)
	var added atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			ok, err := store.AddGroupJoinRule(1, JoinActionAllow, "test")
			if err != nil {
				t.Error(err)
			}
			if ok {
				added.Add(1)
			}
		})
	}
	wg.Wait()
	rows, err := store.GetAllGroupJoinRules()
	if err != nil || len(rows) != 1 || added.Load() != 1 {
		t.Fatalf("rows=%d added=%d err=%v", len(rows), added.Load(), err)
	}
}
