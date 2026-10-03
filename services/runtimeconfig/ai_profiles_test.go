package runtimeconfig

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

func profileUpdate(m *Manager, profiles any, chat, vision string) (bool, error) {
	values := map[string]json.RawMessage{}
	for path, value := range map[string]any{"ai.profiles": profiles, "ai.conversation_profile": chat, "ai.vision_profile": vision} {
		raw, _ := json.Marshal(value)
		values[path] = raw
	}
	return m.Update(Update{Revision: m.View().Revision, Section: "ai", Values: values})
}

func TestProfilesSharedSeparateSelectionAndSnapshotIsolation(t *testing.T) {
	m, s := fixture(t)
	profiles := []config.AIProfile{
		{ID: "chat", Name: "聊天", BaseURL: "https://chat.example/v1", Model: "chat-model", APIKey: "chat-secret"},
		{ID: "both", Name: "多模态", BaseURL: "https://vision.example/v1", Model: "vision-model", APIKey: "vision-secret", Vision: true},
	}
	if reload, err := profileUpdate(m, profiles, "both", "both"); err != nil || reload {
		t.Fatalf("shared roles: %v %v", reload, err)
	}
	c := config.Current()
	if c.AI.DeepSeekKey != "vision-secret" || c.AI.BaseURL != c.AI.VL.BaseURL || c.AI.Model != c.AI.VL.Model || c.AI.VL.Key != c.AI.DeepSeekKey {
		t.Fatal("shared profile not used for both roles")
	}
	c.AI.Profiles[1].APIKey = "external-mutation"
	if config.Current().AI.Profiles[1].APIKey != "vision-secret" {
		t.Fatal("snapshot is mutable")
	}
	profiles[0].APIKey = ""
	profiles[1].APIKey = ""
	if _, err := profileUpdate(m, profiles, "chat", "both"); err != nil {
		t.Fatal(err)
	}
	c = config.Current()
	if c.AI.Model != "chat-model" || c.AI.DeepSeekKey != "chat-secret" || c.AI.VL.Model != "vision-model" || c.AI.VL.Key != "vision-secret" {
		t.Fatal("separate profiles lost credential or role")
	}
	var stored map[string]map[string]json.RawMessage
	if err := json.Unmarshal(s.data, &stored); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"deepseek_key", "base_url", "model", "vl"} {
		if _, ok := stored["ai"][key]; ok {
			t.Fatal("legacy aliases persisted")
		}
	}
	next := New(s, nil)
	rev := s.revision
	if err := next.Initialize(); err != nil {
		t.Fatal(err)
	}
	if s.revision != rev || config.Current().AI.DeepSeekKey != "chat-secret" {
		t.Fatal("restart was not idempotent")
	}
	if _, err := profileUpdate(m, []config.AIProfile{}, "", ""); err != nil {
		t.Fatal(err)
	}
	if c = config.Current(); c.AI.Model != "" || c.AI.DeepSeekKey != "" || c.AI.VL.Key != "" || c.AI.Profiles == nil {
		t.Fatal("empty profiles fell back to old models")
	}
}

func TestProfileSecretsFollowIDsAndClearExplicitly(t *testing.T) {
	m, _ := fixture(t)
	initial := []config.AIProfile{
		{ID: "a", Name: "A", BaseURL: "https://example.com/v1", Model: "a", APIKey: "private-a"},
		{ID: "b", Name: "B", BaseURL: "https://example.com/v1", Model: "b", APIKey: "private-b", Vision: true},
	}
	if _, err := profileUpdate(m, initial, "a", "b"); err != nil {
		t.Fatal(err)
	}
	view, _ := json.Marshal(m.View())
	if strings.Contains(string(view), "private-a") || strings.Contains(string(view), "private-b") {
		t.Fatal("nested credential exposed")
	}
	swapped := []config.AIProfile{initial[1], initial[0]}
	for i := range swapped {
		swapped[i].APIKey = ""
	}
	if _, err := profileUpdate(m, swapped, "b", "b"); err != nil {
		t.Fatal(err)
	}
	c := config.Current()
	if c.AI.Profiles[0].APIKey != "private-b" || c.AI.Profiles[1].APIKey != "private-a" {
		t.Fatal("keys followed indexes")
	}
	inputs := []map[string]any{
		{"id": "b", "name": "B", "base_url": "https://example.com/v1", "model": "b", "vision": true, "api_key": "", "clear_key": true},
		{"id": "new", "name": "New", "base_url": "https://example.com/v1", "model": "new", "api_key": ""},
	}
	if _, err := profileUpdate(m, inputs, "b", "b"); err != nil {
		t.Fatal(err)
	}
	c = config.Current()
	if c.AI.DeepSeekKey != "" || c.AI.VL.Key != "" || c.AI.Profiles[1].APIKey != "" {
		t.Fatal("explicit clear failed or new ID inherited another key")
	}
}

func TestInvalidProfilesConflictAndPersistenceLeaveRuntimeUntouched(t *testing.T) {
	m, s := fixture(t)
	valid := config.AIProfile{ID: "valid", Name: "valid", Model: "m", BaseURL: "https://example.com/v1", Vision: true, APIKey: "private"}
	if _, err := profileUpdate(m, []config.AIProfile{valid}, "valid", "valid"); err != nil {
		t.Fatal(err)
	}
	baseline := config.Current()
	revision := m.View().Revision
	data := string(s.data)
	textOnly := valid
	textOnly.Vision = false
	badURL := valid
	badURL.BaseURL = "https://user:private@example.com/v1"
	unnamed := valid
	unnamed.Name = ""
	tests := []struct {
		profiles     any
		chat, vision string
	}{
		{[]config.AIProfile{valid, valid}, "valid", "valid"},
		{[]config.AIProfile{textOnly}, "valid", "valid"},
		{[]config.AIProfile{valid}, "missing", "valid"},
		{[]config.AIProfile{}, "valid", "valid"},
		{[]config.AIProfile{badURL}, "valid", "valid"},
		{[]config.AIProfile{unnamed}, "valid", "valid"},
		{nil, "", ""},
		{[]map[string]any{{"id": "valid", "unknown": "private"}}, "valid", "valid"},
		{[]map[string]any{{"id": "valid", "api_key": "private", "clear_key": true}}, "valid", "valid"},
	}
	for i, tt := range tests {
		if _, err := profileUpdate(m, tt.profiles, tt.chat, tt.vision); err == nil {
			t.Fatalf("invalid profile accepted case %d", i)
		}
		if !reflect.DeepEqual(config.Current(), baseline) || m.View().Revision != revision || string(s.data) != data {
			t.Fatal("invalid update changed state")
		}
	}
	s.fail = true
	if _, err := profileUpdate(m, []config.AIProfile{valid}, "valid", ""); !errors.Is(err, ErrPersistence) {
		t.Fatal("persistence failure not reported")
	}
	if !reflect.DeepEqual(config.Current(), baseline) || m.View().Revision != revision {
		t.Fatal("failed persistence published")
	}
	s.fail = false
	if _, err := m.Update(Update{Revision: revision - 1, Section: "ai"}); !errors.Is(err, ErrConflict) {
		t.Fatal("stale update accepted")
	}
}

func TestLegacyProfileMigrationDatabaseWinsAndFailureDoesNotPublish(t *testing.T) {
	_, s := fixture(t)
	legacy := config.Current()
	legacy.AI.Profiles = nil
	legacy.AI.ConversationProfile = ""
	legacy.AI.VisionProfile = ""
	legacy.AI.DeepSeekKey = "database-chat-key"
	legacy.AI.Model = "db-model"
	legacy.AI.VL.Key = "database-vision-key"
	data, err := payload(legacy)
	if err != nil {
		t.Fatal(err)
	}
	s.data = data
	boot := legacy
	boot.AI.DeepSeekKey = "bootstrap-key"
	boot.AI.Model = "bootstrap-model"
	boot.AI.ImportLegacyProfiles()
	config.Publish(boot)
	before := config.Current()
	revision := s.revision
	s.fail = true
	if err := New(s, nil).Initialize(); !errors.Is(err, ErrPersistence) {
		t.Fatal("migration failure not reported")
	}
	if !reflect.DeepEqual(config.Current(), before) || s.revision != revision {
		t.Fatal("failed migration published")
	}
	s.fail = false
	m := New(s, nil)
	if err := m.Initialize(); err != nil {
		t.Fatal(err)
	}
	c := config.Current()
	if c.AI.DeepSeekKey != "database-chat-key" || c.AI.Model != "db-model" || c.AI.VL.Key != "database-vision-key" || len(c.AI.Profiles) != 2 || c.AI.Profiles[0].Vision || !c.AI.Profiles[1].Vision {
		t.Fatal("migration lost database settings or capability")
	}
	if s.revision != revision+1 {
		t.Fatal("migration not saved once")
	}
	if err := New(s, nil).Initialize(); err != nil || s.revision != revision+1 {
		t.Fatal("migration repeated")
	}
}
