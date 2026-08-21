package db

import "testing"

func TestGroupJoinRuleCRUD(t *testing.T) {
	initPostgresForTest(t)
	const g = int64(123)

	added, err := AddGroupJoinRule(g, JoinActionAllow, "交流")
	if err != nil || !added {
		t.Fatalf("add1: added=%v err=%v", added, err)
	}
	added, err = AddGroupJoinRule(g, JoinActionAllow, "交流")
	if err != nil || added {
		t.Fatalf("add dup: added=%v err=%v", added, err)
	}
	if _, err := AddGroupJoinRule(g, JoinActionDeny, "广告"); err != nil {
		t.Fatalf("add deny: %v", err)
	}
	if _, err := AddGroupJoinRule(456, JoinActionAllow, "你好"); err != nil {
		t.Fatalf("add other group: %v", err)
	}

	rows, err := GetAllGroupJoinRules()
	if err != nil || len(rows) != 3 {
		t.Fatalf("getall: n=%d err=%v", len(rows), err)
	}

	removed, err := DeleteGroupJoinRule(g, JoinActionAllow, "交流")
	if err != nil || !removed {
		t.Fatalf("del: removed=%v err=%v", removed, err)
	}
	removed, err = DeleteGroupJoinRule(g, JoinActionAllow, "交流")
	if err != nil || removed {
		t.Fatalf("del again should be false: removed=%v err=%v", removed, err)
	}
}
