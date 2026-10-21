package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDeepSeekNativeThinkingWireFormatAndProviderIsolation(t *testing.T) {
	for _, tc := range []struct{ host, model, effort, mode string }{
		{"api.deepseek.com", "deepseek-flash", "none", "disabled"},
		{"api.deepseek.com", "deepseek-v4-pro", "low", "enabled"},
		{"api.deepseek.com", "deepseek-flash", "high", "enabled"},
		{"api.deepseek.com", "deepseek-reasoner", "none", ""},
		{"api.deepseek.com", "other-model", "none", ""},
		{"api.deepseek.com.example.invalid", "deepseek-flash", "none", ""},
		{"compatible.example.invalid", "deepseek-flash", "none", ""},
	} {
		t.Run(tc.host+"/"+tc.model+"/"+tc.effort, func(t *testing.T) {
			payload := `{"model":"` + tc.model + `","reasoning_effort":"` + tc.effort + `","max_tokens":900,"messages":[{"role":"assistant","reasoning_content":"protocol","tool_calls":[{"id":"a"}]},{"role":"tool","tool_call_id":"a","content":"result"}]}`
			req, err := http.NewRequestWithContext(context.Background(), "POST", "https://"+tc.host+"/v1/chat/completions", strings.NewReader(payload))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer synthetic")
			called := false
			transport := deepSeekTransport{base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				called = true
				body, _ := io.ReadAll(r.Body)
				var got map[string]json.RawMessage
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatal(err)
				}
				var thinking struct {
					Type string `json:"type"`
				}
				_ = json.Unmarshal(got["thinking"], &thinking)
				if thinking.Type != tc.mode || r.Header.Get("Authorization") != "Bearer synthetic" || !strings.Contains(string(got["messages"]), `"reasoning_content":"protocol"`) || !strings.Contains(string(got["messages"]), `"tool_call_id":"a"`) {
					t.Fatalf("unexpected wire request: %s", body)
				}
				if tc.mode == "disabled" {
					if _, exists := got["reasoning_effort"]; exists || r.ContentLength != int64(len(body)) || r.GetBody == nil {
						t.Fatal("invalid disabled request headers or fields")
					}
					replay, _ := r.GetBody()
					defer replay.Close()
					replayed, _ := io.ReadAll(replay)
					if string(replayed) != string(body) {
						t.Fatal("redirect replay lost thinking control")
					}
				} else if string(got["reasoning_effort"]) != `"`+tc.effort+`"` {
					t.Fatal("lost explicit effort")
				}
				if tc.mode == "" && string(body) != payload {
					t.Fatal("changed another provider's wire request")
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
			})}
			response, err := transport.RoundTrip(req)
			if err != nil || !called {
				t.Fatalf("called=%v err=%v", called, err)
			}
			response.Body.Close()
		})
	}
}

func TestDeepSeekNonThinkingPreservesFeatureBudgetsAndExplicitEffort(t *testing.T) {
	for _, tc := range []struct {
		effort          string
		requested, want int
	}{
		{"", 900, 900},
		{"none", 200, 200},
		{"low", 900, 4096},
		{"high", 900, 4096},
	} {
		t.Run(tc.effort, func(t *testing.T) {
			client := WithBackend(completionFunc(func(_ context.Context, r openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
				if r.MaxTokens != tc.want || (tc.effort != "" && r.ReasoningEffort != tc.effort) || len(r.Tools) != 0 {
					t.Fatalf("max=%d effort=%s tools=%d", r.MaxTokens, r.ReasoningEffort, len(r.Tools))
				}
				return answer("摘要"), nil
			}), Settings{Model: "deepseek-flash", ProfileID: "fixture", Feature: "compression", MaxTokens: 4096, ReasoningEffort: tc.effort})
			if _, err := client.Text(context.Background(), openai.ChatCompletionRequest{MaxTokens: tc.requested}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
