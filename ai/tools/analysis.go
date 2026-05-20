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
	registerSummarizeChat()
}

func registerSummarizeChat() {
	ai.Register(ai.ToolMeta{
		Name:        "summarize_chat",
		Description: "总结最近的群聊内容，帮没在的人快速了解话题",
		Tags:        []string{"上下文", "群聊"},
		Triggers:    []string{"总结", "摘要"},
		Patterns:    []string{`(聊了|在聊)(什么|啥)`, `总结.{0,4}(聊|说)`},
		Slots:       []string{"聊天总结", "群聊摘要", "话题总结"},
		Params: []ai.Param{
			{Name: "count", Type: "integer", Description: "获取消息条数（10-50，默认30）", Required: false},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			count := int(ctx.Int("count"))
			if count < 10 {
				count = 30
			}
			if count > 50 {
				count = 50
			}

			messages, err := ctx.BotAPI().GetGroupMsgHistory(ctx.GroupID(), ctx.MessageID(), count)
			if err != nil {
				return "获取聊天记录失败", nil
			}
			if len(messages) == 0 {
				return "暂无聊天记录", nil
			}

			var lines []string
			for _, msg := range messages {
				nick := msg.Sender.Card
				if nick == "" {
					nick = msg.Sender.Nickname
				}
				if nick == "" {
					nick = fmt.Sprintf("%d", msg.UserID)
				}
				var parts []string
				for _, seg := range msg.Message {
					switch seg.Type {
					case "text":
						if t := strings.TrimSpace(seg.Data.Text); t != "" {
							parts = append(parts, t)
						}
					case "image":
						parts = append(parts, "[图片]")
					case "at":
						parts = append(parts, fmt.Sprintf("[@%s]", seg.Data.QQ))
					}
				}
				if len(parts) > 0 {
					lines = append(lines, nick+": "+strings.Join(parts, " "))
				}
			}
			if len(lines) == 0 {
				return "最近没有文字消息", nil
			}

			chatText := strings.Join(lines, "\n")
			cfg := config.C.AI
			client := ai.NewClient(cfg.DeepSeekKey, cfg.BaseURL)
			resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
				Model: cfg.Model,
				Messages: []openai.ChatCompletionMessage{
					{Role: openai.ChatMessageRoleSystem, Content: "用3-5句话总结以下群聊的主要话题和要点，简洁直接。"},
					{Role: openai.ChatMessageRoleUser, Content: chatText},
				},
				MaxTokens:   200,
				Temperature: 0.3,
			})
			if err != nil || len(resp.Choices) == 0 {
				return "总结失败", nil
			}
			return strings.TrimSpace(resp.Choices[0].Message.Content), nil
		},
	})
}
