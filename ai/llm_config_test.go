package ai

import (
	"context"
	"encoding/json"
	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
	"net/http"
	"net/http/httptest"
	"testing"
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
		next.AI.BaseURL = v.url
		next.AI.DeepSeekKey = v.key
		config.Publish(next)
		response, err := llm().CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{Model: "fixture", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "test"}}})
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
	c.AI.Profiles = []config.AIProfile{{ID: "first", Name: "first", BaseURL: first.URL, Model: "first-model", APIKey: "first-key", Vision: true}, {ID: "second", Name: "second", BaseURL: second.URL, Model: "second-model", APIKey: "second-key"}}
	c.AI.ConversationProfile = "first"
	c.AI.VisionProfile = "first"
	config.Publish(c)
	client, model := conversationClient()
	c.AI.ConversationProfile = "second"
	config.Publish(c)
	if _, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{Model: model, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "test"}}}); err != nil {
		t.Fatal(err)
	}
	client, model = conversationClient()
	if _, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{Model: model, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "test"}}}); err != nil {
		t.Fatal(err)
	}
	if config.Current().AI.VL.Model != "first-model" {
		t.Fatal("conversation switch changed vision selection")
	}
}
