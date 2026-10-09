package tools

import "github.com/Yuelioi/yueling-go/ai"

func outcome(status ai.ToolStatus, content string, possibleSideEffects bool) ai.ToolResult {
	return ai.ToolResult{Status: status, Content: content, PossibleSideEffects: possibleSideEffects}
}
