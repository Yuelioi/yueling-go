package db

import (
	"context"
	"testing"
)

func TestConversationCheckpointResetAndIsolation(t *testing.T) {
	initPostgresForTest(t)
	s := NewConversationStore(DB)
	ctx := context.Background()
	generation, _, err := s.Load(ctx, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Save(ctx, 1, 2, generation, []byte(`{"summary":"fictional task"}`)); err != nil {
		t.Fatal(err)
	}
	_, data, err := s.Load(ctx, 1, 2)
	if err != nil || len(data) == 0 {
		t.Fatal("restart lost history", err)
	}
	_, other, err := s.Load(ctx, 2, 2)
	if err != nil || len(other) != 0 {
		t.Fatal("cross-group history", err)
	}
	if err = s.Reset(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err = s.Save(ctx, 1, 2, generation, []byte(`{"summary":"late completion"}`)); err != nil {
		t.Fatal(err)
	}
	next, data, err := s.Load(ctx, 1, 2)
	if err != nil || next <= generation || len(data) != 0 {
		t.Fatal("late save resurrected reset", err)
	}
	if err = s.Save(ctx, 1, 2, next, []byte(`{"summary":"new task"}`)); err != nil {
		t.Fatal(err)
	}
	if err = DB.Exec("UPDATE ai_conversations SET expires_at=now()-interval '1 second' WHERE group_id=1 AND user_id=2").Error; err != nil {
		t.Fatal(err)
	}
	_, data, err = s.Load(ctx, 1, 2)
	if err != nil || len(data) != 0 {
		t.Fatal("expired checkpoint restored", err)
	}
}
