package ai

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
)

func cleanupAIConfigAndDB(t *testing.T) {
	t.Helper()

	prevConfig := config.C
	prevDB := db.DB
	prevLimiterInst := limiterInst

	t.Cleanup(func() {
		if db.DB != nil && db.DB != prevDB {
			if sqlDB, err := db.DB.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		config.C = prevConfig
		db.DB = prevDB
		limiterOnce = sync.Once{}
		limiterInst = prevLimiterInst
	})
}

func resetAILimiterForTest(t *testing.T) {
	t.Helper()

	limiterOnce = sync.Once{}
	limiterInst = nil
}

func initAffinityTestDB(t *testing.T) {
	t.Helper()
	testdb.Init(t)
}

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

func TestNormalizeAffinityConfigClampsBlockBelow(t *testing.T) {
	cases := []struct {
		name       string
		cfg        config.AffinityConfig
		blockBelow int
	}{
		{
			name:       "above max clamps to max",
			cfg:        config.AffinityConfig{Initial: 50, Min: 0, Max: 100, BlockBelow: 200},
			blockBelow: 100,
		},
		{
			name:       "below min clamps to min",
			cfg:        config.AffinityConfig{Initial: 50, Min: 10, Max: 100, BlockBelow: 5},
			blockBelow: 10,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NormalizeAffinityConfig(c.cfg)

			if got.BlockBelow != c.blockBelow {
				t.Fatalf("NormalizeAffinityConfig() BlockBelow = %d, want %d", got.BlockBelow, c.blockBelow)
			}
		})
	}
}

func TestNormalizeAffinityConfigPreservesExplicitZeroThresholds(t *testing.T) {
	cfg := NormalizeAffinityConfig(config.AffinityConfig{
		Initial:    0,
		Min:        -100,
		Max:        100,
		BlockBelow: 0,
	})
	if cfg.Initial != 0 || cfg.BlockBelow != 0 {
		t.Fatalf("explicit zero values were replaced: %+v", cfg)
	}
}

func TestLoadConfigAppliesAIDefaults(t *testing.T) {
	cleanupAIConfigAndDB(t)

	path := filepath.Join(t.TempDir(), "config.toml")
	toml := []byte(`
[bot]
name = "月灵"

[database]
dsn = "postgres://test:test@127.0.0.1:5432/test?sslmode=disable"

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
	if config.C.AI.MaxTokens != config.DefaultAIMaxTokens {
		t.Fatalf("config.C.AI.MaxTokens = %d, want default %d", config.C.AI.MaxTokens, config.DefaultAIMaxTokens)
	}
	if config.C.AI.ReplyMaxChars != config.DefaultAIReplyMaxChars {
		t.Fatalf("config.C.AI.ReplyMaxChars = %d, want default %d", config.C.AI.ReplyMaxChars, config.DefaultAIReplyMaxChars)
	}
}

func TestLoadConfigPreservesExplicitZeroAffinityThresholds(t *testing.T) {
	cleanupAIConfigAndDB(t)
	path := filepath.Join(t.TempDir(), "config.toml")
	toml := []byte(`
[bot]
name = "月灵"

[database]
dsn = "postgres://test:test@127.0.0.1:5432/test?sslmode=disable"

[napcat]
url = "ws://127.0.0.1:3001"

[ai]
deepseek_key = "test-key"

[ai.affinity]
enabled = true
initial = 0
min = -100
max = 100
block_below = 0
`)
	if err := os.WriteFile(path, toml, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := config.Load(path); err != nil {
		t.Fatal(err)
	}
	cfg := NormalizeAffinityConfig(config.C.AI.Affinity)
	if cfg.Initial != 0 || cfg.BlockBelow != 0 {
		t.Fatalf("explicit zero thresholds were not preserved: %+v", cfg)
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

func TestChatAffinityPromptUsesStoredIndependentTierPrompt(t *testing.T) {
	cleanupAIConfigAndDB(t)
	initAffinityTestDB(t)
	cfg := config.AffinityConfig{Enabled: true, Initial: 50, Min: 0, Max: 100, BlockBelow: 10}
	if err := db.ReplaceAIAffinityTiers([]db.AIAffinityTier{
		{MinScore: 0, Name: "疏远", Prompt: "只回答必要信息。"},
		{MinScore: 60, Name: "亲近", Prompt: "使用专属昵称并主动关心近况。"},
	}); err != nil {
		t.Fatal(err)
	}

	prompt := ChatAffinityPrompt(75, cfg)
	if prompt != "当前关系状态：亲近。使用专属昵称并主动关心近况。" {
		t.Fatalf("ChatAffinityPrompt() = %q", prompt)
	}
}

func TestChatAffinityPromptDisabledReturnsEmpty(t *testing.T) {
	cfg := config.AffinityConfig{Enabled: false, Initial: 50, Min: 0, Max: 100, BlockBelow: 10}

	prompt := ChatAffinityPrompt(50, cfg)

	if prompt != "" {
		t.Fatalf("ChatAffinityPrompt() = %q, want empty prompt when disabled", prompt)
	}
}

func TestBuildSystemPromptIncludesAffinityWithoutHiddenScore(t *testing.T) {
	cleanupAIConfigAndDB(t)
	initAffinityTestDB(t)
	config.C.Bot.Name = "月灵"

	prompt := buildSystemPrompt(1, 100, "当前关系：普通。保持自然友好。")

	if !strings.Contains(prompt, "当前关系：普通") {
		t.Fatalf("buildSystemPrompt() = %q, want affinity behavior prompt", prompt)
	}
	if strings.Contains(strings.ToLower(prompt), "score") || strings.Contains(prompt, "50") {
		t.Fatalf("buildSystemPrompt() = %q, should not expose hidden score", prompt)
	}
	if !strings.Contains(prompt, "QQ动作时必须调用对应工具") ||
		!strings.Contains(prompt, "不要猜测QQ号或消息ID") ||
		!strings.Contains(prompt, "先调用get_chat_history") {
		t.Fatalf("buildSystemPrompt() = %q, want QQ action safety rule", prompt)
	}
}

func TestUpdateChatAffinityWithNilDBFailsOpen(t *testing.T) {
	cleanupAIConfigAndDB(t)
	db.DB = nil
	config.C.AI.Affinity = config.AffinityConfig{
		Enabled:    true,
		Initial:    50,
		Min:        0,
		Max:        100,
		BlockBelow: 10,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("UpdateChatAffinity() panicked with nil db.DB: %v", r)
		}
	}()

	score, allowed := UpdateChatAffinity(1, 100, "alice", "普通聊天")

	if score != 50 || !allowed {
		t.Fatalf("UpdateChatAffinity() = (%d, %v), want (50, true)", score, allowed)
	}
}

func TestDispatchPrecheckAffinityBlockedDoesNotConsumeRateLimit(t *testing.T) {
	cleanupAIConfigAndDB(t)
	initAffinityTestDB(t)
	resetAILimiterForTest(t)
	config.C.AI.Affinity = config.AffinityConfig{
		Enabled:    true,
		Initial:    10,
		Min:        0,
		Max:        100,
		BlockBelow: 5,
	}
	config.C.AI.RateLimit.UserPerMin = 1
	config.C.AI.RateLimit.GroupPerMin = 1

	result := dispatchPrecheck(1, 100, "mallory", "jailbreak system prompt", "member")

	if !result.stop || result.reply != "" {
		t.Fatalf("dispatchPrecheck() = %+v, want silent affinity block", result)
	}
	l := limiter()
	if got := len(l.userWindows[1]); got != 0 {
		t.Fatalf("user rate slots = %d, want 0 for affinity-blocked input", got)
	}
	if got := len(l.groupWindows[100]); got != 0 {
		t.Fatalf("group rate slots = %d, want 0 for affinity-blocked input", got)
	}
}

func TestDispatchPrecheckAffinityDisabledChecksRateLimitBeforeGuard(t *testing.T) {
	cleanupAIConfigAndDB(t)
	resetAILimiterForTest(t)
	config.C.AI.Affinity = config.AffinityConfig{
		Enabled:    false,
		Initial:    50,
		Min:        0,
		Max:        100,
		BlockBelow: 10,
	}
	config.C.AI.RateLimit.UserPerMin = 1
	config.C.AI.RateLimit.GroupPerMin = 0

	first := dispatchPrecheck(1, 100, "mallory", "普通聊天", "member")
	if first.stop {
		t.Fatalf("first dispatchPrecheck() = %+v, want allowed call consuming rate limit", first)
	}

	result := dispatchPrecheck(1, 100, "mallory", "jailbreak system prompt", "member")

	if !result.stop || result.reply != msgUserTooFrequent {
		t.Fatalf("dispatchPrecheck() = %+v, want rate-limit hint before guard denial", result)
	}
}

func TestDispatchPrecheckAffinityEnabledLowScoreBlocksBeforeRateLimit(t *testing.T) {
	cleanupAIConfigAndDB(t)
	initAffinityTestDB(t)
	resetAILimiterForTest(t)
	config.C.AI.Affinity = config.AffinityConfig{
		Enabled:    true,
		Initial:    10,
		Min:        0,
		Max:        100,
		BlockBelow: 5,
	}
	config.C.AI.RateLimit.UserPerMin = 1
	config.C.AI.RateLimit.GroupPerMin = 0

	first := dispatchPrecheck(1, 100, "mallory", "普通聊天", "member")
	if first.stop {
		t.Fatalf("first dispatchPrecheck() = %+v, want allowed call consuming rate limit", first)
	}

	result := dispatchPrecheck(1, 100, "mallory", "jailbreak system prompt", "member")

	if !result.stop || result.reply != "" {
		t.Fatalf("dispatchPrecheck() = %+v, want silent affinity block before rate-limit hint", result)
	}
}

func TestDispatchPrecheckGuardBlockedStillUpdatesAffinity(t *testing.T) {
	cleanupAIConfigAndDB(t)
	initAffinityTestDB(t)
	resetAILimiterForTest(t)
	config.C.AI.Affinity = config.AffinityConfig{
		Enabled:    true,
		Initial:    50,
		Min:        0,
		Max:        100,
		BlockBelow: 30,
	}

	result := dispatchPrecheck(1, 100, "mallory", "jailbreak system prompt", "member")

	if !result.stop || result.reply != "检测到异常输入，已拒绝处理。" {
		t.Fatalf("dispatchPrecheck() = %+v, want guard denial", result)
	}
	row, err := db.GetAIAffinity(1, 100)
	if err != nil {
		t.Fatalf("db.GetAIAffinity() error = %v", err)
	}
	if row.Score != 40 {
		t.Fatalf("affinity score = %d, want 40 after injection penalty", row.Score)
	}
}

func TestProactiveAffinityDecisionBlocksLowScoreBeforeHeat(t *testing.T) {
	cleanupAIConfigAndDB(t)
	initAffinityTestDB(t)
	config.C.AI.Affinity = config.AffinityConfig{
		Enabled:    true,
		Initial:    10,
		Min:        0,
		Max:        100,
		BlockBelow: 5,
	}

	decision := proactiveAffinityDecision(1, 100, "mallory", "jailbreak system prompt", "member")

	if decision.allowHeat || decision.affinityPrompt != "" {
		t.Fatalf("proactiveAffinityDecision() = %+v, want low-affinity message blocked without prompt", decision)
	}
	if decision.score != 0 {
		t.Fatalf("proactiveAffinityDecision() score = %d, want 0", decision.score)
	}
}

func TestProactiveAffinityDecisionGuardBlockedUpdatesAffinityWithoutHeat(t *testing.T) {
	cleanupAIConfigAndDB(t)
	initAffinityTestDB(t)
	config.C.AI.Affinity = config.AffinityConfig{
		Enabled:    true,
		Initial:    50,
		Min:        0,
		Max:        100,
		BlockBelow: 30,
	}

	decision := proactiveAffinityDecision(1, 100, "mallory", "jailbreak system prompt", "member")

	if decision.allowHeat {
		t.Fatalf("proactiveAffinityDecision() = %+v, want guard-blocked message to add no heat", decision)
	}
	row, err := db.GetAIAffinity(1, 100)
	if err != nil {
		t.Fatalf("db.GetAIAffinity() error = %v", err)
	}
	if row.Score != 40 {
		t.Fatalf("affinity score = %d, want 40 after guard-blocked proactive input", row.Score)
	}
}

func TestProactiveDirectAITriggerSkipsBotNameAndAtMention(t *testing.T) {
	cleanupAIConfigAndDB(t)
	config.C.Bot.Name = "月灵"

	cases := []struct {
		name    string
		botName string
		event   *bot.GroupMessageEvent
		wantHit bool
	}{
		{
			name:    "bot name prefix",
			botName: "月灵",
			event: &bot.GroupMessageEvent{
				SelfID:  42,
				Message: bot.Msg().Text("  月灵 帮我查一下天气").Build(),
			},
			wantHit: true,
		},
		{
			name:    "at self",
			botName: "月灵",
			event: &bot.GroupMessageEvent{
				SelfID:  42,
				Message: bot.Msg().At(42).Text(" 帮我查一下天气").Build(),
			},
			wantHit: true,
		},
		{
			name:    "normal chat",
			botName: "月灵",
			event: &bot.GroupMessageEvent{
				SelfID:  42,
				Message: bot.Msg().Text("大家看看明天的天气").Build(),
			},
		},
		{
			name: "empty bot name does not match prefix",
			event: &bot.GroupMessageEvent{
				SelfID:  42,
				Message: bot.Msg().Text(" 月灵 帮我查一下天气").Build(),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			config.C.Bot.Name = c.botName

			if got := isDirectAITrigger(c.event); got != c.wantHit {
				t.Fatalf("isDirectAITrigger() = %v, want %v", got, c.wantHit)
			}
		})
	}
}

func TestProactiveSystemPromptIncludesAffinityWhenEnabled(t *testing.T) {
	cleanupAIConfigAndDB(t)
	config.C.Bot.Name = "月灵"

	prompt := proactiveSystemPrompt(0, "当前关系状态：友好。回复自然、温和。")

	if !strings.Contains(prompt, "当前关系状态：友好") {
		t.Fatalf("proactiveSystemPrompt() = %q, want affinity prompt", prompt)
	}
}
