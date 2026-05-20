package tools

import (
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
)

func init() {
	registerGetChatHistory()
	registerResolveUserByName()
	registerGetGroupMembers()
}

// ── 获取聊天记录 ──────────────────────────────────────────────────────────────

func registerGetChatHistory() {
	ai.Register(ai.ToolMeta{
		Name:        "get_chat_history",
		Description: "获取群最近聊天记录，用于理解上下文和确定发言者身份",
		Tags:        []string{"上下文"},
		Triggers:    []string{"聊天记录", "刚才"},
		Patterns:    []string{`刚才.+说`, `上面说`},
		Slots:       []string{"最近消息", "上文", "之前聊了什么"},
		Params: []ai.Param{
			{Name: "count", Type: "integer", Description: "获取条数(1-30)，默认15", Required: false},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			count := int(ctx.Int("count"))
			if count < 1 {
				count = 15
			}
			if count > 30 {
				count = 30
			}
			messages, err := ctx.BotAPI().GetGroupMsgHistory(ctx.GroupID(), ctx.MessageID(), count)
			if err != nil {
				return "获取失败: " + err.Error(), nil
			}
			if len(messages) == 0 {
				return "无记录", nil
			}
			var lines []string
			for _, msg := range messages {
				nick := msg.Sender.Card
				if nick == "" {
					nick = msg.Sender.Nickname
				}
				if nick == "" {
					nick = fmt.Sprintf("%d", msg.UserID)
				}
				var parts []string
				for _, seg := range msg.Message {
					switch seg.Type {
					case "text":
						if t := strings.TrimSpace(seg.Data.Text); t != "" {
							parts = append(parts, t)
						}
					case "image":
						parts = append(parts, "[图片]")
					case "at":
						parts = append(parts, fmt.Sprintf("@%s", seg.Data.QQ))
					}
				}
				if len(parts) > 0 {
					lines = append(lines, fmt.Sprintf("[%d] %s: %s", msg.UserID, nick, strings.Join(parts, " ")))
				}
			}
			if len(lines) == 0 {
				return "无文字消息", nil
			}
			return strings.Join(lines, "\n"), nil
		},
	})
}

// ── 昵称模糊匹配 ──────────────────────────────────────────────────────────────

func registerResolveUserByName() {
	ai.Register(ai.ToolMeta{
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
				return "查找失败: " + err.Error(), nil
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

func registerGetGroupMembers() {
	ai.Register(ai.ToolMeta{
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
				return "获取失败: " + err.Error(), nil
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
