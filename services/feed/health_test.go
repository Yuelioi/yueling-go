package feed

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
)

func TestHealthOutcomeDoesNotExposeCredentialsOrConflateFailures(t *testing.T) {
	for _, test := range []struct{ message, status string }{
		{"HTTP 401: token=secret", "authentication"},
		{"HTTP 500: could not authenticate you; auth_token=secret", "authentication"},
		{"HTTP 403: secret", "forbidden"},
		{"HTTP 429: secret", "rate_limited"},
		{"HTTP 404: secret", "not_found"},
		{"RSSHub 连接超时 secret", "timeout"},
		{"HTTP 500: secret", "unavailable"},
	} {
		status, message := healthOutcome(errors.New(test.message))
		if status != test.status || strings.Contains(message, "secret") {
			t.Fatalf("outcome = %s %s", status, message)
		}
	}
	status, message := healthOutcome(nil)
	if status != "readable" || !strings.Contains(message, "缓存") {
		t.Fatalf("success claims token validity: %s %s", status, message)
	}
}

func TestDiagnosisDeduplicatesURLsIncludesPausedAndDoesNotNeedSender(t *testing.T) {
	var mu sync.Mutex
	calls := map[string]int{}
	m := NewManager(nil, func(url string) (*Feed, error) {
		mu.Lock()
		calls[url]++
		mu.Unlock()
		if url == "bad" {
			return nil, errors.New("HTTP 401 secret")
		}
		return &Feed{}, nil
	})
	rows := []db.FeedSubscription{{ID: 1, GroupID: 100, URL: "ok", Enabled: true}, {ID: 2, GroupID: 200, URL: "ok", Enabled: false}, {ID: 3, GroupID: 200, URL: "bad"}}
	result := m.diagnoseRows(context.Background(), rows)
	if len(result) != 3 || calls["ok"] != 1 || calls["bad"] != 1 || result[1].Status != "readable" || result[2].Status != "authentication" {
		t.Fatalf("results=%+v calls=%v", result, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result = m.diagnoseRows(ctx, rows)
	if result[0].Status != "timeout" || calls["ok"] != 1 {
		t.Fatalf("cancelled check fetched: %+v", result)
	}
}

func TestDiagnosisLeavesSubscriptionCursorScheduleAndOutboxUntouched(t *testing.T) {
	initFeedTestDB(t)
	store := db.NewFeedStore(testdb.Database)
	original := db.FeedSubscription{GroupID: 100, URL: "https://example.com/rss", Name: "test", LastItemID: "old", Enabled: true, ConsecutiveFailures: 2, LastError: "old error", LastCheckedAt: 123, NextCheckAt: 456}
	if err := testdb.Database.Create(&original).Error; err != nil {
		t.Fatal(err)
	}
	m := NewManager(store, func(string) (*Feed, error) { return &Feed{Items: []Item{{Key: "new", Title: "new"}}}, nil })
	result, err := m.Diagnose(context.Background(), 100, original.ID)
	if err != nil || len(result) != 1 || result[0].Status != "readable" {
		t.Fatalf("%+v %v", result, err)
	}
	var after db.FeedSubscription
	if err := testdb.Database.First(&after, original.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, after) {
		t.Fatalf("subscription changed: before=%+v after=%+v", original, after)
	}
	count, err := store.CountFeedPendingItems(100)
	if err != nil || count != 0 {
		t.Fatalf("outbox changed: %d %v", count, err)
	}
	result, err = m.Diagnose(context.Background(), 200, original.ID)
	if err != nil || len(result) != 0 {
		t.Fatalf("cross-group result: %+v %v", result, err)
	}
}

func TestDiagnosisHonorsRequestCancellation(t *testing.T) {
	m := NewManager(nil, nil)
	m.fetchContext = func(ctx context.Context, _ string) (*Feed, error) { <-ctx.Done(); return nil, ctx.Err() }
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	result := m.diagnoseRows(ctx, []db.FeedSubscription{{ID: 1, URL: "slow"}})
	if len(result) != 1 || result[0].Status != "timeout" {
		t.Fatal(result)
	}
}
