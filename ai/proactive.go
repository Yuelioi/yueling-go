package ai

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

const (
	proactiveThreshold = 100
	proactiveCooldown  = 3 * time.Minute
	proactiveMaxDaily  = 5
	proactiveMinStreak = 3
)

type groupHeat struct {
	score      int
	lastSpeak  time.Time
	dailyCount int
	dailyDate  string
	streak     int
	messages   []string
}

// ProactiveManager tracks per-group message heat and triggers proactive AI speech.
type ProactiveManager struct {
	mu    sync.Mutex
	heats map[int64]*groupHeat
}

var Proactive = &ProactiveManager{heats: make(map[int64]*groupHeat)}

func (p *ProactiveManager) heat(groupID int64) *groupHeat {
	h, ok := p.heats[groupID]
	if !ok {
		h = &groupHeat{}
		p.heats[groupID] = h
	}
	today := bot.Today()
	if h.dailyDate != today {
		h.dailyCount = 0
		h.dailyDate = today
	}
	return h
}

var proactiveTopics = []string{
	"月灵", "翻译", "热搜", "查", "计算", "游戏", "物价",
	"运势", "天气", "搜", "帮我",
}
var proactiveQuestions = []string{"?", "？", "吗", "呢", "啥", "什么", "怎么", "谁", "哪"}

// Feed accumulates heat for an incoming group message and fires proactive speech if threshold met.
func (p *ProactiveManager) Feed(api *bot.BotAPI, e *bot.GroupMessageEvent) {
	text := strings.TrimSpace(e.Message.Text())
	if text == "" {
		return
	}

	p.mu.Lock()
	h := p.heat(e.GroupID)

	h.streak++
	h.score += 2
	for _, k := range proactiveTopics {
		if strings.Contains(text, k) {
			h.score += 3
			break
		}
	}
	for _, m := range proactiveQuestions {
		if strings.Contains(text, m) {
			h.score += 5
			break
		}
	}
	if len(h.messages) >= 20 {
		h.messages = h.messages[1:]
	}
	h.messages = append(h.messages, text)

	shouldFire := h.score >= proactiveThreshold &&
		h.streak >= proactiveMinStreak &&
		h.dailyCount < proactiveMaxDaily &&
		(h.lastSpeak.IsZero() || time.Since(h.lastSpeak) >= proactiveCooldown)

	if shouldFire {
		ctx := strings.Join(h.messages, "\n")
		h.score = 0
		h.streak = 0
		h.lastSpeak = time.Now()
		h.dailyCount++
		p.mu.Unlock()
		go p.fire(api, e.GroupID, ctx)
		return
	}
	p.mu.Unlock()
}

// OnBotReplied resets heat after the bot sends a reply, preventing back-to-back proactive messages.
func (p *ProactiveManager) OnBotReplied(groupID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	h := p.heat(groupID)
	h.score = 0
	h.streak = 0
}

func (p *ProactiveManager) fire(api *bot.BotAPI, groupID int64, recentCtx string) {
	name := config.C.Bot.Name
	if name == "" {
		name = "月灵"
	}
	system := fmt.Sprintf(
		"你是%s，活泼可爱的QQ群助手。根据群聊内容自然地插一句话，不超过20字，不回答具体问题，只是自然地参与对话。",
		name,
	)
	resp, err := llm().CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: config.C.AI.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: recentCtx},
		},
		MaxTokens:   50,
		Temperature: 0.9,
	})
	if err != nil {
		log.Printf("[proactive] LLM error: %v", err)
		return
	}
	if len(resp.Choices) == 0 {
		return
	}
	reply := strings.TrimSpace(resp.Choices[0].Message.Content)
	if reply != "" {
		api.SendGroupText(groupID, reply)
	}
}
