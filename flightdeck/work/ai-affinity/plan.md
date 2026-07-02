# AI Chat Affinity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace public/game-backed affinity with hidden AI-chat-only affinity, and delete PK battle.

**Architecture:** Add a dedicated DB model and small pure scoring package inside `ai`, then wire `ai.Dispatch` so each AI-triggering message updates hidden affinity before LLM execution. Remove old public command/tool surfaces and PK registration/docs.

**Tech Stack:** Go 1.25, GORM SQLite, existing `ai`, `db`, `config`, `plugins`, and `bot` packages.

---

## File Structure

- Create `ai/affinity.go`: pure scoring, config normalization, level labels, dispatch-facing helpers.
- Create `ai/affinity_test.go`: unit tests for scoring, clamping, and level/prompt behavior.
- Modify `db/db.go`: add `AIAffinity` model and helper functions.
- Create `db/ai_affinity_test.go`: persistence tests with an isolated sqlite DB.
- Modify `config/config.go`: add `[ai.affinity]` defaults.
- Modify `ai/dispatcher.go`: update hidden score, block below threshold, embed affinity level in prompt.
- Modify `plugins/ai_dispatch/plugin.go`: keep existing empty-reply behavior as the silent block path.
- Delete `plugins/game/pk.go`.
- Modify `cmd/bot/main.go`: remove `game.RegisterPK(b)` and `funny.RegisterChat(b)`.
- Delete `plugins/funny/chat.go`.
- Modify `ai/tools/social.go`: remove `registerAffinityRanking` and its DB dependency if unused.
- Modify `plugins/system/help.go`: remove PK and public affinity entries.
- Modify `README.md`: remove PK and public affinity docs; add hidden AI affinity config note.
- Modify `config.example.toml`: document optional `[ai.affinity]`.

## Task 1: Hidden Affinity Scoring

**Files:**
- Create: `ai/affinity.go`
- Test: `ai/affinity_test.go`

- [ ] **Step 1: Write failing tests**

```go
package ai

import (
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

func TestClassifyAffinityDeltaPenalizesSexualHarassment(t *testing.T) {
	ev := ClassifyAffinityDelta("月灵 可以涩涩吗 发点黄色的")
	if ev.Delta >= 0 {
		t.Fatalf("sexual harassment should lower affinity, got %+v", ev)
	}
	if ev.Reason == "" {
		t.Fatalf("reason should be recorded")
	}
}

func TestClassifyAffinityDeltaRewardsNormalPoliteChatSlowly(t *testing.T) {
	ev := ClassifyAffinityDelta("月灵 请帮我总结一下刚才讨论的部署步骤，谢谢")
	if ev.Delta <= 0 {
		t.Fatalf("normal polite chat should recover slowly, got %+v", ev)
	}
}

func TestApplyAffinityDeltaClampsToRange(t *testing.T) {
	cfg := config.AffinityConfig{Initial: 50, Min: 0, Max: 100, BlockBelow: 10}
	if got := ApplyAffinityDelta(98, 10, cfg); got != 100 {
		t.Fatalf("upper clamp = %d, want 100", got)
	}
	if got := ApplyAffinityDelta(3, -10, cfg); got != 0 {
		t.Fatalf("lower clamp = %d, want 0", got)
	}
}

func TestAffinityLevelPromptDoesNotExposeScore(t *testing.T) {
	prompt := AffinityPrompt(7, config.AffinityConfig{BlockBelow: 10})
	if strings.Contains(prompt, "7") {
		t.Fatalf("prompt should not expose exact score: %q", prompt)
	}
	if !strings.Contains(prompt, "疏远") {
		t.Fatalf("prompt should include a behavioral level: %q", prompt)
	}
}
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```powershell
go test ./ai -run Affinity -count=1
```

Expected: FAIL because `ClassifyAffinityDelta`, `ApplyAffinityDelta`, `AffinityPrompt`, and `config.AffinityConfig` do not exist.

- [ ] **Step 3: Implement minimal scoring**

Create `ai/affinity.go`:

```go
package ai

import (
	"strings"
	"unicode/utf8"

	"github.com/Yuelioi/yueling-go/config"
)

type AffinityEvent struct {
	Delta  int
	Reason string
}

func ClassifyAffinityDelta(text string) AffinityEvent {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return AffinityEvent{Delta: -1, Reason: "empty"}
	}
	negative := []string{"黄色", "涩涩", "色色", "开车", "裸", "做爱", "操你", "傻逼", "弱智", "jailbreak", "ignore previous", "忽略之前"}
	for _, kw := range negative {
		if strings.Contains(lower, kw) {
			return AffinityEvent{Delta: -15, Reason: "harmful_content"}
		}
	}
	if utf8.RuneCountInString(lower) <= 2 {
		return AffinityEvent{Delta: -1, Reason: "low_effort"}
	}
	positive := []string{"谢谢", "请", "帮我", "辛苦", "麻烦"}
	for _, kw := range positive {
		if strings.Contains(lower, kw) {
			return AffinityEvent{Delta: 2, Reason: "polite_chat"}
		}
	}
	return AffinityEvent{Delta: 1, Reason: "normal_chat"}
}

func NormalizeAffinityConfig(cfg config.AffinityConfig) config.AffinityConfig {
	if cfg.Max == 0 {
		cfg.Max = 100
	}
	if cfg.Initial == 0 {
		cfg.Initial = 50
	}
	if cfg.BlockBelow == 0 {
		cfg.BlockBelow = 10
	}
	if cfg.Max < cfg.Min {
		cfg.Max = cfg.Min
	}
	if cfg.Initial < cfg.Min {
		cfg.Initial = cfg.Min
	}
	if cfg.Initial > cfg.Max {
		cfg.Initial = cfg.Max
	}
	return cfg
}

func ApplyAffinityDelta(score, delta int, cfg config.AffinityConfig) int {
	cfg = NormalizeAffinityConfig(cfg)
	score += delta
	if score < cfg.Min {
		return cfg.Min
	}
	if score > cfg.Max {
		return cfg.Max
	}
	return score
}

func AffinityPrompt(score int, cfg config.AffinityConfig) string {
	cfg = NormalizeAffinityConfig(cfg)
	switch {
	case score < cfg.BlockBelow:
		return "当前关系：疏远。若仍需回复，应保持冷淡、简短、拒绝暧昧或越界内容。"
	case score < 35:
		return "当前关系：陌生。回复简洁礼貌，不主动亲近。"
	case score < 70:
		return "当前关系：普通。保持自然友好。"
	default:
		return "当前关系：信任。可以更温和亲切，但仍保持边界。"
	}
}
```

- [ ] **Step 4: Add config type**

Modify `config/config.go`:

```go
type AIConfig struct {
	DeepSeekKey string          `mapstructure:"deepseek_key"`
	BaseURL     string          `mapstructure:"base_url"`
	Model       string          `mapstructure:"model"`
	VL          VLConfig        `mapstructure:"vl"`
	RateLimit   RateLimitConfig `mapstructure:"ratelimit"`
	Context     ContextConfig   `mapstructure:"context"`
	Affinity    AffinityConfig  `mapstructure:"affinity"`
}

type AffinityConfig struct {
	Enabled    bool `mapstructure:"enabled"`
	Initial    int  `mapstructure:"initial"`
	BlockBelow int  `mapstructure:"block_below"`
	Min        int  `mapstructure:"min"`
	Max        int  `mapstructure:"max"`
}
```

Add defaults in `Load`:

```go
viper.SetDefault("ai.affinity.enabled", true)
viper.SetDefault("ai.affinity.initial", 50)
viper.SetDefault("ai.affinity.block_below", 10)
viper.SetDefault("ai.affinity.min", 0)
viper.SetDefault("ai.affinity.max", 100)
```

- [ ] **Step 5: Run tests and verify GREEN**

Run:

```powershell
go test ./ai -run Affinity -count=1
```

Expected: PASS.

## Task 2: Persist Hidden Affinity

**Files:**
- Modify: `db/db.go`
- Test: `db/ai_affinity_test.go`

- [ ] **Step 1: Write failing persistence tests**

```go
package db

import (
	"path/filepath"
	"testing"
)

func TestAIAffinityStartsAtInitialAndAppliesDelta(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	if err := Init(path); err != nil {
		t.Fatalf("init db: %v", err)
	}

	row, err := UpdateAIAffinity(1, 100, "alice", 50, 2, 0, 100, "normal_chat")
	if err != nil {
		t.Fatalf("update affinity: %v", err)
	}
	if row.Score != 52 || row.LastReason != "normal_chat" || row.Nickname != "alice" {
		t.Fatalf("unexpected row: %+v", row)
	}

	row, err = GetAIAffinity(1, 100)
	if err != nil {
		t.Fatalf("get affinity: %v", err)
	}
	if row.Score != 52 {
		t.Fatalf("persisted score = %d, want 52", row.Score)
	}
}

func TestAIAffinityClampsInDBUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	if err := Init(path); err != nil {
		t.Fatalf("init db: %v", err)
	}

	row, err := UpdateAIAffinity(1, 100, "", 50, -99, 0, 100, "harmful_content")
	if err != nil {
		t.Fatalf("update affinity: %v", err)
	}
	if row.Score != 0 {
		t.Fatalf("score = %d, want 0", row.Score)
	}
}
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```powershell
go test ./db -run AIAffinity -count=1
```

Expected: FAIL because `AIAffinity`, `UpdateAIAffinity`, and `GetAIAffinity` do not exist.

- [ ] **Step 3: Implement DB model and helpers**

In `db/db.go`, add:

```go
type AIAffinity struct {
	ID         uint   `gorm:"primarykey;autoIncrement"`
	UserID     int64  `gorm:"uniqueIndex:idx_ai_affinity"`
	GroupID    int64  `gorm:"uniqueIndex:idx_ai_affinity"`
	Nickname   string `gorm:"size:64"`
	Score      int    `gorm:"default:50"`
	LastReason string `gorm:"size:64"`
	UpdatedAt  int64
}
```

Add `&AIAffinity{}` to `allModels`.

Add helpers:

```go
func GetAIAffinity(userID, groupID int64) (*AIAffinity, error) {
	var r AIAffinity
	err := DB.Where("user_id = ? AND group_id = ?", userID, groupID).First(&r).Error
	return &r, err
}

func UpdateAIAffinity(userID, groupID int64, nickname string, initial, delta, minScore, maxScore int, reason string) (*AIAffinity, error) {
	var out AIAffinity
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(AIAffinity{UserID: userID, GroupID: groupID}).
			Attrs(AIAffinity{Score: initial}).
			FirstOrCreate(&out).Error; err != nil {
			return err
		}
		if nickname != "" && out.Nickname != nickname {
			out.Nickname = nickname
		}
		out.Score += delta
		if out.Score < minScore {
			out.Score = minScore
		}
		if out.Score > maxScore {
			out.Score = maxScore
		}
		out.LastReason = reason
		out.UpdatedAt = time.Now().Unix()
		return tx.Save(&out).Error
	})
	return &out, err
}
```

- [ ] **Step 4: Run tests and verify GREEN**

Run:

```powershell
go test ./db -run AIAffinity -count=1
```

Expected: PASS.

## Task 3: Wire Dispatch Behavior

**Files:**
- Modify: `ai/dispatcher.go`
- Test: `ai/affinity_test.go`

- [ ] **Step 1: Add prompt composition test**

Append to `ai/affinity_test.go`:

```go
func TestBuildSystemPromptIncludesAffinityLevelWithoutScore(t *testing.T) {
	prompt := buildSystemPrompt(1, 100, "当前关系：普通。保持自然友好。")
	if !strings.Contains(prompt, "当前关系：普通") {
		t.Fatalf("prompt missing affinity level: %q", prompt)
	}
	if strings.Contains(prompt, "score") || strings.Contains(prompt, "50") {
		t.Fatalf("prompt should not expose hidden score: %q", prompt)
	}
}
```

- [ ] **Step 2: Run test and verify RED**

Run:

```powershell
go test ./ai -run BuildSystemPromptIncludesAffinity -count=1
```

Expected: FAIL because `buildSystemPrompt` still takes only `(userID, groupID int64)`.

- [ ] **Step 3: Modify prompt signature**

Change `buildSystemPrompt` in `ai/dispatcher.go`:

```go
func buildSystemPrompt(userID, groupID int64, affinity string) string {
	base := fmt.Sprintf(
		"你是%s，一个活泼可爱的QQ群助手。请用简洁自然的中文回复，不要过度解释。"+
			"有合适的工具时优先调用工具，不要在没有工具的情况下凭空捏造信息。",
		config.C.Bot.Name,
	)
	if affinity != "" {
		base += affinity
	}
	return base + UserContext(userID) + GroupContext(groupID)
}
```

Update the call site to pass the current affinity prompt.

- [ ] **Step 4: Add dispatch helper**

In `ai/affinity.go`, add:

```go
func UpdateChatAffinity(userID, groupID int64, nickname, text string) (int, bool) {
	cfg := NormalizeAffinityConfig(config.C.AI.Affinity)
	if !cfg.Enabled {
		return cfg.Initial, true
	}
	ev := ClassifyAffinityDelta(text)
	row, err := db.UpdateAIAffinity(userID, groupID, nickname, cfg.Initial, ev.Delta, cfg.Min, cfg.Max, ev.Reason)
	if err != nil {
		logx.Warnf("[ai] affinity update failed user=%d group=%d: %v", userID, groupID, err)
		return cfg.Initial, true
	}
	return row.Score, row.Score >= cfg.BlockBelow
}
```

Add imports for `db` and `services/logx`.

- [ ] **Step 5: Wire `ai.Dispatch`**

In `ai/dispatcher.go`, after `Guard` and before session setup:

```go
score, allowedByAffinity := UpdateChatAffinity(userID, groupID, event.Sender.Nickname, text)
affinityPrompt := AffinityPrompt(score, config.C.AI.Affinity)
if !allowedByAffinity {
	return "", nil
}
```

Use:

```go
Content: buildSystemPrompt(userID, groupID, affinityPrompt),
```

- [ ] **Step 6: Run targeted tests**

Run:

```powershell
go test ./ai -run "Affinity|BuildSystemPrompt" -count=1
```

Expected: PASS.

## Task 4: Remove Public Affinity and PK

**Files:**
- Delete: `plugins/game/pk.go`
- Delete: `plugins/funny/chat.go`
- Modify: `cmd/bot/main.go`
- Modify: `ai/tools/social.go`
- Modify: `plugins/system/help.go`
- Modify: `db/db.go`

- [ ] **Step 1: Remove registrations**

In `cmd/bot/main.go`, delete:

```go
game.RegisterPK(b)
funny.RegisterChat(b)
```

- [ ] **Step 2: Delete command files**

Delete:

```text
plugins/game/pk.go
plugins/funny/chat.go
```

- [ ] **Step 3: Remove old AI ranking tool**

In `ai/tools/social.go`, remove `registerAffinityRanking()` from `init()` and delete the whole `registerAffinityRanking` function. Remove the `github.com/Yuelioi/yueling-go/db` import if it becomes unused.

- [ ] **Step 4: Remove PK DB helper**

In `db/db.go`, delete `UpdatePKResult`.

- [ ] **Step 5: Remove help entries**

In `plugins/system/help.go`, delete the `pluginEntry` blocks with names:

```text
PK对战
好感度
```

- [ ] **Step 6: Run compile tests**

Run:

```powershell
go test ./... -count=1
```

Expected: PASS; no references to `RegisterPK`, `RegisterChat`, `UpdatePKResult`, or `registerAffinityRanking`.

## Task 5: Documentation and Config Example

**Files:**
- Modify: `README.md`
- Modify: `config.example.toml`

- [ ] **Step 1: Update config example**

Add under `[ai.context]` in `config.example.toml`:

```toml
# AI 聊天隐藏好感度。只影响 AI 回复态度/是否静默，不提供用户查询命令。
[ai.affinity]
enabled     = true
initial     = 50
block_below = 10
min         = 0
max         = 100
```

- [ ] **Step 2: Update README**

Remove the `pk @某人` row and the public `查看好感度` row/section wording. Add a short config note near the AI config section:

```markdown
# AI 聊天隐藏好感度。只影响 AI 回复态度/是否静默，不提供用户查询命令。
[ai.affinity]
enabled     = true
initial     = 50
block_below = 10
min         = 0
max         = 100
```

- [ ] **Step 3: Verify removed public strings**

Run:

```powershell
rg -n "pk @|PK对战|查看好感度|查询好感度|好感度排行|affinity_ranking|RegisterPK|RegisterChat|UpdatePKResult" README.md plugins ai db cmd
```

Expected: no matches except intentional internal type/function names containing `AIAffinity`.

## Task 6: Final Verification

**Files:**
- All modified files.

- [ ] **Step 1: Format**

Run:

```powershell
gofmt -w ai\\affinity.go ai\\affinity_test.go db\\db.go db\\ai_affinity_test.go ai\\dispatcher.go ai\\tools\\social.go cmd\\bot\\main.go plugins\\system\\help.go
```

Expected: no output.

- [ ] **Step 2: Full test suite**

Run:

```powershell
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 3: Final grep**

Run:

```powershell
rg -n "RegisterPK|UpdatePKResult|查看好感度|查询好感度|affinity_ranking|好感度排行" .
```

Expected: only Flightdeck plan/design references remain until the work package is archived; no runtime code or README references.

- [ ] **Step 4: Commit implementation**

Run:

```powershell
git add ai db config plugins cmd README.md config.example.toml
git commit -m "feat(ai): make affinity hidden chat state"
```

Expected: commit created.

## Self-Review

- Spec coverage: PK removal, hidden DB state, no user query, AI dispatch blocking, prompt embedding, docs/config updates are each covered.
- Placeholder scan: no unfinished placeholder entries remain.
- Type consistency: `AIAffinity`, `AffinityConfig`, `ClassifyAffinityDelta`, `ApplyAffinityDelta`, `AffinityPrompt`, and `UpdateChatAffinity` names are consistent across tasks.
