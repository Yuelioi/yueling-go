package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	openai "github.com/sashabaranov/go-openai"
)

const (
	maxSemantic   = 100
	maxEpisodic   = 50
	maxProcedural = 10
	decayRate     = 0.95
)

var preferenceTrigers = []string{
	"我喜欢", "我不喜欢", "我讨厌", "以后别", "以后不要",
	"我是", "我的", "叫我", "记住",
	"每次都", "总是", "从来不", "一直",
}

func shouldWriteSemantic(text string) bool {
	for _, t := range preferenceTrigers {
		if strings.Contains(text, t) {
			return true
		}
	}
	return false
}

// ── Write ─────────────────────────────────────────────────────────────────────

func WriteSemantic(userID int64, content, category string) error {
	now := float64(time.Now().Unix())
	var count int64
	db.DB.Model(&db.SemanticMemory{}).Where("user_id = ?", userID).Count(&count)
	if count >= maxSemantic {
		var oldest db.SemanticMemory
		if db.DB.Where("user_id = ?", userID).Order("score asc").First(&oldest).Error == nil {
			db.DB.Delete(&oldest)
		}
	}
	return db.DB.Create(&db.SemanticMemory{
		UserID:       userID,
		Content:      content,
		Category:     category,
		Score:        1.0,
		CreatedAt:    now,
		LastAccessed: now,
	}).Error
}

func WriteEpisodic(userID, groupID int64, inputText, toolName, resultSummary string, steps int) error {
	now := float64(time.Now().Unix())
	var count int64
	db.DB.Model(&db.EpisodicMemory{}).Where("user_id = ?", userID).Count(&count)
	if count >= maxEpisodic {
		var oldest db.EpisodicMemory
		if db.DB.Where("user_id = ?", userID).Order("created_at asc").First(&oldest).Error == nil {
			db.DB.Delete(&oldest)
		}
	}
	return db.DB.Create(&db.EpisodicMemory{
		UserID:        userID,
		GroupID:       groupID,
		InputText:     inputText,
		ToolName:      toolName,
		ResultSummary: resultSummary,
		Steps:         steps,
		CreatedAt:     now,
	}).Error
}

func AddGroupRule(groupID, createdBy int64, rule string) error {
	var count int64
	db.DB.Model(&db.ProceduralMemory{}).Where("group_id = ?", groupID).Count(&count)
	if count >= maxProcedural {
		return fmt.Errorf("群规则已达上限（%d条）", maxProcedural)
	}
	return db.DB.Create(&db.ProceduralMemory{
		GroupID:   groupID,
		Rule:      rule,
		CreatedBy: createdBy,
		CreatedAt: float64(time.Now().Unix()),
	}).Error
}

func RemoveGroupRule(groupID int64, ruleID uint) error {
	return db.DB.Where("id = ? AND group_id = ?", ruleID, groupID).Delete(&db.ProceduralMemory{}).Error
}

// ── Recall ────────────────────────────────────────────────────────────────────

type SemanticItem struct {
	Content  string
	Category string
	Score    float64
}

func RecallSemantic(userID int64, limit int) []SemanticItem {
	var rows []db.SemanticMemory
	db.DB.Where("user_id = ?", userID).Order("score desc").Limit(limit).Find(&rows)
	now := float64(time.Now().Unix())
	out := make([]SemanticItem, 0, len(rows))
	for _, r := range rows {
		daysOld := (now - r.CreatedAt) / 86400
		effective := r.Score * math.Pow(decayRate, daysOld)
		out = append(out, SemanticItem{Content: r.Content, Category: r.Category, Score: effective})
	}
	return out
}

type EpisodicItem struct {
	Input   string
	Tool    string
	Result  string
}

func RecallEpisodic(userID int64, limit int) []EpisodicItem {
	var rows []db.EpisodicMemory
	db.DB.Where("user_id = ?", userID).Order("created_at desc").Limit(limit).Find(&rows)
	out := make([]EpisodicItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, EpisodicItem{Input: r.InputText, Tool: r.ToolName, Result: r.ResultSummary})
	}
	return out
}

func GetGroupRules(groupID int64) []string {
	var rows []db.ProceduralMemory
	db.DB.Where("group_id = ?", groupID).Order("priority desc, created_at asc").Find(&rows)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Rule)
	}
	return out
}

func ListGroupRules(groupID int64) []db.ProceduralMemory {
	var rows []db.ProceduralMemory
	db.DB.Where("group_id = ?", groupID).Order("priority desc, created_at asc").Find(&rows)
	return rows
}

// ── Context builders ──────────────────────────────────────────────────────────

// UserContext returns a string summarising the user's stored preferences,
// ready to be appended to the system prompt.
func UserContext(userID int64) string {
	var sb strings.Builder

	// Structured profile (location, etc.)
	profile, _ := db.GetAllUserProfile(userID)
	if loc, ok := profile["位置"]; ok && loc != "" {
		sb.WriteString(fmt.Sprintf("\n\n用户当前位置：%s（查天气等无需再问城市）", loc))
	}

	// Semantic memory
	items := RecallSemantic(userID, 5)
	if len(items) > 0 {
		sb.WriteString("\n\n用户偏好：")
		for _, m := range items {
			sb.WriteString("\n- " + m.Content)
		}
	}
	return sb.String()
}

// GroupContext returns a string of group rules for the system prompt.
func GroupContext(groupID int64) string {
	rules := GetGroupRules(groupID)
	if len(rules) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\n群规则：")
	for _, r := range rules {
		sb.WriteString("\n- " + r)
	}
	return sb.String()
}

// ── Smart extraction ──────────────────────────────────────────────────────────

// SmartWriteSemantic uses the LLM to extract memories from a conversation turn.
// Run in a goroutine — errors are logged, not returned.
func SmartWriteSemantic(userID int64, userText, botReply string) {
	if !shouldWriteSemantic(userText) {
		return
	}
	existing := RecallSemantic(userID, 20)
	existingStrs := make([]string, 0, len(existing))
	for _, e := range existing {
		existingStrs = append(existingStrs, e.Content)
	}
	existingStr := "无"
	if len(existingStrs) > 0 {
		existingStr = strings.Join(existingStrs, "、")
	}

	prompt := fmt.Sprintf(
		"从以下对话中提取用户的偏好、事实或重要信息。\n"+
			"已有记忆: %s\n"+
			"规则:\n"+
			"- 只提取用户明确表达或强烈暗示的信息\n"+
			"- 不要重复已有记忆中已包含的内容\n"+
			"- 如果没有值得记忆的内容，返回空数组 []\n"+
			"- 返回 JSON 数组，每项: {\"content\": \"简短描述\", \"category\": \"分类\"}\n"+
			"- 分类可选: general/food/location/hobby/work/preference/identity\n"+
			"\n用户: %s\n助手: %s",
		existingStr, userText, botReply,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := llm().CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: config.C.AI.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "你是记忆提取器。只输出JSON数组，不要输出其他内容。"},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens:   200,
		Temperature: 0.1,
	})
	if err != nil {
		log.Printf("[memory] extract failed: %v", err)
		return
	}
	if len(resp.Choices) == 0 {
		return
	}

	raw := strings.TrimSpace(resp.Choices[0].Message.Content)
	if strings.HasPrefix(raw, "```") {
		parts := strings.SplitN(raw, "\n", 2)
		if len(parts) == 2 {
			raw = strings.TrimSuffix(parts[1], "```")
		}
	}

	var items []struct {
		Content  string `json:"content"`
		Category string `json:"category"`
	}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		log.Printf("[memory] parse failed: %v (raw: %s)", err, raw)
		return
	}

	existingSet := make(map[string]bool, len(existingStrs))
	for _, s := range existingStrs {
		existingSet[s] = true
	}
	for _, item := range items {
		if item.Content == "" || existingSet[item.Content] {
			continue
		}
		cat := item.Category
		if cat == "" {
			cat = "general"
		}
		if err := WriteSemantic(userID, item.Content, cat); err != nil {
			log.Printf("[memory] write semantic failed: %v", err)
		}
	}
}
