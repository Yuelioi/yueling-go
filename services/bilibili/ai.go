package bilibili

import (
	"context"
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

func AIReply(ctx context.Context, text string) (string, error) {
	return AIReplyWithPrompt(ctx, text, config.Current().Bilibili.AIPrompt)
}

func AIReplyWithPrompt(ctx context.Context, text, systemPrompt string) (string, error) {
	cfg := config.Current().AI
	clientConfig := openai.DefaultConfig(cfg.DeepSeekKey)
	clientConfig.BaseURL = cfg.BaseURL
	prompt := systemPrompt + "\n回复不超过 500 个汉字。仅根据当前私信回复；不调用工具，不声称已执行任何外部操作。"
	response, err := openai.NewClientWithConfig(clientConfig).CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:     cfg.Model,
		Messages:  []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: prompt}, {Role: openai.ChatMessageRoleUser, Content: truncate(text, 4000)}},
		MaxTokens: min(max(cfg.MaxTokens, 1), 1024),
	})
	if err != nil {
		return "", fmt.Errorf("AI 生成失败")
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("AI 返回空结果")
	}
	reply := strings.TrimSpace(response.Choices[0].Message.Content)
	if reply == "" {
		return "", fmt.Errorf("AI 返回空回复")
	}
	return truncate(reply, 500), nil
}
