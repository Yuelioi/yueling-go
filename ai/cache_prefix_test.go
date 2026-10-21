package ai

import (
	"context"
	"github.com/Yuelioi/yueling-go/internal/testconfig"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestCompactionLeavesHeadroomForFollowingTurns(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	cfg := old
	testconfig.Set(&cfg.AI, "Model", "fixture")
	config.Publish(cfg)
	calls := 0
	e := NewEngine(EngineDependencies{ModelBackend: intelligenceModel(func(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		calls++
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: "stop", Message: openai.ChatCompletionMessage{Content: "历史摘要"}}}}, nil
	})})
	s := newSession(1, 100)
	for i := 0; i < 9; i++ {
		s.pushUser("问题")
		msg := openai.ChatCompletionMessage{Role: "assistant", Content: "答案"}
		if i == 0 {
			msg.ToolCalls = []openai.ToolCall{{ID: "old", Function: openai.FunctionCall{Name: "action", Arguments: "{}"}}}
		}
		s.pushAssistant(msg)
		if i == 0 {
			s.pushToolResult("old", "旧任务已完成")
		}
	}
	e.compactConversation(context.Background(), s)
	for i := 0; i < 3; i++ {
		s.pushUser("追问")
		s.pushAssistant(openai.ChatCompletionMessage{Role: "assistant", Content: "答案"})
		e.compactConversation(context.Background(), s)
	}
	if calls != 1 || len(s.Messages) != 14 {
		t.Fatalf("compacted every turn: calls=%d messages=%d", calls, len(s.Messages))
	}
}
