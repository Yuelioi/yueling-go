package llm

import (
	"context"
	"errors"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestTextKeepsOutputLimitCauseAndDoesNotRepeat(t *testing.T) {
	calls := 0
	c := WithBackend(completionFunc(func(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		calls++
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{
			Message:      openai.ChatCompletionMessage{ReasoningContent: "synthetic reasoning"},
			FinishReason: openai.FinishReasonLength,
		}}, Usage: openai.Usage{CompletionTokens: 300}}, nil
	}), Settings{Model: "test", MaxTokens: 300})
	reply, err := c.Text(context.Background(), openai.ChatCompletionRequest{})
	var failure *Error
	if reply != "" || !errors.As(err, &failure) || failure.Detail != "output_limit" || calls != 1 {
		t.Fatalf("output exhaustion lost its cause or was repeated: reply=%q err=%v calls=%d", reply, err, calls)
	}
	if !strings.Contains(UserMessage(err), "输出额度") {
		t.Fatalf("output exhaustion still looks like an unspecified format error: %q", UserMessage(err))
	}
	if failure.Response.MaxTokens != 300 || failure.Response.CompletionTokens != 300 || failure.Response.FinishReason != "length" || failure.Response.ContentChars != 0 || failure.Response.ReasoningChars == 0 || failure.Response.Attempts != 1 {
		t.Fatalf("missing diagnostic counts: %+v", failure.Response)
	}
}

func TestTextUsesSupportedNonThinkingDefault(t *testing.T) {
	for _, test := range []struct{ name, model, configured, requested, want string }{
		{"v4_default", "deepseek-v4-pro", "", "", "none"},
		{"flash_default", "deepseek-flash", "", "", "none"},
		{"explicit_config", "deepseek-v4-pro", "high", "", "high"},
		{"explicit_request", "deepseek-v4-pro", "high", "low", "low"},
		{"legacy_reasoner", "deepseek-reasoner", "", "", ""},
		{"other_provider", "compatible-model", "", "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := WithBackend(completionFunc(func(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
				if req.ReasoningEffort != test.want || req.MaxTokens != 300 || len(req.Tools) != 0 {
					t.Errorf("text policy: effort=%q want=%q budget=%d tools=%d", req.ReasoningEffort, test.want, req.MaxTokens, len(req.Tools))
				}
				return answer("完整总结"), nil
			}), Settings{Model: test.model, MaxTokens: 300, ReasoningEffort: test.configured})
			_, err := c.Text(context.Background(), openai.ChatCompletionRequest{ReasoningEffort: test.requested})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTextFailureDetailsRemainDistinctAndRedacted(t *testing.T) {
	for _, test := range []struct {
		name, detail string
		response     openai.ChatCompletionResponse
	}{
		{"no_choices", "empty_choices", openai.ChatCompletionResponse{}},
		{"empty", "empty_content", answer("")},
		{"only_reasoning", "reasoning_only", openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{ReasoningContent: "private-reasoning-marker"}, FinishReason: openai.FinishReasonStop}}}},
		{"markup", "protocol_markup", answer("<tool_call>private-content-marker</tool_call>")},
		{"unexpected_tools", "unexpected_tool_calls", openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{ToolCalls: []openai.ToolCall{{ID: "call_1", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "private-tool-marker", Arguments: `{}`}}}}, FinishReason: openai.FinishReasonToolCalls}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := WithBackend(completionFunc(func(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
				return test.response, nil
			}), Settings{Model: "test", MaxTokens: 300})
			_, err := c.Text(context.Background(), openai.ChatCompletionRequest{})
			var failure *Error
			if !errors.As(err, &failure) || failure.Detail != test.detail || failure.Response.Attempts != 2 {
				t.Fatalf("incorrect failure classification: %v", err)
			}
			if strings.Contains(err.Error(), "private-") || strings.Contains(UserMessage(err), "private-") {
				t.Fatal("diagnostic exposed response contents")
			}
		})
	}
	response := answer("")
	response.Choices[0].FinishReason = openai.FinishReason("private-finish-marker\nsecret")
	meta := responseMetadata(response, openai.ChatCompletionRequest{}, 1)
	if meta.FinishReason != "other" {
		t.Fatal("untrusted finish reason reached diagnostic metadata")
	}
}
