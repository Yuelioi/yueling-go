package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
	"github.com/Yuelioi/yueling-go/services/llm"
)

func TestModelUsageScopesAndWeightedCacheRates(t *testing.T) {
	initWebUITestDB(t)
	s := newTestServer()
	s.modelUsage = db.NewModelUsageStore(testdb.Database)
	for _, fixture := range []struct {
		group int64
		u     llm.UsageRecord
	}{
		{100, llm.UsageRecord{Model: "a", PromptTokens: 100, CachedTokens: 80, CacheReported: true, DurationMS: 10}},
		{100, llm.UsageRecord{Model: "a", PromptTokens: 900, CachedTokens: 90, CacheReported: true, DurationMS: 20}},
		{100, llm.UsageRecord{Model: "a", PromptTokens: 500, DurationMS: 30}},
		{200, llm.UsageRecord{Model: "a", PromptTokens: 100}},
		{200, llm.UsageRecord{Model: "b", PromptTokens: 100, CacheReported: true}},
	} {
		if err := s.modelUsage.Record(context.Background(), fixture.group, fixture.u); err != nil {
			t.Fatal(err)
		}
	}
	cookie := login(t, s)
	if rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/ai/model-usage", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatal("usage not protected", rec.Code)
	}
	fetch := func(path string) []db.ModelUsageStats {
		rec := testAPIRequest(t, s, http.MethodGet, path, "", cookie)
		if rec.Code != http.StatusOK {
			t.Fatal(rec.Body.String())
		}
		var got struct {
			Stats []db.ModelUsageStats `json:"stats"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got.Stats
	}
	rows := fetch("/api/webui/ai/model-usage")
	if len(rows) != 3 {
		t.Fatal(rows)
	}
	row := rows[0]
	if row.Requests != 3 || row.PromptTokens != 1500 || row.CacheReportedPromptTokens != 1000 || row.CachedTokens != 170 || row.UncachedTokens != 830 || row.DurationMS != 60 || row.CacheHitRate == nil || *row.CacheHitRate != 0.17 {
		t.Fatalf("wrong weighted denominator: %+v", row)
	}
	if rows[1].CacheHitRate != nil || rows[2].CacheHitRate == nil || *rows[2].CacheHitRate != 0 {
		t.Fatal("unknown conflated with zero")
	}
	if got := fetch("/api/webui/ai/model-usage?group_id=100&days=1"); len(got) != 1 || got[0].GroupID != 100 {
		t.Fatal("group leak", got)
	}
	for _, query := range []string{"?days=0", "?days=91", "?group_id=-1", "?group_id=no"} {
		if rec := testAPIRequest(t, s, http.MethodGet, "/api/webui/ai/model-usage"+query, "", cookie); rec.Code != http.StatusBadRequest {
			t.Fatal(query, rec.Code)
		}
	}
}
