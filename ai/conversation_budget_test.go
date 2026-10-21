package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Yuelioi/yueling-go/internal/testconfig"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestCasualConversationUsesRecentTurnsWithoutSummaryCalls(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	cfg := old
	testconfig.Set(&cfg.AI, "Model", "fixture")
	config.Publish(cfg)
	calls := 0
	var lastInputChars, fullInputChars int
	var s *Session
	e := NewEngine(EngineDependencies{ModelBackend: intelligenceModel(func(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		calls++
		users := 0
		for _, msg := range req.Messages {
			if msg.Role == "user" {
				users++
			}
		}
		if users > 3 || strings.Contains(req.Messages[0].Content, "OLDER_SUMMARY") {
			t.Fatalf("casual request grew: users=%d", users)
		}
		if calls == 12 {
			data, _ := json.Marshal(req.Messages)
			lastInputChars = len([]rune(string(data)))
			full := append([]openai.ChatCompletionMessage{req.Messages[0]}, s.Messages...)
			data, _ = json.Marshal(full)
			fullInputChars = len([]rune(string(data)))
		}
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: "stop", Message: openai.ChatCompletionMessage{Role: "assistant", Content: "接着聊", ReasoningContent: "retained protocol"}}}}, nil
	})})
	s = e.Sessions.Get(100, 1)
	s.historySummary = "OLDER_SUMMARY"
	for i := 0; i < 12; i++ {
		identity := turnIdentity(&bot.GroupMessageEvent{GroupID: 100, UserID: int64(i%2 + 1)})
		if _, err := e.runConversation(context.Background(), s, fmt.Sprintf("第%d句：%s", i, strings.Repeat("聊天", 20))+identity, "rules", nil, func(openai.ToolCall) ToolResult {
			t.Fatal("chat unexpectedly executed a tool")
			return ToolResult{}
		}); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 12 || len(s.Messages) <= 6 || e.Sessions.Get(100, 2) != s || len(e.Sessions.Get(101, 1).Messages) != 0 {
		t.Fatalf("calls=%d retained_messages=%d", calls, len(s.Messages))
	}
	if lastInputChars >= fullInputChars {
		t.Fatalf("history projection did not shrink input: sent=%d full=%d", lastInputChars, fullInputChars)
	}
	t.Logf("12 casual turns: model_calls=%d last_request_chars=%d full_history_request_chars=%d (character counts, not tokens)", calls, lastInputChars, fullInputChars)
}

func TestRecentContextKeepsOversizedToolTurnWhole(t *testing.T) {
	s := newSession(1, 2)
	s.pushUser("较早闲聊")
	s.pushAssistant(openai.ChatCompletionMessage{Role: "assistant", Content: "闲聊回答"})
	s.pushUser("读取资料")
	s.pushAssistant(openai.ChatCompletionMessage{Role: "assistant", ReasoningContent: "required reasoning", ToolCalls: []openai.ToolCall{{ID: "read", Function: openai.FunctionCall{Name: "read", Arguments: "{}"}}}})
	s.pushToolResult("read", strings.Repeat("资料", 2000))
	s.pushAssistant(openai.ChatCompletionMessage{Role: "assistant", Content: "资料已读取"})
	start := s.recentHistoryStart()
	if start != 2 || s.Messages[start+1].ReasoningContent != "required reasoning" || s.Messages[start+2].ToolCallID != "read" {
		t.Fatalf("partial tool turn: start=%d", start)
	}
}
