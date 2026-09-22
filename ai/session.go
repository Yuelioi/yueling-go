package ai

import (
	"context"
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
	ExecutedTools map[string]ToolResult
	gate          chan struct{}
	gateOnce      sync.Once
	lifetime      context.Context
	cancelLife    context.CancelFunc
	UserID        int64
	GroupID       int64
	Messages      []openai.ChatCompletionMessage
	ToolState     map[string]any // structured side-data, NOT fed to LLM
	UsedTools     map[string]int // tool name → call count this turn
	StepCount     int
	LastInput     string
	SummaryTask   *SummaryTask // Business task context, separate from provider protocol messages.
	expiresAt     time.Time
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
	s.ToolState = map[string]any{}
	s.UsedTools = map[string]int{}
	s.ExecutedTools = map[string]ToolResult{}
	s.StepCount = 0
}

// ---- Manager ----

// SessionManager stores active sessions and evicts expired ones.
type SessionManager struct {
	mu        sync.Mutex
	sessions  map[string]*Session
	requests  map[messageKey]*messageReceipt
	lastEvict time.Time
}

var Sessions = &SessionManager{sessions: map[string]*Session{}}

func sessionKey(groupID, userID int64) string {
	return fmt.Sprintf("%d:%d", groupID, userID)
}

// Get returns the existing session or creates a fresh one.
func (m *SessionManager) Get(groupID, userID int64) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	if m.lastEvict.IsZero() || now.Sub(m.lastEvict) >= sessionTTL {
		m.evictExpiredLocked(now)
		m.lastEvict = now
	}

	if m.sessions == nil {
		m.sessions = map[string]*Session{}
	}
	key := sessionKey(groupID, userID)
	s, ok := m.sessions[key]
	if !ok || now.After(s.expiresAt) {
		if ok {
			s.invalidate()
		}
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
	now := time.Now()
	m.evictExpiredLocked(now)
	m.lastEvict = now
}

func (m *SessionManager) evictExpiredLocked(now time.Time) {
	for key, session := range m.sessions {
		if now.After(session.expiresAt) {
			session.invalidate()
			delete(m.sessions, key)
		}
	}
}

// Delete cancels this conversation's active and queued work before removing it.
// A subsequent Get returns a separate lifetime; cancellation never crosses groups.
func (m *SessionManager) Delete(groupID, userID int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := sessionKey(groupID, userID)
	session, existed := m.sessions[key]
	if existed {
		session.invalidate()
	}
	delete(m.sessions, key)
	return existed
}

// trimHistory drops whole turns, never separating a call from its tool results.
func (s *Session) trimHistory() {
	const maxHistoryChars = 24000
	size := 0
	start := len(s.Messages)
	turns := 0
	for i := len(s.Messages) - 1; i >= 0; i-- {
		size += len([]rune(s.Messages[i].Content)) + len([]rune(s.Messages[i].ReasoningContent))
		for _, call := range s.Messages[i].ToolCalls {
			size += len([]rune(call.Function.Arguments))
		}
		if s.Messages[i].Role == openai.ChatMessageRoleUser {
			turns++
			if size > maxHistoryChars || turns > 8 {
				break
			}
			start = i
		}
	}
	s.Messages = append([]openai.ChatCompletionMessage(nil), s.Messages[start:]...)
}

func (s *Session) initializeRuntime() {
	s.gateOnce.Do(func() {
		s.gate = make(chan struct{}, 1)
		s.lifetime, s.cancelLife = context.WithCancel(context.Background())
	})
}

func (s *Session) invalidate() {
	s.initializeRuntime()
	s.cancelLife()
}

func (s *Session) invalidated() bool {
	s.initializeRuntime()
	return s.lifetime.Err() != nil
}

// turnContext binds an individual request to both caller cancellation and the
// conversation lifetime. Its cleanup detaches the callback after normal use.
func (s *Session) turnContext(parent context.Context) (context.Context, context.CancelFunc) {
	s.initializeRuntime()
	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(s.lifetime, cancel)
	if s.invalidated() {
		cancel()
	}
	return ctx, func() {
		stop()
		cancel()
	}
}

func (s *Session) acquire(ctx context.Context) error {
	s.initializeRuntime()
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.invalidated() {
		return context.Canceled
	}
	select {
	case s.gate <- struct{}{}:
		// A release and reset can make both select branches ready together.
		if err := ctx.Err(); err != nil {
			s.release()
			return err
		}
		if s.invalidated() {
			s.release()
			return context.Canceled
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.lifetime.Done():
		return context.Canceled
	}
}
func (s *Session) release() { <-s.gate }
