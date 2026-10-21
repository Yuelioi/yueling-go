package tools

import (
	"context"
	"github.com/Yuelioi/yueling-go/internal/testconfig"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestCasualDispatchUsesCompactCatalogWithoutExtraModelCall(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	cfg := old
	cfg.AI = testconfig.AI("fixture", "", "synthetic")
	config.Publish(cfg)
	var request openai.ChatCompletionRequest
	calls := 0
	engine := ai.NewEngine(ai.EngineDependencies{ModelBackend: imageRequestModel(func(req openai.ChatCompletionRequest) openai.ChatCompletionResponse {
		request = req
		calls++
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: "stop", Message: openai.ChatCompletionMessage{Role: "assistant", Content: "今天聊点开心的。"}}}}
	})})
	RegisterAll(engine.Registry, Dependencies{})
	event := &bot.GroupMessageEvent{GroupID: 987001, UserID: 987001, MessageID: 1, Message: bot.Msg().Text("今天心情不错").Build()}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := engine.Dispatch(ctx, &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: event}}, func(context.Context, string) error { cancel(); return nil }); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(request.Tools) == 0 {
		t.Fatalf("casual dispatch added model calls or lost semantic tools: calls=%d tools=%d", calls, len(request.Tools))
	}
	for _, tool := range request.Tools {
		if tool.Function.Name == "invoke_available_tool" || tool.Function.Name == "discover_available_tools" {
			t.Fatal("wrapper advertised")
		}
	}
	if strings.Contains(request.Messages[0].Content, "本轮已加载的工具参数定义") {
		t.Fatal("schema embedded in prompt")
	}
}
