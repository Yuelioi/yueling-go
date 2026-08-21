package feed

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
)

func TestManagerAddPlatformUsesConfiguredRSSHubAndFeedTitle(t *testing.T) {
	initFeedTestDB(t)
	oldConfig := config.C
	config.C.Feed.RSSHubBase = "http://rsshub:1200"
	t.Cleanup(func() { config.C = oldConfig })
	manager := NewManager(func(rawURL string) (*Feed, error) {
		if rawURL != "http://rsshub:1200/bilibili/user/video/2267573/1" {
			t.Fatalf("platform URL = %q", rawURL)
		}
		return &Feed{Title: "示例 UP 主的投稿", Items: []Item{{Key: "current"}}}, nil
	})
	row, _, err := manager.AddPlatform(100, 1, PlatformBilibiliVideo, "2267573", "")
	if err != nil {
		t.Fatal(err)
	}
	if row.Name != "示例 UP 主的投稿" {
		t.Fatalf("platform subscription name = %q", row.Name)
	}
}

type recordingSender struct {
	groups []int64
	texts  []string
	err    error
}

func TestFormatPendingNotificationAppliesPerGroupLength(t *testing.T) {
	title := strings.Repeat("长", 200)
	items := []db.FeedPendingItem{{FeedName: "Tibo", Title: title, Link: "https://example.com/item"}}

	full := formatPendingNotification(items, 0)
	if !strings.Contains(full, title) || strings.Contains(full, "长长长…") {
		t.Fatalf("full notification lost content: %q", full)
	}
	compact := formatPendingNotification(items, 160)
	if strings.Contains(compact, title) || !strings.Contains(compact, strings.Repeat("长", 159)+"…") {
		t.Fatalf("compact notification did not apply limit: %q", compact)
	}
}

func TestDeliverySettingsValidateItemLength(t *testing.T) {
	for _, value := range []int{0, MinItemMaxChars, 320, MaxItemMaxChars} {
		if err := validateItemMaxChars(value); err != nil {
			t.Fatalf("rejected valid item_max_chars %d: %v", value, err)
		}
	}
	for _, value := range []int{-1, 1, MinItemMaxChars - 1, MaxItemMaxChars + 1} {
		if err := validateItemMaxChars(value); err == nil {
			t.Fatalf("accepted invalid item_max_chars %d", value)
		}
	}
}

func TestSetQuietHoursPreservesItemLength(t *testing.T) {
	initFeedTestDB(t)
	manager := NewManager(nil)
	if _, err := manager.SetDeliverySettings(100, true, "23:00", "08:00", 320); err != nil {
		t.Fatal(err)
	}
	setting, err := manager.SetQuietHours(100, false, "", "")
	if err != nil || setting.QuietEnabled || setting.ItemMaxChars != 320 {
		t.Fatalf("setting=%+v err=%v", setting, err)
	}
}

func (s *recordingSender) SendGroupText(groupID int64, text string) error {
	if s.err != nil {
		return s.err
	}
	s.groups = append(s.groups, groupID)
	s.texts = append(s.texts, text)
	return nil
}

func initFeedTestDB(t *testing.T) {
	t.Helper()
	testdb.Init(t)
}

func TestManagerAddStartsFromCurrentNewestItem(t *testing.T) {
	initFeedTestDB(t)
	manager := NewManager(func(string) (*Feed, error) {
		return &Feed{Title: "项目更新", Items: []Item{{Key: "current", Title: "当前版本"}}}, nil
	})
	row, parsed, err := manager.Add(100, 42, "HTTPS://EXAMPLE.COM/feed#fragment", "")
	if err != nil {
		t.Fatal(err)
	}
	if row.Name != "项目更新" || row.LastItemID != "current" || row.URL != "https://example.com/feed" || parsed.Title != "项目更新" {
		t.Fatalf("added row=%+v parsed=%+v", row, parsed)
	}
	if _, _, err := manager.Add(100, 42, "https://example.com/feed", "重复"); !errors.Is(err, ErrDuplicateSubscription) {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestManagerCheckSendsOnlyNewItemsAndAdvancesCursor(t *testing.T) {
	initFeedTestDB(t)
	row, err := db.CreateFeedSubscription(100, 42, "https://example.com/feed", "项目更新", "old")
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager(func(string) (*Feed, error) {
		return &Feed{Items: []Item{
			{Key: "new-2", Title: "版本 2", Link: "https://example.com/2"},
			{Key: "new-1", Title: "版本 1"},
			{Key: "old", Title: "旧版本"},
		}}, nil
	})
	sender := &recordingSender{}
	result, err := manager.CheckGroup(sender, 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.Updated != 1 || result.Items != 2 || result.Delivered != 2 || result.Queued != 0 || len(sender.texts) != 1 || strings.Contains(sender.texts[0], "旧版本") {
		t.Fatalf("result=%+v messages=%q", result, sender.texts)
	}
	rows, _ := db.ListFeedSubscriptions(100)
	if len(rows) != 1 || rows[0].ID != row.ID || rows[0].LastItemID != "new-2" {
		t.Fatalf("rows after check = %+v", rows)
	}
}

func TestManagerDisabledPluginSkipsBacklogAndAdvancesCursor(t *testing.T) {
	initFeedTestDB(t)
	row, err := db.CreateFeedSubscription(100, 42, "https://example.com/feed", "项目更新", "old")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetGroupPluginDisabled(100, catalog.PluginFeedSubscription, true); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(func(string) (*Feed, error) {
		return &Feed{Items: []Item{{Key: "new", Title: "新版本"}, {Key: "old", Title: "旧版本"}}}, nil
	})
	sender := &recordingSender{}
	result, err := manager.CheckGroup(sender, 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.Updated != 0 || len(sender.texts) != 0 {
		t.Fatalf("disabled result=%+v messages=%q", result, sender.texts)
	}
	rows, _ := db.ListFeedSubscriptions(100)
	if len(rows) != 1 || rows[0].ID != row.ID || rows[0].LastItemID != "new" {
		t.Fatalf("disabled cursor was not advanced: %+v", rows)
	}
}

func TestManagerQueuesAndRetriesWhenSendFails(t *testing.T) {
	initFeedTestDB(t)
	_, err := db.CreateFeedSubscription(100, 42, "https://example.com/feed", "项目更新", "old")
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager(func(string) (*Feed, error) {
		return &Feed{Items: []Item{{Key: "new", Title: "新版本"}, {Key: "old", Title: "旧版本"}}}, nil
	})
	result, err := manager.CheckGroup(&recordingSender{err: errors.New("offline")}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed != 1 {
		t.Fatalf("result = %+v", result)
	}
	rows, _ := db.ListFeedSubscriptions(100)
	if rows[0].LastItemID != "new" {
		t.Fatalf("cursor did not advance with durable outbox: %+v", rows[0])
	}
	if pending, _ := db.CountFeedPendingItems(100); pending != 1 || result.Queued != 1 {
		t.Fatalf("pending=%d result=%+v", pending, result)
	}

	sender := &recordingSender{}
	result, err = manager.CheckGroup(sender, 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.Delivered != 1 || result.Queued != 0 || len(sender.texts) != 1 || !strings.Contains(sender.texts[0], "新版本") {
		t.Fatalf("retry result=%+v messages=%q", result, sender.texts)
	}
	if pending, _ := db.CountFeedPendingItems(100); pending != 0 {
		t.Fatalf("pending after retry=%d", pending)
	}
}

func TestManagerQuietHoursQueueThenDeliverMergedUpdate(t *testing.T) {
	initFeedTestDB(t)
	oldConfig := config.C
	config.C.Bot.Timezone = "Asia/Shanghai"
	t.Cleanup(func() { config.C = oldConfig })
	if _, err := db.CreateFeedSubscription(100, 42, "https://example.com/a", "A源", "old-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateFeedSubscription(100, 42, "https://example.com/b", "B源", "old-b"); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(func(rawURL string) (*Feed, error) {
		if strings.HasSuffix(rawURL, "/a") {
			return &Feed{Items: []Item{{Key: "new-a", Title: "A更新"}, {Key: "old-a"}}}, nil
		}
		return &Feed{Items: []Item{{Key: "new-b", Title: "B更新"}, {Key: "old-b"}}}, nil
	})
	location, _ := time.LoadLocation("Asia/Shanghai")
	manager.now = func() time.Time { return time.Date(2026, 8, 13, 23, 30, 0, 0, location) }
	if _, err := manager.SetQuietHours(100, true, "23:00", "08:00"); err != nil {
		t.Fatal(err)
	}
	sender := &recordingSender{}
	result, err := manager.CheckGroup(sender, 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.Items != 2 || result.Delivered != 0 || result.Queued != 2 || len(sender.texts) != 0 {
		t.Fatalf("quiet result=%+v messages=%q", result, sender.texts)
	}

	manager.now = func() time.Time { return time.Date(2026, 8, 14, 8, 1, 0, 0, location) }
	result, err = manager.CheckGroup(sender, 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.Delivered != 2 || result.Queued != 0 || len(sender.texts) != 1 ||
		!strings.Contains(sender.texts[0], "【A源】") || !strings.Contains(sender.texts[0], "【B源】") {
		t.Fatalf("delivery result=%+v messages=%q", result, sender.texts)
	}
}

func TestManagerPollAllUsesExponentialFailureBackoff(t *testing.T) {
	initFeedTestDB(t)
	if _, err := db.CreateFeedSubscription(100, 42, "https://example.com/feed", "故障源", "old"); err != nil {
		t.Fatal(err)
	}
	calls := 0
	manager := NewManager(func(string) (*Feed, error) {
		calls++
		return nil, errors.New("upstream unavailable")
	})
	base := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return base }
	if result, err := manager.PollAll(&recordingSender{}); err != nil || result.Failed != 1 || calls != 1 {
		t.Fatalf("first poll result=%+v err=%v calls=%d", result, err, calls)
	}
	rows, _ := db.ListFeedSubscriptions(100)
	if rows[0].ConsecutiveFailures != 1 || rows[0].NextCheckAt != base.Add(10*time.Minute).Unix() || rows[0].LastError == "" {
		t.Fatalf("first failure status=%+v", rows[0])
	}

	manager.now = func() time.Time { return base.Add(5 * time.Minute) }
	if result, err := manager.PollAll(&recordingSender{}); err != nil || result.Checked != 0 || calls != 1 {
		t.Fatalf("early poll result=%+v err=%v calls=%d", result, err, calls)
	}
	manager.now = func() time.Time { return base.Add(11 * time.Minute) }
	if result, err := manager.PollAll(&recordingSender{}); err != nil || result.Failed != 1 || calls != 2 {
		t.Fatalf("retry poll result=%+v err=%v calls=%d", result, err, calls)
	}
	rows, _ = db.ListFeedSubscriptions(100)
	if rows[0].ConsecutiveFailures != 2 || rows[0].NextCheckAt != base.Add(31*time.Minute).Unix() {
		t.Fatalf("second failure status=%+v", rows[0])
	}
}

func TestQuietHoursValidationAndBoundaries(t *testing.T) {
	if start, end, err := ParseQuietHours("23:00", "08:00"); err != nil || start != "23:00" || end != "08:00" {
		t.Fatalf("parsed=%s-%s err=%v", start, end, err)
	}
	for _, values := range [][2]string{{"23", "08:00"}, {"25:00", "08:00"}, {"08:00", "08:00"}} {
		if _, _, err := ParseQuietHours(values[0], values[1]); err == nil {
			t.Fatalf("accepted invalid quiet hours %v", values)
		}
	}
	setting := db.FeedGroupSetting{QuietEnabled: true, QuietStart: "23:00", QuietEnd: "08:00"}
	location, _ := time.LoadLocation("Asia/Shanghai")
	if !isQuietTime(setting, time.Date(2026, 8, 13, 23, 0, 0, 0, location)) ||
		isQuietTime(setting, time.Date(2026, 8, 14, 8, 0, 0, 0, location)) {
		t.Fatal("overnight quiet boundaries are incorrect")
	}
}

func TestManagerPauseSkipsFetchAndResumeMakesSourceDue(t *testing.T) {
	initFeedTestDB(t)
	row, err := db.CreateFeedSubscription(100, 42, "https://example.com/feed", "项目更新", "old")
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager(func(string) (*Feed, error) {
		return &Feed{Items: []Item{{Key: "new", Title: "新版本"}, {Key: "old"}}}, nil
	})
	if _, err := manager.SetEnabled(row.ID, 100, false); err != nil {
		t.Fatal(err)
	}
	sender := &recordingSender{}
	result, err := manager.CheckGroup(sender, 100)
	if err != nil || result.Checked != 0 || len(sender.texts) != 0 {
		t.Fatalf("paused result=%+v err=%v messages=%q", result, err, sender.texts)
	}
	resumed, err := manager.SetEnabled(row.ID, 100, true)
	if err != nil || resumed.NextCheckAt != 0 {
		t.Fatalf("resumed=%+v err=%v", resumed, err)
	}
	result, err = manager.CheckGroup(sender, 100)
	if err != nil || result.Checked != 1 || result.Delivered != 1 || len(sender.texts) != 1 {
		t.Fatalf("resumed result=%+v err=%v messages=%q", result, err, sender.texts)
	}
}

func TestManagerPauseWaitsForInFlightCheckAndClearsItsOutbox(t *testing.T) {
	initFeedTestDB(t)
	row, err := db.CreateFeedSubscription(100, 42, "https://example.com/feed", "项目更新", "old")
	if err != nil {
		t.Fatal(err)
	}
	fetchStarted := make(chan struct{})
	releaseFetch := make(chan struct{})
	manager := NewManager(func(string) (*Feed, error) {
		close(fetchStarted)
		<-releaseFetch
		return &Feed{Items: []Item{{Key: "new", Title: "新版本"}, {Key: "old"}}}, nil
	})
	checkDone := make(chan CheckResult, 1)
	go func() {
		result, _ := manager.CheckGroup(&recordingSender{err: errors.New("qq unavailable")}, 100)
		checkDone <- result
	}()
	<-fetchStarted

	type pauseResult struct {
		row *db.FeedSubscription
		err error
	}
	pauseDone := make(chan pauseResult, 1)
	go func() {
		paused, pauseErr := manager.SetEnabled(row.ID, 100, false)
		pauseDone <- pauseResult{row: paused, err: pauseErr}
	}()
	select {
	case result := <-pauseDone:
		t.Fatalf("pause completed during in-flight check: %+v", result)
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseFetch)
	if result := <-checkDone; result.Queued != 1 || result.Failed != 1 {
		t.Fatalf("in-flight check result=%+v", result)
	}
	paused := <-pauseDone
	if paused.err != nil || paused.row.Enabled {
		t.Fatalf("pause result=%+v err=%v", paused.row, paused.err)
	}
	if pending, err := db.CountFeedPendingItems(100); err != nil || pending != 0 {
		t.Fatalf("pending after pause=%d err=%v", pending, err)
	}
}
