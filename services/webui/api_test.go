package webui

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
)

type stubGroupLister struct {
	groups []bot.GroupInfo
	err    error
}

func (s stubGroupLister) GetGroupList() ([]bot.GroupInfo, error) {
	return s.groups, s.err
}

func testAPIRequest(t *testing.T, s *Server, method, target, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return serve(s, req)
}

func initWebUITestDB(t *testing.T) {
	t.Helper()
	oldDB := db.DB
	if err := db.Init(filepath.Join(t.TempDir(), "webui-test.db")); err != nil {
		t.Fatalf("init db: %v", err)
	}
	tempDB := db.DB
	t.Cleanup(func() {
		if sqlDB, err := tempDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		db.DB = oldDB
	})
}

func withTestAffinityConfig(t *testing.T) {
	t.Helper()
	old := config.C
	config.C.AI.Affinity = config.AffinityConfig{
		Enabled:    true,
		Initial:    40,
		BlockBelow: 15,
		Min:        0,
		Max:        80,
	}
	t.Cleanup(func() {
		config.C = old
	})
}

func TestGroupsRequireLiveBot(t *testing.T) {
	s := newTestServer()
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/groups", "", cookie)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"ok":false`) {
		t.Fatalf("body=%s, want error JSON", rec.Body.String())
	}
}

func TestGroupsReturnsListFromResolver(t *testing.T) {
	s := newTestServer()
	s.resolveGroupLister = func() groupLister {
		return stubGroupLister{groups: []bot.GroupInfo{
			{GroupID: 100, GroupName: "alpha"},
			{GroupID: 200, GroupName: "beta"},
		}}
	}
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/groups", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		OK     bool            `json:"ok"`
		Groups []bot.GroupInfo `json:"groups"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !got.OK || len(got.Groups) != 2 || got.Groups[0].GroupID != 100 || got.Groups[1].GroupName != "beta" {
		t.Fatalf("response = %+v, want stub groups", got)
	}
}

func TestGroupsReturnsBadGatewayOnListerError(t *testing.T) {
	s := newTestServer()
	s.resolveGroupLister = func() groupLister {
		return stubGroupLister{err: errors.New("napcat unavailable")}
	}
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/groups", "", cookie)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "napcat unavailable") {
		t.Fatalf("body=%s, want upstream error", rec.Body.String())
	}
}

func TestPluginsRequiresSession(t *testing.T) {
	s := newTestServer()

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/plugins", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPluginsReturnsCatalogAfterLogin(t *testing.T) {
	s := newTestServer()
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/plugins", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		OK      bool `json:"ok"`
		Plugins []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !got.OK || len(got.Plugins) == 0 {
		t.Fatalf("response = %+v, want non-empty catalog", got)
	}
}

func TestGroupPluginToggleAndApplyAll(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodPut, "/api/webui/groups/100/plugins/29", `{"disabled":true}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle code=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = testAPIRequest(t, s, http.MethodGet, "/api/webui/groups/100/plugins", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list code=%d body=%s", rec.Code, rec.Body.String())
	}
	var listed struct {
		OK       bool         `json:"ok"`
		Disabled map[int]bool `json:"disabled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode disabled map: %v", err)
	}
	if !listed.OK || !listed.Disabled[29] {
		t.Fatalf("listed = %+v, want plugin 29 disabled", listed)
	}

	rec = testAPIRequest(t, s, http.MethodPost, "/api/webui/plugins/34/apply-all", `{"group_ids":[100,200],"disabled":true}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply code=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, groupID := range []int64{100, 200} {
		disabled, err := db.IsGroupPluginDisabled(groupID, 34)
		if err != nil {
			t.Fatalf("is disabled group %d: %v", groupID, err)
		}
		if !disabled {
			t.Fatalf("group %d plugin 34 disabled = false, want true", groupID)
		}
	}
}

func TestPluginAPIsRejectInvalidParamsAndBodies(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"zero group", http.MethodGet, "/api/webui/groups/0/plugins", ""},
		{"negative group", http.MethodPut, "/api/webui/groups/-1/plugins/29", `{"disabled":true}`},
		{"zero plugin", http.MethodPut, "/api/webui/groups/100/plugins/0", `{"disabled":true}`},
		{"bad toggle body", http.MethodPut, "/api/webui/groups/100/plugins/29", `{`},
		{"empty apply groups", http.MethodPost, "/api/webui/plugins/29/apply-all", `{"group_ids":[],"disabled":true}`},
		{"bad apply group id", http.MethodPost, "/api/webui/plugins/29/apply-all", `{"group_ids":[100,0],"disabled":true}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := testAPIRequest(t, s, tt.method, tt.path, tt.body, cookie)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestPluginsRequireDisabledField(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodPut, "/api/webui/groups/100/plugins/29", `{}`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("toggle code=%d body=%s", rec.Code, rec.Body.String())
	}
	disabled, err := db.IsGroupPluginDisabled(100, 29)
	if err != nil {
		t.Fatalf("check toggle mutation: %v", err)
	}
	if disabled {
		t.Fatalf("missing disabled field mutated single toggle")
	}

	rec = testAPIRequest(t, s, http.MethodPost, "/api/webui/plugins/34/apply-all", `{"group_ids":[100,200]}`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("apply code=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, groupID := range []int64{100, 200} {
		disabled, err := db.IsGroupPluginDisabled(groupID, 34)
		if err != nil {
			t.Fatalf("check apply mutation group %d: %v", groupID, err)
		}
		if disabled {
			t.Fatalf("missing disabled field mutated apply-all for group %d", groupID)
		}
	}
}

func TestAffinityListSetAdjustAndReset(t *testing.T) {
	initWebUITestDB(t)
	withTestAffinityConfig(t)
	s := newTestServer()
	cookie := login(t, s)

	row, err := db.UpdateAIAffinity(1, 100, "alice", 40, 5, 0, 80, "seed")
	if err != nil {
		t.Fatalf("seed affinity: %v", err)
	}
	if _, err := db.UpdateAIAffinity(2, 200, "bob", 40, 0, 0, 80, "seed"); err != nil {
		t.Fatalf("seed other affinity: %v", err)
	}

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/affinity?group_id=100&q=ali", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list code=%d body=%s", rec.Code, rec.Body.String())
	}
	var listed struct {
		OK         bool            `json:"ok"`
		Affinity   []db.AIAffinity `json:"affinity"`
		BlockBelow int             `json:"block_below"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if !listed.OK || listed.BlockBelow != 15 || len(listed.Affinity) != 1 || listed.Affinity[0].Nickname != "alice" {
		t.Fatalf("listed = %+v, want alice with block_below 15", listed)
	}

	rowID := strconv.FormatUint(uint64(row.ID), 10)
	rec = testAPIRequest(t, s, http.MethodPut, "/api/webui/affinity/"+rowID+"/score", `{"score":999}`, cookie)
	got := decodeAffinityMutation(t, rec, http.StatusOK)
	if got.Score != 80 || got.LastReason != "webui_set" {
		t.Fatalf("after set = %+v, want score 80 reason webui_set", got)
	}

	rec = testAPIRequest(t, s, http.MethodPost, "/api/webui/affinity/"+rowID+"/adjust", `{"delta":-200}`, cookie)
	got = decodeAffinityMutation(t, rec, http.StatusOK)
	if got.Score != 0 || got.LastReason != "webui_adjust" {
		t.Fatalf("after adjust = %+v, want score 0 reason webui_adjust", got)
	}

	rec = testAPIRequest(t, s, http.MethodPost, "/api/webui/affinity/"+rowID+"/reset", "", cookie)
	got = decodeAffinityMutation(t, rec, http.StatusOK)
	if got.Score != 40 || got.LastReason != "webui_reset" {
		t.Fatalf("after reset = %+v, want score 40 reason webui_reset", got)
	}
}

func TestAffinityAPIsRejectInvalidParamsAndBodies(t *testing.T) {
	initWebUITestDB(t)
	withTestAffinityConfig(t)
	s := newTestServer()
	cookie := login(t, s)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"bad query group", http.MethodGet, "/api/webui/affinity?group_id=abc", ""},
		{"zero query group", http.MethodGet, "/api/webui/affinity?group_id=0", ""},
		{"bad score id", http.MethodPut, "/api/webui/affinity/abc/score", `{"score":1}`},
		{"zero score id", http.MethodPut, "/api/webui/affinity/0/score", `{"score":1}`},
		{"bad score body", http.MethodPut, "/api/webui/affinity/1/score", `{`},
		{"zero delta", http.MethodPost, "/api/webui/affinity/1/adjust", `{"delta":0}`},
		{"bad adjust body", http.MethodPost, "/api/webui/affinity/1/adjust", `{`},
		{"bad reset id", http.MethodPost, "/api/webui/affinity/no/reset", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := testAPIRequest(t, s, tt.method, tt.path, tt.body, cookie)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAffinityMissingRowReturnsNotFound(t *testing.T) {
	initWebUITestDB(t)
	withTestAffinityConfig(t)
	s := newTestServer()
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodPut, "/api/webui/affinity/999/score", `{"score":10}`, cookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func decodeAffinityMutation(t *testing.T, rec *httptest.ResponseRecorder, wantCode int) db.AIAffinity {
	t.Helper()
	if rec.Code != wantCode {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		OK       bool          `json:"ok"`
		Affinity db.AIAffinity `json:"affinity"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode mutation: %v", err)
	}
	if !got.OK {
		t.Fatalf("response = %+v, want ok", got)
	}
	return got.Affinity
}
