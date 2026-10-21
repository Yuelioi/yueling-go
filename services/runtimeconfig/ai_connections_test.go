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
		bindingsCopy := map[string]string{"conversation": "chat", "vision": "codex"}
		for k, v := range bindings {
			bindingsCopy[k] = v
		}
		bindings = bindingsCopy
		values := map[string]json.RawMessage{}
		for k, v := range map[string]any{"ai.connections": c, "ai.profiles": p, "ai.bindings": bindings} {
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
	profiles[1].Capabilities = []string{"chat", "vision", "structured_output"}
	if err := save(profiles, connections, nil); err == nil {
		t.Fatal("unsupported Codex structured output capability")
	}
	if m.View().Revision != before {
		t.Fatal("failed validation changed saved state")
	}
	profiles[1].Capabilities = []string{"chat", "vision", "image_generation", "image_edit"}
	if err := save(profiles, connections, map[string]string{"image_generation": "codex", "image_edit": "codex"}); err != nil {
		t.Fatal(err)
	}
}

func TestAccountTransportCannotBeChangedInPlace(t *testing.T) {
	for _, kind := range []string{"openai", "codex"} {
		t.Run(kind, func(t *testing.T) {
			m, _ := fixture(t)
			connection := config.AIConnection{ID: "transport", Name: "共享账号", Kind: kind}
			if kind == "openai" {
				connection.BaseURL = "https://example.com/v1"
			}
			connections := append(config.Current().AI.Connections, connection)
			if _, err := update(m, "ai", "ai.connections", connections); err != nil {
				t.Fatal(err)
			}
			before := m.View().Revision
			if kind == "codex" {
				connections[len(connections)-1].Kind = "openai"
				connections[len(connections)-1].BaseURL = "https://api.deepseek.com"
			} else {
				connections[len(connections)-1].Kind = "codex"
				connections[len(connections)-1].BaseURL = ""
			}
			if _, err := update(m, "ai", "ai.connections", connections); err == nil || !strings.Contains(err.Error(), "新建账号") {
				t.Fatalf("transport switch was not rejected with a repair action: %v", err)
			}
			if m.View().Revision != before {
				t.Fatal("rejected transport switch changed saved configuration")
			}
		})
	}
}

func TestGPTModelCannotBeSavedToOfficialDeepSeekAccount(t *testing.T) {
	for _, address := range []string{"https://api.deepseek.com", "https://api.deepseek.com/v1", "https://proxy.example.com/v1"} {
		t.Run(address, func(t *testing.T) {
			m, _ := fixture(t)
			a := config.Current().AI.Clone()
			a.Connections = append(a.Connections, config.AIConnection{ID: "image-api", Name: "图片账号", Kind: "openai", BaseURL: address})
			a.Profiles = append(a.Profiles, config.AIProfile{ID: "image", Name: "codex", Model: "gpt-6.1-sol", ConnectionID: "image-api", Capabilities: []string{"image_generation", "image_edit"}})
			values := map[string]json.RawMessage{}
			for path, value := range map[string]any{"ai.connections": a.Connections, "ai.profiles": a.Profiles} {
				values[path], _ = json.Marshal(value)
			}
			before := m.View().Revision
			_, err := m.Update(Update{Revision: before, Section: "ai", Values: values})
			if strings.Contains(address, "proxy.example.com") {
				if err != nil {
					t.Fatal("custom image provider should remain configurable", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "DeepSeek") || m.View().Revision != before {
				t.Fatalf("misrouted GPT model was not rejected: %v", err)
			}
		})
	}
}

func TestExistingMisrouteCanBeRepairedWithoutChangingSearchAccount(t *testing.T) {
	m, store := fixture(t)
	a := config.Current().AI.Clone()
	a.Connections = append(a.Connections, config.AIConnection{ID: "shared", Name: "DeepSeek", Kind: "openai", BaseURL: "https://api.deepseek.com", APIKey: "existing-search-key"})
	a.Profiles = append(a.Profiles,
		config.AIProfile{ID: "image", Name: "codex", Model: "gpt-6.1-sol", ConnectionID: "shared", Capabilities: []string{"image_generation", "image_edit"}},
		config.AIProfile{ID: "search", Name: "搜索", ConnectionID: "shared", Capabilities: []string{"web_search"}},
	)
	a.Bindings = map[string]string{"search": "search", "image_edit": "image", "image_generation": "image"}
	var document map[string]any
	if err := json.Unmarshal(store.data, &document); err != nil {
		t.Fatal(err)
	}
	document["ai"] = a
	store.data, _ = json.Marshal(document)
	m = New(store, nil)
	if err := m.Initialize(); err != nil {
		t.Fatal("existing conflict must remain loadable for repair", err)
	}
	connections := append(config.Current().AI.Connections, config.AIConnection{ID: "subscription", Name: "Codex 订阅", Kind: "codex"})
	if _, err := update(m, "ai", "ai.connections", connections); err != nil {
		t.Fatal("adding an account for repair must not be blocked by unchanged old conflicts", err)
	}
	profiles := config.Current().AI.Clone().Profiles
	for i := range profiles {
		if profiles[i].ID == "image" {
			profiles[i].ConnectionID = "subscription"
		}
	}
	if _, err := update(m, "ai", "ai.profiles", profiles); err != nil {
		t.Fatal(err)
	}
	for _, feature := range []string{"image_generation", "image_edit"} {
		p, c, err := config.Current().AI.Select(feature)
		if err != nil || p.Model != "gpt-6.1-sol" || c.Kind != "codex" {
			t.Fatalf("image route not repaired: %s %+v %+v %v", feature, p, c, err)
		}
	}
	_, search, err := config.Current().AI.Select("search")
	if err != nil || search.ID != "shared" || search.APIKey != "existing-search-key" || search.Kind != "openai" {
		t.Fatal("repair changed the search connection or its credential")
	}
}
