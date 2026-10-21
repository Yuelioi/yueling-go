package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

type intelligenceModel func(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)

func (f intelligenceModel) CreateChatCompletion(c context.Context, r openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return f(c, r)
}
func TestDiscoveryBoundsAndPermissions(t *testing.T) {
	a := &ToolMeta{Name: "a", Requires: []string{"b", "forbidden"}}
	b := &ToolMeta{Name: "b", Requires: []string{"a"}}
	exposed := map[string]bool{}
	d := newToolDiscovery([]*ToolMeta{a, b}, exposed)
	if d.discover(`{"names":["a","forbidden"]}`).Status != ToolRejected || len(exposed) != 0 {
		t.Fatal("partially exposed invalid selection")
	}
	if d.discover(`{"names":["a"]}`).Status != ToolSucceeded || !exposed["a"] || !exposed["b"] || exposed["forbidden"] {
		t.Fatal("dependency permission boundary")
	}
	if d.discover(`{"names":["a"]}`).Status != ToolRejected {
		t.Fatal("unbounded discovery")
	}
}
func TestConversationDiscoversUnmatchedToolAndExecutesOnce(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	cfg := old
	cfg.AI.Model = "fixture"
	config.Publish(cfg)
	meta := &ToolMeta{Name: "weather", Description: "查询指定城市的天气", Triggers: []string{"天气"}}
	if len(Route("杭州出门要带伞吗", []*ToolMeta{meta})) != 0 {
		t.Fatal("fixture must miss keyword routing")
	}
	exposed := map[string]bool{}
	d := newToolDiscovery([]*ToolMeta{meta}, exposed)
	steps, executions := 0, 0
	e := NewEngine(EngineDependencies{ModelBackend: intelligenceModel(func(_ context.Context, r openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		steps++
		msg := openai.ChatCompletionMessage{Role: "assistant"}
		switch steps {
		case 1:
			if len(r.Tools) != 2 || r.Tools[0].Function.Name != discoveryName {
				t.Fatal("discovery unavailable")
			}
			msg.ToolCalls = []openai.ToolCall{{ID: "d", Type: "function", Function: openai.FunctionCall{Name: discoveryName, Arguments: `{"names":["weather"]}`}}}
		case 2:
			if len(r.Tools) != 2 || r.Tools[1].Function.Name != invocationName || !strings.Contains(r.Messages[len(r.Messages)-1].Content, "weather") {
				t.Fatal("schema not loaded")
			}
			msg.ToolCalls = []openai.ToolCall{{ID: "w", Type: "function", Function: openai.FunctionCall{Name: invocationName, Arguments: `{"name":"weather","arguments":"{}"}`}}}
		default:
			msg.Content = "杭州有雨，带伞。"
		}
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: msg, FinishReason: "stop"}}}, nil
	})})
	reply, err := e.runConversation(context.Background(), newSession(1, 2), "杭州出门要带伞吗", d.prompt(), nil, func(tc openai.ToolCall) ToolResult {
		if !exposed[tc.Function.Name] {
			t.Fatal("tool not authorized")
		}
		executions++
		return ToolResult{Status: ToolSucceeded, Content: "杭州下雨", Invoked: true}
	}, d)
	if err != nil || !strings.Contains(reply, "带伞") || executions != 1 || steps != 3 {
		t.Fatalf("reply=%s err=%v executions=%d steps=%d", reply, err, executions, steps)
	}
}
func TestCompactionPreservesFactsWithoutReasoning(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	cfg := old
	cfg.AI.Model = "fixture"
	config.Publish(cfg)
	s := newSession(1, 2)
	for i := 0; i < 10; i++ {
		s.pushUser("名字已改成老王")
		msg := openai.ChatCompletionMessage{Role: "assistant", Content: "尚未保存，等待选图", ReasoningContent: "SECRET_REASONING"}
		if i == 0 {
			msg.ToolCalls = []openai.ToolCall{{ID: "old", Function: openai.FunctionCall{Name: "query", Arguments: "{}"}}}
		}
		s.pushAssistant(msg)
		if i == 0 {
			s.pushToolResult("old", "待选图片")
		}
	}
	e := NewEngine(EngineDependencies{ModelBackend: intelligenceModel(func(_ context.Context, r openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		if strings.Contains(r.Messages[1].Content, "SECRET_REASONING") {
			t.Fatal("reasoning sent for summarization")
		}
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: "assistant", Content: "名字：老王；状态：尚未保存，等待选图。"}, FinishReason: "stop"}}}, nil
	})})
	e.compactConversation(context.Background(), s)
	if len(s.Messages) != 8 || !strings.Contains(s.retainedContext(), "尚未保存") {
		t.Fatal("compaction lost task status")
	}
}
