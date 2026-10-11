package webui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
	"github.com/Yuelioi/yueling-go/services/feed"
)

func TestAdministrationIncludesLiveUnconfiguredAndStoredOfflineGroups(t *testing.T) {
	initWebUITestDB(t)
	for _, value := range []any{
		&db.GroupAISetting{GroupID: 100, StylePrompt: "brief"},
		&db.GroupPluginDisabled{GroupID: 100, PluginID: 4},
		&db.GroupJoinSetting{GroupID: 100, Mode: db.JoinModeDisabled},
		&db.FeedSubscription{GroupID: 300, URL: "https://example.com/rss", Name: "offline", Enabled: true, ConsecutiveFailures: 2},
		&db.GroupKnowledge{GroupID: 0, Title: "shared", Content: "shared"},
		&db.DailyDigest{GroupID: 100, Enabled: true, SendTime: "21:30"},
	} {
		if err := testdb.Database.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := newTestServer()
	s.resolveGroupLister = func() groupLister {
		return stubGroupLister{groups: []bot.GroupInfo{{GroupID: 100, GroupName: "alpha"}, {GroupID: 200, GroupName: "empty"}}}
	}
	cookie := login(t, s)
	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/group-administration", "", cookie)
	var got struct {
		Groups []struct {
			db.GroupAdministration
			GroupName string `json:"group_name"`
			Connected bool   `json:"connected"`
		}
		Warning string `json:"warning"`
	}
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Groups) != 4 || got.Groups[0].KnowledgeCount != 1 || !got.Groups[1].CustomStyle || got.Groups[1].JoinMode != "disabled" || got.Groups[1].DigestTime != "21:30" || len(got.Groups[1].DisabledPlugins) != 1 || got.Groups[2].JoinMode != "inherit" || got.Groups[3].FailingFeeds != 1 || got.Groups[3].Connected {
		t.Fatalf("overview=%+v", got)
	}
	s.resolveGroupLister = func() groupLister { return nil }
	rec = testAPIRequest(t, s, http.MethodGet, "/api/webui/group-administration", "", cookie)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Bot 未连接") || !strings.Contains(rec.Body.String(), `"group_id":300`) {
		t.Fatalf("offline=%s", rec.Body.String())
	}
}

func TestAllKnowledgeIsAdminOnlyAndSeparateFromSharedScope(t *testing.T) {
	initWebUITestDB(t)
	for _, id := range []int64{0, 100, 200} {
		if err := testdb.Database.Create(&db.GroupKnowledge{GroupID: id, Title: "entry", Content: "text"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := newTestServer()
	cookie := login(t, s)
	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/knowledge", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated=%d", rec.Code)
	}
	for _, tc := range []struct {
		path  string
		count int
	}{{"/api/webui/knowledge", 3}, {"/api/webui/knowledge?group_id=0", 1}, {"/api/webui/knowledge?group_id=100", 1}} {
		rec = testAPIRequest(t, s, http.MethodGet, tc.path, "", cookie)
		var got struct {
			Knowledge []db.GroupKnowledge `json:"knowledge"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != 200 || len(got.Knowledge) != tc.count {
			t.Fatalf("%s: %s %v", tc.path, rec.Body.String(), err)
		}
	}
}

func TestChatOverviewKeepsGroupTotalsSeparateAndExcludesOtherPeriods(t *testing.T) {
	initWebUITestDB(t)
	now := bot.Now()
	if err := testdb.Database.Create(&[]db.GroupChatMessage{
		{GroupID: 100, MessageID: 1, UserID: 1, Content: "text", CreatedAt: now.Add(-time.Minute).Unix()},
		{GroupID: 200, MessageID: 1, UserID: 1, Content: "", CreatedAt: now.Add(-time.Minute).Unix()},
		{GroupID: 100, MessageID: 2, UserID: 2, Content: "excluded", StatExcluded: true, CreatedAt: now.Add(-time.Minute).Unix()},
		{GroupID: 100, MessageID: 3, UserID: 3, Content: "old", CreatedAt: now.Add(-40 * 24 * time.Hour).Unix()},
	}).Error; err != nil {
		t.Fatal(err)
	}
	s := newTestServer()
	cookie := login(t, s)
	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/chat-overview?period=7days", "", cookie)
	var got struct {
		Groups []db.GroupChatOverview `json:"groups"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != 200 || len(got.Groups) != 2 {
		t.Fatalf("%s %v", rec.Body.String(), err)
	}
	if got.Groups[0].Total != 1 || got.Groups[0].TextTotal != 1 || got.Groups[1].Participants != 1 || got.Groups[1].TextTotal != 0 {
		t.Fatal(got)
	}
}

func TestFeedDiagnosisWorksWithoutBotAndRequiresAuthentication(t *testing.T) {
	initWebUITestDB(t)
	if err := testdb.Database.Create(&db.FeedSubscription{GroupID: 100, URL: "https://example.com/rss", Name: "test"}).Error; err != nil {
		t.Fatal(err)
	}
	s := newTestServer()
	s.feeds = feed.NewManager(db.NewFeedStore(testdb.Database), func(string) (*feed.Feed, error) { return &feed.Feed{}, nil })
	rec := testAPIRequest(t, s, http.MethodPost, "/api/webui/feeds/diagnose", `{}`, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated=%d", rec.Code)
	}
	cookie := login(t, s)
	rec = testAPIRequest(t, s, http.MethodPost, "/api/webui/feeds/diagnose", `{"group_id":100}`, cookie)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"status":"readable"`) {
		t.Fatal(rec.Body.String())
	}
	rec = testAPIRequest(t, s, http.MethodPost, "/api/webui/feeds/diagnose", `{"group_id":-1}`, cookie)
	if rec.Code != 400 {
		t.Fatal(rec.Body.String())
	}
}
