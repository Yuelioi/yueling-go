package ai

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/config"
	model "github.com/Yuelioi/yueling-go/services/llm"
	openai "github.com/sashabaranov/go-openai"
)

// Optional test transport. Production clients capture current configuration per operation.
var _client *openai.Client

func modelClient() *model.Client {
	settings := model.FromConfig(config.C.AI)
	if _client != nil {
		return model.WithBackend(_client, settings)
	}
	return model.New(settings)
}

// NewClient is retained for callers constructing a standalone compatible transport.
func NewClient(apiKey, baseURL string) *openai.Client {
	cfg := openai.DefaultConfig(apiKey)
	if base := strings.TrimRight(strings.TrimSpace(baseURL), "/"); base != "" {
		cfg.BaseURL = base
	}
	cfg.HTTPClient = &http.Client{Timeout: 45 * time.Second}
	return openai.NewClientWithConfig(cfg)
}

func completeChat(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	started := time.Now()
	response, err := modelClient().Complete(ctx, req)
	traceStage(ctx, "model_tools", started, err)
	return response, err
}
func completeText(ctx context.Context, req openai.ChatCompletionRequest) (string, error) {
	started := time.Now()
	response, err := modelClient().Text(ctx, req)
	traceStage(ctx, "model_text", started, err)
	return response, err
}
func modelErrorStatus(err error) int   { return model.Status(err) }
func modelErrorReply(err error) string { return model.UserMessage(err) }
