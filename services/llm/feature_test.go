package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestFeatureClientPinsModelCredentialsAndUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatCompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "summary-model" || req.MaxTokens != 512 || r.Header.Get("Authorization") != "Bearer summary-key" {
			t.Errorf("mixed profile: model=%s budget=%d", req.Model, req.MaxTokens)
		}
		_ = json.NewEncoder(w).Encode(openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: openai.FinishReasonStop, Message: openai.ChatCompletionMessage{Content: "OK"}}}, Usage: openai.Usage{PromptTokens: 15, CompletionTokens: 2}})
	}))
	defer server.Close()
	cfg := config.AIConfig{Connections: []config.AIConnection{{ID: "connection", Kind: "openai", APIKey: "summary-key", BaseURL: server.URL}}, Profiles: []config.AIProfile{{ID: "p", ConnectionID: "connection", Model: "summary-model", Capabilities: []string{"chat"}, MaxTokens: 512}}, Bindings: map[string]string{"summary": "p"}}
	var global, scoped []UsageRecord
	SetDefaultUsageObserver(func(u UsageRecord) { global = append(global, u) })
	defer SetDefaultUsageObserver(nil)
	client := New(ForFeature(cfg, "summary"))
	cfg.Profiles[0].Model = "changed"
	req := openai.ChatCompletionRequest{Model: "stale-default", MaxTokens: 4096, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "fixture"}}}
	if _, err := client.Text(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	ctx := WithUsageObserver(context.Background(), func(u UsageRecord) { scoped = append(scoped, u) })
	if _, err := client.Text(ctx, req); err != nil {
		t.Fatal(err)
	}
	if len(global) != 1 || len(scoped) != 1 || scoped[0].ProfileID != "p" || scoped[0].Feature != "summary" {
		t.Fatal(global, scoped)
	}
	cfg.Bindings["summary"] = "off"
	if _, err := New(ForFeature(cfg, "summary")).Text(context.Background(), req); err == nil {
		t.Fatal("disabled feature called a model")
	}
}
