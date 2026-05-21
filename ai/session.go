package ai

import (
	"fmt"
	"sync"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const (
	sessionTTL = 5 * time.Minute
	maxSteps   = 5
	maxToolUse = 2 // max calls per tool per turn
)

// Session holds one user's in-flight conversation state.
type Session struct {
	UserID    int64
	GroupID   int64
	Messages  []openai.ChatCompletionMessage
	ToolState map[string]any // structured side-data, NOT fed to LLM
	UsedTools map[string]int // tool name → call count this turn
	StepCount int
	LastInput string
	expiresAt time.Time
}

func newSession(userID, groupID int64) *Session {
	return &Session{
		UserID:    userID,
		GroupID:   groupID,
		ToolState: map[string]any{},
		UsedTools: map[string]int{},
		expiresAt: time.Now().Add(sessionTTL),
	}
}

func (s *Session) touch()                   { s.expiresAt = time.Now().Add(sessionTTL) }
func (s *Session) expired() bool            { return time.Now().After(s.expiresAt) }
func (s *Session) canCall(name string) bool { return s.UsedTools[name] < maxToolUse }

func (s *Session) pushUser(text string) {
	s.Messages = append(s.Messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: text,
	})
}

func (s *Session) pushAssistant(msg openai.ChatCompletionMessage) {
	s.Messages = append(s.Messages, msg)
}

func (s *Session) pushToolResult(callID, result string) {
	s.Messages = append(s.Messages, openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: callID,
		Content:    result,
	})
}

// resetTurn clears per-turn state while keeping the conversation history.
func (s *Session) resetTurn() {
	s.UsedTools = map[string]int{}
	s.StepCount = 0
}

// ---- Manager ----

// SessionManager stores active sessions and evicts expired ones.
type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

var Sessions = &SessionManager{sessions: map[string]*Session{}}

func sessionKey(groupID, userID int64) string {
	return fmt.Sprintf("%d:%d", groupID, userID)
}

// Get returns the existing session or creates a fresh one.
func (m *SessionManager) Get(groupID, userID int64) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := sessionKey(groupID, userID)
	s, ok := m.sessions[key]
	if !ok || s.expired() {
		s = newSession(userID, groupID)
		m.sessions[key] = s
	}
	s.touch()
	return s
}

// Evict removes all expired sessions. Intended for periodic cleanup.
func (m *SessionManager) Evict() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, s := range m.sessions {
		if s.expired() {
			delete(m.sessions, k)
		}
	}
}
