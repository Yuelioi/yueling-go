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
func executeTool(ctx context.Context, api *bot.BotAPI, event *bot.GroupMessageEvent, session *Session, perm PermLevel, tc openai.ToolCall, exposed map[string]bool) ToolResult {
	if ctx.Err() != nil || session.invalidated() {
		return rejectedTool("请求已取消，未执行该操作")
	}
	meta, ok := GetTool(tc.Function.Name)
	if !ok {
		return rejectedTool("请求的工具不存在")
	}
	if meta.Permission > perm {
		return rejectedTool("权限不足，无法调用该工具")
	}
	if exposed != nil && !exposed[meta.Name] {
		return rejectedTool("该工具未被本轮请求匹配，拒绝调用")
	}
	if !toolEnabledInGroup(meta, event.GroupID) {
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
		actionID, code := Confirms.StoreWithContext(event.UserID, event.GroupID, meta.Name, params, event, session.ToolState)
		return ToolResult{Status: ToolConfirmation, Content: fmt.Sprintf(
			"[需要确认] 准备执行「%s」。30秒内回复「%s 确认 %s %s」继续。", meta.Description, configuredBotName(), code, actionID)}
	}
	if meta.Handler == nil {
		return ToolResult{Status: ToolFailed, Content: "该功能暂时无法执行。"}
	}
	session.UsedTools[meta.Name]++
	tctx := newToolCtx(api, event, session, perm, params)
	tctx.ctx = ctx
	started := time.Now()
	text, err := invokeTool(meta, tctx)
	result := ToolResult{Status: ToolReported, Content: text, Invoked: true, PossibleSideEffects: !meta.ReadOnly}
	if err != nil {
		result.Status = ToolFailed
		result.Content = "操作未能确认完成，不能声称成功或自动重试。"
		if meta.ReadOnly {
			result.Content = "资料查询失败，不能把失败当成没有记录或编造结果。"
		}
	}
	if session.ExecutedTools == nil {
		session.ExecutedTools = map[string]ToolResult{}
	}
	session.ExecutedTools[executionKey] = result
	// Legacy tools may return failure text with nil error; never label that success.
	logx.Infof("[tool] end name=%s group=%d user=%d message=%d status=%s possible_effects=%t duration_ms=%d result_bytes=%d", meta.Name, event.GroupID, event.UserID, event.MessageID, result.Status, result.PossibleSideEffects, time.Since(started).Milliseconds(), len(result.Content))
	return result
}

func invokeTool(meta *ToolMeta, ctx *ToolContext) (result string, err error) {
	defer func() {
		if recover() != nil {
			result = ""
			err = fmt.Errorf("tool handler panicked: %s", meta.Name)
		}
	}()
	if ctx.Context().Err() != nil {
		return "", ctx.Context().Err()
	}
	return meta.Handler(ctx)
}
