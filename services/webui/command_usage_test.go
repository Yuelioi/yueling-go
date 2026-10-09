package webui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
)

func TestGroupCommandUsageReturnsScopedStats(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)
	now := time.Now()

	if err := db.NewCommandUsageStore(testdb.Database).RecordCommandUsageAt(100, 1, 31, "ping", now); err != nil {
		t.Fatal(err)
	}
	if err := db.NewCommandUsageStore(testdb.Database).RecordCommandUsageAt(100, 2, 31, "ping", now); err != nil {
		t.Fatal(err)
	}
	if err := db.NewCommandUsageStore(testdb.Database).RecordCommandUsageAt(200, 3, 31, "ping", now); err != nil {
		t.Fatal(err)
	}

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/groups/100/command-usage?days=7", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		OK          bool                      `json:"ok"`
		Stats       db.GroupCommandUsageStats `json:"stats"`
		PluginNames map[string]string         `json:"plugin_names"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.OK || got.Stats.GroupID != 100 || got.Stats.TotalCalls != 2 ||
		got.Stats.UniqueUsers != 2 || len(got.Stats.TopCommands) != 1 ||
		got.Stats.TopCommands[0].Command != "ping" || got.PluginNames["31"] == "" {
		t.Fatalf("response=%+v", got)
	}

	rec = testAPIRequest(t, s, http.MethodGet, "/api/webui/command-usage?group_id=0&days=7", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("all groups code=%d body=%s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Stats.GroupID != 0 || got.Stats.TotalCalls != 3 || got.Stats.UniqueUsers != 3 ||
		len(got.Stats.TopCommands) != 1 || got.Stats.TopCommands[0].Calls != 3 {
		t.Fatalf("all groups response=%+v", got)
	}
}

func TestGroupCommandUsageRejectsInvalidRange(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)

	for _, path := range []string{
		"/api/webui/groups/no/command-usage",
		"/api/webui/groups/0/command-usage",
		"/api/webui/groups/100/command-usage?days=0",
		"/api/webui/groups/100/command-usage?days=91",
		"/api/webui/groups/100/command-usage?days=no",
		"/api/webui/command-usage?group_id=-1",
		"/api/webui/command-usage?group_id=no",
		"/api/webui/command-usage?days=91",
	} {
		rec := testAPIRequest(t, s, http.MethodGet, path, "", cookie)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("path=%s code=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}
}
