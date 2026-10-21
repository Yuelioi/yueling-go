package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAIOnlyAndEnvironmentOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ai.toml")
	if err := os.WriteFile(path, []byte("[ai.bindings]\nconversation='p'\n[[ai.connections]]\nid='c'\napi_key='fixture'\n[[ai.profiles]]\nid='p'\nconnection_id='c'\nmodel='fixture-model'\ncapabilities=['chat','tools']\n"), 0600); err != nil {
		t.Fatal(err)
	}
	original := C
	t.Setenv("YUELING_AI_REASONING_EFFORT", "high")
	settings, err := LoadAI(path)
	if err != nil {
		t.Fatal(err)
	}
	if settings.MaxTokens != DefaultAIMaxTokens || settings.ReplyMaxChars != DefaultAIReplyMaxChars || settings.ReasoningEffort != "high" || settings.Profiles[0].Model != "fixture-model" {
		t.Fatalf("unexpected configuration defaults")
	}
	if C.AI.ReasoningEffort != original.AI.ReasoningEffort {
		t.Fatal("diagnostic mutated global config")
	}
}
