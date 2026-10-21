package ai

import (
	"context"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestImageTurnPreservesLongParentBudget(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	cfg := old
	cfg.AI.Model = "fixture"
	config.Publish(cfg)
	calls := 0
	engine := NewEngine(EngineDependencies{ModelBackend: intelligenceModel(func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		calls++
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 90*time.Second {
			t.Error("individual model request lost its own timeout")
		}
		message := openai.ChatCompletionMessage{Role: "assistant", Content: "已完成。"}
		if calls == 1 {
			message.Content = ""
			message.ToolCalls = []openai.ToolCall{{ID: "image", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "image_fixture", Arguments: "{}"}}}
		}
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: message, FinishReason: openai.FinishReasonStop}}}, nil
	})})
	ran := false
	engine.Registry.Register(ToolMeta{Name: "image_fixture", ReadOnly: true, Triggers: []string{"GIF改图"}, ResultHandler: func(c *ToolContext) ToolResult {
		ran = true
		deadline, ok := c.Context().Deadline()
		if !ok || time.Until(deadline) < 110*time.Second {
			t.Error("image tool is still capped below its request timeout")
		}
		return ToolResult{Status: ToolSucceeded, Content: "已检查"}
	}})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	event := &bot.GroupMessageEvent{SelfID: 1, GroupID: 100, UserID: 42, MessageID: 1, Message: bot.Msg().Text("GIF改图").Build(), Sender: bot.Sender{Role: "member"}}
	if err := engine.Dispatch(ctx, &bot.GroupContext{MsgCtx: &bot.MsgCtx{Event: event}}, func(context.Context, string) error { cancel(); return nil }); err != nil {
		t.Fatal(err)
	}
	if !ran || calls != 2 {
		t.Fatal("image tool was not exercised")
	}
}
