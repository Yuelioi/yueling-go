package ai

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services/logx"
	openai "github.com/sashabaranov/go-openai"
)

var confirmationPattern = regexp.MustCompile(`^确认\s+(\d{4})\s+([0-9a-fA-F]{8})$`)

func handleConfirmation(parent context.Context, gctx *bot.GroupContext, deliver func(context.Context, string) error) (bool, error) {
	command := normalizeControlText(gctx.Text())
	match := confirmationPattern.FindStringSubmatch(command)
	if len(match) != 3 {
		return false, nil
	}
	// Do not consume a one-time confirmation for work already canceled by its caller.
	if err := parent.Err(); err != nil {
		return true, err
	}
	// Capture the original lifetime before verification or authorization can block.
	// A reset during those checks must not move this action into a fresh session.
	session := Sessions.Get(gctx.GroupID(), gctx.UserID())
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	ctx, stopTurn := session.turnContext(ctx)
	defer stopTurn()
	if err := ctx.Err(); err != nil {
		if session.invalidated() {
			return true, nil
		}
		return true, err
	}
	notice := func(text string) (bool, error) {
		return true, sendTurnReply(parent, session, deliver, text)
	}
	code := match[1]
	actionID := strings.ToLower(match[2])
	pending, ok := Confirms.Verify(actionID, code, gctx.UserID(), gctx.GroupID())
	if !ok {
		return notice("确认码无效、已过期，或不属于当前群聊。")
	}
	meta, ok := GetTool(pending.ToolName)
	if !ok {
		return notice("待确认的操作已不可用，请重新发起。")
	}
	permission := userPermLevel(gctx.Role(), gctx.UserID())
	if meta.Permission > permission {
		return notice("你当前没有权限执行该操作。")
	}
	if !toolEnabledInGroup(meta, gctx.GroupID()) {
		return notice("该功能在本群已禁用。")
	}

	if err := session.acquire(ctx); err != nil {
		if session.invalidated() {
			return true, nil
		}
		return true, sendTurnReply(parent, session, deliver, modelErrorReply(err))
	}
	defer session.release()
	toolEvent := gctx.Event
	if pending.event != nil {
		toolEvent = pending.event
	}
	for key, value := range pending.toolState {
		session.ToolState[key] = value
	}
	toolContext := newToolCtx(gctx.BotAPI, toolEvent, session, permission, pending.Params)
	toolContext.ctx = ctx
	started := time.Now()
	result, err := invokeTool(meta, toolContext)
	traceErr := err
	if traceErr == nil {
		traceErr = ctx.Err()
	}
	traceStage(ctx, "confirmed_tool", started, traceErr)
	if session.invalidated() {
		return true, nil
	}
	if parent.Err() != nil {
		return true, parent.Err()
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		logx.Errorf("[tool] confirmed action failed name=%s error_type=%T", meta.Name, err)
		return true, sendTurnReply(parent, session, deliver, "操作未能确认完成，请先核对结果，避免重复提交。")
	}
	session.pushUser(command)
	session.pushAssistant(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: result, ReasoningContent: "Application-confirmed action result."})
	return true, sendTurnReply(parent, session, deliver, result)
}
