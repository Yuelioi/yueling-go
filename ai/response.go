package ai

import (
	"regexp"
	"strings"
)

var toolProtocolTerm = regexp.MustCompile(`(?i)\b(?:tool|function)[ _-]?calls?\b`)

func leaksToolNarration(content, userInput string) bool {
	// Preserve deliberate questions about the protocol; ordinary requests should see results.
	if toolProtocolTerm.MatchString(userInput) || strings.Contains(userInput, "工具调用") || strings.Contains(userInput, "函数调用") {
		return false
	}
	return toolProtocolTerm.MatchString(content)
}
