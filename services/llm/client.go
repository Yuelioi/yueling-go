// Package llm owns model transport policy for every AI feature. It never executes tools.
package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

type Completer interface {
	CreateChatCompletion(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

// Settings are captured once per operation. Credentials never appear in returned errors.
type Settings struct {
	APIKey          string
	BaseURL         string
	Model           string
	MaxTokens       int
	ReasoningEffort string
	Timeout         time.Duration
}

type Client struct {
	backend  Completer
	settings Settings
	wait     func(context.Context, time.Duration) error
}

func New(settings Settings) *Client {
	cfg := openai.DefaultConfig(settings.APIKey)
	if base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/"); base != "" {
		cfg.BaseURL = base
	}
	cfg.HTTPClient = &http.Client{Timeout: requestTimeout(settings)}
	return WithBackend(openai.NewClientWithConfig(cfg), settings)
}

// WithBackend is the same entry point used by tests and alternate compatible transports.
func WithBackend(backend Completer, settings Settings) *Client {
	return &Client{backend: backend, settings: settings, wait: waitContext}
}

func requestTimeout(s Settings) time.Duration {
	if s.Timeout > 0 {
		return s.Timeout
	}
	return 45 * time.Second
}

// Complete retries only failed HTTP requests, preserving the exact transcript.
func (c *Client) Complete(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout(c.settings))
	defer cancel()
	if req.Model == "" {
		req.Model = c.settings.Model
	}
	if strings.TrimSpace(req.Model) == "" {
		return openai.ChatCompletionResponse{}, &Error{Kind: Configuration}
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = c.settings.MaxTokens
		if req.MaxTokens <= 0 {
			req.MaxTokens = 4096
		}
	}
	// DeepSeek reasoning consumes the output budget too. Short legacy feature
	// budgets (50/200/700) otherwise expire before final content is produced.
	if strings.HasPrefix(req.Model, "deepseek-v4") || req.Model == "deepseek-flash" || req.Model == "deepseek-reasoner" {
		floor := c.settings.MaxTokens
		if floor <= 0 {
			floor = 4096
		}
		if req.MaxTokens < floor {
			req.MaxTokens = floor
		}
		if req.ReasoningEffort == "" {
			req.ReasoningEffort = c.settings.ReasoningEffort
			if req.ReasoningEffort == "" && req.Model != "deepseek-reasoner" {
				req.ReasoningEffort = "low"
			}
		}
	} else if req.ReasoningEffort == "" {
		req.ReasoningEffort = c.settings.ReasoningEffort
	}
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return openai.ChatCompletionResponse{}, classify(err)
		}
		response, err := c.backend.CreateChatCompletion(ctx, req)
		if err == nil {
			return response, nil
		}
		failure := classify(err)
		if attempt == 2 || (failure.Kind != Busy && failure.Kind != Unavailable) {
			return openai.ChatCompletionResponse{}, failure
		}
		if err := c.wait(ctx, time.Duration(attempt+1)*250*time.Millisecond); err != nil {
			return openai.ChatCompletionResponse{}, classify(err)
		}
	}
	panic("unreachable")
}

// Text accepts only a complete user-facing response. Protocol failures get one
// bounded regeneration, without accepting or executing any tool calls.
func (c *Client) Text(ctx context.Context, req openai.ChatCompletionRequest) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout(c.settings))
	defer cancel()
	req.Tools = nil
	req.ToolChoice = nil
	for attempt := 0; attempt < 2; attempt++ {
		response, err := c.Complete(ctx, req)
		if err != nil {
			return "", err
		}
		if len(response.Choices) > 0 {
			choice := response.Choices[0]
			if err := ValidateChoice(choice); err == nil && len(choice.Message.ToolCalls) == 0 {
				return strings.TrimSpace(choice.Message.Content), nil
			}
		}
		// Keep JSON / multimodal input unchanged. Never feed malformed output back.
		req.Messages = append([]openai.ChatCompletionMessage(nil), req.Messages...)
		instruction := "上一响应不完整。请按原要求输出完整最终内容，不输出推理标签或工具协议；本次不能调用工具。"
		if len(req.Messages) > 0 && req.Messages[0].Role == openai.ChatMessageRoleSystem {
			req.Messages[0].Content += "\n" + instruction
		} else {
			req.Messages = append([]openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: instruction}}, req.Messages...)
		}
	}
	return "", &Error{Kind: InvalidResponse}
}

func waitContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type Kind string

const (
	Configuration   Kind = "configuration"
	Authentication  Kind = "authentication"
	Compatibility   Kind = "compatibility"
	Busy            Kind = "rate_limit"
	Unavailable     Kind = "unavailable"
	Timeout         Kind = "timeout"
	Canceled        Kind = "canceled"
	InvalidResponse Kind = "invalid_response"
	Transport       Kind = "transport"
)

type Error struct {
	Kind   Kind
	Status int
	Detail string
}

func (e *Error) Error() string {
	return fmt.Sprintf("model request failed: kind=%s status=%d detail=%s", e.Kind, e.Status, e.Detail)
}
func (e *Error) Is(target error) bool {
	return e.Kind == Timeout && target == context.DeadlineExceeded || e.Kind == Canceled && target == context.Canceled
}
func Status(err error) int {
	var e *Error
	if errors.As(err, &e) {
		return e.Status
	}
	return classify(err).Status
}
func classify(err error) *Error {
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Kind: Timeout}
	}
	if errors.Is(err, context.Canceled) {
		return &Error{Kind: Canceled}
	}
	var existing *Error
	if errors.As(err, &existing) {
		return existing
	}
	status := 0
	var api *openai.APIError
	if errors.As(err, &api) {
		status = api.HTTPStatusCode
	}
	var request *openai.RequestError
	if errors.As(err, &request) {
		status = request.HTTPStatusCode
	}
	kind := Transport
	switch status {
	case 401, 403:
		kind = Authentication
	case 400, 404, 422:
		kind = Compatibility
	case 429:
		kind = Busy
	case 500, 502, 503, 504:
		kind = Unavailable
	}
	return &Error{Kind: kind, Status: status}
}
func UserMessage(err error) string {
	switch classify(err).Kind {
	case Configuration:
		return "AI 服务尚未配置完整，请联系管理员。"
	case Authentication:
		return "AI 服务认证失败，请联系管理员检查密钥和模型权限。"
	case Compatibility:
		return "AI 服务不接受当前请求，请联系管理员检查模型和接口兼容性。"
	case Busy:
		return "AI 服务请求额度受限，请稍后再试。"
	case Timeout:
		return "AI 响应超时，请稍后再试。"
	case Canceled:
		return "本次 AI 请求已取消。"
	case InvalidResponse:
		return "AI 回复生成不完整，请重试。"
	default:
		return "AI 服务连接失败，请稍后再试。"
	}
}
