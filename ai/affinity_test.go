package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
)

func cleanupAIConfigAndDB(t *testing.T) {
	t.Helper()

	prevConfig := config.C
	prevDB := testdb.Database
	prevLimiterInst := testEngine.limiter

	t.Cleanup(func() {
		if testdb.Database != nil && testdb.Database != prevDB {
			if sqlDB, err := testdb.Database.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		config.C = prevConfig
		testdb.Database = prevDB
		testEngine.limiter = prevLimiterInst
	})
}

func resetAILimiterForTest(t *testing.T) {
	t.Helper()

	testEngine.limiter = newAILimiter(0, 0, time.Minute)
}

func initAffinityTestDB(t *testing.T) {
	t.Helper()
	testdb.Init(t)
	previousGroups := testEngine.Groups
	previousProactiveGroups := testEngine.Proactive.groups
	testEngine.Groups = db.NewGroupStore(testdb.Database)
	testEngine.Proactive.groups = testEngine.Groups
	t.Cleanup(func() { testEngine.Groups = previousGroups; testEngine.Proactive.groups = previousProactiveGroups })
	previousAffinity := testEngine.Affinity
	testEngine.Affinity = NewAffinityManager(db.NewAffinityStore(testdb.Database))
	previousProactiveAffinity := testEngine.Proactive.affinity
	testEngine.Proactive.affinity = testEngine.Affinity
	t.Cleanup(func() {
		testEngine.Affinity = previousAffinity
		testEngine.Proactive.affinity = previousProactiveAffinity
	})
	previousChats := testEngine.Chats
	testEngine.Chats = db.NewChatStore(testdb.Database)
	t.Cleanup(func() { testEngine.Chats = previousChats })
	previousMemory := testEngine.Memory
	testEngine.Memory = NewMemoryManager(testdb.Database)
	t.Cleanup(func() { testEngine.Memory = previousMemory })
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
max_tokens = 4096
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
max_tokens = 4096

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
	if err := db.NewAffinityStore(testdb.Database).ReplaceAIAffinityTiers([]db.AIAffinityTier{
		{MinScore: 0, Name: "疏远", Prompt: "只回答必要信息。"},
		{MinScore: 60, Name: "亲近", Prompt: "使用专属昵称并主动关心近况。"},
	}); err != nil {
		t.Fatal(err)
	}

	prompt := testEngine.Affinity.ChatAffinityPrompt(75, cfg)
	if prompt != "当前关系状态：亲近。使用专属昵称并主动关心近况。" {
		t.Fatalf("testEngine.Affinity.ChatAffinityPrompt() = %q", prompt)
	}
}

func TestChatAffinityPromptDisabledReturnsEmpty(t *testing.T) {
	cfg := config.AffinityConfig{Enabled: false, Initial: 50, Min: 0, Max: 100, BlockBelow: 10}

	prompt := testEngine.Affinity.ChatAffinityPrompt(50, cfg)

	if prompt != "" {
		t.Fatalf("testEngine.Affinity.ChatAffinityPrompt() = %q, want empty prompt when disabled", prompt)
	}
}

func TestBuildSystemPromptIncludesAffinityWithoutHiddenScore(t *testing.T) {
	cleanupAIConfigAndDB(t)
	initAffinityTestDB(t)
	config.C.Bot.Name = "月灵"

	prompt := testEngine.buildSystemPrompt(100, "") + testEngine.turnRuntime(1, "当前关系：普通。保持自然友好。", "")

	if !strings.Contains(prompt, "当前关系：普通") {
		t.Fatalf("testEngine.buildSystemPrompt() = %q, want affinity behavior prompt", prompt)
	}
	_, relationship, found := strings.Cut(prompt, "【关系上下文】")
	if !found || strings.Contains(strings.ToLower(relationship), "score") || strings.Contains(relationship, "50") {
		t.Fatalf("relationship context = %q, should not expose hidden score", relationship)
	}
	if !strings.Contains(prompt, "只使用本轮公布的工具") ||
		!strings.Contains(prompt, "不要猜测QQ号或消息ID") ||
		!strings.Contains(prompt, "先调用get_chat_history") {
		t.Fatalf("testEngine.buildSystemPrompt() = %q, want QQ action safety rule", prompt)
	}
}

func TestUpdateChatAffinityWithNilDBFailsOpen(t *testing.T) {
	cleanupAIConfigAndDB(t)
	testdb.Database = nil
	config.C.AI.Affinity = config.AffinityConfig{
		Enabled:    true,
		Initial:    50,
		Min:        0,
		Max:        100,
		BlockBelow: 10,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("testEngine.Affinity.UpdateChatAffinity() panicked with nil testdb.Database: %v", r)
		}
	}()

	score, allowed := testEngine.Affinity.UpdateChatAffinity(1, 100, "alice", "普通聊天")

	if score != 50 || !allowed {
		t.Fatalf("testEngine.Affinity.UpdateChatAffinity() = (%d, %v), want (50, true)", score, allowed)
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

	result := testEngine.dispatchPrecheck(1, 100, "mallory", "jailbreak system prompt", "member")

	if !result.stop || result.reply != "" {
		t.Fatalf("dispatchPrecheck() = %+v, want silent affinity block", result)
	}
	l := testEngine.limiter
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

	first := testEngine.dispatchPrecheck(1, 100, "mallory", "普通聊天", "member")
	if first.stop {
		t.Fatalf("first dispatchPrecheck() = %+v, want allowed call consuming rate limit", first)
	}

	result := testEngine.dispatchPrecheck(1, 100, "mallory", "jailbreak system prompt", "member")

	if !result.stop || result.reply != "" {
		t.Fatalf("dispatchPrecheck() = %+v, want silent rate limit before guard denial", result)
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

	first := testEngine.dispatchPrecheck(1, 100, "mallory", "普通聊天", "member")
	if first.stop {
		t.Fatalf("first dispatchPrecheck() = %+v, want allowed call consuming rate limit", first)
	}

	result := testEngine.dispatchPrecheck(1, 100, "mallory", "jailbreak system prompt", "member")

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

	result := testEngine.dispatchPrecheck(1, 100, "mallory", "jailbreak system prompt", "member")

	if !result.stop || result.reply != "检测到异常输入，已拒绝处理。" {
		t.Fatalf("dispatchPrecheck() = %+v, want guard denial", result)
	}
	row, err := db.NewAffinityStore(testdb.Database).GetAIAffinity(1, 100)
	if err != nil {
		t.Fatalf("db.NewAffinityStore(testdb.Database).GetAIAffinity() error = %v", err)
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

	decision := testEngine.Affinity.proactiveDecision(1, 100, "mallory", "jailbreak system prompt", "member")

	if decision.allowHeat || decision.affinityPrompt != "" {
		t.Fatalf("testEngine.Affinity.proactiveDecision() = %+v, want low-affinity message blocked without prompt", decision)
	}
	if decision.score != 0 {
		t.Fatalf("testEngine.Affinity.proactiveDecision() score = %d, want 0", decision.score)
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

	decision := testEngine.Affinity.proactiveDecision(1, 100, "mallory", "jailbreak system prompt", "member")

	if decision.allowHeat {
		t.Fatalf("testEngine.Affinity.proactiveDecision() = %+v, want guard-blocked message to add no heat", decision)
	}
	row, err := db.NewAffinityStore(testdb.Database).GetAIAffinity(1, 100)
	if err != nil {
		t.Fatalf("db.NewAffinityStore(testdb.Database).GetAIAffinity() error = %v", err)
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

	prompt := proactiveSystemPrompt(testEngine.Groups, 0, "当前关系状态：友好。回复自然、温和。")

	if !strings.Contains(prompt, "当前关系状态：友好") {
		t.Fatalf("proactiveSystemPrompt(testEngine.Groups, ) = %q, want affinity prompt", prompt)
	}
}
