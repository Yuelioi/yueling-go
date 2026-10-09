package tools

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
)

var profileKeyAliases = map[string]string{
	"称呼": "称呼", "昵称": "称呼", "叫我": "称呼",
	"位置": "位置", "所在地": "位置", "城市": "位置",
	"时区": "时区", "timezone": "时区",
	"回复风格": "回复风格", "回答风格": "回复风格",
	"steam地区": "Steam地区", "steam区服": "Steam地区",
	"生日": "生日", "常用语言": "常用语言", "语言": "常用语言",
}

func (t *toolset) registerProfileTools() {
	t.registry.Register(ai.ToolMeta{
		Name:        "manage_user_context",
		Description: "管理用户明确指定的个人资料和长期记忆。资料适合称呼、位置、时区等稳定字段；其他偏好用 remember。用户要求记住、忘记或查看对自己的了解时使用",
		Tags:        []string{"用户资料", "长期记忆"},
		Triggers:    []string{"记住我", "记住这件事", "以后叫我", "我住在", "我的资料", "你记得我", "删除我的", "忘掉", "别记得"},
		Patterns:    []string{`(记住|别忘了).{0,20}(我|喜欢|讨厌|习惯)`, `(删除|忘掉).{0,10}(记忆|资料|位置|称呼|时区)`},
		Slots:       []string{"设置称呼", "设置位置", "用户画像", "长期记忆", "查看记忆"},
		PluginID:    catalog.PluginUserProfile,
		Params: []ai.Param{
			{Name: "action", Type: "string", Description: "操作", Required: true, Enum: []string{"set_profile", "remove_profile", "list_profile", "remember", "forget", "list_memories"}},
			{Name: "key", Type: "string", Description: "资料键：称呼、位置、时区、回复风格、Steam地区、生日、常用语言", Required: false},
			{Name: "value", Type: "string", Description: "资料值", Required: false},
			{Name: "content", Type: "string", Description: "用户明确要求记住的事实或偏好", Required: false},
			{Name: "category", Type: "string", Description: "记忆分类", Required: false, Enum: []string{"general", "food", "location", "hobby", "work", "preference", "identity"}},
			{Name: "memory_id", Type: "integer", Description: "要忘记的记忆 ID", Required: false},
		},
		ResultHandler: t.manageUserContext,
	})
}

func (t *toolset) manageUserContext(ctx *ai.ToolContext) ai.ToolResult {
	switch strings.ToLower(strings.TrimSpace(ctx.String("action"))) {
	case "set_profile":
		key, ok := canonicalProfileKey(ctx.String("key"))
		value := strings.TrimSpace(ctx.String("value"))
		if !ok || value == "" {
			return outcome(ai.ToolRejected, "资料键仅支持：称呼、位置、时区、回复风格、Steam地区、生日、常用语言", false)
		}
		if len([]rune(value)) > 256 {
			return outcome(ai.ToolRejected, "资料值最多256个字符", false)
		}
		if key == "时区" {
			if _, err := time.LoadLocation(value); err != nil {
				return outcome(ai.ToolRejected, "时区请使用 IANA 名称，例如 Asia/Shanghai", false)
			}
		}
		if err := t.memory.SetUserProfile(ctx.UserID(), key, value); err != nil {
			return outcome(ai.ToolFailed, "操作未能确认完成，请先核对实际结果，避免重复提交。", true)
		}
		return outcome(ai.ToolSucceeded, fmt.Sprintf("已记下你的%s：%s", key, value), true)

	case "remove_profile":
		key, ok := canonicalProfileKey(ctx.String("key"))
		if !ok {
			return outcome(ai.ToolRejected, "请提供要删除的资料键", false)
		}
		if err := t.memory.DeleteUserProfile(ctx.UserID(), key); err != nil {
			return outcome(ai.ToolFailed, "操作未能确认完成，请先核对实际结果，避免重复提交。", true)
		}
		return outcome(ai.ToolSucceeded, "已删除资料："+key, true)

	case "list_profile":
		profile, err := t.memory.GetAllUserProfile(ctx.UserID())
		if err != nil {
			return outcome(ai.ToolFailed, "读取或操作失败，请稍后核对结果", false)
		}
		if len(profile) == 0 {
			return outcome(ai.ToolEmpty, "你还没有主动设置个人资料", false)
		}
		keys := make([]string, 0, len(profile))
		for key := range profile {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		lines := make([]string, 0, len(keys))
		for _, key := range keys {
			lines = append(lines, fmt.Sprintf("%s：%s", key, profile[key]))
		}
		return outcome(ai.ToolSucceeded, "你的资料：\n"+strings.Join(lines, "\n"), false)

	case "remember":
		content := strings.TrimSpace(ctx.String("content"))
		if content == "" || len([]rune(content)) > 256 {
			return outcome(ai.ToolRejected, "记忆内容不能为空且最多256个字符", false)
		}
		if containsSensitiveProfileContent(content) {
			return outcome(ai.ToolRejected, "为保护隐私，我不会保存密码、验证码、证件号或密钥", false)
		}
		category := strings.TrimSpace(ctx.String("category"))
		if category == "" {
			category = "general"
		}
		if err := t.memory.WriteSemanticDetailed(ctx.UserID(), content, category, "explicit", 1, 1.5); err != nil {
			return outcome(ai.ToolFailed, "操作未能确认完成，请先核对实际结果，避免重复提交。", true)
		}
		return outcome(ai.ToolSucceeded, "记住了："+content, true)

	case "list_memories":
		rows, err := t.memory.ListSemanticMemoryRecords(ctx.UserID(), 20)
		if err != nil {
			return outcome(ai.ToolFailed, "读取或操作失败，请稍后核对结果", false)
		}
		if len(rows) == 0 {
			return outcome(ai.ToolEmpty, "我还没有保存你的长期记忆", false)
		}
		lines := make([]string, 0, len(rows))
		for _, row := range rows {
			lines = append(lines, fmt.Sprintf("ID %d · %s", row.ID, row.Content))
		}
		return outcome(ai.ToolSucceeded, "我记得：\n"+strings.Join(lines, "\n"), false)

	case "forget":
		id := ctx.Int("memory_id")
		if id <= 0 {
			return outcome(ai.ToolRejected, "请先查看记忆并提供要忘记的记忆ID", false)
		}
		deleted, err := t.memory.DeleteSemanticMemory(ctx.UserID(), uint(id))
		if err != nil {
			return outcome(ai.ToolFailed, "删除未能确认完成，请先核对记忆列表", true)
		}
		if !deleted {
			return outcome(ai.ToolRejected, "没有找到属于你的这条记忆", false)
		}
		return outcome(ai.ToolSucceeded, fmt.Sprintf("已忘记记忆 %d", id), true)
	}
	return outcome(ai.ToolRejected, "未知操作", false)
}

func canonicalProfileKey(raw string) (string, bool) {
	key, ok := profileKeyAliases[strings.ToLower(strings.TrimSpace(raw))]
	return key, ok
}

func containsSensitiveProfileContent(value string) bool {
	lower := strings.ToLower(value)
	for _, keyword := range []string{"密码", "验证码", "身份证", "银行卡", "私钥", "access token", "api key", "密钥"} {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}
