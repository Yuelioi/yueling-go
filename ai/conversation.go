package ai

import (
	"context"
	"errors"

	"github.com/Yuelioi/yueling-go/config"
	model "github.com/Yuelioi/yueling-go/services/llm"
	"github.com/Yuelioi/yueling-go/services/logx"
	openai "github.com/sashabaranov/go-openai"
)

// runConversation owns the model/tool protocol. The caller supplies authorized tool execution.
func (e *Engine) runConversation(ctx context.Context, session *Session, userInput, systemPrompt string, llmTools []openai.Tool, execute func(openai.ToolCall) ToolResult) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, turnTimeout)
	defer cancel()
	settings := config.Current().AI
	client := e.modelClientFor(settings)
	client = client.StartTurn(ctx)
	defer client.Close()
	replyInput := userInput
	historyStart := session.recentHistoryStart()
	turnStart := len(session.Messages)
	session.pushUser(userInput)
	defer session.finishProtocolTurn(turnStart, userInput)
	executed := false
	possibleSideEffects := false
	fail := func(reply string) (string, error) {
		if !executed {
			session.Messages = session.Messages[:turnStart]
		} else if possibleSideEffects {
			// Preserve completed actions when synthesis fails, so follow-up requests see what ran.
			reply += " 部分操作可能已执行，请先核对结果，避免重复提交。"
		}
		return reply, errors.New("conversation did not complete")
	}
	ids := map[string]bool{}
	stepBudget := maxSteps
	for step := 0; step < stepBudget; step++ {
		session.StepCount++
		msgs := []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: systemPrompt}}
		msgs = append(msgs, session.Messages[historyStart:]...)
		req := openai.ChatCompletionRequest{Messages: msgs, Tools: llmTools, MaxTokens: settings.MaxTokens, Temperature: 0.7}
		if step == stepBudget-1 && len(llmTools) > 0 {
			req.ToolChoice = "none"
		}
		resp, err := completeChat(ctx, client, req)
		if err != nil {
			// Avoid logging provider bodies, which can echo conversation content or credentials.
			logx.Errorf("[ai] model request failed group=%d user=%d step=%d status=%d error=%v", session.GroupID, session.UserID, step, modelErrorStatus(err), err)
			return fail(modelErrorReply(err))
		}
		var validationErr error
		if len(resp.Choices) == 0 {
			validationErr = &model.Error{Kind: model.InvalidResponse, Detail: "empty_choices"}
		} else {
			validationErr = model.ValidateChoice(resp.Choices[0])
			if validationErr == nil && len(resp.Choices[0].Message.ToolCalls) == 0 && leaksToolNarration(resp.Choices[0].Message.Content, replyInput) {
				validationErr = &model.Error{Kind: model.InvalidResponse, Detail: "tool_narration"}
			}
		}
		if validationErr != nil {
			logx.Warnf("[ai] invalid model response group=%d user=%d step=%d error=%v", session.GroupID, session.UserID, step, validationErr)
			return fail(model.UserMessage(validationErr))
		}
		msg := resp.Choices[0].Message
		msg.Role = openai.ChatMessageRoleAssistant
		if len(msg.ToolCalls) == 0 {
			// Reasoning belongs to this in-flight turn and is removed by finishProtocolTurn.
			session.pushAssistant(msg)
			return msg.Content, nil
		}
		if step == stepBudget-1 {
			return fail("本次请求步骤较多，暂时未能完成，请缩小范围后重试。")
		}
		advertised := make(map[string]bool, len(llmTools))
		for _, tool := range llmTools {
			if tool.Function != nil {
				advertised[tool.Function.Name] = true
			}
		}
		for _, call := range msg.ToolCalls {
			if !advertised[call.Function.Name] || call.ID == "" || ids[call.ID] {
				return fail(model.UserMessage(&model.Error{Kind: model.InvalidResponse, Detail: "unadvertised_or_duplicate_tool"}))
			}
			ids[call.ID] = true
		}
		session.pushAssistant(msg)
		for _, tc := range msg.ToolCalls {
			result := rejectedTool("请求已取消，未执行该操作")
			if ctx.Err() == nil {
				result = execute(tc)
				executed = executed || result.Invoked
				possibleSideEffects = possibleSideEffects || result.PossibleSideEffects
			}
			session.pushToolResult(tc.ID, result.modelContent())
		}
	}
	return fail("本次请求步骤较多，暂时未能完成，请缩小范围后重试。")
}

func limitToolResult(text string) string {
	const limit = 12000
	runes := []rune(text)
	if len(runes) > limit {
		return string(runes[:limit]) + "\n[资料已截断，请按已提供内容回答并说明范围]"
	}
	if text == "" {
		return "操作未返回内容，不能据此声称成功。"
	}
	return text
}
