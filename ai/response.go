package ai

import (
	"regexp"
	"strings"

	"github.com/Yuelioi/yueling-go/services/llm"
	"github.com/Yuelioi/yueling-go/services/logx"
)

func userFacingReply(reply string) string {
	if llm.ContainsProtocolMarkup(reply) {
		logx.Warnf("[ai] blocked protocol markup at delivery boundary")
		return "本次回复格式异常，请先核对操作结果，避免重复提交。"
	}
	return reply
}

var toolProtocolTerm = regexp.MustCompile(`(?i)\b(?:tool|function)[ _-]?calls?\b`)

func leaksToolNarration(content, userInput string) bool {
	// Preserve deliberate questions about the protocol; ordinary requests should see results.
	if toolProtocolTerm.MatchString(userInput) || strings.Contains(userInput, "工具调用") || strings.Contains(userInput, "函数调用") {
		return false
	}
	return toolProtocolTerm.MatchString(content)
}
