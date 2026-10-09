package ai

import (
	"encoding/json"
	"github.com/Yuelioi/yueling-go/bot"
	"strings"
	"time"
)

const proactiveContextTTL = 5 * time.Minute
const proactiveContextMessages = 8

type proactiveMessage struct {
	Speaker string    `json:"speaker"`
	Text    string    `json:"text"`
	At      time.Time `json:"-"`
}

func (h *groupHeat) expireMessages(now time.Time) {
	first := 0
	for first < len(h.messages) && now.Sub(h.messages[first].At) > proactiveContextTTL {
		first++
	}
	if first > 0 {
		h.messages = append([]proactiveMessage(nil), h.messages[first:]...)
		if len(h.messages) == 0 {
			h.score = 0
			h.streak = 0
		}
	}
}
func (h *groupHeat) appendMessage(e *bot.GroupMessageEvent, text string, now time.Time) {
	name := strings.TrimSpace(e.Sender.Card)
	if name == "" {
		name = strings.TrimSpace(e.Sender.Nickname)
	}
	if name == "" {
		name = "一位群友"
	}
	h.messages = append(h.messages, proactiveMessage{Speaker: clipProactive(name, 32), Text: clipProactive(text, 400), At: now})
	if len(h.messages) > proactiveContextMessages {
		h.messages = append([]proactiveMessage(nil), h.messages[len(h.messages)-proactiveContextMessages:]...)
	}
}
func clipProactive(text string, limit int) string {
	r := []rune(text)
	if len(r) > limit {
		return string(r[:limit]) + "…"
	}
	return text
}
func proactiveContext(messages []proactiveMessage) string {
	if len(messages) == 0 {
		return "{}"
	}
	latest := len(messages) - 1
	recent := max(0, latest-2)
	payload, _ := json.Marshal(struct {
		Background []proactiveMessage `json:"background_context,omitempty"`
		Recent     []proactiveMessage `json:"recent_context,omitempty"`
		Latest     proactiveMessage   `json:"latest_message"`
	}{messages[:recent], messages[recent:latest], messages[latest]})
	return string(payload)
}
