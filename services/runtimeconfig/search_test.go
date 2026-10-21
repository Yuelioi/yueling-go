package runtimeconfig

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

func TestSearchProfilePersistsWithoutInferenceModel(t *testing.T) {
	m, store := fixture(t)
	a := config.Current().AI
	a.Connections = append(a.Connections, config.AIConnection{ID: "search-api", Name: "搜索账号", Kind: "openai", BaseURL: "https://api.deepseek.com/v1", APIKey: "search-private-key"})
	a.Profiles = append(a.Profiles, config.AIProfile{ID: "search", Name: "独立搜索", ConnectionID: "search-api", Capabilities: []string{"web_search"}})
	a.Bindings = map[string]string{"search": "search"}
	values := map[string]json.RawMessage{}
	for path, value := range map[string]any{"ai.connections": a.Connections, "ai.profiles": a.Profiles, "ai.bindings": a.Bindings} {
		values[path], _ = json.Marshal(value)
	}
	if _, err := m.Update(Update{Revision: m.View().Revision, Section: "ai", Values: values}); err != nil {
		t.Fatal(err)
	}
	if !config.Current().AI.Available("search") {
		t.Fatal("selected search unavailable")
	}
	view, _ := json.Marshal(m.View())
	if strings.Contains(string(view), "search-private-key") || strings.Contains(string(view), "tools.tavily_key") {
		t.Fatal("obsolete or secret field exposed")
	}
	if err := New(store, nil).Initialize(); err != nil {
		t.Fatal(err)
	}
	if !config.Current().AI.Available("search") {
		t.Fatal("selection lost after restart")
	}
	bad := config.Current()
	bad.AI.Connections[len(bad.AI.Connections)-1].Kind = "codex"
	bad.AI.Connections[len(bad.AI.Connections)-1].BaseURL = ""
	bad.AI.Connections[len(bad.AI.Connections)-1].APIKey = ""
	if err := Validate(bad); err == nil {
		t.Fatal("Codex accepted for DeepSeek search")
	}
}

func TestRetiredZssmBindingsMigrateBeforeValidation(t *testing.T) {
	_, store := fixture(t)
	var document map[string]any
	if err := json.Unmarshal(store.data, &document); err != nil {
		t.Fatal(err)
	}
	document["ai"].(map[string]any)["bindings"] = map[string]string{"zssm": "legacy-chat"}
	document["plugins"].(map[string]any)["disabled"] = []int{35}
	store.data, _ = json.Marshal(document)
	if err := New(store, nil).Initialize(); err == nil {
		t.Fatal("retired binding accepted without explicit migration")
	}
}
