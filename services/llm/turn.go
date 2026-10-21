package llm

import (
	"context"
	"encoding/json"

	openai "github.com/sashabaranov/go-openai"
)

// StartTurn binds native tool continuation to one business turn. Call Close even
// after failure: a provider can still be waiting for a host tool result.
func (c *Client) StartTurn(ctx context.Context) *Client {
	copy := *c
	if c.settings.Kind == "codex" {
		if _, ok := c.backend.(*codexBackend); ok {
			copy.backend = newCodexTurn(ctx, c.settings)
		}
	}
	return &copy
}
func (c *Client) Close() {
	if backend, ok := c.backend.(*codexTurn); ok {
		backend.Close()
	}
}

type turnResponse struct {
	response openai.ChatCompletionResponse
	err      error
}
type codexTurn struct {
	ctx      context.Context
	cancel   context.CancelFunc
	settings Settings
	outputs  chan turnResponse
	results  chan string
	done     chan struct{}
	started  bool // Complete is serialized by the conversation runtime.
	pending  string
	tools    string
}

func newCodexTurn(ctx context.Context, settings Settings) *codexTurn {
	ctx, cancel := context.WithCancel(ctx)
	return &codexTurn{ctx: ctx, cancel: cancel, settings: settings, outputs: make(chan turnResponse), results: make(chan string), done: make(chan struct{})}
}
func (t *codexTurn) Close() {
	t.cancel()
	if t.started {
		<-t.done
	}
}
func (t *codexTurn) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	tools, _ := json.Marshal(req.Tools)
	if !t.started {
		t.started = true
		t.tools = string(tools)
		go func() {
			defer close(t.done)
			response, err := (&codexBackend{settings: t.settings}).completeWithTools(t.ctx, req, nil, func(call openai.ToolCall) (string, error) {
				response := openai.ChatCompletionResponse{Model: req.Model, Choices: []openai.ChatCompletionChoice{{FinishReason: openai.FinishReasonToolCalls, Message: openai.ChatCompletionMessage{Role: "assistant", ToolCalls: []openai.ToolCall{call}}}}}
				select {
				case t.outputs <- turnResponse{response: response}:
				case <-t.ctx.Done():
					return "", t.ctx.Err()
				}
				select {
				case result := <-t.results:
					return result, nil
				case <-t.ctx.Done():
					return "", t.ctx.Err()
				}
			})
			select {
			case t.outputs <- turnResponse{response: response, err: err}:
			case <-t.ctx.Done():
			}
		}()
	} else {
		if string(tools) != t.tools || t.pending == "" {
			return openai.ChatCompletionResponse{}, &Error{Kind: Configuration, Detail: "codex_turn_contract"}
		}
		result := ""
		matches := 0
		for _, msg := range req.Messages {
			if msg.Role == "tool" && msg.ToolCallID == t.pending {
				result = msg.Content
				matches++
			}
		}
		if matches != 1 {
			return openai.ChatCompletionResponse{}, &Error{Kind: Configuration, Detail: "codex_tool_result_pairing"}
		}
		select {
		case t.results <- result:
		case <-ctx.Done():
			t.cancel()
			return openai.ChatCompletionResponse{}, ctx.Err()
		case <-t.ctx.Done():
			return openai.ChatCompletionResponse{}, t.ctx.Err()
		}
	}
	select {
	case output := <-t.outputs:
		t.pending = ""
		if len(output.response.Choices) > 0 && len(output.response.Choices[0].Message.ToolCalls) > 0 {
			t.pending = output.response.Choices[0].Message.ToolCalls[0].ID
		}
		return output.response, output.err
	case <-ctx.Done():
		t.cancel()
		return openai.ChatCompletionResponse{}, ctx.Err()
	case <-t.ctx.Done():
		return openai.ChatCompletionResponse{}, t.ctx.Err()
	}
}
