package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/logx"
	openai "github.com/sashabaranov/go-openai"
)

// userPermLevel maps a sender role + userID to a PermLevel.
func userPermLevel(role string, userID int64) PermLevel {
	for _, su := range config.C.Bot.SuperUsers {
		if userID == su {
			return PermSuperUser
		}
	}
	switch role {
	case "owner":
		return PermOwner
	case "admin":
		return PermAdmin
	}
	return PermMember
}

// filterByPerm returns only tools the user is allowed to call.
func filterByPerm(tools []*ToolMeta, perm PermLevel) []*ToolMeta {
	out := tools[:0:0]
	for _, t := range tools {
		if t.Permission <= perm {
			out = append(out, t)
		}
	}
	return out
}

func buildSystemPrompt(userID, groupID int64) string {
	base := fmt.Sprintf(
		"你是%s，一个活泼可爱的QQ群助手。请用简洁自然的中文回复，不要过度解释。"+
			"有合适的工具时优先调用工具，不要在没有工具的情况下凭空捏造信息。",
		config.C.Bot.Name,
	)
	return base + UserContext(userID) + GroupContext(groupID)
}

// Dispatch runs the ReAct loop for a group message and returns the reply text.
// It is safe to call from multiple goroutines.
func Dispatch(ctx context.Context, gctx *bot.GroupContext) (string, error) {
	event := gctx.Event
	userID := event.UserID
	groupID := event.GroupID
	text := event.Message.Text()
	role := event.Sender.Role

	// ── Rate limit ──────────────────────────────────────────────────────────
	if !defaultLimiter.Allow(fmt.Sprintf("%d", userID)) {
		return "你发消息太频繁了，稍后再试吧。", nil
	}

	// ── Security guard ──────────────────────────────────────────────────────
	switch Guard(text, role) {
	case GuardBlockInjection:
		return "检测到异常输入，已拒绝处理。", nil
	case GuardBlockPerm:
		return "你没有权限执行该操作。", nil
	}

	// ── Session ─────────────────────────────────────────────────────────────
	session := Sessions.Get(groupID, userID)
	session.resetTurn()
	session.LastInput = text

	// ── Tool set ─────────────────────────────────────────────────────────────
	perm := userPermLevel(role, userID)
	allowed := filterByPerm(AllTools(), perm)

	routed := Route(text, allowed)
	toolSet := allowed
	if len(routed) > 0 {
		toolSet = make([]*ToolMeta, len(routed))
		for i, r := range routed {
			toolSet[i] = r.Tool
		}
	}

	llmTools := make([]openai.Tool, len(toolSet))
	for i, t := range toolSet {
		llmTools[i] = t.schema()
	}

	// ── Conversation ─────────────────────────────────────────────────────────
	session.pushUser(text)

	// ── ReAct loop ───────────────────────────────────────────────────────────
	for step := 0; step < maxSteps; step++ {
		session.StepCount++

		msgs := make([]openai.ChatCompletionMessage, 0, len(session.Messages)+1)
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: buildSystemPrompt(userID, groupID),
		})
		msgs = append(msgs, session.Messages...)

		req := openai.ChatCompletionRequest{
			Model:       config.C.AI.Model,
			Messages:    msgs,
			Tools:       llmTools,
			MaxTokens:   512,
			Temperature: 0.7,
		}

		resp, err := llm().CreateChatCompletion(ctx, req)
		if err != nil {
			logx.Errorf("[ai] LLM error step=%d user=%d: %v", step, userID, err)
			return "AI 暂时不可用，请稍后再试。", nil
		}
		if len(resp.Choices) == 0 {
			break
		}

		msg := resp.Choices[0].Message
		session.pushAssistant(msg)

		// No tool calls → LLM gave a direct answer.
		if len(msg.ToolCalls) == 0 {
			if msg.Content != "" {
				go SmartWriteSemantic(userID, text, msg.Content)
				return msg.Content, nil
			}
			break
		}

		// ── Execute tool calls ────────────────────────────────────────────────
		for _, tc := range msg.ToolCalls {
			result := executeTool(ctx, gctx.BotAPI, event, session, perm, tc)
			session.pushToolResult(tc.ID, result)
		}
	}

	return "抱歉，我现在无法处理这个请求。", nil
}

// executeTool runs one tool call and returns a result string for the LLM.
func executeTool(
	ctx context.Context,
	api *bot.BotAPI,
	event *bot.GroupMessageEvent,
	session *Session,
	perm PermLevel,
	tc openai.ToolCall,
) string {
	meta, ok := GetTool(tc.Function.Name)
	if !ok {
		return fmt.Sprintf("工具 %q 不存在", tc.Function.Name)
	}

	if meta.Permission > perm {
		return "权限不足，无法调用该工具"
	}

	if !session.canCall(meta.Name) {
		return "该工具本轮调用次数已达上限"
	}

	// High-risk tools require confirmation.
	if meta.ConfirmRequired {
		var params map[string]any
		json.Unmarshal([]byte(tc.Function.Arguments), &params)
		actionID, code := Confirms.Store(event.UserID, event.GroupID, meta.Name, params)
		return fmt.Sprintf(
			"[需要确认] 请回复确认码 %s 来执行操作（30秒内有效，操作ID: %s）",
			code, actionID,
		)
	}

	var params map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
		return fmt.Sprintf("参数解析失败: %v", err)
	}

	session.UsedTools[meta.Name]++

	logx.Infof("[tool] → %s %v", meta.Name, params)
	tctx := newToolCtx(api, event, session, params)
	result, err := meta.Handler(tctx)
	if err != nil {
		logx.Errorf("[tool] ✗ %s: %v", meta.Name, err)
		return fmt.Sprintf("工具执行失败: %v", err)
	}
	preview := result
	if len(preview) > 80 {
		preview = preview[:80] + "..."
	}
	logx.Infof("[tool] ✓ %s → %q", meta.Name, preview)
	return result
}
