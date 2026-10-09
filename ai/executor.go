package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services/logx"
	openai "github.com/sashabaranov/go-openai"
)

type ToolStatus string

const (
	ToolSucceeded    ToolStatus = "succeeded"
	ToolEmpty        ToolStatus = "empty"
	ToolReported     ToolStatus = "reported"
	ToolRejected     ToolStatus = "rejected"
	ToolFailed       ToolStatus = "failed"
	ToolConfirmation ToolStatus = "confirmation_required"
)

// ToolResult separates execution facts from legacy handlers' human-readable reports.
// Reported means the handler returned; it does not assert business success.
type ToolResult struct {
	Status              ToolStatus `json:"status"`
	Content             string     `json:"content"`
	PossibleSideEffects bool       `json:"possible_side_effects"`
	Invoked             bool       `json:"-"`
}

func (r ToolResult) modelContent() string {
	r.Content = limitToolResult(r.Content)
	data, _ := json.Marshal(r)
	return string(data)
}

func rejectedTool(message string) ToolResult {
	return ToolResult{Status: ToolRejected, Content: message}
}

// executeTool owns authorization, argument validation, confirmation and per-turn deduplication.
func (e *Engine) executeTool(ctx context.Context, api *bot.BotAPI, event *bot.GroupMessageEvent, session *Session, perm PermLevel, tc openai.ToolCall, exposed map[string]bool) ToolResult {
	if ctx.Err() != nil || session.invalidated() {
		return rejectedTool("请求已取消，未执行该操作")
	}
	meta, ok := e.Registry.GetTool(tc.Function.Name)
	if !ok {
		return rejectedTool("请求的工具不存在")
	}
	if meta.Permission > perm {
		return rejectedTool("权限不足，无法调用该工具")
	}
	if exposed != nil && !exposed[meta.Name] {
		return rejectedTool("该工具未被本轮请求匹配，拒绝调用")
	}
	if !toolEnabledInGroup(e.Groups, meta, event.GroupID) {
		return rejectedTool("该功能在本群已禁用")
	}
	params, err := parseToolArguments(meta, tc.Function.Arguments)
	if err != nil {
		return rejectedTool(err.Error())
	}
	canonical, _ := json.Marshal(params)
	executionKey := meta.Name + ":params:" + string(canonical)
	if meta.ActionKey != nil {
		if key := meta.ActionKey(params); key != "" {
			executionKey = meta.Name + ":action:" + key
		}
	}
	if previous, ok := session.ExecutedTools[executionKey]; ok {
		return previous
	}
	if !session.canCall(meta.Name) {
		return rejectedTool("该工具本轮调用次数已达上限")
	}
	if meta.ConfirmRequired {
		session.UsedTools[meta.Name] = maxToolUse
		actionID, code := e.Confirms.StoreWithContext(event.UserID, event.GroupID, meta.Name, params, event, session.ToolState, session)
		return ToolResult{Status: ToolConfirmation, Content: fmt.Sprintf(
			"[需要确认] 准备执行「%s」。30秒内回复「%s 确认 %s %s」继续。", meta.Description, configuredBotName(), code, actionID)}
	}
	if meta.Handler == nil && meta.ResultHandler == nil {
		return ToolResult{Status: ToolFailed, Content: "该功能暂时无法执行。"}
	}
	session.UsedTools[meta.Name]++
	tctx := newToolCtx(api, event, session, perm, params)
	tctx.ctx = ctx
	started := time.Now()
	result, _ := invokeTool(meta, tctx)
	if session.ExecutedTools == nil {
		session.ExecutedTools = map[string]ToolResult{}
	}
	session.ExecutedTools[executionKey] = result
	// Legacy tools may return failure text with nil error; never label that success.
	logx.Infof("[tool] end name=%s group=%d user=%d message=%d status=%s possible_effects=%t duration_ms=%d result_bytes=%d", meta.Name, event.GroupID, event.UserID, event.MessageID, result.Status, result.PossibleSideEffects, time.Since(started).Milliseconds(), len(result.Content))
	return result
}

func invokeTool(meta *ToolMeta, ctx *ToolContext) (result ToolResult, err error) {
	defer func() {
		if recover() != nil {
			err = fmt.Errorf("tool handler panicked: %s", meta.Name)
		}
		if err != nil {
			result = ToolResult{Status: ToolFailed, Invoked: result.Invoked, PossibleSideEffects: result.Invoked && !meta.ReadOnly,
				Content: "操作未能确认完成，不能声称成功或自动重试。"}
			if meta.ReadOnly {
				result.Content = "资料查询失败，不能把失败当成没有记录或编造结果。"
			}
		}
	}()
	if ctx.Context().Err() != nil || (ctx.session != nil && ctx.session.invalidated()) {
		return rejectedTool("请求已取消，未执行该操作"), nil
	}
	if meta.ResultHandler != nil {
		result.Invoked = true
		outcome := meta.ResultHandler(ctx)
		switch outcome.Status {
		case ToolSucceeded, ToolEmpty, ToolRejected, ToolFailed:
		default:
			return result, fmt.Errorf("invalid tool outcome: %s", meta.Name)
		}
		outcome.Invoked = true
		return outcome, nil
	}
	if meta.Handler == nil {
		return ToolResult{Status: ToolFailed, Content: "该功能暂时无法执行。"}, nil
	}
	result = ToolResult{Status: ToolReported, Invoked: true, PossibleSideEffects: !meta.ReadOnly}
	result.Content, err = meta.Handler(ctx)
	return result, err
}
