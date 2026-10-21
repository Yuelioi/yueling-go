package tools

import (
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
)

func (t *toolset) registerContextTools() {
	t.registerGetChatHistory()
	t.registerResolveUserByName()
	t.registerGetGroupMembers()
	t.registerGetUserAvatar()
}

func (t *toolset) registerGetUserAvatar() {
	t.registry.Register(ai.ToolMeta{
		Name: "get_user_avatar", Permission: ai.PermMember, ReadOnly: true,
		Description: "获取QQ用户头像的图片编号，无需管理员权限或对方先发送图片。用户要求用自己、@对象或指定QQ的头像生图、改图、识图时先调用，再将image_id交给edit_image或inspect_image；昵称不明确时先查找用户。这里只取得图片引用，不代表已看懂头像。",
		Tags:        []string{"图片", "上下文"}, Triggers: []string{"头像"},
		Params: []ai.Param{{Name: "user_id", Type: "integer", Description: "目标QQ号。用户说‘我的头像’时传当前speaker_id；省略时使用唯一的@对象（排除机器人），无@对象则使用发言者。多个对象时必须明确指定；取机器人头像时显式传其QQ号。"}},
		ResultHandler: func(c *ai.ToolContext) ai.ToolResult {
			userID := c.Int("user_id")
			if _, explicit := c.Params["user_id"]; !explicit {
				mentions := c.MentionedUserIDs()
				switch len(mentions) {
				case 0:
					userID = c.UserID()
				case 1:
					userID = mentions[0]
				default:
					return outcome(ai.ToolRejected, "有多个被@的用户，请明确要使用谁的头像并指定user_id", false)
				}
			}
			id, err := c.UserAvatarImage(userID)
			if err != nil {
				return outcome(ai.ToolRejected, err.Error(), false)
			}
			return outcome(ai.ToolSucceeded, fmt.Sprintf("QQ用户%d的头像引用：image_id=%s。参考该头像生图或换风格请调用edit_image；需要理解画面时调用inspect_image。", userID, id), false)
		},
	})
}

// ── 获取聊天记录 ──────────────────────────────────────────────────────────────

func (t *toolset) registerGetChatHistory() {
	t.registry.Register(ai.ToolMeta{
		ReadOnly:    true,
		Name:        "get_chat_history",
		Description: "读取本群最近聊天，用于接续话题、查找发言者或找回图片；返回图片编号可继续识图、改图。消息和用户编号仅用于内部定位，对外使用昵称。",
		Tags:        []string{"上下文"},
		Triggers:    []string{"聊天记录", "刚才", "刚刚", "那条", "那个人", "上面"},
		Patterns:    []string{`(刚才|刚刚).+说`, `上面说`, `说.+的(人|消息)`},
		Slots:       []string{"最近消息", "上文", "之前聊了什么", "指代目标"},
		Params: []ai.Param{
			{Name: "count", Type: "integer", Description: "获取条数(1-30)，默认见配置", Required: false},
			{Name: "image_query", Type: "string", Description: "按生成要求或来源关键词找回会话图片；填写时只查图片索引，不读取聊天"},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			count := ai.ResolveCount(int(ctx.Int("count")), config.Current().AI.Context.ChatHistory, 1, 30)
			if query := strings.TrimSpace(ctx.String("image_query")); query != "" {
				return ctx.ImageHistory(query, count), nil
			}
			messages, err := ctx.BotAPI().GetGroupMsgHistory(ctx.GroupID(), ctx.MessageID(), count)
			if err != nil {
				return "暂时没能读取群聊信息，请稍后再试。", err
			}
			ctx.SetState(qqHistoryStateKey, historyReferences(messages))
			if len(messages) == 0 {
				return "无记录", nil
			}
			history := formatChatHistory(messages)
			var imageReferences []string
			for i := len(messages) - 1; i >= 0 && len(imageReferences) < 8; i-- {
				msg := messages[i]
				for _, id := range ctx.MessageImageReferences(msg.MessageID, msg.UserID, msg.ImageURLs(), 8-len(imageReferences)) {
					imageReferences = append(imageReferences, fmt.Sprintf("%s：消息%d中的图片，发送者QQ%d", id, msg.MessageID, msg.UserID))
					if len(imageReferences) == 8 {
						break
					}
				}
			}
			if len(imageReferences) > 0 {
				history += "\n【记录中的图片编号，可用于识图或改图】\n" + strings.Join(imageReferences, "\n")
			}
			if history == "" {
				return "无文字消息", nil
			}
			return history, nil
		},
	})
}

func historyReferences(messages []bot.HistoryMessage) []qqHistoryReference {
	refs := make([]qqHistoryReference, 0, len(messages))
	for _, msg := range messages {
		refs = append(refs, qqHistoryReference{
			messageID: msg.MessageID,
			userID:    msg.UserID,
		})
	}
	return refs
}

func formatChatHistory(messages []bot.HistoryMessage) string {
	var lines []string
	for _, msg := range messages {
		nick := msg.Sender.Card
		if nick == "" {
			nick = msg.Sender.Nickname
		}
		if nick == "" {
			nick = "一位群友"
		}
		var parts []string
		for _, seg := range msg.Message {
			switch seg.Type {
			case "text":
				if text := strings.TrimSpace(seg.Data.Text); text != "" {
					parts = append(parts, text)
				}
			case "image":
				parts = append(parts, "[图片]")
			case "at":
				parts = append(parts, fmt.Sprintf("@%s", seg.Data.QQ))
			}
		}
		if len(parts) > 0 {
			lines = append(lines, fmt.Sprintf(
				"[消息ID:%d 用户ID:%d] %s: %s",
				msg.MessageID,
				msg.UserID,
				nick,
				strings.Join(parts, " "),
			))
		}
	}
	return strings.Join(lines, "\n")
}

// ── 昵称模糊匹配 ──────────────────────────────────────────────────────────────

func (t *toolset) registerResolveUserByName() {
	t.registry.Register(ai.ToolMeta{
		ReadOnly:    true,
		Name:        "resolve_user_by_name",
		Description: "通过昵称/群名片模糊匹配查找用户QQ号，用于确定发言者身份",
		Tags:        []string{"上下文", "群组"},
		Triggers:    []string{"谁"},
		Patterns:    []string{`那个\S+是谁`},
		Slots:       []string{"查找用户", "找人", "QQ号"},
		Params: []ai.Param{
			{Name: "name", Type: "string", Description: "昵称或群名片关键词", Required: true},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			name := strings.TrimSpace(ctx.String("name"))
			if name == "" {
				return "请提供昵称关键词", nil
			}
			members, err := ctx.BotAPI().GetGroupMemberList(ctx.GroupID())
			if err != nil {
				return "暂时没能查找群成员，请稍后再试。", err
			}
			var matches []string
			for _, m := range members {
				card := m.Card
				nick := m.Nickname
				if strings.Contains(card, name) || strings.Contains(nick, name) {
					display := card
					if display == "" {
						display = nick
					}
					matches = append(matches, fmt.Sprintf("%s → %d", display, m.UserID))
				}
				if len(matches) >= 5 {
					break
				}
			}
			if len(matches) == 0 {
				return fmt.Sprintf("未找到包含'%s'的群成员", name), nil
			}
			return strings.Join(matches, "\n"), nil
		},
	})
}

// ── 群成员列表 ────────────────────────────────────────────────────────────────

func (t *toolset) registerGetGroupMembers() {
	t.registry.Register(ai.ToolMeta{
		ReadOnly:    true,
		Name:        "get_group_members",
		Description: "获取群成员列表，可按关键词筛选，用于确认成员身份",
		Tags:        []string{"上下文", "群组"},
		Triggers:    []string{"群友", "成员"},
		Slots:       []string{"群成员列表", "管理员", "群里有没有"},
		Params: []ai.Param{
			{Name: "keyword", Type: "string", Description: "搜索关键词，空=返回最近活跃成员", Required: false},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			keyword := strings.TrimSpace(ctx.String("keyword"))
			members, err := ctx.BotAPI().GetGroupMemberList(ctx.GroupID())
			if err != nil {
				return "暂时没能读取群聊信息，请稍后再试。", err
			}

			type entry struct {
				UserID       int64
				Card         string
				Nickname     string
				Role         string
				LastSentTime int64
			}
			var list []entry
			for _, m := range members {
				if keyword == "" || strings.Contains(m.Card, keyword) || strings.Contains(m.Nickname, keyword) {
					list = append(list, entry{m.UserID, m.Card, m.Nickname, m.Role, m.LastSentTime})
				}
			}
			if len(list) == 0 {
				return "无匹配成员", nil
			}

			for i := 1; i < len(list); i++ {
				for j := i; j > 0 && list[j].LastSentTime > list[j-1].LastSentTime; j-- {
					list[j], list[j-1] = list[j-1], list[j]
				}
			}

			limit := 15
			if len(list) < limit {
				limit = len(list)
			}
			var lines []string
			for _, m := range list[:limit] {
				name := m.Card
				if name == "" {
					name = m.Nickname
				}
				tag := ""
				switch m.Role {
				case "admin":
					tag = " [管理]"
				case "owner":
					tag = " [群主]"
				}
				lines = append(lines, fmt.Sprintf("%s(%d)%s", name, m.UserID, tag))
			}
			if len(list) > 15 {
				lines = append(lines, fmt.Sprintf("... 共%d人", len(list)))
			}
			return strings.Join(lines, "\n"), nil
		},
	})
}
