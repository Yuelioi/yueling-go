package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

var cmdToTarget = map[string]string{
	"中译英": "English",
	"中译日": "日本語",
	"英译中": "中文",
	"英译日": "日本語",
	"日译英": "English",
	"日译中": "中文",
}

func RegisterTranslate(b *bot.Bot) {
	b.OnCommand("翻译", "中译英", "中译日", "英译中", "英译日", "日译英", "日译中").
		Handle(func(ctx *bot.CommandContext) error {
			text := strings.TrimSpace(strings.Join(ctx.Args, " "))
			if text == "" {
				return ctx.Reply("请输入需要翻译的内容")
			}
			if ok, hint := ai.AllowAICall(ctx.UserID(), ctx.GroupID()); !ok {
				return ctx.Reply(hint)
			}
			ctx.React(bot.EmojiProcessing)

			target, ok := cmdToTarget[ctx.Cmd]
			if !ok {
				target = "中文"
			}

			result, err := translateText(text, target)
			if err != nil {
				return ctx.Reply("翻译失败: " + err.Error())
			}
			return ctx.Reply(result)
		})
}

func translateText(text, target string) (string, error) {
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
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("无响应")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}
