package tools

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
)

const (
	maxGroupCardRunes    = 30
	maxSpecialTitleRunes = 18
	qqHistoryStateKey    = "qq.history.references"
)

type qqHistoryReference struct {
	messageID int32
	userID    int64
}

type qqActions interface {
	SetEssenceMsg(messageID int32) error
	DeleteEssenceMsg(messageID int32) error
	DeleteMsg(messageID int32) error
	SetGroupBan(groupID, userID int64, seconds int) error
	SetGroupCard(groupID, userID int64, card string) error
	SetGroupSpecialTitle(groupID, userID int64, title string) error
	GroupPoke(groupID, userID int64) error
}

type qqInvocation struct {
	groupID         int64
	callerID        int64
	botID           int64
	permission      ai.PermLevel
	mentionedUsers  []int64
	replyMessageID  int32
	hasReply        bool
	historyMessages map[int32]bool
	historyUsers    map[int64]bool
}

func (t *toolset) registerQQTools() {
	t.registerSetEssenceTool()
	t.registerDeleteEssenceTool()
	t.registerRevokeMessageTool()
	t.registerBanMemberTool()
	t.registerSetGroupCardTool()
	t.registerSetSpecialTitleTool()
	t.registerGroupPokeTool()
}

func invocationFromToolContext(ctx *ai.ToolContext) qqInvocation {
	replyMessageID, hasReply := ctx.ReplyMessageID()
	inv := qqInvocation{
		groupID:         ctx.GroupID(),
		callerID:        ctx.UserID(),
		botID:           ctx.SelfID(),
		permission:      ctx.Permission(),
		mentionedUsers:  ctx.MentionedUserIDs(),
		replyMessageID:  replyMessageID,
		hasReply:        hasReply,
		historyMessages: map[int32]bool{},
		historyUsers:    map[int64]bool{},
	}
	if raw, ok := ctx.GetState(qqHistoryStateKey); ok {
		if refs, ok := raw.([]qqHistoryReference); ok {
			for _, ref := range refs {
				if ref.messageID != 0 {
					inv.historyMessages[ref.messageID] = true
				}
				if ref.userID != 0 {
					inv.historyUsers[ref.userID] = true
				}
			}
		}
	}
	return inv
}

func (t *toolset) registerRevokeMessageTool() {
	t.registry.Register(ai.ToolMeta{
		Name:        "revoke_message",
		Requires:    []string{"get_chat_history"},
		Description: "撤回当前群消息。优先使用真实引用消息；若用户指代“刚才那条消息”，必须先调用get_chat_history，并把其返回的真实消息ID传入，禁止猜测ID；仅管理员可调用",
		Tags:        []string{"QQ动作", "消息"},
		Triggers:    []string{"撤回", "删除消息"},
		Patterns:    []string{`(撤回|删除).{0,8}(这条|那条|消息)`},
		Slots:       []string{"撤回消息", "消息ID", "引用消息"},
		PluginID:    catalog.PluginBan,
		Permission:  ai.PermAdmin,
		Risk:        ai.RiskMedium,
		Params: []ai.Param{
			{Name: "message_id", Type: "integer", Description: "get_chat_history返回的真实消息ID；已引用目标消息时可省略", Required: false},
		},
		ResultHandler: func(ctx *ai.ToolContext) ai.ToolResult {
			messageID, _ := integerToolParam(ctx, "message_id")
			return handleRevokeMessage(invocationFromToolContext(ctx), messageID, ctx.BotAPI())
		},
	})
}

func (t *toolset) registerBanMemberTool() {
	t.registry.Register(ai.ToolMeta{
		Name:        "ban_member",
		Requires:    []string{"get_chat_history"},
		Description: "禁言当前群成员。目标优先取本消息额外@的成员；若用户说“刚才说某话的人”，必须先调用get_chat_history，再传入其返回的真实用户ID，禁止猜测ID；所有群成员可调用",
		Tags:        []string{"QQ动作", "禁言"},
		Triggers:    []string{"禁言", "闭嘴", "ban"},
		Patterns:    []string{`(禁言|闭嘴).{0,10}(刚才|那个人|群友)`},
		Slots:       []string{"禁言成员", "用户ID", "群成员"},
		PluginID:    catalog.PluginBan,
		Permission:  ai.PermMember,
		Params: []ai.Param{
			{Name: "user_id", Type: "integer", Description: "get_chat_history返回的真实用户ID；消息已@目标成员时可省略", Required: false},
			{Name: "duration", Type: "integer", Description: "禁言秒数，默认600；传0表示解除禁言，最大2592000", Required: false},
		},
		ResultHandler: func(ctx *ai.ToolContext) ai.ToolResult {
			userID, _ := integerToolParam(ctx, "user_id")
			duration, durationProvided := integerToolParam(ctx, "duration")
			return handleBanMember(
				invocationFromToolContext(ctx),
				userID,
				duration,
				durationProvided,
				ctx.BotAPI(),
			)
		},
	})
}

func (t *toolset) registerSetEssenceTool() {
	t.registry.Register(ai.ToolMeta{
		Name:        "set_essence_message",
		Description: "把当前消息所引用的群消息设为精华；消息目标直接取自真实引用上下文，不需要提供消息ID",
		Tags:        []string{"QQ动作", "群聊", "精华"},
		Triggers:    []string{"设精", "加精", "设为精华"},
		Patterns:    []string{`(这条|这个).{0,4}(设精|加精|精华)`},
		Slots:       []string{"精华消息", "引用消息"},
		PluginID:    catalog.PluginEssence,
		Permission:  ai.PermAdmin,
		Risk:        ai.RiskMedium,
		Params:      []ai.Param{},
		ResultHandler: func(ctx *ai.ToolContext) ai.ToolResult {
			return handleSetEssence(invocationFromToolContext(ctx), ctx.BotAPI())
		},
	})
}

func (t *toolset) registerDeleteEssenceTool() {
	t.registry.Register(ai.ToolMeta{
		Name:        "delete_essence_message",
		Description: "把当前消息所引用的群消息移出精华，仅群管理员、群主或超级管理员可用",
		Tags:        []string{"QQ动作", "群管", "精华"},
		Triggers:    []string{"取消精华", "移出精华", "取消设精"},
		Patterns:    []string{`(这条|这个).{0,4}(取消精华|移出精华)`},
		Slots:       []string{"删除精华", "引用消息"},
		PluginID:    catalog.PluginEssence,
		Permission:  ai.PermAdmin,
		Risk:        ai.RiskMedium,
		Params:      []ai.Param{},
		ResultHandler: func(ctx *ai.ToolContext) ai.ToolResult {
			return handleDeleteEssence(invocationFromToolContext(ctx), ctx.BotAPI())
		},
	})
}

func (t *toolset) registerSetGroupCardTool() {
	t.registry.Register(ai.ToolMeta{
		Name:        "set_group_card",
		Description: "修改群名片。默认修改发起者本人；若消息还@了其他成员，则以第一个被@成员为目标，普通成员不能修改别人",
		Tags:        []string{"QQ动作", "群名片", "趣味"},
		Triggers:    []string{"群名片", "改名片", "改群昵称"},
		Patterns:    []string{`(把我|给我).{0,5}(改名|叫)`},
		Slots:       []string{"修改群名片", "群昵称", "改名"},
		PluginID:    catalog.PluginRandomRename,
		Permission:  ai.PermMember,
		Risk:        ai.RiskLow,
		Params: []ai.Param{
			{Name: "card", Type: "string", Description: "新的群名片，最多30个字符", Required: true},
		},
		ResultHandler: func(ctx *ai.ToolContext) ai.ToolResult {
			return handleSetGroupCard(invocationFromToolContext(ctx), ctx.String("card"), ctx.BotAPI())
		},
	})
}

func (t *toolset) registerSetSpecialTitleTool() {
	t.registry.Register(ai.ToolMeta{
		Name:        "set_special_title",
		Description: "设置群专属头衔。默认设置发起者本人；若消息还@了其他成员，则以第一个被@成员为目标，普通成员不能修改别人",
		Tags:        []string{"QQ动作", "头衔", "趣味"},
		Triggers:    []string{"头衔", "设置头衔", "专属头衔", "给我头衔"},
		Patterns:    []string{`(给我|给他|给她).{0,5}头衔`},
		Slots:       []string{"头衔", "群头衔", "特殊头衔", "称号"},
		PluginID:    catalog.PluginRandomRename,
		Permission:  ai.PermMember,
		Risk:        ai.RiskLow,
		Params: []ai.Param{
			{Name: "title", Type: "string", Description: "新的专属头衔，最多18个字符", Required: true},
		},
		ResultHandler: func(ctx *ai.ToolContext) ai.ToolResult {
			return handleSetSpecialTitle(invocationFromToolContext(ctx), ctx.String("title"), ctx.BotAPI())
		},
	})
}

func (t *toolset) registerGroupPokeTool() {
	t.registry.Register(ai.ToolMeta{
		Name:        "poke_group_member",
		Description: "在当前群戳一戳成员；目标取消息中除机器人外第一个被@用户，未@其他人时戳发起者本人",
		Tags:        []string{"QQ动作", "戳一戳", "趣味"},
		Triggers:    []string{"戳一下", "戳一戳", "戳戳"},
		Patterns:    []string{`(戳|拍).{0,3}(他|她|我|一下)`},
		Slots:       []string{"戳人", "拍一拍"},
		PluginID:    catalog.PluginPoke,
		Permission:  ai.PermMember,
		Risk:        ai.RiskLow,
		Params:      []ai.Param{},
		ResultHandler: func(ctx *ai.ToolContext) ai.ToolResult {
			return handleGroupPoke(invocationFromToolContext(ctx), ctx.BotAPI())
		},
	})
}

func handleSetEssence(inv qqInvocation, actions qqActions) ai.ToolResult {
	if !inv.hasReply {
		return ai.ToolResult{Status: ai.ToolRejected, Content: "请先回复要设为精华的消息，再 @我 说设精", PossibleSideEffects: false}
	}
	if err := actions.SetEssenceMsg(inv.replyMessageID); err != nil {
		return ai.ToolResult{Status: ai.ToolFailed, Content: "设精未能确认完成，请先核对实际结果，避免重复提交。", PossibleSideEffects: true}
	}
	return ai.ToolResult{Status: ai.ToolSucceeded, Content: "已将引用消息设为精华", PossibleSideEffects: true}
}

func handleDeleteEssence(inv qqInvocation, actions qqActions) ai.ToolResult {
	if !inv.hasReply {
		return ai.ToolResult{Status: ai.ToolRejected, Content: "请先回复要移出精华的消息", PossibleSideEffects: false}
	}
	if err := actions.DeleteEssenceMsg(inv.replyMessageID); err != nil {
		return ai.ToolResult{Status: ai.ToolFailed, Content: "移出精华未能确认完成，请先核对实际结果，避免重复提交。", PossibleSideEffects: true}
	}
	return ai.ToolResult{Status: ai.ToolSucceeded, Content: "已将引用消息移出精华", PossibleSideEffects: true}
}

func handleRevokeMessage(inv qqInvocation, requestedMessageID int64, actions qqActions) ai.ToolResult {
	messageID := requestedMessageID
	fromReply := false
	if messageID == 0 && inv.hasReply {
		messageID = int64(inv.replyMessageID)
		fromReply = true
	}
	if messageID <= 0 || messageID > 1<<31-1 {
		return ai.ToolResult{Status: ai.ToolRejected, Content: "请先回复要撤回的消息，或先查询聊天记录取得真实消息ID", PossibleSideEffects: false}
	}
	if !fromReply && !inv.historyMessages[int32(messageID)] {
		return ai.ToolResult{Status: ai.ToolRejected, Content: "该消息ID不在当前聊天记录中，请先查询聊天记录后再撤回", PossibleSideEffects: false}
	}
	if err := actions.DeleteMsg(int32(messageID)); err != nil {
		return ai.ToolResult{Status: ai.ToolFailed, Content: "撤回未能确认完成，请先核对实际结果，避免重复提交。", PossibleSideEffects: true}
	}
	return ai.ToolResult{Status: ai.ToolSucceeded, Content: fmt.Sprintf("已撤回消息 %d", messageID), PossibleSideEffects: true}
}

func handleBanMember(
	inv qqInvocation,
	requestedUserID int64,
	duration int64,
	durationProvided bool,
	actions qqActions,
) ai.ToolResult {
	target := requestedUserID
	fromMention := false
	if target == 0 && len(inv.mentionedUsers) > 0 {
		target = inv.mentionedUsers[0]
		fromMention = true
	}
	if target <= 0 {
		return ai.ToolResult{Status: ai.ToolRejected, Content: "请 @ 要禁言的成员，或先查询聊天记录取得真实用户ID", PossibleSideEffects: false}
	}
	if target == inv.botID {
		return ai.ToolResult{Status: ai.ToolRejected, Content: "不能禁言机器人自己", PossibleSideEffects: false}
	}
	if !fromMention && !inv.isMentionedUser(target) && !inv.historyUsers[target] {
		return ai.ToolResult{Status: ai.ToolRejected, Content: "该用户ID不在当前消息或聊天记录中，请先 @成员或查询聊天记录", PossibleSideEffects: false}
	}
	if !durationProvided {
		duration = 600
	}
	if duration < 0 || duration > 30*24*60*60 {
		return ai.ToolResult{Status: ai.ToolRejected, Content: "禁言时长必须在 0 到 2592000 秒之间", PossibleSideEffects: false}
	}
	if err := actions.SetGroupBan(inv.groupID, target, int(duration)); err != nil {
		return ai.ToolResult{Status: ai.ToolFailed, Content: "禁言未能确认完成，请先核对实际结果，避免重复提交。", PossibleSideEffects: true}
	}
	if duration == 0 {
		return ai.ToolResult{Status: ai.ToolSucceeded, Content: fmt.Sprintf("已解除 %d 的禁言", target), PossibleSideEffects: true}
	}
	return ai.ToolResult{Status: ai.ToolSucceeded, Content: fmt.Sprintf("已禁言 %d，时长 %s", target, formatBanDuration(int(duration))), PossibleSideEffects: true}
}

func (inv qqInvocation) isMentionedUser(userID int64) bool {
	for _, mentioned := range inv.mentionedUsers {
		if mentioned == userID {
			return true
		}
	}
	return false
}

func handleSetGroupCard(inv qqInvocation, rawCard string, actions qqActions) ai.ToolResult {
	card, problem := validateQQText(rawCard, "群名片", maxGroupCardRunes)
	if problem != "" {
		return ai.ToolResult{Status: ai.ToolRejected, Content: problem, PossibleSideEffects: false}
	}
	target, allowed := inv.editableTarget()
	if !allowed {
		return ai.ToolResult{Status: ai.ToolRejected, Content: "普通群成员只能修改自己的群名片", PossibleSideEffects: false}
	}
	if err := actions.SetGroupCard(inv.groupID, target, card); err != nil {
		return ai.ToolResult{Status: ai.ToolFailed, Content: "修改群名片未能确认完成，请先核对实际结果，避免重复提交。", PossibleSideEffects: true}
	}
	if target == inv.callerID {
		return ai.ToolResult{Status: ai.ToolSucceeded, Content: fmt.Sprintf("已将你的群名片改为「%s」", card), PossibleSideEffects: true}
	}
	return ai.ToolResult{Status: ai.ToolSucceeded, Content: fmt.Sprintf("已将 %d 的群名片改为「%s」", target, card), PossibleSideEffects: true}
}

func handleSetSpecialTitle(inv qqInvocation, rawTitle string, actions qqActions) ai.ToolResult {
	title, problem := validateQQText(rawTitle, "专属头衔", maxSpecialTitleRunes)
	if problem != "" {
		return ai.ToolResult{Status: ai.ToolRejected, Content: problem, PossibleSideEffects: false}
	}
	target, allowed := inv.editableTarget()
	if !allowed {
		return ai.ToolResult{Status: ai.ToolRejected, Content: "普通群成员只能修改自己的专属头衔", PossibleSideEffects: false}
	}
	if err := actions.SetGroupSpecialTitle(inv.groupID, target, title); err != nil {
		return ai.ToolResult{Status: ai.ToolFailed, Content: "设置专属头衔未能确认完成，请先核对实际结果，避免重复提交。", PossibleSideEffects: true}
	}
	if target == inv.callerID {
		return ai.ToolResult{Status: ai.ToolSucceeded, Content: fmt.Sprintf("已将你的专属头衔设为「%s」", title), PossibleSideEffects: true}
	}
	return ai.ToolResult{Status: ai.ToolSucceeded, Content: fmt.Sprintf("已将 %d 的专属头衔设为「%s」", target, title), PossibleSideEffects: true}
}

func handleGroupPoke(inv qqInvocation, actions qqActions) ai.ToolResult {
	target := inv.callerID
	if len(inv.mentionedUsers) > 0 {
		target = inv.mentionedUsers[0]
	}
	if err := actions.GroupPoke(inv.groupID, target); err != nil {
		return ai.ToolResult{Status: ai.ToolFailed, Content: "戳一戳未能确认完成，请先核对实际结果，避免重复提交。", PossibleSideEffects: true}
	}
	return ai.ToolResult{Status: ai.ToolSucceeded, Content: fmt.Sprintf("戳了戳 %d", target), PossibleSideEffects: true}
}

func (inv qqInvocation) editableTarget() (int64, bool) {
	target := inv.callerID
	if len(inv.mentionedUsers) > 0 {
		target = inv.mentionedUsers[0]
	}
	return target, target == inv.callerID || inv.permission >= ai.PermAdmin
}

func validateQQText(raw, label string, maxRunes int) (string, string) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", "请提供新的" + label
	}
	if utf8.RuneCountInString(value) > maxRunes {
		return "", fmt.Sprintf("%s不能超过 %d 个字符", label, maxRunes)
	}
	return value, ""
}

func integerToolParam(ctx *ai.ToolContext, key string) (int64, bool) {
	raw, ok := ctx.Params[key]
	if !ok || raw == nil {
		return 0, false
	}
	switch value := raw.(type) {
	case float64:
		const maxExactJSONInteger = float64(1<<53 - 1)
		if value != math.Trunc(value) || value < -maxExactJSONInteger || value > maxExactJSONInteger {
			return 0, false
		}
		return int64(value), true
	case int64:
		return value, true
	case int:
		return int64(value), true
	case string:
		parsed, err := strconv.ParseInt(value, 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func formatBanDuration(seconds int) string {
	switch {
	case seconds%3600 == 0:
		return fmt.Sprintf("%d 小时", seconds/3600)
	case seconds%60 == 0:
		return fmt.Sprintf("%d 分钟", seconds/60)
	default:
		return fmt.Sprintf("%d 秒", seconds)
	}
}
