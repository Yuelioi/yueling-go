package ai

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services/logx"
	openai "github.com/sashabaranov/go-openai"
)

var confirmationPattern = regexp.MustCompile(`^确认\s+(\d{4})\s+([0-9a-fA-F]{8})$`)

func handleConfirmation(gctx *bot.GroupContext) (string, bool) {
	command := normalizeControlText(gctx.Text())
	match := confirmationPattern.FindStringSubmatch(command)
	if len(match) != 3 {
		return "", false
	}
	code := match[1]
	actionID := strings.ToLower(match[2])
	pending, ok := Confirms.Verify(actionID, code, gctx.UserID(), gctx.GroupID())
	if !ok {
		return "确认码无效、已过期，或不属于当前群聊。", true
	}
	meta, ok := GetTool(pending.ToolName)
	if !ok {
		return "待确认的操作已不可用，请重新发起。", true
	}
	permission := userPermLevel(gctx.Role(), gctx.UserID())
	if meta.Permission > permission {
		return "你当前没有权限执行该操作。", true
	}
	if !toolEnabledInGroup(meta, gctx.GroupID()) {
		return "该功能在本群已禁用。", true
	}

	session := Sessions.Get(gctx.GroupID(), gctx.UserID())
	session.mu.Lock()
	defer session.mu.Unlock()
	toolEvent := gctx.Event
	if pending.event != nil {
		toolEvent = pending.event
	}
	for key, value := range pending.toolState {
		session.ToolState[key] = value
	}
	toolContext := newToolCtx(gctx.BotAPI, toolEvent, session, permission, pending.Params)
	result, err := meta.Handler(toolContext)
	if err != nil {
		logx.Errorf("[tool] confirmed action %s failed: %v", meta.Name, err)
		return fmt.Sprintf("操作执行失败: %v", err), true
	}
	session.pushUser(command)
	session.pushAssistant(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: result})
	return result, true
}
