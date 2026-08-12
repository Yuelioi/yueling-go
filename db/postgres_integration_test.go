package db

import (
	"os"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/config"
)

func TestPostgresZhparserChatQueries(t *testing.T) {
	dsn := os.Getenv("YUELING_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set YUELING_TEST_DATABASE_DSN to run PostgreSQL integration tests")
	}
	oldDB := DB
	if err := Init(config.DatabaseConfig{DSN: dsn, MaxOpen: 5, MaxIdle: 1, ConnMaxLifetime: 5}); err != nil {
		t.Fatal(err)
	}
	openedDB := DB
	t.Cleanup(func() {
		openedDB.Where("group_id = ?", int64(9_202_608_13)).Delete(&FeedPendingItem{})
		openedDB.Where("group_id = ?", int64(9_202_608_13)).Delete(&FeedSubscription{})
		openedDB.Where("group_id = ?", int64(9_202_608_13)).Delete(&FeedGroupSetting{})
		openedDB.Where("group_id = ?", int64(9_202_608_13)).Delete(&GroupChatMessage{})
		openedDB.Where("group_id = ?", int64(9_202_608_13)).Delete(&GroupKnowledge{})
		if sqlDB, err := openedDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		DB = oldDB
	})

	// Applying migrations twice must be harmless.
	if err := runMigrations(DB); err != nil {
		t.Fatalf("idempotent migrations: %v", err)
	}

	start := time.Unix(1_800_000_000, 0)
	groupID := int64(9_202_608_13)
	if err := SaveGroupChatMessages([]GroupChatMessage{
		{GroupID: groupID, MessageID: 1, UserID: 10, Nickname: "甲", Content: "今晚一起吃火锅", CreatedAt: start.Unix()},
		{GroupID: groupID, MessageID: 2, UserID: 10, Nickname: "甲", Content: "这个火锅真的很好吃", CreatedAt: start.Add(time.Minute).Unix()},
		{GroupID: groupID, MessageID: 3, UserID: 20, Nickname: "乙", Content: "100%好吃", CreatedAt: start.Add(2 * time.Minute).Unix()},
		{GroupID: groupID, MessageID: 4, UserID: 20, Nickname: "乙", Content: "今日词云", StatExcluded: true, CreatedAt: start.Add(3 * time.Minute).Unix()},
	}); err != nil {
		t.Fatal(err)
	}

	end := start.Add(time.Hour)
	summary, err := GetGroupChatSummary(groupID, 0, start, end)
	if err != nil || summary.Total != 3 || summary.Participants != 2 {
		t.Fatalf("summary=%+v err=%v", summary, err)
	}
	words, err := GetGroupChatTopWords(groupID, 0, start, end, 100)
	if err != nil {
		t.Fatal(err)
	}
	foundHotpot := false
	for _, word := range words {
		if word.Text == "火锅" && word.Count == 2 {
			foundHotpot = true
		}
	}
	if !foundHotpot {
		t.Fatalf("zhparser words=%+v, want 火锅 in two messages", words)
	}

	users, err := FindGroupChatUsersSaying(groupID, start, end, "100%", 8)
	if err != nil || len(users) != 1 || users[0].UserID != 20 || users[0].Count != 1 {
		t.Fatalf("literal trigram search=%+v err=%v", users, err)
	}
	var fullTextMatches int64
	if err := DB.Model(&GroupChatMessage{}).
		Where("group_id = ? AND search_vector @@ plainto_tsquery('public.chinese_zhparser', ?)", groupID, "火锅").
		Count(&fullTextMatches).Error; err != nil || fullTextMatches != 2 {
		t.Fatalf("full text matches=%d err=%v", fullTextMatches, err)
	}

	if _, err := CreateGroupKnowledge(groupID, 10, "入群规则", "新成员入群后需要把群名片改为游戏昵称", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateGroupKnowledge(groupID, 10, "活动安排", "周六晚上八点组织副本活动", ""); err != nil {
		t.Fatal(err)
	}
	knowledge, err := SearchGroupKnowledge(groupID, "新成员的群名片有什么要求", 5)
	if err != nil || len(knowledge) == 0 || knowledge[0].Title != "入群规则" {
		t.Fatalf("knowledge=%+v err=%v", knowledge, err)
	}

	feedRow, err := CreateFeedSubscription(groupID, 10, "https://example.com/integration.xml", "集成源", "old")
	if err != nil {
		t.Fatal(err)
	}
	inserted, err := RecordFeedFetchSuccess(feedRow.ID, "new", start.Unix(), start.Add(10*time.Minute).Unix(), []FeedPendingItem{{
		SubscriptionID: feedRow.ID, GroupID: groupID, FeedName: feedRow.Name,
		ItemKey: "new", Title: "新的订阅内容", Link: "https://example.com/new", QueuedAt: start.Unix(),
	}})
	if err != nil || inserted != 1 {
		t.Fatalf("feed outbox inserted=%d err=%v", inserted, err)
	}
	if _, err := SetFeedGroupQuietHours(groupID, true, "23:00", "08:00"); err != nil {
		t.Fatal(err)
	}
	feedRows, err := ListFeedSubscriptions(groupID)
	if err != nil || len(feedRows) != 1 || feedRows[0].LastItemID != "new" || feedRows[0].LastSuccessAt != start.Unix() {
		t.Fatalf("feed rows=%+v err=%v", feedRows, err)
	}
	pending, err := ListFeedPendingItems(groupID, 10)
	if err != nil || len(pending) != 1 || pending[0].Title != "新的订阅内容" {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
	paused, err := SetFeedSubscriptionEnabled(feedRow.ID, groupID, false)
	if err != nil || paused.Enabled {
		t.Fatalf("paused=%+v err=%v", paused, err)
	}
	if pendingCount, err := CountFeedPendingItems(groupID); err != nil || pendingCount != 0 {
		t.Fatalf("pending after pause=%d err=%v", pendingCount, err)
	}
	if active, err := ListActiveFeedSubscriptions(groupID); err != nil || len(active) != 0 {
		t.Fatalf("active after pause=%+v err=%v", active, err)
	}
	resumed, err := SetFeedSubscriptionEnabled(feedRow.ID, groupID, true)
	if err != nil || !resumed.Enabled || resumed.NextCheckAt != 0 {
		t.Fatalf("resumed=%+v err=%v", resumed, err)
	}
}
