package db

import (
	"testing"
	"time"
)

func TestBusinessStoresKeepTheirOwnedDatabase(t *testing.T) {
	initPostgresForTest(t)
	groups := NewGroupStore(DB)
	chats := NewChatStore(DB)
	affinity := NewAffinityStore(DB)
	usage := NewCommandUsageStore(DB)
	games := NewGameStore(DB)
	now := time.Now()
	if err := groups.SetGroupPluginDisabled(10, 20, true); err != nil {
		t.Fatal(err)
	}
	if err := chats.SaveGroupChatMessage(GroupChatMessage{GroupID: 10, UserID: 1, MessageID: 1, Content: "隔离记录", CreatedAt: now.Unix()}); err != nil {
		t.Fatal(err)
	}
	if _, err := affinity.UpdateAIAffinity(1, 10, "昵称", 50, 1, 0, 100, "test"); err != nil {
		t.Fatal(err)
	}
	if err := usage.RecordCommandUsageAt(10, 1, 20, "命令", now); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := games.CheckIn(1, 10, "昵称"); err != nil {
		t.Fatal(err)
	}
	initPostgresForTest(t)
	if disabled, err := groups.IsGroupPluginDisabled(10, 20); err != nil || !disabled {
		t.Fatalf("owned groups: %v %v", disabled, err)
	}
	if disabled, err := NewGroupStore(DB).IsGroupPluginDisabled(10, 20); err != nil || disabled {
		t.Fatalf("isolated groups: %v %v", disabled, err)
	}
	rows, err := chats.GetGroupChatMessages(10, 0, now.Add(-time.Minute), now.Add(time.Minute), 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("owned chats: %d %v", len(rows), err)
	}
	rows, err = NewChatStore(DB).GetGroupChatMessages(10, 0, now.Add(-time.Minute), now.Add(time.Minute), 10)
	if err != nil || len(rows) != 0 {
		t.Fatalf("isolated chats: %d %v", len(rows), err)
	}
	if row, err := affinity.GetAIAffinity(1, 10); err != nil || row.Score != 51 {
		t.Fatalf("owned affinity: %+v %v", row, err)
	}
	stats, err := usage.GetGroupCommandUsageStats(10, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := NewCommandUsageStore(DB).GetGroupCommandUsageStats(10, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalCalls != 1 || fresh.TotalCalls != 0 {
		t.Fatalf("usage: %+v %+v", stats, fresh)
	}
	if row, err := games.GetOrCreateGameRecord(1, 10, ""); err != nil || row.Score != 10 {
		t.Fatalf("owned game: %+v %v", row, err)
	}
}
