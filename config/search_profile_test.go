package config

import "testing"

func TestSearchRequiresAnExplicitIndependentProfile(t *testing.T) {
	a := AIConfig{
		Connections: []AIConnection{{ID: "chat-api", Kind: "openai", BaseURL: "https://chat.example/v1", APIKey: "chat-key"}, {ID: "search-api", Kind: "openai", BaseURL: "https://api.deepseek.com/v1", APIKey: "search-key"}},
		Profiles:    []AIProfile{{ID: "chat", ConnectionID: "chat-api", Model: "chat-model", Capabilities: []string{"chat", "tools"}}, {ID: "search", ConnectionID: "search-api", Capabilities: []string{"web_search"}}},
		Bindings:    map[string]string{"conversation": "chat"},
	}
	if a.Available("search") {
		t.Fatal("search silently inherited conversation")
	}
	a.Bindings = map[string]string{"search": "search"}
	p, c, err := a.Select("search")
	if err != nil || p.Model != "" || c.APIKey != "search-key" || !a.Available("search") {
		t.Fatalf("%+v %+v %v", p, c, err)
	}
	a.Bindings["conversation"] = "different-chat"
	_, c, err = a.Select("search")
	if err != nil || c.APIKey != "search-key" {
		t.Fatal("chat change changed search", err)
	}
	for _, id := range []string{"off", "missing", "chat"} {
		a.Bindings["search"] = id
		if a.Available("search") {
			t.Fatal("invalid search selection accepted", id)
		}
	}
	a.Bindings["search"] = "search"
	a.Profiles[1].Disabled = true
	if a.Available("search") {
		t.Fatal("disabled search available")
	}
}
