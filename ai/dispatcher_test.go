package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
)

func TestDispatchDoesNotPersistEmptyAssistantResponse(t *testing.T) {
	cleanupAIConfigAndDB(t)
	initAffinityTestDB(t)

	previousClient := _client
	previousSessions := Sessions
	previousRegistry := global
	t.Cleanup(func() {
		_client = previousClient
		Sessions = previousSessions
		global = previousRegistry
	})
	resetAILimiterForTest(t)
	Sessions = &SessionManager{sessions: map[string]*Session{}}
	global = &registry{tools: map[string]*ToolMeta{}}

	var mu sync.Mutex
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requestCount++

		var request struct {
			Messages  []map[string]any `json:"messages"`
			MaxTokens int              `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch requestCount {
		case 1:
			if request.MaxTokens != 2048 {
				t.Errorf("max_tokens = %d, want configured 2048", request.MaxTokens)
			}
			systemPrompt, _ := request.Messages[0]["content"].(string)
			if !strings.Contains(systemPrompt, "120个字符以内") {
				t.Errorf("system prompt does not contain configured reply limit: %q", systemPrompt)
			}
			fmt.Fprint(w, `{"id":"tool-call","object":"chat.completion","created":1,"model":"test","choices":[{"index":0,"message":{"role":"assistant","reasoning_content":"需要先查询记录","tool_calls":[{"id":"call_1","type":"function","function":{"name":"repro_tool","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`)
		case 2:
			fmt.Fprint(w, `{"id":"truncated","object":"chat.completion","created":1,"model":"test","choices":[{"index":0,"message":{"role":"assistant","content":null,"reasoning_content":"输出额度在推理阶段耗尽"},"finish_reason":"length"}],"usage":{"completion_tokens":512,"prompt_tokens":100,"total_tokens":612,"completion_tokens_details":{"reasoning_tokens":512}}}`)
		default:
			for _, message := range request.Messages {
				role, _ := message["role"].(string)
				content, hasContent := message["content"]
				_, hasToolCalls := message["tool_calls"]
				if role == "assistant" && (!hasContent || content == "") && !hasToolCalls {
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprint(w, `{"error":{"message":"Invalid assistant message: content or tool_calls must be set","type":"invalid_request_error"}}`)
					return
				}
			}
			fmt.Fprint(w, `{"id":"recovered","object":"chat.completion","created":1,"model":"test","choices":[{"index":0,"message":{"role":"assistant","content":"第二次请求正常"},"finish_reason":"stop"}]}`)
		}
	}))
	defer server.Close()

	config.C.Bot.Name = "月灵"
	config.C.AI = config.AIConfig{
		DeepSeekKey:   "test",
		BaseURL:       server.URL + "/v1",
		Model:         "test",
		MaxTokens:     2048,
		ReplyMaxChars: 120,
	}
	_client = nil
	Register(ToolMeta{
		Name:     "repro_tool",
		Triggers: []string{"禁言"},
		Handler: func(*ToolContext) (string, error) {
			return "工具结果", nil
		},
	})

	dispatch := func(messageID int32, text string) string {
		t.Helper()
		event := &bot.GroupMessageEvent{
			SelfID:    907741045,
			MessageID: messageID,
			GroupID:   797038305,
			UserID:    435826135,
			Message:   bot.Msg().Text(text).Build(),
			Sender:    bot.Sender{Nickname: "月离", Role: "member"},
		}
		groupContext := &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: event}}
		reply, err := Dispatch(context.Background(), groupContext)
		if err != nil {
			t.Fatalf("Dispatch() error = %v", err)
		}
		return reply
	}

	firstReply := dispatch(1, "禁言上面说脏话的")
	if !strings.Contains(firstReply, "生成不完整") {
		t.Errorf("first Dispatch() reply = %q, want an incomplete-response hint", firstReply)
	}

	secondReply := dispatch(2, "禁言上面骂人的")
	if secondReply != "第二次请求正常" {
		t.Errorf("second Dispatch() reply = %q, want recovery without poisoned history", secondReply)
	}
}
