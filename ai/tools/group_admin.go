package tools

import (
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
)

func (t *toolset) registerGroupAdminTools() {
	t.registerGroupRulesTool()
	t.registerKnowledgeAdminTool()
}

func (t *toolset) registerGroupRulesTool() {
	t.registry.Register(ai.ToolMeta{
		Name:        "manage_group_rules",
		Description: "用自然语言维护注入 AI 上下文的本群规则，仅管理员可用",
		Tags:        []string{"群管", "群规则"},
		Triggers:    []string{"添加群规则", "删除群规则", "修改群规则", "本群规则"},
		Patterns:    []string{`(记一条|加一条|新增).{0,8}群规`, `(删掉|移除).{0,8}规则`},
		Slots:       []string{"管理群规则", "AI群规则"},
		PluginID:    catalog.PluginRules,
		Permission:  ai.PermAdmin,
		Params: []ai.Param{
			{Name: "action", Type: "string", Description: "操作", Required: true, Enum: []string{"add", "list", "remove"}},
			{Name: "rule", Type: "string", Description: "add 的规则内容", Required: false},
			{Name: "rule_id", Type: "integer", Description: "remove 的规则 ID", Required: false},
		},
		ResultHandler: func(ctx *ai.ToolContext) ai.ToolResult {
			switch ctx.String("action") {
			case "add":
				rule := strings.TrimSpace(ctx.String("rule"))
				if rule == "" || utf8Len(rule) > 500 {
					return outcome(ai.ToolRejected, "规则不能为空且最多500个字符", false)
				}
				if err := t.memory.AddGroupRule(ctx.GroupID(), ctx.UserID(), rule); err != nil {
					return outcome(ai.ToolFailed, "操作未能确认完成，请先核对实际结果，避免重复提交。", true)
				}
				return outcome(ai.ToolSucceeded, "群规则已添加："+rule, true)
			case "list":
				rows, err := t.memory.ReadGroupRules(ctx.GroupID())
				if err != nil {
					return outcome(ai.ToolFailed, "读取群规则失败", false)
				}
				if len(rows) == 0 {
					return outcome(ai.ToolEmpty, "本群暂无 AI 群规则", false)
				}
				lines := make([]string, 0, len(rows))
				for _, row := range rows {
					lines = append(lines, fmt.Sprintf("ID %d · %s", row.ID, row.Rule))
				}
				return outcome(ai.ToolSucceeded, "本群 AI 规则：\n"+strings.Join(lines, "\n"), false)
			case "remove":
				id := ctx.Int("rule_id")
				if id <= 0 {
					return outcome(ai.ToolRejected, "请提供规则ID", false)
				}
				if err := t.memory.RemoveGroupRule(ctx.GroupID(), uint(id)); err != nil {
					return outcome(ai.ToolFailed, "操作未能确认完成，请先核对实际结果，避免重复提交。", true)
				}
				return outcome(ai.ToolSucceeded, fmt.Sprintf("群规则 %d 已删除", id), true)
			}
			return outcome(ai.ToolRejected, "未知操作", false)
		},
	})
}

func (t *toolset) registerKnowledgeAdminTool() {
	t.registry.Register(ai.ToolMeta{
		Name:        "manage_group_knowledge",
		Description: "维护当前群知识库；可保存文字、导入网页，并给条目设置精确快捷触发词（发送后不调用 AI 直接回复）。仅管理员可用",
		Tags:        []string{"知识库", "群资料", "快捷触发"},
		Triggers:    []string{"添加到知识库", "保存到知识库", "导入知识库", "删除知识库", "设置快捷词", "快捷触发词"},
		Patterns:    []string{`(记到|放进|加入).{0,8}(知识库|群资料)`, `(导入|收录).{0,12}https?://`, `(有人|群友)说.{0,20}(就|回复)`, `给知识.{0,8}(设置|添加).{0,6}快捷词`},
		Slots:       []string{"维护知识库", "添加群文档", "导入网页资料", "固定口令", "快捷回复"},
		PluginID:    catalog.PluginGroupKnowledge,
		Permission:  ai.PermAdmin,
		Params: []ai.Param{
			{Name: "action", Type: "string", Description: "操作", Required: true, Enum: []string{"add_text", "add_url", "list", "set_shortcuts", "remove"}},
			{Name: "title", Type: "string", Description: "标题", Required: false},
			{Name: "content", Type: "string", Description: "add_text 的正文", Required: false},
			{Name: "url", Type: "string", Description: "add_url 的公网 HTTP/HTTPS 地址", Required: false},
			{Name: "shortcuts", Type: "array", ItemsType: "string", Description: "add_text、add_url 或 set_shortcuts 的精确触发词；空数组表示清空", Required: false},
			{Name: "knowledge_id", Type: "integer", Description: "set_shortcuts 或 remove 的条目 ID", Required: false},
		},
		ResultHandler: func(ctx *ai.ToolContext) ai.ToolResult {
			switch ctx.String("action") {
			case "add_text":
				row, err := t.knowledge.AddTextWithShortcuts(ctx.GroupID(), ctx.UserID(), ctx.String("title"), ctx.String("content"), ctx.StringSlice("shortcuts"))
				if err != nil {
					return outcome(ai.ToolFailed, "操作未能确认完成，请先核对实际结果，避免重复提交。", true)
				}
				return outcome(ai.ToolSucceeded, formatKnowledgeSaved("已保存", row), true)
			case "add_url":
				row, err := t.knowledge.AddURLWithShortcuts(ctx.Context(), ctx.GroupID(), ctx.UserID(), ctx.String("title"), ctx.String("url"), ctx.StringSlice("shortcuts"))
				if err != nil {
					return outcome(ai.ToolFailed, "操作未能确认完成，请先核对实际结果，避免重复提交。", true)
				}
				return outcome(ai.ToolSucceeded, formatKnowledgeSaved("已导入", row), true)
			case "list":
				rows, err := t.knowledge.List(ctx.GroupID())
				if err != nil {
					return outcome(ai.ToolFailed, "读取或操作失败，请稍后核对结果", false)
				}
				if len(rows) == 0 {
					return outcome(ai.ToolEmpty, "本群知识库为空", false)
				}
				lines := make([]string, 0, len(rows))
				for _, row := range rows {
					shortcutValues := make([]string, 0, len(row.Shortcuts))
					for _, shortcut := range row.Shortcuts {
						shortcutValues = append(shortcutValues, shortcut.Trigger)
					}
					detail := ""
					if len(shortcutValues) > 0 {
						detail = " · 快捷词 " + strings.Join(shortcutValues, "、")
					}
					lines = append(lines, fmt.Sprintf("ID %d · %s%s", row.ID, row.Title, detail))
				}
				return outcome(ai.ToolSucceeded, "本群知识库：\n"+strings.Join(lines, "\n"), false)
			case "set_shortcuts":
				id := ctx.Int("knowledge_id")
				if id <= 0 {
					return outcome(ai.ToolRejected, "请提供知识条目ID", false)
				}
				if _, provided := ctx.Params["shortcuts"]; !provided {
					return outcome(ai.ToolRejected, "请提供快捷触发词；如果是要清空，请明确传入空列表", false)
				}
				shortcuts, err := t.knowledge.SetShortcuts(uint(id), ctx.GroupID(), ctx.StringSlice("shortcuts"))
				if err != nil {
					return outcome(ai.ToolFailed, "操作未能确认完成，请先核对实际结果，避免重复提交。", true)
				}
				if len(shortcuts) == 0 {
					return outcome(ai.ToolSucceeded, fmt.Sprintf("知识 #%d 的快捷触发词已清空", id), true)
				}
				values := make([]string, 0, len(shortcuts))
				for _, shortcut := range shortcuts {
					values = append(values, shortcut.Trigger)
				}
				return outcome(ai.ToolSucceeded, fmt.Sprintf("知识 #%d 的快捷触发词已设为：%s", id, strings.Join(values, "、")), true)
			case "remove":
				id := ctx.Int("knowledge_id")
				if id <= 0 {
					return outcome(ai.ToolRejected, "请提供知识条目ID", false)
				}
				if err := t.knowledge.Remove(uint(id), ctx.GroupID()); err != nil {
					return outcome(ai.ToolFailed, "操作未能确认完成，请先核对实际结果，避免重复提交。", true)
				}
				return outcome(ai.ToolSucceeded, fmt.Sprintf("知识条目 %d 已删除", id), true)
			}
			return outcome(ai.ToolRejected, "未知操作", false)
		},
	})
}

func formatKnowledgeSaved(prefix string, row *db.GroupKnowledge) string {
	result := fmt.Sprintf("%s知识 #%d：%s", prefix, row.ID, row.Title)
	if len(row.Shortcuts) == 0 {
		return result
	}
	values := make([]string, 0, len(row.Shortcuts))
	for _, shortcut := range row.Shortcuts {
		values = append(values, shortcut.Trigger)
	}
	return result + "；快捷触发词：" + strings.Join(values, "、")
}
