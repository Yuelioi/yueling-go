package ai

import (
	"context"
	"encoding/json"
	"github.com/Yuelioi/yueling-go/internal/testconfig"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestLLMUsesChangedEndpointAndCredential(t *testing.T) {
	old := config.Current()
	defer config.Publish(old)
	newEndpoint := func(key string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+key {
				t.Error("stale credential")
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
		}))
	}
	first := newEndpoint("first-key")
	defer first.Close()
	second := newEndpoint("second-key")
	defer second.Close()
	for _, v := range []struct{ url, key string }{{first.URL, "first-key"}, {second.URL, "second-key"}} {
		next := config.Current()
		testconfig.Set(&next.AI, "BaseURL", v.url)
		testconfig.Set(&next.AI, "DeepSeekKey", v.key)
		config.Publish(next)
		response, err := modelClient().Complete(context.Background(), openai.ChatCompletionRequest{Model: "fixture", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "test"}}})
		if err != nil || len(response.Choices) != 1 {
			t.Fatalf("request failed: %v", err)
		}
	}
}

func TestConversationProfileSwitchKeepsModelAndClientTogether(t *testing.T) {
	old := config.Current()
	defer config.Publish(old)
	endpoint := func(model, key string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Model string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Model != model || r.Header.Get("Authorization") != "Bearer "+key {
				t.Error("mixed profile model and credentials")
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
		}))
	}
	first := endpoint("first-model", "first-key")
	defer first.Close()
	second := endpoint("second-model", "second-key")
	defer second.Close()
	c := old
	c.AI.Profiles = []config.AIProfile{{ID: "first", Name: "first", ConnectionID: "first", Model: "first-model", Capabilities: []string{"chat", "tools", "vision"}}, {ID: "second", Name: "second", ConnectionID: "second", Model: "second-model", Capabilities: []string{"chat", "tools"}}}
	c.AI.Connections = []config.AIConnection{{ID: "first", Kind: "openai", BaseURL: first.URL, APIKey: "first-key"}, {ID: "second", Kind: "openai", BaseURL: second.URL, APIKey: "second-key"}}
	c.AI.Bindings = map[string]string{}
	c.AI.Bindings["conversation"] = "first"
	c.AI.Bindings["vision"] = "first"
	config.Publish(c)
	client := modelClient()
	c.AI.Bindings["conversation"] = "second"
	config.Publish(c)
	if _, err := client.Complete(context.Background(), openai.ChatCompletionRequest{Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "test"}}}); err != nil {
		t.Fatal(err)
	}
	client = modelClient()
	if _, err := client.Complete(context.Background(), openai.ChatCompletionRequest{Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "test"}}}); err != nil {
		t.Fatal(err)
	}
	if config.Current().AI.Bindings["vision"] != "first" {
		t.Fatal("conversation switch changed vision selection")
	}
}

func TestConversationKeepsModelSettingsUntilNextTurn(t *testing.T) {
	old, oldClient := config.Current(), testEngine.modelBackend
	t.Cleanup(func() { config.Publish(old); testEngine.modelBackend = oldClient })
	testEngine.modelBackend = nil
	var firstCalls, secondCalls atomic.Int32
	endpoint := func(name, key string, tokens int, calls *atomic.Int32, withTool bool) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req openai.ChatCompletionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Error(err)
			}
			if req.Model != name || req.MaxTokens != tokens || r.Header.Get("Authorization") != "Bearer "+key {
				t.Errorf("settings changed inside a turn: model=%s tokens=%d", req.Model, req.MaxTokens)
			}
			message := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: name}
			if calls.Add(1) == 1 && withTool {
				message.Content = ""
				message.ToolCalls = []openai.ToolCall{{ID: "switch", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "query", Arguments: `{}`}}}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: message, FinishReason: openai.FinishReasonStop}}})
		}))
	}
	first := endpoint("first", "first-key", 128, &firstCalls, true)
	defer first.Close()
	second := endpoint("second", "second-key", 256, &secondCalls, false)
	defer second.Close()
	initial := old
	initial.AI = func() config.AIConfig {
		a := testconfig.AI("first", first.URL, "first-key")
		a.MaxTokens = 128
		return a
	}()
	config.Publish(initial)
	next := initial
	next.AI = func() config.AIConfig {
		a := testconfig.AI("second", second.URL, "second-key")
		a.MaxTokens = 256
		return a
	}()
	session := newSession(1, 2)
	reply, err := testEngine.runConversation(context.Background(), session, "查询", "system", []openai.Tool{(&ToolMeta{Name: "query"}).Schema()}, func(openai.ToolCall) ToolResult {
		config.Publish(next)
		return ToolResult{Status: ToolSucceeded, Content: "资料", Invoked: true}
	})
	if err != nil || reply != "first" {
		t.Errorf("active turn: reply=%q err=%v", reply, err)
	}
	reply, err = testEngine.runConversation(context.Background(), session, "继续", "system", nil, func(openai.ToolCall) ToolResult { t.Error("unexpected action"); return ToolResult{} })
	if err != nil || reply != "second" {
		t.Errorf("next turn: reply=%q err=%v", reply, err)
	}
	if firstCalls.Load() != 2 || secondCalls.Load() != 1 {
		t.Errorf("requests first=%d second=%d", firstCalls.Load(), secondCalls.Load())
	}
}
