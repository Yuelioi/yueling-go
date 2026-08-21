package db

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestFeedSubscriptionCRUDAndGroupIsolation(t *testing.T) {
	initPostgresForTest(t)

	row, err := CreateFeedSubscription(100, 1, "https://example.com/feed.xml", "示例", "first")
	if err != nil {
		t.Fatal(err)
	}
	if row.ID == 0 || !row.Enabled {
		t.Fatalf("created row = %+v", row)
	}
	if count, err := CountFeedSubscriptions(100); err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if err := UpdateFeedSubscriptionCursor(row.ID, "second"); err != nil {
		t.Fatal(err)
	}
	rows, err := ListFeedSubscriptions(100)
	if err != nil || len(rows) != 1 || rows[0].LastItemID != "second" {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	if err := DeleteFeedSubscription(row.ID, 200); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("wrong-group delete error = %v", err)
	}
	if err := DeleteFeedSubscription(row.ID, 100); err != nil {
		t.Fatal(err)
	}
}

func TestFeedSubscriptionRejectsDuplicateURLPerGroup(t *testing.T) {
	initPostgresForTest(t)

	if _, err := CreateFeedSubscription(100, 1, "https://example.com/feed", "one", "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateFeedSubscription(100, 2, "https://example.com/feed", "two", "b"); err == nil {
		t.Fatal("duplicate feed URL was accepted in one group")
	}
	if _, err := CreateFeedSubscription(200, 2, "https://example.com/feed", "two", "b"); err != nil {
		t.Fatalf("same URL should be allowed in another group: %v", err)
	}
}

func TestFeedOutboxHealthAndQuietSettings(t *testing.T) {
	initPostgresForTest(t)

	row, err := CreateFeedSubscription(100, 1, "https://example.com/feed", "示例", "old")
	if err != nil {
		t.Fatal(err)
	}
	inserted, err := RecordFeedFetchSuccess(row.ID, "new", 1000, 1600, []FeedPendingItem{{
		SubscriptionID: row.ID, GroupID: 100, FeedName: "示例", ItemKey: "new", Title: "新内容", QueuedAt: 1000,
	}})
	if err != nil || inserted != 1 {
		t.Fatalf("inserted=%d err=%v", inserted, err)
	}
	if pending, err := CountFeedPendingItems(100); err != nil || pending != 1 {
		t.Fatalf("pending=%d err=%v", pending, err)
	}
	rows, err := ListFeedSubscriptions(100)
	if err != nil || rows[0].LastItemID != "new" || rows[0].LastSuccessAt != 1000 || rows[0].NextCheckAt != 1600 {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	if err := RecordFeedFetchFailure(row.ID, 2, "timeout", 1700, 2900); err != nil {
		t.Fatal(err)
	}
	rows, _ = ListFeedSubscriptions(100)
	if rows[0].ConsecutiveFailures != 2 || rows[0].LastError != "timeout" || rows[0].NextCheckAt != 2900 {
		t.Fatalf("failure status=%+v", rows[0])
	}
	setting, err := SetFeedGroupSetting(100, true, "23:00", "08:00", 320)
	if err != nil || !setting.QuietEnabled || setting.ItemMaxChars != 320 {
		t.Fatalf("setting=%+v err=%v", setting, err)
	}
	loaded, err := GetFeedGroupSetting(100)
	if err != nil || loaded.QuietStart != "23:00" || loaded.QuietEnd != "08:00" || loaded.ItemMaxChars != 320 {
		t.Fatalf("loaded=%+v err=%v", loaded, err)
	}
	disabled, err := SetFeedSubscriptionEnabled(row.ID, 100, false)
	if err != nil || disabled.Enabled {
		t.Fatalf("disabled=%+v err=%v", disabled, err)
	}
	if pending, err := CountFeedPendingItems(100); err != nil || pending != 0 {
		t.Fatalf("pending after pause=%d err=%v", pending, err)
	}
	if all, err := ListFeedSubscriptions(100); err != nil || len(all) != 1 || all[0].Enabled {
		t.Fatalf("all feeds=%+v err=%v", all, err)
	}
	if active, err := ListActiveFeedSubscriptions(100); err != nil || len(active) != 0 {
		t.Fatalf("active feeds=%+v err=%v", active, err)
	}
	inserted, err = RecordFeedFetchSuccess(row.ID, "paused-item", 3000, 3600, []FeedPendingItem{{
		SubscriptionID: row.ID, GroupID: 100, FeedName: "示例", ItemKey: "paused-item", Title: "暂停期间内容", QueuedAt: 3000,
	}})
	if err != nil || inserted != 0 {
		t.Fatalf("paused fetch inserted=%d err=%v", inserted, err)
	}
	if pending, err := CountFeedPendingItems(100); err != nil || pending != 0 {
		t.Fatalf("pending after paused fetch=%d err=%v", pending, err)
	}
	enabled, err := SetFeedSubscriptionEnabled(row.ID, 100, true)
	if err != nil || !enabled.Enabled || enabled.NextCheckAt != 0 {
		t.Fatalf("enabled=%+v err=%v", enabled, err)
	}
	if err := DeleteFeedSubscription(row.ID, 100); err != nil {
		t.Fatal(err)
	}
	if pending, err := CountFeedPendingItems(100); err != nil || pending != 0 {
		t.Fatalf("pending after delete=%d err=%v", pending, err)
	}
}
