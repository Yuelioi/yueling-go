package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
)

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

func TestAffinityListSetAdjustAndReset(t *testing.T) {
	initWebUITestDB(t)
	withTestAffinityConfig(t)
	s := newTestServer()
	cookie := login(t, s)

	row, err := db.NewAffinityStore(testdb.Database).UpdateAIAffinity(1, 100, "alice", 40, 5, 0, 80, "seed")
	if err != nil {
		t.Fatalf("seed affinity: %v", err)
	}
	if _, err := db.NewAffinityStore(testdb.Database).UpdateAIAffinity(2, 200, "bob", 40, 0, 0, 80, "seed"); err != nil {
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

func TestAffinityTierSettingsGetSetAndReset(t *testing.T) {
	initWebUITestDB(t)
	withTestAffinityConfig(t)
	s := newTestServer()
	cookie := login(t, s)

	rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/affinity/settings", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"custom":false`) ||
		!strings.Contains(rec.Body.String(), `"min_score":0`) {
		t.Fatalf("initial settings code=%d body=%s", rec.Code, rec.Body.String())
	}

	body := `{"tiers":[{"min_score":0,"name":"疏远","prompt":"只回答必要信息。"},{"min_score":55,"name":"亲近","prompt":"使用更熟悉的语气。"}]}`
	rec = testAPIRequest(t, s, http.MethodPut, "/api/webui/affinity/settings", body, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"custom":true`) ||
		!strings.Contains(rec.Body.String(), "使用更熟悉的语气") {
		t.Fatalf("set settings code=%d body=%s", rec.Code, rec.Body.String())
	}

	tiers, err := db.NewAffinityStore(testdb.Database).ListAIAffinityTiers()
	if err != nil || len(tiers) != 2 || tiers[1].MinScore != 55 {
		t.Fatalf("stored tiers=%+v err=%v", tiers, err)
	}

	rec = testAPIRequest(t, s, http.MethodDelete, "/api/webui/affinity/settings", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"custom":false`) {
		t.Fatalf("reset settings code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAffinityTierSettingsRejectInvalidRanges(t *testing.T) {
	initWebUITestDB(t)
	withTestAffinityConfig(t)
	s := newTestServer()
	cookie := login(t, s)

	tests := []string{
		`{"tiers":[]}`,
		`{"tiers":[{"min_score":1,"name":"缺少下界","prompt":"x"}]}`,
		`{"tiers":[{"min_score":0,"name":"重复一","prompt":"x"},{"min_score":0,"name":"重复二","prompt":"y"}]}`,
		`{"tiers":[{"min_score":0,"name":"","prompt":"x"}]}`,
		`{"tiers":[{"min_score":0,"name":"普通","prompt":""}]}`,
		`{"tiers":[{"min_score":81,"name":"越界","prompt":"x"}]}`,
	}
	for _, body := range tests {
		rec := testAPIRequest(t, s, http.MethodPut, "/api/webui/affinity/settings", body, cookie)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body=%s code=%d response=%s", body, rec.Code, rec.Body.String())
		}
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
