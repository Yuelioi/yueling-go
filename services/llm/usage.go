package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

type UsageRecord struct {
	Model            string `json:"model"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	CachedTokens     int    `json:"cached_tokens"`
	CacheReported    bool   `json:"cache_reported"`
	DurationMS       int64  `json:"duration_ms"`
}
type usageObserverKey struct{}

func WithUsageObserver(ctx context.Context, observe func(UsageRecord)) context.Context {
	return context.WithValue(ctx, usageObserverKey{}, observe)
}
func observeUsage(ctx context.Context, model string, usage openai.Usage, elapsed time.Duration) {
	observe, ok := ctx.Value(usageObserverKey{}).(func(UsageRecord))
	if !ok {
		return
	}
	record := UsageRecord{Model: model, PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens, DurationMS: elapsed.Milliseconds()}
	if details := usage.PromptTokensDetails; details != nil && details.CachedTokens >= 0 && details.CachedTokens <= usage.PromptTokens {
		record.CacheReported = true
		record.CachedTokens = details.CachedTokens
	}
	observe(record)
}

// Normalize provider-specific counters before the SDK discards unknown fields.
// Missing cache usage stays unknown; it must not be reported as a zero hit rate.
type usageTransport struct{ base http.RoundTripper }

func (t usageTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	res, err := t.base.RoundTrip(req)
	if err != nil || res.StatusCode != http.StatusOK || !strings.Contains(res.Header.Get("Content-Type"), "application/json") {
		return res, err
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		res.Body.Close()
		return nil, err
	}
	// Preserve an oversized response without buffering the remaining bytes.
	if len(body) == 16<<20 {
		res.Body = &prefixedBody{Reader: io.MultiReader(bytes.NewReader(body), res.Body), Closer: res.Body}
		return res, nil
	}
	res.Body.Close()
	normalized := normalizeUsage(body)
	res.Body = io.NopCloser(bytes.NewReader(normalized))
	res.ContentLength = int64(len(normalized))
	res.Header.Set("Content-Length", strconv.Itoa(len(normalized)))
	return res, nil
}

type prefixedBody struct {
	io.Reader
	io.Closer
}

func normalizeUsage(body []byte) []byte {
	var response map[string]json.RawMessage
	if json.Unmarshal(body, &response) != nil {
		return body
	}
	var usage map[string]json.RawMessage
	if json.Unmarshal(response["usage"], &usage) != nil || usage == nil {
		return body
	}
	var details map[string]json.RawMessage
	_ = json.Unmarshal(usage["prompt_tokens_details"], &details)
	if details == nil {
		details = map[string]json.RawMessage{}
	}
	if hit, ok := usage["prompt_cache_hit_tokens"]; ok {
		details["cached_tokens"] = hit
	}
	var cached int
	raw, exists := details["cached_tokens"]
	if !exists || string(raw) == "null" || json.Unmarshal(raw, &cached) != nil || cached < 0 {
		delete(usage, "prompt_tokens_details")
	} else {
		usage["prompt_tokens_details"], _ = json.Marshal(details)
	}
	response["usage"], _ = json.Marshal(usage)
	result, err := json.Marshal(response)
	if err != nil {
		return body
	}
	return result
}
