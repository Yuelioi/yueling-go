package ai

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/services/chatsummary"
	openai "github.com/sashabaranov/go-openai"
)

const (
	maxDigestMessages   = 100
	maxDigestInputRunes = 16000
	maxDigestLineRunes  = 320
)

type GroupHistorySource interface {
	GetGroupMsgHistory(groupID int64, messageID int32, count int) ([]bot.HistoryMessage, error)
}

type GroupDigestSender interface {
	GroupHistorySource
	SendGroupText(groupID int64, text string) error
}

func digestHistoryText(messages []bot.HistoryMessage) string {
	var lines []string
	total := 0
	for _, message := range messages {
		name := message.Sender.Card
		if name == "" {
			name = message.Sender.Nickname
		}
		if name == "" {
			name = "一位群友"
		}
		var parts []string
		for _, segment := range message.Message {
			switch segment.Type {
			case "text":
				if value := strings.TrimSpace(segment.Data.Text); value != "" {
					parts = append(parts, value)
				}
			case "image":
				parts = append(parts, "[图片]")
			case "at":
				parts = append(parts, "[提及群友]")
			}
		}
		if len(parts) == 0 {
			continue
		}
		line := name + ": " + strings.Join(parts, " ")
		if utf8.RuneCountInString(line) > maxDigestLineRunes {
			line = string([]rune(line)[:maxDigestLineRunes]) + "…"
		}
		lineRunes := utf8.RuneCountInString(line) + 1
		if total+lineRunes > maxDigestInputRunes {
			break
		}
		lines = append(lines, line)
		total += lineRunes
	}
	return strings.Join(lines, "\n")
}

// GenerateGroupDigest summarizes the latest group history for scheduled delivery.
func GenerateGroupDigest(ctx context.Context, source GroupHistorySource, groupID int64, count int) (string, error) {
	count = ResolveCount(count, config.Current().AI.Context.Summary, 10, maxDigestMessages)
	messages, err := source.GetGroupMsgHistory(groupID, 0, count)
	if err != nil {
		return "", err
	}
	history := digestHistoryText(messages)
	if history == "" {
		return "", nil
	}
	text, err := completeText(ctx, openai.ChatCompletionRequest{
		Model: config.Current().AI.Model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: chatsummary.PresentationInstruction("summary") + "这是一份定时群聊回顾，只包含最近这段聊天，不代表全天。直接回顾有内容的话题，不设空栏目；有实际约定才顺带提及。通常不超过500字。",
			},
			{Role: openai.ChatMessageRoleUser, Content: history},
		},
		MaxTokens:   700,
		Temperature: 0.2,
	})
	if err != nil {
		return "", err
	}
	return text, nil

}

// GenerateAndSendGroupDigest creates and sends one digest immediately.
func GenerateAndSendGroupDigest(ctx context.Context, sender GroupDigestSender, groupID int64, count int) (string, error) {
	if !config.PluginEnabled(catalog.PluginDailyDigest) {
		return "", fmt.Errorf("群聊日报插件已全局关闭")
	}
	summary, err := GenerateGroupDigest(ctx, sender, groupID, count)
	if err != nil || summary == "" {
		return summary, err
	}
	if err := sender.SendGroupText(groupID, "🌙 群聊日报\n\n"+summary); err != nil {
		return "", err
	}
	return summary, nil
}
