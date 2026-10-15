package ai

import (
	"context"
	"time"

	"github.com/Yuelioi/yueling-go/config"
	model "github.com/Yuelioi/yueling-go/services/llm"
	openai "github.com/sashabaranov/go-openai"
)

func modelClient() *model.Client { return model.New(model.FromConfig(config.Current().AI)) }

func (e *Engine) modelClientFor(cfg config.AIConfig) *model.Client {
	return e.modelClientForFeature(cfg, "conversation")
}
func (e *Engine) modelClientForFeature(cfg config.AIConfig, feature string) *model.Client {
	settings := model.ForFeature(cfg, feature)
	if e.modelBackend != nil {
		return model.WithBackend(e.modelBackend, settings)
	}
	return model.New(settings)
}

func completeChat(ctx context.Context, client *model.Client, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	started := time.Now()
	req.Model = ""
	response, err := client.Complete(ctx, req)
	traceStage(ctx, "model_tools", started, err)
	return response, err
}
func completeText(ctx context.Context, req openai.ChatCompletionRequest) (string, error) {
	return completeFeature("conversation")(ctx, req)
}
func completeFeature(feature string) func(context.Context, openai.ChatCompletionRequest) (string, error) {
	return func(ctx context.Context, req openai.ChatCompletionRequest) (string, error) {
		started := time.Now()
		req.Model = ""
		response, err := model.New(model.ForFeature(config.Current().AI, feature)).Text(ctx, req)
		traceStage(ctx, "model_text", started, err)
		return response, err
	}
}
func modelErrorStatus(err error) int   { return model.Status(err) }
func modelErrorReply(err error) string { return model.UserMessage(err) }
