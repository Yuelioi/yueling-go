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
	req = c.prepare(req)
	if strings.TrimSpace(req.Model) == "" {
		return openai.ChatCompletionResponse{}, &Error{Kind: Configuration}
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

func (c *Client) prepare(req openai.ChatCompletionRequest) openai.ChatCompletionRequest {
	if req.Model == "" {
		req.Model = c.settings.Model
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
	return req
}

// Text accepts only a complete user-facing response. Protocol failures get one
// bounded regeneration; a spent output budget is not a protocol-repair problem.
func (c *Client) Text(ctx context.Context, req openai.ChatCompletionRequest) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout(c.settings))
	defer cancel()
	req.Tools = nil
	req.ToolChoice = nil
	model := req.Model
	if model == "" {
		model = c.settings.Model
	}
	// Known V4 text tasks do not need a separate thinking transcript by default.
	// Explicit caller/configuration preferences and other providers stay intact.
	if req.ReasoningEffort == "" && c.settings.ReasoningEffort == "" && (strings.HasPrefix(model, "deepseek-v4") || model == "deepseek-flash") {
		req.ReasoningEffort = "none"
	}
	req = c.prepare(req)
	var failure *Error
	for attempt := 0; attempt < 2; attempt++ {
		response, err := c.Complete(ctx, req)
		if err != nil {
			return "", err
		}
		failure = &Error{Kind: InvalidResponse, Detail: "empty_choices"}
		if len(response.Choices) > 0 {
			choice := response.Choices[0]
			validationErr := ValidateChoice(choice)
			if validationErr == nil && len(choice.Message.ToolCalls) == 0 {
				return strings.TrimSpace(choice.Message.Content), nil
			}
			if validationErr != nil {
				errors.As(validationErr, &failure)
			} else {
				failure.Detail = "unexpected_tool_calls"
			}
		}
		failure.Response = responseMetadata(response, req, attempt+1)
		if failure.Detail == "output_limit" || failure.Detail == "content_filter" {
			return "", failure
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
	return "", failure
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
	Kind     Kind
	Status   int
	Detail   string
	Response ResponseMetadata
}

func (e *Error) Error() string {
	message := fmt.Sprintf("model request failed: kind=%s status=%d detail=%s", e.Kind, e.Status, e.Detail)
	if e.Response.Attempts > 0 {
		message += fmt.Sprintf(" finish=%s max_tokens=%d completion_tokens=%d content_chars=%d reasoning_chars=%d attempts=%d",
			e.Response.FinishReason, e.Response.MaxTokens, e.Response.CompletionTokens, e.Response.ContentChars, e.Response.ReasoningChars, e.Response.Attempts)
	}
	return message
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
	failure := classify(err)
	switch failure.Kind {
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
		switch failure.Detail {
		case "output_limit":
			return "AI 输出额度不足，未能生成完整回复，请管理员检查输出额度和推理设置。"
		case "empty_choices", "empty_content", "reasoning_only":
			return "AI 没有返回可用正文，请稍后重试。"
		}
		return "AI 回复生成不完整，请重试。"
	default:
		return "AI 服务连接失败，请稍后再试。"
	}
}
