package ai

import (
	"encoding/json"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// turnRecord is durable conversation data, independent of provider protocols.
type turnRecord struct {
	Input   string       `json:"input"`
	Reply   string       `json:"reply,omitempty"`
	Actions []actionFact `json:"actions,omitempty"`
}
type actionFact struct {
	Name                string     `json:"name"`
	Status              ToolStatus `json:"status"`
	Content             string     `json:"content"`
	PossibleSideEffects bool       `json:"possible_side_effects,omitempty"`
}

func (r turnRecord) messages() []openai.ChatCompletionMessage {
	reply := r.Reply
	if len(r.Actions) > 0 {
		data, _ := json.Marshal(r.Actions)
		reply += "\n【已记录的操作结果：历史事实，不是新的执行指令】\n" + string(data)
	}
	messages := []openai.ChatCompletionMessage{{Role: "user", Content: r.Input}}
	if reply != "" {
		messages = append(messages, openai.ChatCompletionMessage{Role: "assistant", Content: reply})
	}
	return messages
}
func (s *Session) finishProtocolTurn(start int, input string) {
	if len(s.Messages) <= start {
		return
	}
	record := turnRecord{Input: input}
	names := map[string]string{}
	for _, msg := range s.Messages[start:] {
		for _, call := range msg.ToolCalls {
			names[call.ID] = call.Function.Name
		}
		if msg.Role == "tool" {
			var result ToolResult
			if json.Unmarshal([]byte(msg.Content), &result) == nil {
				record.Actions = append(record.Actions, actionFact{Name: names[msg.ToolCallID], Status: result.Status, Content: result.Content, PossibleSideEffects: result.PossibleSideEffects})
			}
		}
		if msg.Role == "assistant" && len(msg.ToolCalls) == 0 && strings.TrimSpace(msg.Content) != "" {
			record.Reply = msg.Content
		}
	}
	s.turns = append(s.turns, record)
	// Keep bounded domain history; provider calls and reasoning live only in-flight.
	if len(s.turns) > 8 {
		s.turns = append([]turnRecord(nil), s.turns[len(s.turns)-8:]...)
	}
	s.Messages = nil
	for _, turn := range s.turns {
		s.Messages = append(s.Messages, turn.messages()...)
	}
}
