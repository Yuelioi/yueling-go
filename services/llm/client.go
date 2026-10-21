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
	Kind            string
	CodexHome       string
	ProfileID       string
	Feature         string
	configError     error
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
}

func New(settings Settings) *Client {
	if settings.Kind == "codex" {
		return WithBackend(&codexBackend{settings: settings}, settings)
	}
	cfg := openai.DefaultConfig(settings.APIKey)
	if base := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/"); base != "" {
		cfg.BaseURL = base
	}
	cfg.HTTPClient = &http.Client{Timeout: requestTimeout(settings), Transport: usageTransport{base: deepSeekTransport{base: http.DefaultTransport}}}
	return WithBackend(openai.NewClientWithConfig(cfg), settings)
}

// WithBackend is the same entry point used by tests and alternate compatible transports.
func WithBackend(backend Completer, settings Settings) *Client {
	return &Client{backend: backend, settings: settings}
}

func requestTimeout(s Settings) time.Duration {
	if s.Timeout > 0 {
		return s.Timeout
	}
	return 45 * time.Second
}

// Complete makes one request. Failed generations are never silently replayed.
func (c *Client) Complete(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout(c.settings))
	defer cancel()
	if err := ctx.Err(); err != nil {
		return openai.ChatCompletionResponse{}, classify(err)
	}
	if c.settings.configError != nil {
		return openai.ChatCompletionResponse{}, &Error{Kind: Configuration}
	}
	req = c.prepare(req)
	if strings.TrimSpace(req.Model) == "" {
		return openai.ChatCompletionResponse{}, &Error{Kind: Configuration}
	}
	started := time.Now()
	response, err := c.backend.CreateChatCompletion(ctx, req)
	if err != nil {
		return openai.ChatCompletionResponse{}, classify(err)
	}
	observeUsage(withUsageSource(ctx, c.settings), req.Model, response.Usage, time.Since(started))
	return response, nil
}

func (c *Client) prepare(req openai.ChatCompletionRequest) openai.ChatCompletionRequest {
	if c.settings.Feature != "" && c.settings.Model != "" {
		req.Model = c.settings.Model
	}
	if c.settings.ProfileID != "" && c.settings.MaxTokens > 0 && (req.MaxTokens <= 0 || req.MaxTokens > c.settings.MaxTokens) {
		req.MaxTokens = c.settings.MaxTokens
	}
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
		if req.ReasoningEffort == "" {
			req.ReasoningEffort = c.settings.ReasoningEffort
			if req.ReasoningEffort == "" && req.Model != "deepseek-reasoner" {
				req.ReasoningEffort = "low"
			}
		}
		if req.ReasoningEffort != "none" {
			floor := c.settings.MaxTokens
			if floor <= 0 {
				floor = 4096
			}
			if req.MaxTokens < floor {
				req.MaxTokens = floor
			}
		}
	} else if req.ReasoningEffort == "" {
		req.ReasoningEffort = c.settings.ReasoningEffort
	}
	return req
}

// Text validates a single complete response and never regenerates malformed output.
func (c *Client) Text(ctx context.Context, req openai.ChatCompletionRequest) (string, error) {
	req.Tools = nil
	req.ToolChoice = nil
	model := req.Model
	if model == "" {
		model = c.settings.Model
	}
	if req.ReasoningEffort == "" && c.settings.ReasoningEffort == "" && (strings.HasPrefix(model, "deepseek-v4") || model == "deepseek-flash") {
		req.ReasoningEffort = "none"
	}
	req = c.prepare(req)
	response, err := c.Complete(ctx, req)
	if err != nil {
		return "", err
	}
	failure := &Error{Kind: InvalidResponse, Detail: "empty_choices"}
	if len(response.Choices) > 0 {
		choice := response.Choices[0]
		if err := ValidateChoice(choice); err != nil {
			errors.As(err, &failure)
		} else if len(choice.Message.ToolCalls) > 0 {
			failure.Detail = "unexpected_tool_calls"
		} else {
			return strings.TrimSpace(choice.Message.Content), nil
		}
	}
	failure.Response = responseMetadata(response, req, 1)
	return "", failure
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
		if failure.Detail == "animation_plan" {
			return "新GIF需一次提供2至12个非空动作帧，每帧20至5000毫秒（默认400）；帧描述或总提示词不能过长。"
		}
		if failure.Detail == "animation_source_conflict" {
			return "已有动态GIF直接逐帧编辑会保留原时序，请省略新建动画的动作帧参数；重做动作请使用静态参考图。"
		}
		if failure.Detail == "image_reference_limit" {
			return "参考图需要1至3张，头像也计入总数，请先选择要使用的图片。"
		}
		if failure.Detail == "multiple_animated_references" {
			return "一次编辑最多使用一个动态GIF，可同时添加静态参考图，总数不超过3张。"
		}
		if failure.Detail == "gif_edit_limits" {
			return "GIF逐帧AI编辑仅支持最多12帧、总像素1200万及4096×4096以内，请先缩短或缩小；基础处理可直接使用图片变换。"
		}
		return "AI 服务尚未配置完整，请联系管理员。"
	case Authentication:
		return "AI 服务认证失败，请联系管理员检查密钥和模型权限。"
	case Compatibility:
		if failure.Detail == "probe_tool_missing" {
			return "模型没有调用测试工具；此次工具能力测试未通过，其他能力需分别检查。"
		}
		if failure.Detail == "probe_text_mismatch" {
			return "模型未按测试要求回复，无法确认消息是否正确传递；此次聊天测试未通过。"
		}
		if failure.Detail == "codex_model_unsupported" {
			return "当前 Codex 接入拒绝了此模型，请检查客户端版本及账号可用模型。"
		}
		if failure.Detail == "codex_image_missing" {
			return "模型未返回生成图片，请确认账号图片能力可用，或调整描述后重试。"
		}
		if failure.Detail == "codex_image_failed" {
			return "Codex 图片生成未完成，请检查账号图片额度后重试。"
		}
		return "AI 服务不接受当前请求，请联系管理员检查模型和接口兼容性。"
	case Busy:
		return "AI 服务请求额度受限，请稍后再试。"
	case Timeout:
		return "AI 响应超时，请稍后再试。"
	case Canceled:
		return "本次 AI 请求已取消。"
	case InvalidResponse:
		switch failure.Detail {
		case "transparency_missing":
			return "模型返回的图片没有可用的透明背景，未作为抠图成功发送；请检查模型是否支持透明输出。"
		case "gif_encode_failed":
			return "GIF帧已编辑，但合成结果超限或失败，未发送残缺动画。"
		case "unexpected_animation":
			return "图片模型返回了意外的动画格式，未发送不完整结果。"
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
