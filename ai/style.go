package ai

import (
	"strings"

	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/services/logx"
)

const DefaultGroupStylePrompt = "活泼、亲切，像熟悉群聊氛围的群友一样自然交流；使用简洁的中文，避免过度卖萌和长篇解释。"

func groupStylePrompt(groupID int64) string {
	if db.DB == nil {
		return DefaultGroupStylePrompt
	}
	defaultPrompt := DefaultGroupStylePrompt
	configuredDefault, configured, err := db.GetGroupAIStylePrompt(db.DefaultAIStyleGroupID)
	if err != nil {
		logx.Errorf("[ai] load default group style: %v", err)
	} else if configured && strings.TrimSpace(configuredDefault) != "" {
		defaultPrompt = strings.TrimSpace(configuredDefault)
	}
	if groupID <= 0 {
		return defaultPrompt
	}
	prompt, custom, err := db.GetGroupAIStylePrompt(groupID)
	if err != nil {
		logx.Errorf("[ai] load group style group=%d: %v", groupID, err)
		return defaultPrompt
	}
	if !custom || strings.TrimSpace(prompt) == "" {
		return defaultPrompt
	}
	return strings.TrimSpace(prompt)
}

func groupStyleInstruction(groupID int64) string {
	return "【自定义提示词】\n" + groupStylePrompt(groupID)
}
