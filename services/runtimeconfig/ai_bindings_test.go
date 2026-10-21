package runtimeconfig

import (
	"encoding/json"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

// Mirrors the feature editor: selecting a primary model removes its explicit
// "off" binding and writes the corresponding default profile in the same save.
func TestSelectingPrimaryModelsReplacesDisabledBindings(t *testing.T) {
	m, store := fixture(t)
	profiles := []config.AIProfile{{ID: "model", Name: "多模态", Model: "fixture-model", BaseURL: "https://fixture.invalid/v1", APIKey: "fixture-key", Capabilities: []string{"chat", "tools", "vision", "image_generation"}}}
	if _, err := profileUpdate(m, profiles, "", ""); err != nil {
		t.Fatal(err)
	}
	update := func(values map[string]any) {
		t.Helper()
		raw := map[string]json.RawMessage{}
		for path, value := range values {
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			raw[path] = data
		}
		if _, err := m.Update(Update{Revision: m.View().Revision, Section: "ai", Values: raw}); err != nil {
			t.Fatal(err)
		}
	}
	update(map[string]any{"ai.bindings": map[string]string{"conversation": "off", "vision": "off", "image_generation": "model"}})
	update(map[string]any{"ai.conversation_profile": "model", "ai.vision_profile": "model", "ai.bindings": map[string]string{"image_generation": "model"}})
	verify := func() {
		t.Helper()
		current := config.Current().AI
		for _, feature := range []string{"conversation", "vision", "image_generation"} {
			profile, _, err := current.Select(feature)
			if err != nil || profile.ID != "model" {
				t.Errorf("%s reset after saving selected model: binding=%q profile=%q error=%v", feature, current.Bindings[feature], profile.ID, err)
			}
		}
		for _, feature := range []string{"conversation", "vision"} {
			if _, exists := current.Bindings[feature]; exists {
				t.Errorf("deleted %s binding reappeared", feature)
			}
		}
	}
	verify()
	if err := New(store, nil).Initialize(); err != nil {
		t.Fatal(err)
	}
	verify()
}

func TestBindingsSnapshotReplacementAndUnrelatedSaves(t *testing.T) {
	for _, tc := range []struct {
		name        string
		replacement any
		want        map[string]string
	}{
		{"empty", map[string]string{}, map[string]string{}},
		{"null", nil, nil},
		{"retained override", map[string]string{"summary": "off"}, map[string]string{"summary": "off"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := fixture(t)
			if _, err := update(m, "ai", "ai.bindings", map[string]string{"conversation": "off", "vision": "off", "memory": "off", "summary": "off"}); err != nil {
				t.Fatal(err)
			}
			if _, err := update(m, "ai", "ai.bindings", tc.replacement); err != nil {
				t.Fatal(err)
			}
			got := config.Current().AI.Bindings
			if len(got) != len(tc.want) {
				t.Fatalf("removed binding survived: got %v want %v", got, tc.want)
			}
			for key, want := range tc.want {
				if got[key] != want {
					t.Fatalf("retained binding changed: %s", key)
				}
			}
			if _, err := update(m, "bot", "bot.name", "新名字"); err != nil {
				t.Fatal(err)
			}
			got = config.Current().AI.Bindings
			if len(got) != len(tc.want) {
				t.Fatal("unrelated save changed bindings", got)
			}
			for key, want := range tc.want {
				if got[key] != want {
					t.Fatal("unrelated save changed retained override")
				}
			}
		})
	}
}
