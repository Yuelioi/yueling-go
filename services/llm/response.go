package llm

import (
	"encoding/json"
	"regexp"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

var protocolMarkup = regexp.MustCompile(`(?i)<\s*/?\s*(tool[_ ]?calls?|function[_ ]?calls?|think|analysis)\b|<\|[^>]*(?:tool|function|analysis)[^>]*\|>|"tool_calls"\s*:`)

// Reject protocol artifacts, not ordinary explanations mentioning tool calling.
func ValidateChoice(choice openai.ChatCompletionChoice) error {
	msg := choice.Message
	if choice.FinishReason == openai.FinishReasonLength {
		return &Error{Kind: InvalidResponse, Detail: "output_limit"}
	}
	if msg.FunctionCall != nil {
		return &Error{Kind: InvalidResponse, Detail: "legacy_function_call"}
	}
	if len(msg.ToolCalls) == 0 {
		if strings.TrimSpace(msg.Content) == "" || protocolMarkup.MatchString(msg.Content) {
			return &Error{Kind: InvalidResponse, Detail: "invalid_protocol"}
		}
		return nil
	}
	if len(msg.ToolCalls) > 8 {
		return &Error{Kind: InvalidResponse, Detail: "invalid_protocol"}
	}
	seen := map[string]bool{}
	for _, call := range msg.ToolCalls {
		var params map[string]any
		if call.ID == "" || seen[call.ID] || call.Type != openai.ToolTypeFunction || call.Function.Name == "" || json.Unmarshal([]byte(call.Function.Arguments), &params) != nil || params == nil {
			return &Error{Kind: InvalidResponse, Detail: "invalid_protocol"}
		}
		seen[call.ID] = true
	}
	return nil
}
