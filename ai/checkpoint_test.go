package ai

import (
	"context"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
	openai "github.com/sashabaranov/go-openai"
)

func TestCheckpointRestoresConversationAndAttachmentsAcrossEngines(t *testing.T) {
	testdb.Init(t)
	store := db.NewConversationStore(testdb.Database)
	ctx := context.Background()
	first := NewEngine(EngineDependencies{Conversations: store})
	s := first.Sessions.Get(202, 101)
	if err := first.restoreConversation(ctx, s); err != nil {
		t.Fatal(err)
	}
	s.pushUser("把第二张保存为老王，先不要执行")
	s.pushAssistant(openai.ChatCompletionMessage{Role: "assistant", Content: "等待你的确认选图。"})
	s.images = []sessionImage{{ID: "image_2", URL: "https://example.invalid/fixture.png", Source: "当前消息第2张"}}
	s.imageSequence = 2
	s.routeCandidates = []string{"save_quotation"}
	s.historySummary = "名字已从小明改成老王，尚未保存。"
	if err := first.saveConversation(ctx, s); err != nil {
		t.Fatal(err)
	}
	second := NewEngine(EngineDependencies{Conversations: store})
	restored := second.Sessions.Get(202, 101)
	if err := second.restoreConversation(ctx, restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Messages) != 2 || len(restored.images) != 1 || restored.imageSequence != 2 || len(restored.routeCandidates) != 1 || restored.historySummary != s.historySummary || restored.lastTurnAt.IsZero() {
		t.Fatal("checkpoint omitted conversation data")
	}
	other := second.Sessions.Get(203, 101)
	if err := second.restoreConversation(ctx, other); err != nil || len(other.Messages) > 0 || len(other.images) > 0 {
		t.Fatal("group isolation failed", err)
	}
	if _, handled := second.handleLocalControl(202, 101, "重新开始"); !handled || !restored.invalidated() {
		t.Fatal("reset did not cancel restored session")
	}
	// A different engine still holding the old generation must not restore history.
	if err := first.saveConversation(ctx, s); err != nil {
		t.Fatal(err)
	}
	next := second.Sessions.Get(202, 101)
	if err := second.restoreConversation(ctx, next); err != nil || len(next.Messages) > 0 || len(next.images) > 0 {
		t.Fatal("late save resurrected conversation", err)
	}
}
func TestSessionSurvivesTenMinutePause(t *testing.T) {
	m := &SessionManager{}
	s := m.Get(1, 2)
	s.expiresAt = s.expiresAt.Add(-10 * time.Minute)
	if m.Get(1, 2) != s {
		t.Fatal("lost conversation after ten-minute pause")
	}
}
