package ai

import (
	"context"
	"github.com/Yuelioi/yueling-go/internal/testconfig"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestNativeToolRoundTripUsesBusinessSchema(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	cfg := old
	testconfig.Set(&cfg.AI, "Model", "fixture")
	config.Publish(cfg)
	calls, actions := 0, 0
	meta := &ToolMeta{Name: "edit_image", Params: []Param{{Name: "image_ids", Type: "array", ItemsType: "string", Required: true}, {Name: "prompt", Type: "string", Required: true}}}
	e := NewEngine(EngineDependencies{ModelBackend: intelligenceModel(func(_ context.Context, r openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		calls++
		if len(r.Tools) != 1 || r.Tools[0].Function.Name != "edit_image" {
			t.Fatalf("business schema not native: %+v", r.Tools)
		}
		msg := openai.ChatCompletionMessage{Role: "assistant", Content: "完成"}
		finish := openai.FinishReasonStop
		if calls == 1 {
			msg.Content = ""
			msg.ToolCalls = []openai.ToolCall{{ID: "c", Type: "function", Function: openai.FunctionCall{Name: "edit_image", Arguments: `{"image_ids":["image_13"],"prompt":"水彩"}`}}}
			finish = openai.FinishReasonToolCalls
		} else if r.Messages[len(r.Messages)-1].ToolCallID != "c" {
			t.Fatal("missing matching native result")
		}
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: finish, Message: msg}}}, nil
	})})
	s := newSession(1, 2)
	_, err := e.runConversation(context.Background(), s, "改成水彩", "rules", []openai.Tool{meta.Schema()}, func(tc openai.ToolCall) ToolResult {
		actions++
		if tc.Function.Arguments != `{"image_ids":["image_13"],"prompt":"水彩"}` {
			t.Fatal(tc)
		}
		return ToolResult{Status: ToolSucceeded, Content: "done", Invoked: true}
	})
	if err != nil || calls != 2 || actions != 1 {
		t.Fatalf("calls=%d actions=%d err=%v", calls, actions, err)
	}
	for _, m := range s.Messages {
		if len(m.ToolCalls) > 0 || m.Role == "tool" || m.ReasoningContent != "" {
			t.Fatal("provider protocol leaked across turns")
		}
	}
}

type intelligenceModel func(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)

func (f intelligenceModel) CreateChatCompletion(c context.Context, r openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return f(c, r)
}

func TestNativeToolBatchRejectsUnadvertisedAndDuplicateIDsBeforeExecution(t *testing.T) {
	for _, forged := range []bool{false, true} {
		t.Run(map[bool]string{false: "duplicate", true: "unadvertised"}[forged], func(t *testing.T) {
			old := config.Current()
			t.Cleanup(func() { config.Publish(old) })
			cfg := old
			testconfig.Set(&cfg.AI, "Model", "fixture")
			config.Publish(cfg)
			calls := []openai.ToolCall{{ID: "c", Type: "function", Function: openai.FunctionCall{Name: "allowed", Arguments: `{}`}}, {ID: "c", Type: "function", Function: openai.FunctionCall{Name: "allowed", Arguments: `{}`}}}
			if forged {
				calls[1].ID = "other"
				calls[1].Function.Name = "forbidden"
			}
			e := NewEngine(EngineDependencies{ModelBackend: intelligenceModel(func(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
				return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: openai.FinishReasonToolCalls, Message: openai.ChatCompletionMessage{ToolCalls: calls}}}}, nil
			})})
			executed := 0
			s := newSession(1, 2)
			_, err := e.runConversation(context.Background(), s, "query", "rules", []openai.Tool{(&ToolMeta{Name: "allowed"}).Schema()}, func(openai.ToolCall) ToolResult { executed++; return ToolResult{Invoked: true} })
			if err == nil || executed != 0 || len(s.Messages) != 0 {
				t.Fatal("invalid batch partially executed")
			}
		})
	}
}
