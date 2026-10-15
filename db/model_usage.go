package db

import (
	"context"
	"fmt"

	"github.com/Yuelioi/yueling-go/services/llm"
	"github.com/Yuelioi/yueling-go/util"
	"gorm.io/gorm"
)

type ModelUsageStore struct{ database *gorm.DB }

func NewModelUsageStore(database *gorm.DB) *ModelUsageStore {
	return &ModelUsageStore{database: database}
}
func (s *ModelUsageStore) Record(ctx context.Context, groupID int64, usage llm.UsageRecord) error {
	if s == nil || s.database == nil {
		return nil
	}
	known, prompt := 0, 0
	if usage.CacheReported {
		known = 1
		prompt = usage.PromptTokens
	}
	return s.database.WithContext(ctx).Exec(`INSERT INTO ai_model_usage
 (usage_date,group_id,model,profile_id,feature,requests,prompt_tokens,completion_tokens,cache_reported_requests,cache_reported_prompt_tokens,cached_tokens,duration_ms)
 VALUES (?,?,?,?,?,1,?,?,?,?,?,?)
 ON CONFLICT (usage_date,group_id,model,profile_id,feature) DO UPDATE SET
 requests=ai_model_usage.requests+1,
 prompt_tokens=ai_model_usage.prompt_tokens+EXCLUDED.prompt_tokens,
 completion_tokens=ai_model_usage.completion_tokens+EXCLUDED.completion_tokens,
 cache_reported_requests=ai_model_usage.cache_reported_requests+EXCLUDED.cache_reported_requests,
 cache_reported_prompt_tokens=ai_model_usage.cache_reported_prompt_tokens+EXCLUDED.cache_reported_prompt_tokens,
 cached_tokens=ai_model_usage.cached_tokens+EXCLUDED.cached_tokens,
 duration_ms=ai_model_usage.duration_ms+EXCLUDED.duration_ms`, util.Now().Format("2006-01-02"), groupID, usage.Model, usage.ProfileID, usage.Feature, usage.PromptTokens, usage.CompletionTokens, known, prompt, usage.CachedTokens, usage.DurationMS).Error
}

type ModelUsageStats struct {
	ProfileID                 string   `json:"profile_id"`
	Feature                   string   `json:"feature"`
	GroupID                   int64    `json:"group_id"`
	Model                     string   `json:"model"`
	Requests                  int64    `json:"requests"`
	PromptTokens              int64    `json:"prompt_tokens"`
	CompletionTokens          int64    `json:"completion_tokens"`
	CacheReportedRequests     int64    `json:"cache_reported_requests"`
	CacheReportedPromptTokens int64    `json:"cache_reported_prompt_tokens"`
	CachedTokens              int64    `json:"cached_tokens"`
	UncachedTokens            int64    `json:"uncached_tokens"`
	DurationMS                int64    `json:"duration_ms"`
	CacheHitRate              *float64 `json:"cache_hit_rate"`
}

func (s *ModelUsageStore) Stats(ctx context.Context, groupID *int64, days int) ([]ModelUsageStats, error) {
	if days < 1 || days > 90 {
		return nil, fmt.Errorf("days must be between 1 and 90")
	}
	rows := []ModelUsageStats{}
	if s == nil || s.database == nil {
		return rows, nil
	}
	query := s.database.WithContext(ctx).Table("ai_model_usage").Select(`group_id,model,profile_id,feature,SUM(requests) AS requests,SUM(prompt_tokens) AS prompt_tokens,SUM(completion_tokens) AS completion_tokens,SUM(cache_reported_requests) AS cache_reported_requests,SUM(cache_reported_prompt_tokens) AS cache_reported_prompt_tokens,SUM(cached_tokens) AS cached_tokens,SUM(duration_ms) AS duration_ms`).Where("usage_date >= ?", util.Now().AddDate(0, 0, 1-days).Format("2006-01-02"))
	if groupID != nil {
		query = query.Where("group_id = ?", *groupID)
	}
	if err := query.Group("group_id,model,profile_id,feature").Order("group_id,model,profile_id,feature").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		r := &rows[i]
		r.UncachedTokens = r.CacheReportedPromptTokens - r.CachedTokens
		if r.CacheReportedPromptTokens > 0 {
			rate := float64(r.CachedTokens) / float64(r.CacheReportedPromptTokens)
			r.CacheHitRate = &rate
		}
	}
	return rows, nil
}
