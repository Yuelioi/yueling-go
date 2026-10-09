package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
	"github.com/Yuelioi/yueling-go/services/feed"
)

func TestFeedAPILifecycleAndManualCheck(t *testing.T) {
	initWebUITestDB(t)
	oldConfig := config.C
	config.C.Feed.RSSHubBase = "https://rss.example"
	t.Cleanup(func() { config.C = oldConfig })
	items := []feed.Item{{Key: "current", Title: "当前版本"}}
	manager := feed.NewManagerWithTranslator(db.NewFeedStore(testdb.Database), func(string) (*feed.Feed, error) {
		return &feed.Feed{Title: "项目动态", Items: items}, nil
	}, func(_ context.Context, text string) (string, error) {
		return "中文：" + text, nil
	})

	s := newTestServer()
	s.feeds = manager
	sender := &stubFeedSender{}
	s.resolveFeedSender = func() feedSender { return sender }
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodPost, "/api/webui/groups/100/feeds", `{"url":"https://example.com/feed.xml","name":""}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"项目动态"`) || !strings.Contains(rec.Body.String(), `"latest_title":"当前版本"`) {
		t.Fatalf("add code=%d body=%s", rec.Code, rec.Body.String())
	}
	var added struct {
		Feed db.FeedSubscription `json:"feed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &added); err != nil || added.Feed.ID == 0 {
		t.Fatalf("decode added feed: row=%+v err=%v", added.Feed, err)
	}
	rec = testAPIRequest(t, s, http.MethodPost, "/api/webui/groups/100/feeds/platform", `{"platform":"bilibili_video","target":"2267573","name":"UP投稿"}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"url":"https://rss.example/bilibili/user/video/2267573/1"`) {
		t.Fatalf("platform add code=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = testAPIRequest(t, s, http.MethodGet, "/api/webui/feeds", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"group_id":100`) {
		t.Fatalf("list code=%d body=%s", rec.Code, rec.Body.String())
	}
	feedPath := "/api/webui/groups/100/feeds/" + strconv.FormatUint(uint64(added.Feed.ID), 10)
	rec = testAPIRequest(t, s, http.MethodPut, feedPath, `{"enabled":false}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("pause feed code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = testAPIRequest(t, s, http.MethodGet, "/api/webui/feeds", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("paused feed missing from list code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = testAPIRequest(t, s, http.MethodPut, feedPath, `{"enabled":true}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":true`) {
		t.Fatalf("resume feed code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = testAPIRequest(t, s, http.MethodPut, feedPath, `{"translate_to_chinese":true}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"translate_to_chinese":true`) {
		t.Fatalf("enable source translation code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = testAPIRequest(t, s, http.MethodPut, "/api/webui/groups/100/feeds/settings", `{"quiet_enabled":true,"quiet_start":"23:00","quiet_end":"08:00","item_max_chars":320,"poll_interval_minutes":15}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"quiet_enabled":true`) || !strings.Contains(rec.Body.String(), `"quiet_start":"23:00"`) || !strings.Contains(rec.Body.String(), `"item_max_chars":320`) || !strings.Contains(rec.Body.String(), `"poll_interval_minutes":15`) || strings.Contains(rec.Body.String(), `"translate_to_chinese"`) {
		t.Fatalf("set feed settings code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = testAPIRequest(t, s, http.MethodGet, "/api/webui/groups/100/feeds/settings", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"quiet_end":"08:00"`) || !strings.Contains(rec.Body.String(), `"item_max_chars":320`) || strings.Contains(rec.Body.String(), `"translate_to_chinese"`) || !strings.Contains(rec.Body.String(), `"pending_count":0`) {
		t.Fatalf("get feed settings code=%d body=%s", rec.Code, rec.Body.String())
	}
	// Turn quiet delivery back off before the manual delivery assertion below.
	rec = testAPIRequest(t, s, http.MethodPut, "/api/webui/groups/100/feeds/settings", `{"quiet_enabled":false}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"quiet_enabled":false`) || !strings.Contains(rec.Body.String(), `"item_max_chars":320`) || strings.Contains(rec.Body.String(), `"translate_to_chinese"`) {
		t.Fatalf("disable feed settings code=%d body=%s", rec.Code, rec.Body.String())
	}

	items = []feed.Item{{Key: "new", Title: "English release", Link: "https://example.com/new"}, {Key: "current", Title: "当前版本"}}
	rec = testAPIRequest(t, s, http.MethodPost, "/api/webui/feeds/check", `{}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"updated":2`) || sender.groupID != 100 || !strings.Contains(sender.text, "中文：English release") {
		t.Fatalf("check code=%d body=%s sender=%+v", rec.Code, rec.Body.String(), sender)
	}

	path := "/api/webui/groups/100/feeds/" + strconv.FormatUint(uint64(added.Feed.ID), 10)
	rec = testAPIRequest(t, s, http.MethodDelete, path, "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = testAPIRequest(t, s, http.MethodDelete, path, "", cookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing delete code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFeedAPIRejectsInvalidInputAndOfflineCheck(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)

	tests := []struct {
		method string
		path   string
		body   string
		code   int
	}{
		{http.MethodPost, "/api/webui/groups/no/feeds", `{"url":"https://example.com/feed"}`, http.StatusBadRequest},
		{http.MethodPost, "/api/webui/groups/100/feeds", `{`, http.StatusBadRequest},
		{http.MethodPost, "/api/webui/groups/100/feeds", `{"url":"file:///tmp/feed"}`, http.StatusBadRequest},
		{http.MethodPost, "/api/webui/groups/100/feeds/platform", `{"platform":"x_user","target":"bad handle!"}`, http.StatusBadRequest},
		{http.MethodGet, "/api/webui/groups/no/feeds/settings", "", http.StatusBadRequest},
		{http.MethodPut, "/api/webui/groups/100/feeds/settings", `{`, http.StatusBadRequest},
		{http.MethodPut, "/api/webui/groups/100/feeds/settings", `{"quiet_enabled":true,"quiet_start":"08:00","quiet_end":"08:00"}`, http.StatusBadRequest},
		{http.MethodPut, "/api/webui/groups/100/feeds/settings", `{"quiet_enabled":false,"item_max_chars":79}`, http.StatusBadRequest},
		{http.MethodDelete, "/api/webui/groups/100/feeds/no", "", http.StatusBadRequest},
		{http.MethodPut, "/api/webui/groups/100/feeds/no", `{"enabled":true}`, http.StatusBadRequest},
		{http.MethodPut, "/api/webui/groups/100/feeds/999", `{"enabled":true}`, http.StatusNotFound},
		{http.MethodPut, "/api/webui/groups/100/feeds/1", `{}`, http.StatusBadRequest},
		{http.MethodPost, "/api/webui/groups/100/feeds/check", `{}`, http.StatusServiceUnavailable},
		{http.MethodPost, "/api/webui/feeds/check", `{}`, http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		rec := testAPIRequest(t, s, tt.method, tt.path, tt.body, cookie)
		if rec.Code != tt.code {
			t.Fatalf("%s %s code=%d body=%s", tt.method, tt.path, rec.Code, rec.Body.String())
		}
	}
}
