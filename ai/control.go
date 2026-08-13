package ai

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
)

var forgetMemoryPattern = regexp.MustCompile(`^(?:忘记|删除)(?:我的)?第\s*(\d+)\s*条(?:长期|AI|偏好)?记忆$`)
var forgetMemoryIDPattern = regexp.MustCompile(`^(?:忘掉|忘记|删除)(?:我的)?(?:长期|AI|偏好)?记忆\s*(\d+)$`)

var conversationResetCommands = map[string]bool{
	"新对话":     true,
	"重新开始":    true,
	"重置对话":    true,
	"清空对话上下文": true,
	"忘记刚才的对话": true,
}

var memoryListCommands = map[string]bool{
	"你记得我什么": true,
	"查看我的记忆": true,
	"我的AI记忆": true,
	"我的长期记忆": true,
}

var memoryClearCommands = map[string]bool{
	"清空我的AI记忆": true,
	"清空我的长期记忆": true,
}

func configuredBotName() string {
	if name := strings.TrimSpace(config.C.Bot.Name); name != "" {
		return name
	}
	return "月灵"
}

func normalizeControlText(text string) string {
	text = strings.TrimSpace(text)
	if name := configuredBotName(); strings.HasPrefix(text, name) {
		text = strings.TrimPrefix(text, name)
	}
	return strings.Trim(text, " \t\r\n，,。.!！：:、")
}

// handleLocalControl handles deterministic AI controls without spending an LLM call.
func handleLocalControl(groupID, userID int64, text string) (string, bool) {
	command := normalizeControlText(text)
	if conversationResetCommands[command] {
		Sessions.Delete(groupID, userID)
		return "好的，我们从这里重新开始。", true
	}

	if memoryListCommands[command] {
		profile, profileErr := db.GetAllUserProfile(userID)
		items, err := ListSemanticMemoryRecords(userID, 20)
		if err != nil || profileErr != nil {
			return "暂时无法读取个人资料和长期记忆，请稍后再试。", true
		}
		if len(items) == 0 && len(profile) == 0 {
			return "我还没有保存你的个人资料或长期记忆。", true
		}
		var lines []string
		if len(profile) > 0 {
			keys := make([]string, 0, len(profile))
			for key := range profile {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			lines = append(lines, "【你主动设置的资料】")
			for _, key := range keys {
				lines = append(lines, fmt.Sprintf("- %s：%s", key, profile[key]))
			}
		}
		if len(items) > 0 {
			lines = append(lines, "【长期记忆】")
			for index, item := range items {
				lines = append(lines, fmt.Sprintf("%d. [ID %d] %s", index+1, item.ID, item.Content))
			}
		}
		return "我记得这些：\n" + strings.Join(lines, "\n") +
			"\n可以说「删除我的位置」「忘掉记忆 ID」或「忘记第 N 条记忆」。", true
	}

	if match := forgetMemoryIDPattern.FindStringSubmatch(command); len(match) == 2 {
		memoryID, _ := strconv.ParseUint(match[1], 10, 64)
		deleted, err := DeleteSemanticMemory(userID, uint(memoryID))
		if err != nil || !deleted {
			return "没有找到属于你的这条记忆。", true
		}
		return fmt.Sprintf("已忘记记忆 %d。", memoryID), true
	}

	if match := forgetMemoryPattern.FindStringSubmatch(command); len(match) == 2 {
		position, _ := strconv.Atoi(match[1])
		items, err := ListSemanticMemoryRecords(userID, 20)
		if err != nil {
			return "暂时无法读取长期记忆，请稍后再试。", true
		}
		if position < 1 || position > len(items) {
			return fmt.Sprintf("没有第 %d 条记忆，目前共 %d 条。", position, len(items)), true
		}
		deleted, err := DeleteSemanticMemory(userID, items[position-1].ID)
		if err != nil || !deleted {
			return "删除失败，请稍后再试。", true
		}
		return fmt.Sprintf("已忘记：%s", items[position-1].Content), true
	}

	if memoryClearCommands[command] {
		count, err := ClearSemanticMemories(userID)
		if err != nil {
			return "清空失败，请稍后再试。", true
		}
		Sessions.Delete(groupID, userID)
		if count == 0 {
			return "我没有保存你的长期偏好；当前对话也已重置。", true
		}
		return fmt.Sprintf("已清空 %d 条长期偏好，当前对话也已重置。", count), true
	}

	if command == "忘记关于我的信息" {
		memoryCount, memoryErr := ClearSemanticMemories(userID)
		profileCount, profileErr := db.ClearUserProfiles(userID)
		if memoryErr != nil || profileErr != nil {
			return "清空失败，请稍后再试。", true
		}
		Sessions.Delete(groupID, userID)
		return fmt.Sprintf("已清空 %d 条长期记忆和 %d 条个人资料，当前对话也已重置。", memoryCount, profileCount), true
	}

	return "", false
}
