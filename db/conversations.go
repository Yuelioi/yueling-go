package db

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// ConversationStore persists bounded conversation checkpoints. Reset advances a
// generation so late completions cannot resurrect a cleared conversation.
type ConversationStore struct{ database *gorm.DB }

func NewConversationStore(database *gorm.DB) *ConversationStore {
	return &ConversationStore{database: database}
}
func (s *ConversationStore) Load(ctx context.Context, group, user int64) (int64, []byte, error) {
	tx := s.database.WithContext(ctx)
	if err := tx.Exec("INSERT INTO ai_conversations(group_id,user_id) VALUES (?,?) ON CONFLICT DO NOTHING", group, user).Error; err != nil {
		return 0, nil, err
	}
	var row struct {
		Generation int64
		Payload    string
		ExpiresAt  time.Time
	}
	err := tx.Raw("SELECT generation,payload::text,expires_at FROM ai_conversations WHERE group_id=? AND user_id=?", group, user).Scan(&row).Error
	if !row.ExpiresAt.After(time.Now()) {
		return row.Generation, nil, err
	}
	return row.Generation, []byte(row.Payload), err
}
func (s *ConversationStore) Save(ctx context.Context, group, user, generation int64, payload []byte) error {
	return s.database.WithContext(ctx).Exec("UPDATE ai_conversations SET payload=?::jsonb,expires_at=now()+interval '24 hours' WHERE group_id=? AND user_id=? AND generation=?", string(payload), group, user, generation).Error
}
func (s *ConversationStore) Reset(ctx context.Context, group, user int64) error {
	return s.database.WithContext(ctx).Exec("INSERT INTO ai_conversations(group_id,user_id,generation) VALUES (?,?,1) ON CONFLICT (group_id,user_id) DO UPDATE SET generation=ai_conversations.generation+1,payload='{}',expires_at=now()", group, user).Error
}
