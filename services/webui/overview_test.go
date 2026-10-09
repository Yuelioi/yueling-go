package webui

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
)

func TestOverviewReturnsOperationalCounts(t *testing.T) {
	initWebUITestDB(t)
	withTestAffinityConfig(t)
	s := newTestServer()
	s.resolveGroupLister = func() groupLister {
		return stubGroupLister{groups: []bot.GroupInfo{{GroupID: 100}, {GroupID: 200}}}
	}
	cookie := login(t, s)

	if _, err := db.NewAffinityStore(testdb.Database).UpdateAIAffinity(1, 100, "alice", 40, -30, 0, 80, "seed"); err != nil {
		t.Fatal(err)
	}
	if err := testdb.Database.Create(&db.SemanticMemory{UserID: 1, Content: "偏好", Score: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := db.NewScheduleStore(testdb.Database).UpsertDailyDigest(100, 1, "21:30", "30 21 * * *", 80); err != nil {
		t.Fatal(err)
	}
	if _, err := db.NewFeedStore(testdb.Database).CreateFeedSubscription(100, 1, "https://example.com/feed.xml", "更新", "current"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.NewKnowledgeStore(testdb.Database).CreateGroupKnowledge(100, 1, "规则", "文明交流", ""); err != nil {
		t.Fatal(err)
	}

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/overview", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, fragment := range []string{`"bot_connected":true`, `"group_count":2`, `"low_affinity_count":1`, `"memory_count":1`, `"digest_count":1`, `"feed_count":1`, `"knowledge_count":1`} {
		if !strings.Contains(rec.Body.String(), fragment) {
			t.Fatalf("body=%s, want %s", rec.Body.String(), fragment)
		}
	}
}
