package ai

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestSharedConversationPreservesPrefixAcrossSpeakers(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	cfg := old
	cfg.AI.Model = "fixture"
	config.Publish(cfg)
	var requests []openai.ChatCompletionRequest
	e := NewEngine(EngineDependencies{ModelBackend: intelligenceModel(func(_ context.Context, r openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		requests = append(requests, r)
		return openai.ChatCompletionResponse{Usage: openai.Usage{PromptTokens: 100}, Choices: []openai.ChatCompletionChoice{{FinishReason: "stop", Message: openai.ChatCompletionMessage{Role: "assistant", Content: "收到"}}}}, nil
	})})
	a := &ToolMeta{Name: "a"}
	b := &ToolMeta{Name: "b"}
	catalog := []*ToolMeta{b, a}
	s := e.Sessions.Get(100, 1)
	for i := 0; i < 2; i++ {
		allowed := catalog
		if i == 1 {
			allowed = []*ToolMeta{a}
		}
		d := newToolDiscovery(allowed, map[string]bool{})
		d.catalog = catalog
		prompt := e.buildSystemPrompt(100, d.prompt())
		if strings.Contains(prompt, "当前时间是") || strings.Contains(prompt, "speaker_id") {
			t.Fatal("dynamic data in prefix")
		}
		runtime := e.turnRuntime(int64(i+1), "关系资料", "问题")
		if _, err := e.runConversation(context.Background(), s, runtime+" 发言者"+string(rune('A'+i)), prompt, nil, func(openai.ToolCall) ToolResult { return ToolResult{} }, d); err != nil {
			t.Fatal(err)
		}
		s.lastTurnAt = time.Now()
	}
	if !reflect.DeepEqual(requests[0].Tools, requests[1].Tools) {
		t.Fatal("permission changes rewrote gateway schemas")
	}
	if !reflect.DeepEqual(requests[0].Messages, requests[1].Messages[:len(requests[0].Messages)]) {
		t.Fatal("next speaker rewrote prior prefix")
	}
	if e.Sessions.Get(100, 2) != s || e.Sessions.Get(200, 1) == s {
		t.Fatal("group scope broken")
	}
}
func TestGatewayCannotExecuteUndiscoveredOrForbiddenTool(t *testing.T) {
	calls := 0
	execute := func(tc openai.ToolCall) ToolResult {
		calls++
		if tc.Function.Name != "allowed" || tc.Function.Arguments != "{}" {
			t.Fatal(tc)
		}
		return ToolResult{Status: ToolSucceeded}
	}
	d := newToolDiscovery([]*ToolMeta{{Name: "allowed"}}, map[string]bool{})
	call := func(name string) ToolResult {
		raw, _ := json.Marshal(map[string]string{"name": name, "arguments": "{}"})
		return d.invoke(openai.ToolCall{Function: openai.FunctionCall{Arguments: string(raw)}}, execute)
	}
	if call("allowed").Status != ToolRejected || call("forbidden").Status != ToolRejected || calls != 0 {
		t.Fatal("unauthorized execution")
	}
	if d.discover(`{"names":["allowed"]}`).Status != ToolSucceeded || call("allowed").Status != ToolSucceeded || calls != 1 {
		t.Fatal("discovered tool failed")
	}
}
func TestCompactionLeavesHeadroomForFollowingTurns(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	cfg := old
	cfg.AI.Model = "fixture"
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
