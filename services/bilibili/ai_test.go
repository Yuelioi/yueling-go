package bilibili

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

func TestAIReplyUsesConfiguredModelWithoutTools(t *testing.T) {
	previous := config.C
	t.Cleanup(func() { config.C = previous })
	var called atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected AI endpoint or auth")
		}
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Tools json.RawMessage `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if req.Model != "test-model" || len(req.Tools) > 0 || len(req.Messages) != 2 || req.Messages[1].Role != "user" || req.Messages[1].Content != "测试私信" {
			t.Errorf("AI request=%+v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"测试回复"}}]}`))
	}))
	defer server.Close()
	config.C.AI = config.AIConfig{DeepSeekKey: "test-key", BaseURL: server.URL, Model: "test-model", MaxTokens: 1000}
	config.C.Bilibili.AIPrompt = "简洁地回复"
	got, err := AIReply(context.Background(), "测试私信")
	if err != nil || got != "测试回复" || !called.Load() {
		t.Fatalf("AI reply=%q err=%v called=%v", got, err, called.Load())
	}
}
