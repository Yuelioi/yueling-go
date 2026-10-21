package runtimeconfig

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

func TestCanonicalBindingsCredentialsAndRestart(t *testing.T) {
	m, store := fixture(t)
	apply := func(connections any, profiles any, bindings any) error {
		values := map[string]json.RawMessage{}
		for path, value := range map[string]any{"ai.connections": connections, "ai.profiles": profiles, "ai.bindings": bindings} {
			values[path], _ = json.Marshal(value)
		}
		_, err := m.Update(Update{Revision: m.View().Revision, Section: "ai", Values: values})
		return err
	}
	connections := []config.AIConnection{{ID: "a", Name: "A", Kind: "openai", BaseURL: "https://example.com/v1", APIKey: "private-a"}, {ID: "b", Name: "B", Kind: "openai", BaseURL: "https://example.com/v1", APIKey: "private-b"}}
	profiles := []config.AIProfile{{ID: "a", Name: "A", ConnectionID: "a", Model: "chat", Capabilities: []string{"chat", "tools"}}, {ID: "b", Name: "B", ConnectionID: "b", Model: "vision", Capabilities: []string{"chat", "tools", "vision"}}}
	bindings := map[string]string{"conversation": "a", "vision": "b", "summary": "off"}
	if err := apply(connections, profiles, bindings); err != nil {
		t.Fatal(err)
	}
	p, c, err := config.Current().AI.Select("conversation")
	if err != nil || p.Model != "chat" || c.APIKey != "private-a" {
		t.Fatal("wrong binding", err)
	}
	bindings["conversation"] = "b"
	connections[0].APIKey = ""
	connections[1].APIKey = ""
	if err := apply(connections, profiles, bindings); err != nil {
		t.Fatal(err)
	}
	_, c, err = config.Current().AI.Select("conversation")
	if err != nil || c.APIKey != "private-b" {
		t.Fatal("credential did not follow connection", err)
	}
	view, _ := json.Marshal(m.View())
	if strings.Contains(string(view), "private-") {
		t.Fatal("secret exposed")
	}
	revision := store.revision
	if err := New(store, nil).Initialize(); err != nil || store.revision != revision {
		t.Fatal("restart changed config", err)
	}
	if _, _, err := config.Current().AI.Select("translation"); err == nil {
		t.Fatal("missing feature silently inherited model")
	}
	for _, old := range []string{"ai.model", "ai.deepseek_key", "ai.conversation_profile", "ai.vision_profile"} {
		if _, err := update(m, "ai", old, "a"); err == nil {
			t.Fatal("legacy API accepted", old)
		}
	}
	// A binding replacement must remove deleted keys, including explicit off.
	if _, err := update(m, "ai", "ai.bindings", map[string]string{"conversation": "a"}); err != nil {
		t.Fatal(err)
	}
	if len(config.Current().AI.Bindings) != 1 {
		t.Fatal("map merge retained removed bindings")
	}
	snapshot := config.Current()
	snapshot.AI.Profiles[0].Capabilities[0] = "bad"
	snapshot.AI.Connections[0].APIKey = "bad"
	if config.Current().AI.Profiles[0].Capabilities[0] != "chat" {
		t.Fatal("mutable snapshot")
	}
	before := config.Current()
	store.fail = true
	if _, err := update(m, "ai", "ai.bindings", map[string]string{"conversation": "b"}); !errors.Is(err, ErrPersistence) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, config.Current()) {
		t.Fatal("failed persistence published")
	}
}

func TestCanonicalConfigRejectsAmbiguousOrDanglingRoutes(t *testing.T) {
	m, _ := fixture(t)
	for _, profiles := range []any{
		[]config.AIProfile{{ID: "a", Name: "A", Model: "m", BaseURL: "https://example.com/v1", APIKey: "secret"}},
		[]config.AIProfile{{ID: "a", Name: "A", ConnectionID: "missing", Model: "m", Capabilities: []string{"chat"}}},
	} {
		if _, err := update(m, "ai", "ai.profiles", profiles); err == nil {
			t.Fatal("ambiguous profile accepted")
		}
	}
	if _, err := update(m, "ai", "ai.bindings", map[string]string{"conversation": "missing"}); err == nil {
		t.Fatal("dangling binding accepted")
	}
}
