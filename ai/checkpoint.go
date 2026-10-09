package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

type ConversationStore interface {
	Load(context.Context, int64, int64) (int64, []byte, error)
	Save(context.Context, int64, int64, int64, []byte) error
	Reset(context.Context, int64, int64) error
}
type conversationCheckpoint struct {
	Task          *SummaryTask                   `json:"summary_task,omitempty"`
	Candidates    []string                       `json:"candidates"`
	Messages      []openai.ChatCompletionMessage `json:"messages"`
	Summary       string                         `json:"summary"`
	Images        []sessionImage                 `json:"images"`
	ImageSequence uint64                         `json:"image_sequence"`
	UpdatedAt     time.Time                      `json:"updated_at"`
}

func (e *Engine) restoreConversation(ctx context.Context, s *Session) error {
	if e.Conversations == nil || s.restored {
		return nil
	}
	generation, data, err := e.Conversations.Load(ctx, s.GroupID, s.UserID)
	if err != nil {
		return err
	}
	if s.invalidated() {
		return context.Canceled
	}
	var saved conversationCheckpoint
	if len(data) > 0 {
		if len(data) > 512*1024 {
			return fmt.Errorf("conversation checkpoint too large")
		}
		if err := json.Unmarshal(data, &saved); err != nil {
			return err
		}
	}
	s.generation = generation
	s.restored = true
	s.Messages = saved.Messages
	s.historySummary = saved.Summary
	s.images = saved.Images
	s.imageSequence = saved.ImageSequence
	s.lastTurnAt = saved.UpdatedAt
	s.routeCandidates = saved.Candidates
	s.SummaryTask = saved.Task
	return nil
}
func (e *Engine) saveConversation(ctx context.Context, s *Session) error {
	if e.Conversations == nil || !s.restored || s.invalidated() {
		return nil
	}
	s.lastTurnAt = time.Now()
	// Persist complete recent protocol turns, never partial calls or transient locks.
	saved := conversationCheckpoint{Task: s.SummaryTask, Candidates: s.routeCandidates, Messages: s.Messages, Summary: s.historySummary, Images: s.images, ImageSequence: s.imageSequence, UpdatedAt: s.lastTurnAt}
	data, err := json.Marshal(saved)
	if err != nil {
		return err
	}
	if len(data) > 512*1024 {
		return fmt.Errorf("conversation checkpoint too large")
	}
	return e.Conversations.Save(ctx, s.GroupID, s.UserID, s.generation, data)
}

// Serialize Get with durable reset. The generation check also rejects saves
// already in flight when the session is canceled.
func (e *Engine) resetConversation(groupID, userID int64) error {
	e.Sessions.mu.Lock()
	defer e.Sessions.mu.Unlock()
	key := sessionKey(groupID, userID)
	if s := e.Sessions.sessions[key]; s != nil {
		s.invalidate()
	}
	delete(e.Sessions.sessions, key)
	if e.Conversations == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return e.Conversations.Reset(ctx, groupID, userID)
}
