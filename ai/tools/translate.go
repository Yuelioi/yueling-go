package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

func init() {
	ai.Register(ai.ToolMeta{
		Name:        "translate",
		Description: "将文本翻译成目标语言，支持中英日韩等主流语言互译",
		Tags:        []string{"翻译", "translate"},
		Triggers:    []string{"翻译", "translate", "怎么说", "英文", "中文", "日文"},
		Slots:       []string{"text", "target_lang"},
		Params: []ai.Param{
			{Name: "text", Type: "string", Description: "要翻译的文本", Required: true},
			{Name: "target_lang", Type: "string", Description: "目标语言，如【中文】【English】【日本語】", Required: true},
		},
		Handler: translateHandler,
	})
}

func translateHandler(ctx *ai.ToolContext) (string, error) {
	text := ctx.String("text")
	target := ctx.String("target_lang")
	if text == "" || target == "" {
		return "请提供要翻译的文本和目标语言", nil
	}

	cfg := config.C.AI
	client := ai.NewClient(cfg.DeepSeekKey, cfg.BaseURL)

	prompt := fmt.Sprintf("请将以下文本翻译成%s，只返回翻译结果，不要解释：\n\n%s", target, text)
	resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: cfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens:   1024,
		Temperature: 0.3,
	})
	if err != nil {
		return "翻译失败: " + err.Error(), nil
	}
	if len(resp.Choices) == 0 {
		return "翻译失败：无响应", nil
	}

	result := strings.TrimSpace(resp.Choices[0].Message.Content)
	return fmt.Sprintf("%s → %s：\n%s", detectLang(text), target, result), nil
}

func detectLang(text string) string {
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			return "中文"
		}
		if r >= 0x3040 && r <= 0x30FF {
			return "日文"
		}
	}
	return "原文"
}
