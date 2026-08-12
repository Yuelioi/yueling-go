package subscription

import (
	"strings"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/db"
)

func TestFormatSubscriptionListBoundsLongURL(t *testing.T) {
	longURL := "https://example.com/" + strings.Repeat("segment/", 20)
	got := formatSubscriptionList([]db.FeedSubscription{{ID: 7, Name: "项目动态", URL: longURL}})
	if !strings.Contains(got, "本群订阅（1/10）") || !strings.Contains(got, "ID 7 · 项目动态") {
		t.Fatalf("formatted list = %q", got)
	}
	if strings.Contains(got, longURL) || !strings.Contains(got, "…") {
		t.Fatalf("long URL was not truncated: %q", got)
	}
}

func TestFormatFeedHealthLine(t *testing.T) {
	location, _ := time.LoadLocation("Asia/Shanghai")
	failing := formatFeedHealthLine(db.FeedSubscription{
		ID: 7, Name: "项目动态", Enabled: true, ConsecutiveFailures: 3,
		LastError: "upstream timeout", NextCheckAt: time.Date(2026, 8, 13, 12, 30, 0, 0, location).Unix(),
	}, location)
	if !strings.Contains(failing, "异常 3 次") || !strings.Contains(failing, "08-13 12:30") || !strings.Contains(failing, "upstream timeout") {
		t.Fatalf("failing line=%q", failing)
	}
	paused := formatFeedHealthLine(db.FeedSubscription{ID: 8, Name: "暂停源"}, location)
	if !strings.Contains(paused, "已暂停") {
		t.Fatalf("paused line=%q", paused)
	}
}
