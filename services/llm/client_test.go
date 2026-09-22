package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

type completionFunc func(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)

func (f completionFunc) CreateChatCompletion(ctx context.Context, r openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return f(ctx, r)
}
func answer(text string) openai.ChatCompletionResponse {
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: "assistant", Content: text}, FinishReason: openai.FinishReasonStop}}}
}

func TestRetryPolicyAndRedactedErrors(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 422, 429, 500, 502, 503, 504} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			c := WithBackend(completionFunc(func(ctx context.Context, r openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
				calls++
				return openai.ChatCompletionResponse{}, &openai.APIError{HTTPStatusCode: status, Message: "secret-key-and-conversation"}
			}), Settings{Model: "test"})
			c.wait = func(context.Context, time.Duration) error { return nil }
			_, err := c.Complete(context.Background(), openai.ChatCompletionRequest{})
			expected := 1
			if status == 429 || status >= 500 {
				expected = 3
			}
			if calls != expected || err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
			if Status(err) != status {
				t.Fatalf("lost status: %v", err)
			}
		})
	}
}

func TestDeadlineStopsRetries(t *testing.T) {
	calls := 0
	c := WithBackend(completionFunc(func(ctx context.Context, r openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		calls++
		return openai.ChatCompletionResponse{}, &openai.APIError{HTTPStatusCode: 503}
	}), Settings{Model: "test", Timeout: time.Millisecond})
	_, err := c.Complete(context.Background(), openai.ChatCompletionRequest{})
	if calls != 1 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestDeepSeekBudgetAndReasoningProtocol(t *testing.T) {
	c := WithBackend(completionFunc(func(ctx context.Context, r openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		if r.MaxTokens != 4096 || r.ReasoningEffort != "low" {
			t.Errorf("budget=%d effort=%s", r.MaxTokens, r.ReasoningEffort)
		}
		if r.Messages[0].ReasoningContent != "preserve exactly" {
			t.Error("lost reasoning")
		}
		return answer("摘要"), nil
	}), Settings{Model: "deepseek-v4-pro"})
	_, err := c.Complete(context.Background(), openai.ChatCompletionRequest{MaxTokens: 50, Messages: []openai.ChatCompletionMessage{{Role: "assistant", Content: "previous", ReasoningContent: "preserve exactly"}}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTextRejectsIncompleteAndToolProtocol(t *testing.T) {
	for _, response := range []openai.ChatCompletionResponse{
		{}, answer("<tool_call>internal</tool_call>"),
		{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: "partial"}, FinishReason: openai.FinishReasonLength}}},
		{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{ReasoningContent: "only thought"}}}},
	} {
		calls := 0
		c := WithBackend(completionFunc(func(ctx context.Context, r openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
			calls++
			return response, nil
		}), Settings{Model: "test"})
		text, err := c.Text(context.Background(), openai.ChatCompletionRequest{})
		if text != "" || err == nil || calls != 2 {
			t.Errorf("text=%q err=%v calls=%d", text, err, calls)
		}
	}
}

func TestTextCorrectionPreservesJSONRequest(t *testing.T) {
	calls := 0
	c := WithBackend(completionFunc(func(ctx context.Context, r openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		calls++
		if r.ResponseFormat == nil || r.ResponseFormat.Type != openai.ChatCompletionResponseFormatTypeJSONObject {
			t.Fatal("lost output format")
		}
		if calls == 1 {
			return answer(""), nil
		}
		return answer(`{"translation":"你好"}`), nil
	}), Settings{Model: "test"})
	req := openai.ChatCompletionRequest{ResponseFormat: &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject}, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "hello"}}}
	got, err := c.Text(context.Background(), req)
	if err != nil || got != `{"translation":"你好"}` || len(req.Messages) != 1 {
		t.Fatalf("got=%s err=%v", got, err)
	}
}
