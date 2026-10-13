package llm

import (
	"context"
	"encoding/json"
	"fmt"
	openai "github.com/sashabaranov/go-openai"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderUsageNormalizationAndObservation(t *testing.T) {
	for _, tt := range []struct {
		name, usage string
		known       bool
		hit         int
	}{
		{"deepseek", `{"prompt_tokens":100,"completion_tokens":20,"prompt_cache_hit_tokens":80,"prompt_cache_miss_tokens":20}`, true, 80},
		{"standard", `{"prompt_tokens":100,"prompt_tokens_details":{"cached_tokens":60}}`, true, 60},
		{"zero", `{"prompt_tokens":100,"prompt_cache_hit_tokens":0}`, true, 0},
		{"missing", `{"prompt_tokens":100}`, false, 0},
		{"null", `{"prompt_tokens":100,"prompt_tokens_details":{"cached_tokens":null}}`, false, 0},
		{"audio_only", `{"prompt_tokens":100,"prompt_tokens_details":{"audio_tokens":10}}`, false, 0},
		{"invalid", `{"prompt_tokens":100,"prompt_cache_hit_tokens":200}`, false, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"正文"},"finish_reason":"stop"}],"usage":%s}`, tt.usage)
			}))
			defer server.Close()
			var records []UsageRecord
			ctx := WithUsageObserver(context.Background(), func(record UsageRecord) { records = append(records, record) })
			response, err := New(Settings{BaseURL: server.URL, Model: "fixture"}).Complete(ctx, openai.ChatCompletionRequest{})
			if err != nil || len(records) != 1 {
				t.Fatalf("%v %+v", err, records)
			}
			if records[0].CacheReported != tt.known || records[0].CachedTokens != tt.hit || records[0].Model != "fixture" || response.Choices[0].Message.Content != "正文" {
				t.Fatalf("%+v", records)
			}
		})
	}
}
func TestNormalizeUsagePreservesProtocol(t *testing.T) {
	raw := []byte(`{"choices":[{"message":{"role":"assistant","reasoning_content":"think","tool_calls":[{"id":"x","function":{"name":"f","arguments":"{}"}}]}}],"usage":{"prompt_tokens":100,"prompt_cache_hit_tokens":50}}`)
	var before, after map[string]json.RawMessage
	json.Unmarshal(raw, &before)
	json.Unmarshal(normalizeUsage(raw), &after)
	if string(before["choices"]) != string(after["choices"]) {
		t.Fatal("changed model protocol")
	}
}
