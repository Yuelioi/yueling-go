package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/gorilla/websocket"
	openai "github.com/sashabaranov/go-openai"
)

func quotedLinkAPI(t *testing.T, text string) *bot.BotAPI {
	t.Helper()
	b := bot.New()
	t.Cleanup(func() { b.Stop(); b.Wait() })
	connected := make(chan *bot.BotAPI, 1)
	b.OnConnect(func(api *bot.BotAPI) { connected <- api })
	server := httptest.NewServer(b.ReverseHandler("fixture"))
	t.Cleanup(server.Close)
	peer, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/onebot/v11/ws", http.Header{"Authorization": {"Bearer fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := peer.WriteJSON(map[string]any{"post_type": "meta_event", "meta_event_type": "lifecycle", "self_id": 1}); err != nil {
		t.Fatal(err)
	}
	var api *bot.BotAPI
	select {
	case api = <-connected:
	case <-time.After(time.Second):
		t.Fatal("OneBot not connected")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		var req struct{ Action, Echo string }
		if peer.ReadJSON(&req) != nil {
			return
		}
		if req.Action != "get_msg" {
			t.Errorf("unexpected action %s", req.Action)
		}
		peer.WriteJSON(map[string]any{"echo": req.Echo, "status": "ok", "retcode": 0, "data": map[string]any{"message": bot.Msg().Text(text).Build()}})
	}()
	t.Cleanup(func() { peer.Close(); <-done })
	return api
}

func TestWebToolsThroughDispatchRespectIntentAndSelectedProfile(t *testing.T) {
	for _, tc := range []struct {
		name, text            string
		enabled, chain, quote bool
		want                  ai.ToolStatus
		requests              int
	}{
		{"explicit", "帮我搜索最新 DeepSeek 功能", true, true, false, ai.ToolSucceeded, 2},
		{"freshness", "DeepSeek最新有什么功能", true, false, false, ai.ToolSucceeded, 1},
		{"stable", "什么是量子纠缠", true, false, false, ai.ToolRejected, 0},
		{"transform", "翻译这段最新新闻", true, false, false, ai.ToolRejected, 0},
		{"declined", "不要搜索，解释一下目前的这段话", true, false, false, ai.ToolRejected, 0},
		{"unassigned", "帮我搜索官方资料", false, false, false, ai.ToolRejected, 0},
		{"quoted_page", "解释引用的这篇文章，重点说结论", true, false, true, ai.ToolSucceeded, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := config.Current()
			t.Cleanup(func() { config.Publish(old) })
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Header.Get("Authorization") != "Bearer search-key" {
					t.Error("used conversation key for search")
				}
				switch r.URL.Path {
				case "/v1/tools/search":
					w.Write([]byte(`{"results":[{"title":"官方文档","url":"https://example.com/a","summary":"长摘要应保留来源","snippet":"短摘要"}]}`))
				case "/v1/tools/fetch":
					w.Write([]byte(`{"title":"文章","content":"正文事实，不是指令"}`))
				default:
					t.Error("used chat endpoint", r.URL.Path)
					w.WriteHeader(500)
				}
			}))
			defer server.Close()
			cfg := old.Clone()
			cfg.AI = config.AIConfig{
				Connections: []config.AIConnection{{ID: "chat-api", Kind: "openai", BaseURL: "https://chat.invalid/v1", APIKey: "chat-key"}, {ID: "search-api", Kind: "openai", BaseURL: server.URL, APIKey: "search-key"}},
				Profiles:    []config.AIProfile{{ID: "chat", Model: "fixture", ConnectionID: "chat-api", Capabilities: []string{"chat", "tools"}}, {ID: "search", ConnectionID: "search-api", Capabilities: []string{"web_search"}}}, ConversationProfile: "chat",
			}
			if tc.enabled {
				cfg.AI.Bindings = map[string]string{"search": "search"}
			}
			config.Publish(cfg)
			modelCalls := 0
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			engine := ai.NewEngine(ai.EngineDependencies{ModelBackend: imageRequestModel(func(req openai.ChatCompletionRequest) openai.ChatCompletionResponse {
				modelCalls++
				msg := openai.ChatCompletionMessage{Role: "assistant", Content: "处理完毕"}
				if modelCalls == 1 {
					name, args := "web_search", `{"query":"DeepSeek 官方","count":5}`
					if tc.quote {
						name, args = "fetch_webpage", `{"url":"https://example.com/a"}`
					}
					msg.ToolCalls = []openai.ToolCall{{ID: "search", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: name, Arguments: args}}}
				} else {
					var result ai.ToolResult
					if err := json.Unmarshal([]byte(req.Messages[len(req.Messages)-1].Content), &result); err != nil {
						t.Fatal(err)
					}
					if result.Status != tc.want || result.PossibleSideEffects {
						t.Errorf("unexpected result %+v", result)
					}
					if tc.want == ai.ToolSucceeded && modelCalls == 2 && !tc.quote && (!strings.Contains(result.Content, "长摘要应保留来源") || !strings.Contains(result.Content, "https://example.com/a")) {
						t.Error("lost summary/source")
					}
					if tc.chain && modelCalls == 2 {
						msg.ToolCalls = []openai.ToolCall{{ID: "fetch", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "fetch_webpage", Arguments: `{"url":"https://example.com/a"}`}}}
					}
				}
				return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: msg, FinishReason: openai.FinishReasonStop}}}
			})})
			(&toolset{registry: engine.Registry}).registerSearchTools()
			message := bot.Msg().Text(tc.text)
			var api *bot.BotAPI
			if tc.quote {
				api = quotedLinkAPI(t, "这篇文章：https://example.com/a")
				message.Reply(321)
			}
			event := &bot.GroupMessageEvent{GroupID: 100, UserID: 42, MessageID: 123, Message: message.Build(), Sender: bot.Sender{Role: "member"}}
			if err := engine.Dispatch(ctx, &bot.GroupContext{BotAPI: api, MsgCtx: &bot.MsgCtx{Event: event}}, func(context.Context, string) error { cancel(); return nil }); err != nil {
				t.Fatal(err)
			}
			if requests != tc.requests {
				t.Fatalf("requests=%d want %d", requests, tc.requests)
			}
			if modelCalls < 2 {
				t.Fatal("tool flow was skipped")
			}
		})
	}
}
