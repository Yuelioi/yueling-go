package runtimeconfig

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

func TestConnectionsBindingsAndReferencedDeletion(t *testing.T) {
	m, _ := fixture(t)
	connections := []config.AIConnection{{ID: "api", Name: "API", Kind: "openai", BaseURL: "https://example.com/v1", APIKey: "connection-secret"}, {ID: "subscription", Name: "Codex", Kind: "codex"}}
	profiles := []config.AIProfile{{ID: "chat", Name: "Chat", Model: "m", ConnectionID: "api", Capabilities: []string{"chat", "tools"}}, {ID: "codex", Name: "Codex", Model: "m2", ConnectionID: "subscription", Capabilities: []string{"chat", "tools", "vision"}}}
	save := func(p []config.AIProfile, c []config.AIConnection, bindings map[string]string) error {
		values := map[string]json.RawMessage{}
		for k, v := range map[string]any{"ai.connections": c, "ai.profiles": p, "ai.conversation_profile": "chat", "ai.vision_profile": "codex", "ai.bindings": bindings} {
			values[k], _ = json.Marshal(v)
		}
		_, err := m.Update(Update{Revision: m.View().Revision, Section: "ai", Values: values})
		return err
	}
	if err := save(profiles, connections, map[string]string{"summary": "codex", "memory": "off"}); err != nil {
		t.Fatal(err)
	}
	view, _ := json.Marshal(m.View())
	if strings.Contains(string(view), "connection-secret") {
		t.Fatal("credential leaked")
	}
	connections[0].APIKey = ""
	if err := save(profiles, connections, map[string]string{"summary": "codex"}); err != nil {
		t.Fatal(err)
	}
	p, c, err := config.Current().AI.Select("conversation")
	if err != nil || p.ID != "chat" || c.APIKey != "connection-secret" {
		t.Fatal(p, c, err)
	}
	before := m.View().Revision
	if err := save(profiles, connections[:1], nil); err == nil {
		t.Fatal("deleted referenced account")
	}
	profiles[1].Capabilities = []string{"chat"}
	if err := save(profiles, connections, nil); err == nil {
		t.Fatal("removed selected vision capability")
	}
	profiles[1].Capabilities = []string{"chat", "vision", "image_generation"}
	if err := save(profiles, connections, nil); err == nil {
		t.Fatal("unsupported Codex image capability")
	}
	if m.View().Revision != before {
		t.Fatal("failed validation changed saved state")
	}
}
