package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

func TestClassifyAffinityDeltaRejectsSexualHarassment(t *testing.T) {
	event := AffinityEvent{Message: "月灵 可以涩涩吗 发点黄色的"}

	delta, reason := ClassifyAffinityDelta(event)

	if delta >= 0 {
		t.Fatalf("ClassifyAffinityDelta() delta = %d, want negative", delta)
	}
	if reason == "" {
		t.Fatalf("ClassifyAffinityDelta() reason is empty")
	}
}

func TestClassifyAffinityDeltaRewardsPoliteNormalChat(t *testing.T) {
	event := AffinityEvent{Message: "月灵 请帮我总结一下刚才讨论的部署步骤，谢谢"}

	delta, _ := ClassifyAffinityDelta(event)

	if delta <= 0 {
		t.Fatalf("ClassifyAffinityDelta() delta = %d, want positive", delta)
	}
}

func TestApplyAffinityDeltaClampsToConfiguredBounds(t *testing.T) {
	cfg := config.AffinityConfig{Initial: 50, Min: 0, Max: 100, BlockBelow: 10}

	if got := ApplyAffinityDelta(98, 10, cfg); got != 100 {
		t.Fatalf("ApplyAffinityDelta(98, 10) = %d, want 100", got)
	}
	if got := ApplyAffinityDelta(3, -10, cfg); got != 0 {
		t.Fatalf("ApplyAffinityDelta(3, -10) = %d, want 0", got)
	}
}

func TestNormalizeAffinityConfigFillsDefaults(t *testing.T) {
	cfg := NormalizeAffinityConfig(config.AffinityConfig{})

	if cfg.Initial != 50 || cfg.Min != 0 || cfg.Max != 100 || cfg.BlockBelow != 10 {
		t.Fatalf("NormalizeAffinityConfig() = %+v, want initial=50 min=0 max=100 block_below=10", cfg)
	}
}

func TestLoadConfigAppliesAffinityDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	toml := []byte(`
[bot]
name = "月灵"

[napcat]
url = "ws://127.0.0.1:3001"

[ai]
deepseek_key = "test-key"
`)
	if err := os.WriteFile(path, toml, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := config.Load(path); err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	cfg := config.C.AI.Affinity
	if !cfg.Enabled || cfg.Initial != 50 || cfg.Min != 0 || cfg.Max != 100 || cfg.BlockBelow != 10 {
		t.Fatalf("config.C.AI.Affinity = %+v, want enabled=true initial=50 min=0 max=100 block_below=10", cfg)
	}
}

func TestAffinityPromptUsesBehaviorLevelWithoutExactScore(t *testing.T) {
	prompt := AffinityPrompt(7, config.AffinityConfig{BlockBelow: 10})

	if !strings.Contains(prompt, "疏远") {
		t.Fatalf("AffinityPrompt() = %q, want 疏远 level", prompt)
	}
	if strings.Contains(prompt, "7") {
		t.Fatalf("AffinityPrompt() = %q, should not expose exact score", prompt)
	}
}
