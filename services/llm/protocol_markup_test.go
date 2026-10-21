package llm

import (
	"context"
	"errors"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestRejectsDSMLAndDeepSeekProtocolVariants(t *testing.T) {
	for _, content := range []string{
		`<｜｜DSML｜｜ calls><｜｜DSML｜｜ invoke name="invoke_available_tool">private-prompt</｜｜DSML｜｜ invoke></｜｜DSML｜｜ calls>`,
		`<||DSML|| calls><||DSML|| invoke name="invoke_available_tool">private-prompt</||DSML|| invoke>`,
		`</｜｜DSML｜｜ parameter>`,
		`正常开头，然后泄漏 <｜｜DSML｜｜ calls>private-prompt`,
		"```xml\n<｜｜DSML｜｜ invoke name=\"invoke_available_tool\">private-prompt\n```",
		`<｜tool▁calls▁begin｜>private-prompt<｜tool▁calls▁end｜>`,
		`<｜tool▁call▁begin｜>private-prompt`,
		`<||DSML`,
		`&lt;｜｜DSML｜｜ calls&gt;private-prompt`,
		"<｜｜DS\u200bML｜｜ calls>private-prompt",
	} {
		t.Run(content[:min(len(content), 24)], func(t *testing.T) {
			for _, withTools := range []bool{false, true} {
				choice := answer(content).Choices[0]
				if withTools {
					choice.Message.ToolCalls = []openai.ToolCall{{ID: "call_1", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "fixture", Arguments: `{}`}}}
				}
				err := ValidateChoice(choice)
				var failure *Error
				if !errors.As(err, &failure) || failure.Detail != "protocol_markup" {
					t.Errorf("protocol artifact accepted: withTools=%v", withTools)
				}
			}
		})
	}
}

func TestTextDSMLRecoveryDoesNotExposeOrPersistMalformedOutput(t *testing.T) {
	for _, persistent := range []bool{false, true} {
		calls := 0
		client := WithBackend(completionFunc(func(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
			calls++
			for _, msg := range req.Messages {
				if strings.Contains(msg.Content, "private-prompt") {
					t.Error("malformed protocol content fed back to model")
				}
			}
			if calls == 1 || persistent {
				return answer(`<｜｜DSML｜｜ calls>private-prompt</｜｜DSML｜｜ calls>`), nil
			}
			return answer("已完成整理。"), nil
		}), Settings{Model: "fixture"})
		reply, err := client.Text(context.Background(), openai.ChatCompletionRequest{Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "整理内容"}}})
		if calls != 2 || strings.Contains(reply, "DSML") || strings.Contains(reply, "private-prompt") {
			t.Fatalf("unsafe text result: calls=%d", calls)
		}
		if persistent && (err == nil || reply != "") {
			t.Fatal("persistent malformed text accepted")
		}
		if !persistent && (err != nil || reply != "已完成整理。") {
			t.Fatalf("valid correction discarded: %v", err)
		}
	}
}

func TestOrdinaryProtocolDiscussionAndStructuredCallsStillWork(t *testing.T) {
	for _, content := range []string{"DSML 是一种协议格式，tool calling 用于调用工具。", "请保留PNG的Alpha通道。", "数学里的竖线 |x| 表示绝对值。"} {
		if err := ValidateChoice(answer(content).Choices[0]); err != nil {
			t.Fatal("ordinary explanation rejected", err)
		}
	}
	choice := openai.ChatCompletionChoice{Message: openai.ChatCompletionMessage{Role: "assistant", ReasoningContent: `<｜｜DSML｜｜ calls>private reasoning`, ToolCalls: []openai.ToolCall{{ID: "call_1", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "fixture", Arguments: `{"text":"DSML"}`}}}}, FinishReason: openai.FinishReasonToolCalls}
	if err := ValidateChoice(choice); err != nil {
		t.Fatal("valid structured call or separate reasoning blocked", err)
	}
}
