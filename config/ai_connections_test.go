package config

import "testing"

func TestFeatureSelectionInheritanceAndExplicitOff(t *testing.T) {
	a := AIConfig{Connections: []AIConnection{{ID: "account", Kind: "codex"}}, Profiles: []AIProfile{{ID: "chat", ConnectionID: "account", Model: "chat-model", Capabilities: []string{"chat", "tools"}}, {ID: "eyes", ConnectionID: "account", Model: "vision-model", Capabilities: []string{"chat", "vision"}}}, ConversationProfile: "chat", VisionProfile: "eyes", Bindings: map[string]string{"summary": "eyes", "memory": "off"}}
	for feature, model := range map[string]string{"conversation": "chat-model", "translation": "chat-model", "vision": "vision-model", "summary": "vision-model"} {
		p, c, err := a.Select(feature)
		if err != nil || p.Model != model || c.Kind != "codex" {
			t.Fatal(feature, p, c, err)
		}
	}
	for _, feature := range []string{"memory", "image_generation", "unknown"} {
		if _, _, err := a.Select(feature); err == nil {
			t.Fatal(feature)
		}
	}
	a.Bindings["conversation"] = "eyes"
	if _, _, err := a.Select("conversation"); err == nil {
		t.Fatal("text model accepted for tools")
	}
	clone := a.Clone()
	clone.Profiles[0].Capabilities[0] = "bad"
	clone.Bindings["summary"] = "off"
	clone.Connections[0].Kind = "bad"
	if a.Profiles[0].Capabilities[0] != "chat" || a.Bindings["summary"] != "eyes" || a.Connections[0].Kind != "codex" {
		t.Fatal("mutable snapshot")
	}
}

func TestImportConnectionsPreservesLegacySelection(t *testing.T) {
	a := AIConfig{DeepSeekKey: "secret", BaseURL: "https://example.com/v1", Model: "chat", VL: VLConfig{Key: "vision-secret", BaseURL: "https://example.com/v1", Model: "vision"}}
	a.ImportLegacyProfiles()
	if !a.ImportConnections() {
		t.Fatal("migration not applied")
	}
	if a.Profiles[0].APIKey != "" || a.Profiles[0].BaseURL != "" {
		t.Fatal("credentials duplicated")
	}
	a = a.Resolved()
	if a.DeepSeekKey != "secret" || a.VL.Key != "vision-secret" {
		t.Fatal("legacy compatibility lost")
	}
	if a.ImportConnections() {
		t.Fatal("migration repeated")
	}
}
