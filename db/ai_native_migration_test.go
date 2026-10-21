package db

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
	"gorm.io/gorm"
)

func TestNativeRuntimeMigrationPreservesEffectiveBindingsAndUnrelatedData(t *testing.T) {
	initPostgresForTest(t)
	old := `{"bot":{"name":"sentinel"},"image":{"entry":[{"path":"keep"}]},"ai":{"conversation_profile":"chat","vision_profile":"eyes","profiles":[{"id":"chat","name":"Chat","model":"chat","api_key":"private-chat","base_url":"https://example.invalid/v1"},{"id":"eyes","name":"Eyes","model":"vision","vision":true,"api_key":"private-vision","base_url":"https://example.invalid/v1"}],"bindings":{"memory":"off","summary":"eyes","conversation":"off","zssm":"retired"},"max_tokens":4096}}`
	if err := DB.Exec("INSERT INTO runtime_configuration(id,payload,revision) VALUES (1,?::jsonb,7)", old).Error; err != nil {
		t.Fatal(err)
	}
	store := NewConversationStore(DB)
	generation, _, err := store.Load(context.Background(), 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), 1, 0, generation, []byte(`{"messages":[{"content":"old-wrapper"}]}`)); err != nil {
		t.Fatal(err)
	}
	if err := DB.Exec("DELETE FROM schema_migrations WHERE version=19").Error; err != nil {
		t.Fatal(err)
	}
	if err := runMigrations(DB); err != nil {
		t.Fatal(err)
	}
	var row struct {
		Payload  string
		Revision int64
	}
	if err := DB.Raw("SELECT payload::text,revision FROM runtime_configuration WHERE id=1").Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Bot   struct{ Name string }
		Image json.RawMessage
		AI    config.AIConfig
	}
	if err := json.Unmarshal([]byte(row.Payload), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Bot.Name != "sentinel" || row.Revision != 8 || len(doc.Image) == 0 {
		t.Fatal("non-AI config changed")
	}
	if len(doc.AI.Bindings) != len(config.AIFeatures()) || doc.AI.Available("conversation") || doc.AI.Available("memory") || doc.AI.Available("image_edit") {
		t.Fatal("migration changed disabled routes")
	}
	p, c, err := doc.AI.Select("summary")
	if err != nil || p.ID != "eyes" || c.APIKey != "private-vision" {
		t.Fatal("explicit route or credential lost", err)
	}
	p, c, err = doc.AI.Select("translation")
	if err != nil || p.ID != "chat" || c.APIKey != "private-chat" {
		t.Fatal("effective inherited route lost", err)
	}
	if err := runMigrations(DB); err != nil {
		t.Fatal(err)
	}
	var revision int64
	DB.Raw("SELECT revision FROM runtime_configuration WHERE id=1").Scan(&revision)
	if revision != row.Revision {
		t.Fatal("migration repeated on restart")
	}
	for _, p := range doc.AI.Profiles {
		if p.APIKey != "" || p.BaseURL != "" || len(p.Capabilities) == 0 || p.ConnectionID == "" {
			t.Fatal("noncanonical profile")
		}
	}
	next, data, err := store.Load(context.Background(), 1, 0)
	if err != nil || next <= generation || len(data) != 0 {
		t.Fatal("old checkpoint not invalidated", err)
	}
	_ = store.Save(context.Background(), 1, 0, generation, []byte(`{"late":true}`))
	_, data, err = store.Load(context.Background(), 1, 0)
	if err != nil || len(data) != 0 {
		t.Fatal("migration resurrected old generation", err)
	}
}

func TestNativeRuntimeMigrationRejectsDanglingRoutesAtomically(t *testing.T) {
	initPostgresForTest(t)
	old := `{"ai":{"profiles":[],"connections":[],"bindings":{"conversation":"missing"}}}`
	if err := DB.Exec("INSERT INTO runtime_configuration(id,payload,revision) VALUES (1,?::jsonb,3)", old).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Exec("INSERT INTO ai_conversations(group_id,user_id,generation,payload,expires_at) VALUES (1,0,5,'{\"old\":true}',now()+interval '1 hour')").Error; err != nil {
		t.Fatal(err)
	}
	body, _ := migrationFiles.ReadFile("migrations/019_ai_native_runtime.sql")
	if err := DB.Transaction(func(tx *gorm.DB) error { return tx.Exec(string(body)).Error }); err == nil {
		t.Fatal("dangling route accepted")
	}
	var revision, generation int64
	DB.Raw("SELECT revision FROM runtime_configuration WHERE id=1").Scan(&revision)
	DB.Raw("SELECT generation FROM ai_conversations WHERE group_id=1").Scan(&generation)
	if revision != 3 || generation != 5 {
		t.Fatal("failed migration was partially committed")
	}
}
