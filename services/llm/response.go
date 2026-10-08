package llm

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"

	openai "github.com/sashabaranov/go-openai"
)

var protocolMarkup = regexp.MustCompile(`(?i)<\s*/?\s*(tool[_ ]?calls?|function[_ ]?calls?|think|analysis)\b|<\|[^>]*(?:tool|function|analysis)[^>]*\|>|"tool_calls"\s*:`)

// Reject protocol artifacts, not ordinary explanations mentioning tool calling.
func ValidateChoice(choice openai.ChatCompletionChoice) error {
	msg := choice.Message
	if choice.FinishReason == openai.FinishReasonLength {
		return &Error{Kind: InvalidResponse, Detail: "output_limit"}
	}
	if choice.FinishReason == openai.FinishReasonContentFilter {
		return &Error{Kind: InvalidResponse, Detail: "content_filter"}
	}
	if msg.FunctionCall != nil {
		return &Error{Kind: InvalidResponse, Detail: "legacy_function_call"}
	}
	if len(msg.ToolCalls) == 0 {
		if strings.TrimSpace(msg.Content) == "" {
			detail := "empty_content"
			if strings.TrimSpace(msg.ReasoningContent) != "" {
				detail = "reasoning_only"
			}
			return &Error{Kind: InvalidResponse, Detail: detail}
		}
		if protocolMarkup.MatchString(msg.Content) {
			return &Error{Kind: InvalidResponse, Detail: "protocol_markup"}
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

// ResponseMetadata contains only bounded categories and counts, never model text.
type ResponseMetadata struct {
	FinishReason     string
	MaxTokens        int
	CompletionTokens int
	ContentChars     int
	ReasoningChars   int
	Attempts         int
}

func responseMetadata(response openai.ChatCompletionResponse, req openai.ChatCompletionRequest, attempts int) ResponseMetadata {
	meta := ResponseMetadata{FinishReason: "none", MaxTokens: req.MaxTokens, CompletionTokens: response.Usage.CompletionTokens, Attempts: attempts}
	if len(response.Choices) == 0 {
		return meta
	}
	choice := response.Choices[0]
	switch choice.FinishReason {
	case openai.FinishReasonStop, openai.FinishReasonLength, openai.FinishReasonToolCalls, openai.FinishReasonContentFilter, openai.FinishReasonFunctionCall:
		meta.FinishReason = string(choice.FinishReason)
	default:
		meta.FinishReason = "other"
	}
	meta.ContentChars = utf8.RuneCountInString(choice.Message.Content)
	meta.ReasoningChars = utf8.RuneCountInString(choice.Message.ReasoningContent)
	return meta
}
