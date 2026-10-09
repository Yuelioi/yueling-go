package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
	"github.com/Yuelioi/yueling-go/services/chatinsights"
)

func TestChatInsightsReturnsEmptyGroup(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)
	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/chat-insights?group_id=100&period=today", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		OK              bool                    `json:"ok"`
		RetentionDays   int                     `json:"retention_days"`
		RetentionPolicy string                  `json:"retention_policy"`
		Summary         db.GroupChatSummary     `json:"summary"`
		Words           []db.GroupChatWordCount `json:"words"`
		Users           []chatinsights.User     `json:"users"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.OK || got.RetentionDays != 0 || got.RetentionPolicy != "indefinite" || got.Summary.Total != 0 || len(got.Words) != 0 || len(got.Users) != 0 {
		t.Fatalf("response=%+v", got)
	}
}

func TestChatInsightsReturnsGroupWordsAndUserExpressions(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)
	now := bot.Now()
	if err := db.NewChatStore(testdb.Database).SaveGroupChatMessages([]db.GroupChatMessage{
		{GroupID: 100, MessageID: 1, UserID: 10, Nickname: "甲", Content: "今晚一起吃火锅", CreatedAt: now.Add(-3 * time.Minute).Unix()},
		{GroupID: 100, MessageID: 2, UserID: 10, Nickname: "甲", Content: "今晚一起吃火锅", CreatedAt: now.Add(-2 * time.Minute).Unix()},
		{GroupID: 100, MessageID: 3, UserID: 20, Nickname: "乙", Content: "火锅很好吃", CreatedAt: now.Add(-time.Minute).Unix()},
	}); err != nil {
		t.Fatal(err)
	}
	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/chat-insights?group_id=100&period=today", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		StartAt int64                   `json:"start_at"`
		Summary db.GroupChatSummary     `json:"summary"`
		Words   []db.GroupChatWordCount `json:"words"`
		Users   []chatinsights.User     `json:"users"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if got.StartAt != day.Unix() || got.Summary.Total != 3 || len(got.Words) == 0 || len(got.Users) != 2 {
		t.Fatalf("response=%+v", got)
	}
	if got.Users[0].UserID != 10 || len(got.Users[0].Phrases) != 1 || got.Users[0].Phrases[0].Text != "今晚一起吃火锅" {
		t.Fatalf("lead user=%+v", got.Users[0])
	}
}

func TestChatInsightsRejectsInvalidScopeAndPeriod(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)
	for _, path := range []string{
		"/api/webui/chat-insights",
		"/api/webui/chat-insights?group_id=0",
		"/api/webui/chat-insights?group_id=no",
		"/api/webui/chat-insights?group_id=100&period=year",
	} {
		rec := testAPIRequest(t, s, http.MethodGet, path, "", cookie)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("path=%s code=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestChatHistoryManagementIsGroupScoped(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)
	now := time.Now()
	if err := db.NewChatStore(testdb.Database).SaveGroupChatMessages([]db.GroupChatMessage{
		{GroupID: 100, MessageID: 1, UserID: 10, Content: "旧消息", CreatedAt: now.Add(-100 * 24 * time.Hour).Unix()},
		{GroupID: 100, MessageID: 2, UserID: 10, Content: "新消息", CreatedAt: now.Unix()},
		{GroupID: 200, MessageID: 1, UserID: 20, Content: "其他群", CreatedAt: now.Add(-100 * 24 * time.Hour).Unix()},
	}); err != nil {
		t.Fatal(err)
	}

	cutoff := now.Add(-90 * 24 * time.Hour).Unix()
	path := fmt.Sprintf("/api/webui/groups/100/chat-history?before_at=%d", cutoff)
	rec := testAPIRequest(t, s, http.MethodGet, path, "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"total":2`) || !strings.Contains(rec.Body.String(), `"matched":1`) {
		t.Fatalf("preview code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = testAPIRequest(t, s, http.MethodDelete, "/api/webui/groups/100/chat-history", fmt.Sprintf(`{"before_at":%d}`, cutoff), cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"deleted":1`) || !strings.Contains(rec.Body.String(), `"remaining":1`) {
		t.Fatalf("delete code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = testAPIRequest(t, s, http.MethodDelete, "/api/webui/groups/100/chat-history", `{"all":true}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"deleted":1`) || !strings.Contains(rec.Body.String(), `"remaining":0`) {
		t.Fatalf("delete all code=%d body=%s", rec.Code, rec.Body.String())
	}
	stats, err := db.NewChatStore(testdb.Database).GetGroupChatHistoryStats(200, 0)
	if err != nil || stats.Total != 1 {
		t.Fatalf("other group stats=%+v err=%v", stats, err)
	}
}

func TestChatHistoryManagementRejectsUnsafeRequests(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)
	for _, test := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/webui/groups/no/chat-history", ""},
		{http.MethodGet, "/api/webui/groups/100/chat-history?before_at=no", ""},
		{http.MethodDelete, "/api/webui/groups/100/chat-history", `{}`},
		{http.MethodDelete, "/api/webui/groups/100/chat-history", `{"all":true,"before_at":1}`},
		{http.MethodDelete, "/api/webui/groups/100/chat-history", `{"all":true,"before_at":-1}`},
		{http.MethodDelete, "/api/webui/groups/100/chat-history", `{"all":false}`},
		{http.MethodDelete, "/api/webui/groups/100/chat-history", `{"before_at":-1}`},
		{http.MethodDelete, "/api/webui/groups/100/chat-history", fmt.Sprintf(`{"before_at":%d}`, time.Now().Add(23*time.Hour).Unix())},
	} {
		rec := testAPIRequest(t, s, test.method, test.path, test.body, cookie)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s %s code=%d body=%s", test.method, test.path, rec.Code, rec.Body.String())
		}
	}
}
