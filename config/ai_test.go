package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAIOnlyAndEnvironmentOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ai.toml")
	if err := os.WriteFile(path, []byte("[ai]\ndeepseek_key='fixture'\nmodel='fixture-model'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	original := C
	t.Setenv("YUELING_AI_REASONING_EFFORT", "high")
	settings, err := LoadAI(path)
	if err != nil {
		t.Fatal(err)
	}
	if settings.MaxTokens != DefaultAIMaxTokens || settings.ReplyMaxChars != DefaultAIReplyMaxChars || settings.ReasoningEffort != "high" || settings.Model != "fixture-model" {
		t.Fatalf("unexpected configuration defaults")
	}
	if C.AI.Model != original.AI.Model {
		t.Fatal("diagnostic mutated global config")
	}
}
