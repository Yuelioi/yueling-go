# WebUI Admin Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a password-protected WebUI that starts with the bot process and manages per-group plugin disables plus AI affinity scores.

**Architecture:** Go owns production serving: Gin exposes `/api/webui/*` plus `webui/dist` static files, and the Vue SPA talks to those APIs. Runtime management state lives in SQLite. Bot handler registrations carry help-system plugin IDs, and dispatch checks a single plugin gate before executing matched handlers.

**Tech Stack:** Go 1.25, Gin, Gorm/SQLite, Vue 3, TypeScript, Vite 8, Nuxt UI for Vue, Tabler icons via Iconify, pnpm.

---

## References Checked

- Vite 8 is the current major according to the official Vite 8 announcement: https://vite.dev/blog/announcing-vite8
- Nuxt UI supports standalone Vue/Vite apps through its Vite plugin and Vue plugin: https://ui.nuxt.com/docs/getting-started/installation/vue
- Vue's official TypeScript setup points to create-vue / Vite-powered TypeScript projects: https://vuejs.org/guide/typescript/overview

## File Structure

- Modify `config/config.go` for `[webui]`.
- Modify `config.example.toml` and `README.md` to document WebUI configuration.
- Modify `docker-compose.yml` to show an optional WebUI port mapping.
- Modify `.gitignore` to ignore `webui/node_modules` and keep `webui/dist` committed only if the project chooses to ship built assets in git. For this plan, ignore `webui/node_modules` only; keep `webui/dist` out of source until release packaging decides otherwise.
- Create `config/webui_test.go` for config validation tests.
- Create `db/webui.go` and `db/webui_test.go` for admin persistence functions.
- Modify `db/db.go` to add `GroupPluginDisabled` to `allModels`.
- Create `plugins/catalog/ids.go` for stable plugin ID constants.
- Modify `plugins/system/help.go` to export a catalog DTO and avoid duplicate registry finalization.
- Modify `plugins/system/help_image.go` to accept the exported catalog entry type where needed.
- Modify `bot/handler.go`, `bot/bot.go`, and `bot/api.go` for plugin metadata, plugin gate checks, and `GetGroupList`.
- Create `bot/plugin_gate_test.go` and extend `bot/conn_test.go` or create `bot/group_list_test.go`.
- Modify plugin registration files listed in Task 5 to call `.Plugin(...)`.
- Create `services/webui/server.go`, `services/webui/auth.go`, `services/webui/api.go`, `services/webui/static.go`, and tests under `services/webui/`.
- Modify `cmd/bot/main.go` to set the plugin gate and start the WebUI server.
- Create `webui/` frontend app: `package.json`, `pnpm-lock.yaml`, `index.html`, `vite.config.ts`, `tsconfig*.json`, and `src/`.

---

### Task 1: WebUI Config

**Files:**
- Modify: `config/config.go`
- Modify: `config.example.toml`
- Modify: `README.md`
- Modify: `docker-compose.yml`
- Create: `config/webui_test.go`

- [x] **Step 1: Write failing config tests**

Create `config/webui_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func baseConfig(extra string) string {
	return strings.TrimSpace(`
[bot]
name = "月灵"
data_dir = "data"

[napcat]
serve = ":9077"
token = ""

[ai]
deepseek_key = "sk-test"
`+"\n"+extra) + "\n"
}

func TestLoadWebUIDisabledAllowsEmptyPassword(t *testing.T) {
	path := writeConfig(t, baseConfig(`
[webui]
enabled = false
addr = ":9080"
password = ""
`))
	if err := Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if C.WebUI.Enabled {
		t.Fatalf("WebUI.Enabled = true, want false")
	}
}

func TestLoadWebUIEnabledRequiresPassword(t *testing.T) {
	path := writeConfig(t, baseConfig(`
[webui]
enabled = true
addr = ":9080"
password = ""
`))
	err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "webui.password is required") {
		t.Fatalf("Load() error = %v, want webui.password required", err)
	}
}

func TestLoadWebUIEnabledAcceptsPassword(t *testing.T) {
	path := writeConfig(t, baseConfig(`
[webui]
enabled = true
addr = ":9080"
password = "secret"
`))
	if err := Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !C.WebUI.Enabled || C.WebUI.Addr != ":9080" || C.WebUI.Password != "secret" {
		t.Fatalf("C.WebUI = %+v", C.WebUI)
	}
}
```

- [x] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./config -run WebUI -v
```

Expected: FAIL because `Config.WebUI` and `WebUIConfig` do not exist.

- [x] **Step 3: Implement config support**

In `config/config.go`, add `WebUI` to `Config`:

```go
type Config struct {
	Bot     BotConfig     `mapstructure:"bot"`
	NapCat  NapCatConfig  `mapstructure:"napcat"`
	AI      AIConfig      `mapstructure:"ai"`
	Tools   ToolsConfig   `mapstructure:"tools"`
	HTTPAPI HTTPAPIConfig `mapstructure:"http_api"`
	WebUI   WebUIConfig   `mapstructure:"webui"`
	Image   ImageConfig   `mapstructure:"image"`
	Pack    PackConfig    `mapstructure:"pack"`
}
```

Add the config type near `HTTPAPIConfig`:

```go
type WebUIConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
}
```

At the start of `Load`, reset Viper so tests and repeated loads do not leak settings:

```go
func Load(path string) error {
	viper.Reset()
	viper.SetConfigFile(path)
	viper.SetConfigType("toml")
	// existing defaults...
	viper.SetDefault("webui.addr", ":9080")
```

In `validate`, add:

```go
	if c.WebUI.Enabled {
		if c.WebUI.Addr == "" {
			c.WebUI.Addr = ":9080"
		}
		if c.WebUI.Password == "" {
			return fmt.Errorf("webui.password is required when webui.enabled is true")
		}
	}
```

- [x] **Step 4: Document config**

In `config.example.toml`, add after `[http_api]`:

```toml
[webui]
# Bot 内置管理后台。enabled=false 时不监听端口；enabled=true 时 password 必填。
enabled  = false
addr     = ":9080"
password = ""  # 单管理员密码；建议只在可信网络或反向代理鉴权后暴露
```

In `README.md`, add `[webui]` to the config example with the same fields and add a short "WebUI 管理后台" subsection:

```markdown
## WebUI 管理后台

启用 `[webui]` 后，Bot 进程会同时启动一个密码保护的管理后台，用于每群插件禁用和 AI 好感度分数管理。运行期管理数据写入 SQLite，不会写回 `config.toml`。
```

In `docker-compose.yml`, add a commented port near the existing bot ports:

```yaml
      # - "9080:9080"  # WebUI 管理后台（启用 [webui] 后再暴露；建议只在可信网络访问）
```

- [x] **Step 5: Run tests**

Run:

```bash
go test ./config -run WebUI -v
```

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add config/config.go config/webui_test.go config.example.toml README.md docker-compose.yml
git commit -m "feat(config): add webui settings"
```

---

### Task 2: DB Models And Admin Mutations

**Files:**
- Modify: `db/db.go`
- Create: `db/webui.go`
- Create: `db/webui_test.go`

- [x] **Step 1: Write failing DB tests**

Create `db/webui_test.go`:

```go
package db

import "testing"

func TestGroupPluginDisabledCRUDAndBatch(t *testing.T) {
	initTempAIAffinityDB(t)

	disabled, err := IsGroupPluginDisabled(100, 29)
	if err != nil {
		t.Fatalf("initial disabled: %v", err)
	}
	if disabled {
		t.Fatalf("disabled = true, want false")
	}

	if err := SetGroupPluginDisabled(100, 29, true); err != nil {
		t.Fatalf("disable: %v", err)
	}
	disabled, err = IsGroupPluginDisabled(100, 29)
	if err != nil || !disabled {
		t.Fatalf("disabled=%v err=%v, want true nil", disabled, err)
	}

	if err := SetGroupPluginDisabled(100, 29, false); err != nil {
		t.Fatalf("enable: %v", err)
	}
	disabled, err = IsGroupPluginDisabled(100, 29)
	if err != nil || disabled {
		t.Fatalf("disabled=%v err=%v, want false nil", disabled, err)
	}

	if err := SetPluginDisabledForGroups(34, []int64{100, 200}, true); err != nil {
		t.Fatalf("batch disable: %v", err)
	}
	for _, groupID := range []int64{100, 200} {
		disabled, err := IsGroupPluginDisabled(groupID, 34)
		if err != nil || !disabled {
			t.Fatalf("group %d disabled=%v err=%v, want true nil", groupID, disabled, err)
		}
	}
}

func TestAIAffinityAdminListAndMutations(t *testing.T) {
	initTempAIAffinityDB(t)

	if _, err := UpdateAIAffinity(1, 100, "alice", 50, 5, 0, 100, "normal"); err != nil {
		t.Fatalf("seed alice: %v", err)
	}
	if _, err := UpdateAIAffinity(2, 100, "bob", 50, -20, 0, 100, "bad"); err != nil {
		t.Fatalf("seed bob: %v", err)
	}
	if _, err := UpdateAIAffinity(3, 200, "carol", 50, 1, 0, 100, "normal"); err != nil {
		t.Fatalf("seed carol: %v", err)
	}

	rows, err := ListAIAffinityAdmin(100, "ali", 20)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || rows[0].Nickname != "alice" {
		t.Fatalf("rows = %+v, want alice only", rows)
	}

	row, err := SetAIAffinityScore(rows[0].ID, 999, 0, 100, "webui_set")
	if err != nil {
		t.Fatalf("set score: %v", err)
	}
	if row.Score != 100 || row.LastReason != "webui_set" {
		t.Fatalf("after set = %+v, want score 100 reason webui_set", row)
	}

	row, err = AdjustAIAffinityScore(row.ID, -150, 0, 100, "webui_adjust")
	if err != nil {
		t.Fatalf("adjust score: %v", err)
	}
	if row.Score != 0 || row.LastReason != "webui_adjust" {
		t.Fatalf("after adjust = %+v, want score 0 reason webui_adjust", row)
	}

	row, err = ResetAIAffinityScore(row.ID, 50, 0, 100, "webui_reset")
	if err != nil {
		t.Fatalf("reset score: %v", err)
	}
	if row.Score != 50 || row.LastReason != "webui_reset" {
		t.Fatalf("after reset = %+v, want score 50 reason webui_reset", row)
	}
}
```

- [x] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./db -run "GroupPluginDisabled|AIAffinityAdmin" -v
```

Expected: FAIL because the DB model/functions do not exist.

- [x] **Step 3: Add model to DB**

In `db/db.go`, add:

```go
type GroupPluginDisabled struct {
	ID       uint  `gorm:"primarykey;autoIncrement"`
	GroupID  int64 `gorm:"uniqueIndex:idx_group_plugin_disabled"`
	PluginID int   `gorm:"uniqueIndex:idx_group_plugin_disabled"`
}
```

Add it to `allModels`:

```go
var allModels = []any{
	&AutoReply{}, &UserGameRecord{}, &AIAffinity{}, &Reminder{},
	&SemanticMemory{}, &EpisodicMemory{}, &ProceduralMemory{},
	&UserTag{}, &TodoItem{}, &UserProfile{},
	&GroupJoinRule{},
	&GroupPluginDisabled{},
}
```

- [x] **Step 4: Implement admin DB functions**

Create `db/webui.go`:

```go
package db

import (
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func IsGroupPluginDisabled(groupID int64, pluginID int) (bool, error) {
	var count int64
	err := DB.Model(&GroupPluginDisabled{}).
		Where("group_id = ? AND plugin_id = ?", groupID, pluginID).
		Count(&count).Error
	return count > 0, err
}

func GetDisabledPlugins(groupID int64) (map[int]bool, error) {
	var rows []GroupPluginDisabled
	if err := DB.Where("group_id = ?", groupID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[int]bool, len(rows))
	for _, row := range rows {
		out[row.PluginID] = true
	}
	return out, nil
}

func SetGroupPluginDisabled(groupID int64, pluginID int, disabled bool) error {
	if disabled {
		return DB.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&GroupPluginDisabled{GroupID: groupID, PluginID: pluginID}).Error
	}
	return DB.Where("group_id = ? AND plugin_id = ?", groupID, pluginID).
		Delete(&GroupPluginDisabled{}).Error
}

func SetPluginDisabledForGroups(pluginID int, groupIDs []int64, disabled bool) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		for _, groupID := range groupIDs {
			if disabled {
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
					Create(&GroupPluginDisabled{GroupID: groupID, PluginID: pluginID}).Error; err != nil {
					return err
				}
				continue
			}
			if err := tx.Where("group_id = ? AND plugin_id = ?", groupID, pluginID).
				Delete(&GroupPluginDisabled{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func ListAIAffinityAdmin(groupID int64, query string, limit int) ([]AIAffinity, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := DB.Model(&AIAffinity{})
	if groupID != 0 {
		q = q.Where("group_id = ?", groupID)
	}
	query = strings.TrimSpace(query)
	if query != "" {
		if userID, err := strconv.ParseInt(query, 10, 64); err == nil {
			q = q.Where("user_id = ?", userID)
		} else {
			q = q.Where("nickname LIKE ?", "%"+query+"%")
		}
	}
	var rows []AIAffinity
	err := q.Order("updated_at desc").Limit(limit).Find(&rows).Error
	return rows, err
}

func clampScore(score, minScore, maxScore int) int {
	if maxScore < minScore {
		maxScore = minScore
	}
	if score < minScore {
		return minScore
	}
	if score > maxScore {
		return maxScore
	}
	return score
}

func SetAIAffinityScore(id uint, score, minScore, maxScore int, reason string) (*AIAffinity, error) {
	score = clampScore(score, minScore, maxScore)
	now := time.Now().Unix()
	var row AIAffinity
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&AIAffinity{}).Where("id = ?", id).Updates(map[string]any{
			"score":       score,
			"last_reason": reason,
			"updated_at":  now,
		}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).First(&row).Error
	})
	return &row, err
}

func AdjustAIAffinityScore(id uint, delta, minScore, maxScore int, reason string) (*AIAffinity, error) {
	var current AIAffinity
	if err := DB.Where("id = ?", id).First(&current).Error; err != nil {
		return nil, err
	}
	return SetAIAffinityScore(id, current.Score+delta, minScore, maxScore, reason)
}

func ResetAIAffinityScore(id uint, initial, minScore, maxScore int, reason string) (*AIAffinity, error) {
	return SetAIAffinityScore(id, initial, minScore, maxScore, reason)
}
```

- [x] **Step 5: Run DB tests**

Run:

```bash
go test ./db -run "GroupPluginDisabled|AIAffinityAdmin" -v
```

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add db/db.go db/webui.go db/webui_test.go
git commit -m "feat(db): add webui admin state"
```

---

### Task 3: Export The Plugin Catalog

**Files:**
- Create: `plugins/catalog/ids.go`
- Modify: `plugins/system/help.go`
- Modify: `plugins/system/help_image.go`
- Create: `plugins/system/catalog_test.go`

- [x] **Step 1: Write failing catalog tests**

Create `plugins/system/catalog_test.go`:

```go
package system

import "testing"

func TestCatalogExportsStablePluginEntries(t *testing.T) {
	entries := Catalog()
	if len(entries) == 0 {
		t.Fatal("Catalog() returned no entries")
	}
	seen := map[int]bool{}
	var foundAI bool
	for _, entry := range entries {
		if entry.ID == 0 || entry.Name == "" || entry.Group == "" {
			t.Fatalf("invalid catalog entry: %+v", entry)
		}
		if seen[entry.ID] {
			t.Fatalf("duplicate plugin id %d", entry.ID)
		}
		seen[entry.ID] = true
		if entry.ID == 29 && entry.Name == "AI 助手" {
			foundAI = true
		}
	}
	if !foundAI {
		t.Fatalf("catalog missing AI 助手 entry")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./plugins/system -run Catalog -v
```

Expected: FAIL because `Catalog` does not exist.

- [x] **Step 3: Add stable plugin ID constants**

Create `plugins/catalog/ids.go`:

```go
package catalog

const (
	PluginBan          = 1
	PluginKeyword      = 2
	PluginJoinReview   = 3
	PluginMemberBackup = 4
	PluginRules        = 5
	PluginAutoReply    = 6
	PluginFiles        = 7
	PluginUserTags     = 8
	PluginCheckIn      = 9
	PluginJW3          = 11
	PluginClockIn      = 12
	PluginReminder     = 13
	PluginRandomMember = 14
	PluginRandomRename = 15
	PluginRoll         = 16
	PluginEmoticon     = 17
	PluginRandomImage  = 18
	PluginFortune      = 20
	PluginHot          = 21
	PluginSleep        = 23
	PluginPoke         = 24
	PluginTraceMoe     = 25
	PluginTranslate    = 26
	PluginLinkAnalysis = 28
	PluginAIAssistant  = 29
	PluginHelp         = 30
	PluginSystemTools  = 31
	PluginUploadAssets = 32
	PluginMemes        = 33
	PluginPack         = 34
	PluginZSSM         = 35
	PluginEssence      = 36
)
```

- [x] **Step 4: Export catalog entries from help registry**

In `plugins/system/help.go`, import `sync` and add:

```go
type PluginCatalogEntry struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Group    string   `json:"group"`
	Desc     string   `json:"desc"`
	Usage    string   `json:"usage"`
	Commands []string `json:"commands"`
}

var finalizeOnce sync.Once

func ensureRegistry() {
	finalizeOnce.Do(finalizeRegistry)
}

func Catalog() []PluginCatalogEntry {
	ensureRegistry()
	out := make([]PluginCatalogEntry, 0, len(pluginRegistry))
	for _, entry := range pluginRegistry {
		commands := append([]string(nil), entry.Commands...)
		out = append(out, PluginCatalogEntry{
			ID:       entry.ID,
			Name:     entry.Name,
			Group:    entry.Group,
			Desc:     entry.Desc,
			Usage:    entry.Usage,
			Commands: commands,
		})
	}
	return out
}
```

Change `RegisterHelp` to call `ensureRegistry()` instead of `finalizeRegistry()` directly:

```go
func RegisterHelp(b *bot.Bot) {
	ensureRegistry()
	// existing pre-render and handler registration...
```

Keep the existing `pluginEntry` literal values. Do not reword plugin names, descriptions, commands, or group names while doing this extraction.

- [x] **Step 5: Make help image code compile**

If `plugins/system/help_image.go` references `pluginEntry`, leave it unchanged if `pluginEntry` stays in `help.go`. If you choose to convert render functions to the exported type, update:

```go
func RenderHelpDetailImage(p *pluginEntry) ([]byte, error)
```

to:

```go
func RenderHelpDetailImage(p *PluginCatalogEntry) ([]byte, error)
```

Only make this conversion if it is needed by compilation. The lower-risk path is keeping `pluginEntry` internal and exporting copied DTOs through `Catalog()`.

- [x] **Step 6: Run tests**

Run:

```bash
go test ./plugins/system -run Catalog -v
go test ./plugins/system -v
```

Expected: PASS.

- [x] **Step 7: Commit**

```bash
git add plugins/catalog/ids.go plugins/system/help.go plugins/system/help_image.go plugins/system/catalog_test.go
git commit -m "feat(plugins): expose webui plugin catalog"
```

---

### Task 4: Bot Plugin Gate And Group List API

**Files:**
- Modify: `bot/handler.go`
- Modify: `bot/bot.go`
- Modify: `bot/api.go`
- Create: `bot/plugin_gate_test.go`
- Create: `bot/group_list_test.go`

- [x] **Step 1: Write failing plugin gate tests**

Create `bot/plugin_gate_test.go`:

```go
package bot

import (
	"encoding/json"
	"testing"
)

func textMessage(text string) Message {
	return Message{{Type: "text", Data: json.RawMessage(`{"text":` + strconvQuote(text) + `}`)}}
}

func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func testEvent(text string) *GroupMessageEvent {
	return &GroupMessageEvent{
		SelfID:  999,
		GroupID: 100,
		UserID:  1,
		Message: textMessage(text),
		Sender:  Sender{Nickname: "alice", Role: "member"},
	}
}

func TestPluginGateSkipsDisabledHandlerSilently(t *testing.T) {
	b := New()
	b.SetPluginGate(func(groupID int64, pluginID int) (bool, error) {
		return groupID == 100 && pluginID == 29, nil
	})

	called := false
	b.OnFullMatch("hello").Plugin(29).Handle(func(ctx *GroupContext) error {
		called = true
		return nil
	})

	b.dispatchGroupMessage(&BotAPI{}, testEvent("hello"))
	if called {
		t.Fatalf("disabled plugin handler was called")
	}
}

func TestPluginGateAllowsEnabledHandler(t *testing.T) {
	b := New()
	b.SetPluginGate(func(groupID int64, pluginID int) (bool, error) {
		return false, nil
	})

	called := false
	b.OnFullMatch("hello").Plugin(29).Handle(func(ctx *GroupContext) error {
		called = true
		return nil
	})

	b.dispatchGroupMessage(&BotAPI{}, testEvent("hello"))
	if !called {
		t.Fatalf("enabled plugin handler was not called")
	}
}

func TestPluginGateIgnoresUntaggedHandlers(t *testing.T) {
	b := New()
	b.SetPluginGate(func(groupID int64, pluginID int) (bool, error) {
		return true, nil
	})

	called := false
	b.OnFullMatch("hello").Handle(func(ctx *GroupContext) error {
		called = true
		return nil
	})

	b.dispatchGroupMessage(&BotAPI{}, testEvent("hello"))
	if !called {
		t.Fatalf("untagged handler should not be gated")
	}
}
```

- [x] **Step 2: Write failing group-list parser test**

Create `bot/group_list_test.go`:

```go
package bot

import (
	"encoding/json"
	"testing"
)

func TestParseGroupList(t *testing.T) {
	raw := json.RawMessage(`[{"group_id":100,"group_name":"A"},{"group_id":200,"group_name":"B"}]`)
	groups, err := parseGroupList(raw)
	if err != nil {
		t.Fatalf("parseGroupList() error = %v", err)
	}
	if len(groups) != 2 || groups[0].GroupID != 100 || groups[0].GroupName != "A" || groups[1].GroupID != 200 {
		t.Fatalf("groups = %+v", groups)
	}
}
```

- [x] **Step 3: Run tests to verify they fail**

Run:

```bash
go test ./bot -run "PluginGate|ParseGroupList" -v
```

Expected: FAIL because `.Plugin`, `SetPluginGate`, and `parseGroupList` do not exist.

- [x] **Step 4: Add plugin metadata and gate**

In `bot/handler.go`, extend `reg`:

```go
type reg struct {
	eventType  string
	matcher    Matcher
	conditions []Condition
	handler    any
	priority   int
	block      bool
	pluginID   int
}
```

Add builder method:

```go
func (b *Builder) Plugin(id int) *Builder {
	b.r.pluginID = id
	return b
}
```

In `bot/bot.go`, add:

```go
type PluginGate func(groupID int64, pluginID int) (bool, error)

type Bot struct {
	regs         []*reg
	connectHooks []func(*BotAPI)
	pluginGate   PluginGate
}

func (b *Bot) SetPluginGate(g PluginGate) {
	b.pluginGate = g
}

func (b *Bot) pluginDisabled(groupID int64, pluginID int) bool {
	if pluginID == 0 || b.pluginGate == nil {
		return false
	}
	disabled, err := b.pluginGate(groupID, pluginID)
	if err != nil {
		logx.Warnf("[plugin] disable check failed group=%d plugin=%d: %v", groupID, pluginID, err)
		return false
	}
	return disabled
}
```

In `dispatchGroupMessage`, after `commandMatched` is set and before conditions/handler execution, add:

```go
		if b.pluginDisabled(e.GroupID, r.pluginID) {
			continue
		}
```

Keep `commandMatched` assignment before the plugin gate so disabled command plugins still prevent low-priority passive handlers from treating the command as normal chat.

- [x] **Step 5: Add group list wrapper**

In `bot/api.go`, add:

```go
func parseGroupList(raw json.RawMessage) ([]GroupInfo, error) {
	var groups []GroupInfo
	if err := json.Unmarshal(raw, &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

func (a *BotAPI) GetGroupList() ([]GroupInfo, error) {
	raw, err := a.call("get_group_list", map[string]any{})
	if err != nil {
		return nil, err
	}
	return parseGroupList(raw)
}
```

- [x] **Step 6: Run bot tests**

Run:

```bash
go test ./bot -run "PluginGate|ParseGroupList" -v
```

Expected: PASS.

- [x] **Step 7: Commit**

```bash
git add bot/handler.go bot/bot.go bot/api.go bot/plugin_gate_test.go bot/group_list_test.go
git commit -m "feat(bot): add plugin gate"
```

---

### Task 5: Tag Existing Plugin Registrations

**Files:**
- Modify plugin files from the registration grep list.

- [x] **Step 1: Add plugin IDs to every managed registration**

For every `b.On...` registration that corresponds to a help catalog plugin, insert `.Plugin(catalog.PluginX)` in the builder chain and import:

```go
	"github.com/Yuelioi/yueling-go/plugins/catalog"
```

Use this exact mapping:

```text
cmd/bot/main.go ping smoke test        -> catalog.PluginSystemTools
plugins/group/ban.go ban/unban/kick    -> catalog.PluginBan
plugins/group/ban.go revoke            -> catalog.PluginBan
plugins/group/ban.go muteall           -> catalog.PluginBan
plugins/group/essence.go               -> catalog.PluginEssence
plugins/group/keyword.go passive reply -> catalog.PluginAutoReply
plugins/system/reply.go                -> catalog.PluginAutoReply
plugins/system/rules.go                -> catalog.PluginRules
plugins/group/join_review.go request + commands -> catalog.PluginJoinReview
plugins/group/member_backup.go         -> catalog.PluginMemberBackup
plugins/group/files.go                 -> catalog.PluginFiles
plugins/user/tag.go                    -> catalog.PluginUserTags
plugins/game/checkin.go 签到/积分/排行 -> catalog.PluginCheckIn
plugins/game/jw3.go                    -> catalog.PluginJW3
plugins/memo/plugin.go                 -> catalog.PluginReminder
plugins/random/member.go               -> catalog.PluginRandomMember
plugins/random/rename.go               -> catalog.PluginRandomRename
plugins/random/roll.go                 -> catalog.PluginRoll
plugins/emoticon/emoticon.go 添加表情 -> catalog.PluginUploadAssets
plugins/emoticon/emoticon.go passive space trigger -> catalog.PluginEmoticon
plugins/image/image.go random/external calls -> catalog.PluginRandomImage
plugins/image/image.go add commands    -> catalog.PluginUploadAssets
plugins/quotation/quotation.go 语录     -> catalog.PluginRandomImage
plugins/quotation/quotation.go 添加语录 -> catalog.PluginUploadAssets
plugins/funny/fortune.go               -> catalog.PluginFortune
plugins/funny/hot.go                   -> catalog.PluginHot
plugins/funny/sleep.go                 -> catalog.PluginSleep
plugins/funny/poke.go                  -> catalog.PluginPoke
plugins/funny/trace_moe.go             -> catalog.PluginTraceMoe
plugins/funny/memes.go                 -> catalog.PluginMemes
plugins/funny/repeater.go              -> catalog.PluginEmoticon only if WebUI should treat repeater as part of "表情包"; otherwise leave untagged for this first version
plugins/tools/translate.go             -> catalog.PluginTranslate
plugins/tools/link_analysis.go         -> catalog.PluginLinkAnalysis
plugins/tools/clockin.go               -> catalog.PluginClockIn
plugins/tools/zssm.go                  -> catalog.PluginZSSM
plugins/tools/pack.go                  -> catalog.PluginPack
plugins/ai_dispatch/plugin.go          -> catalog.PluginAIAssistant
plugins/ai_proactive/plugin.go         -> catalog.PluginAIAssistant
plugins/system/help.go                 -> catalog.PluginHelp
plugins/system/reboot.go               -> catalog.PluginSystemTools
```

For image dynamic registrations, the code should look like:

```go
b.OnFullMatch(e.Call...).Plugin(catalog.PluginRandomImage).Handle(...)
b.OnCommand(add).Plugin(catalog.PluginUploadAssets).Handle(...)
```

For AI:

```go
b.OnGroupMessage(aiTrigger{}).
	Plugin(catalog.PluginAIAssistant).
	Priority(1).
	Handle(...)
```

and:

```go
b.OnGroupMessage().
	Plugin(catalog.PluginAIAssistant).
	Priority(0).
	Handle(...)
```

- [x] **Step 2: Run a grep sanity check**

Run:

```bash
rg -n "b\\.On(Command|GroupMessage|Keyword|Regex|FullMatch|Notice|Request)" plugins cmd\\bot\\main.go
```

Expected: each managed registration has `.Plugin(...)` in its builder chain. Any untagged registration must be intentionally unmanageable in WebUI; for the first version, acceptable untagged registrations are only low-level internals that are not in the help catalog.

- [x] **Step 3: Run plugin compile tests**

Run:

```bash
go test ./plugins/... ./cmd/bot
```

Expected: PASS.

- [x] **Step 4: Commit**

```bash
git add cmd/bot/main.go plugins
git commit -m "feat(plugins): tag handlers for webui gating"
```

---

### Task 6: WebUI Server Auth And Static Shell

**Files:**
- Run: `go get github.com/gin-gonic/gin@latest`
- Create: `services/webui/server.go`
- Create: `services/webui/auth.go`
- Create: `services/webui/static.go`
- Create: `services/webui/server_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

- [x] **Step 1: Add Gin dependency**

Run:

```bash
go get github.com/gin-gonic/gin@latest
```

Expected: `go.mod` and `go.sum` update.

- [x] **Step 2: Write failing auth tests**

Create `services/webui/server_test.go`:

```go
package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

func newTestServer() *Server {
	return New(config.WebUIConfig{Enabled: true, Addr: ":0", Password: "secret"})
}

func TestLoginSetsSessionCookie(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/webui/auth/login", strings.NewReader(`{"password":"secret"}`))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Fatalf("login did not set a session cookie")
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/webui/auth/login", strings.NewReader(`{"password":"wrong"}`))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestProtectedRouteRequiresSession(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/webui/auth/me", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}
```

- [x] **Step 3: Run tests to verify they fail**

Run:

```bash
go test ./services/webui -run "Login|Protected" -v
```

Expected: FAIL because `services/webui` does not exist.

- [x] **Step 4: Implement server skeleton**

Create `services/webui/server.go`:

```go
package webui

import (
	"net/http"
	"sync/atomic"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/logx"
	"github.com/gin-gonic/gin"
)

type Server struct {
	cfg     config.WebUIConfig
	current atomic.Pointer[bot.BotAPI]
	sessions *sessionStore
}

func New(cfg config.WebUIConfig) *Server {
	gin.SetMode(gin.ReleaseMode)
	return &Server{
		cfg:      cfg,
		sessions: newSessionStore(),
	}
}

func (s *Server) BindBot(b *bot.Bot) {
	b.OnConnect(func(api *bot.BotAPI) { s.current.Store(api) })
}

func (s *Server) Handler() http.Handler {
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api/webui")
	api.POST("/auth/login", s.handleLogin)
	protected := api.Group("")
	protected.Use(s.requireSession)
	protected.POST("/auth/logout", s.handleLogout)
	protected.GET("/auth/me", s.handleMe)

	s.mountStatic(r)
	return r
}

func (s *Server) Start(addr string) {
	logx.Infof("[webui] serving on %s", addr)
	if err := http.ListenAndServe(addr, s.Handler()); err != nil {
		logx.Fatalf("[webui] server error: %v", err)
	}
}
```

Create `services/webui/auth.go`:

```go
package webui

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const sessionCookieName = "yueling_webui_session"
const sessionTTL = 24 * time.Hour

type sessionStore struct {
	mu       sync.Mutex
	sessions map[string]time.Time
}

func newSessionStore() *sessionStore {
	return &sessionStore{sessions: map[string]time.Time{}}
}

func (s *sessionStore) create() string {
	var raw [32]byte
	_, _ = rand.Read(raw[:])
	token := hex.EncodeToString(raw[:])
	s.mu.Lock()
	s.sessions[token] = time.Now().Add(sessionTTL)
	s.mu.Unlock()
	return token
}

func (s *sessionStore) valid(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	expires, ok := s.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(expires) {
		delete(s.sessions, token)
		return false
	}
	return true
}

func (s *sessionStore) delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

func (s *Server) handleLogin(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid json"})
		return
	}
	if req.Password != s.cfg.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "unauthorized"})
		return
	}
	token := s.sessions.create()
	c.SetCookie(sessionCookieName, token, int(sessionTTL.Seconds()), "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleLogout(c *gin.Context) {
	if token, err := c.Cookie(sessionCookieName); err == nil {
		s.sessions.delete(token)
	}
	c.SetCookie(sessionCookieName, "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleMe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "authenticated": true})
}

func (s *Server) requireSession(c *gin.Context) {
	token, err := c.Cookie(sessionCookieName)
	if err != nil || !s.sessions.valid(token) {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "unauthorized"})
		c.Abort()
		return
	}
	c.Next()
}
```

Create `services/webui/static.go`:

```go
package webui

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) mountStatic(r *gin.Engine) {
	dist := filepath.Join("webui", "dist")
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "not found"})
			return
		}
		path := filepath.Join(dist, filepath.Clean(c.Request.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			c.File(path)
			return
		}
		index := filepath.Join(dist, "index.html")
		if _, err := os.Stat(index); err == nil {
			c.File(index)
			return
		}
		c.String(http.StatusNotFound, "webui frontend is not built")
	})
}
```

- [x] **Step 5: Run auth tests**

Run:

```bash
go test ./services/webui -run "Login|Protected" -v
```

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add go.mod go.sum services/webui
git commit -m "feat(webui): add authenticated server"
```

---

### Task 7: WebUI JSON APIs

**Files:**
- Modify: `services/webui/server.go`
- Create or modify: `services/webui/api.go`
- Create: `services/webui/api_test.go`

- [x] **Step 1: Write failing API tests for protected behavior and offline groups**

Create `services/webui/api_test.go`:

```go
package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func loginCookie(t *testing.T, s *Server) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/webui/auth/login", strings.NewReader(`{"password":"secret"}`))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login code=%d body=%s", rec.Code, rec.Body.String())
	}
	return rec.Result().Cookies()[0]
}

func TestGroupsRequireLiveBot(t *testing.T) {
	s := newTestServer()
	cookie := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodGet, "/api/webui/groups", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPluginsRequiresSession(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/webui/plugins", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}
```

- [x] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./services/webui -run "Groups|Plugins" -v
```

Expected: FAIL because routes do not exist.

- [x] **Step 3: Register API routes**

In `services/webui/server.go`, inside the protected group, add:

```go
	protected.GET("/groups", s.handleGroups)
	protected.GET("/plugins", s.handlePlugins)
	protected.GET("/groups/:groupID/plugins", s.handleGroupPlugins)
	protected.PUT("/groups/:groupID/plugins/:pluginID", s.handleSetGroupPlugin)
	protected.POST("/plugins/:pluginID/apply-all", s.handleApplyPluginAll)
	protected.GET("/affinity", s.handleAffinityList)
	protected.PUT("/affinity/:id/score", s.handleAffinitySetScore)
	protected.POST("/affinity/:id/adjust", s.handleAffinityAdjust)
	protected.POST("/affinity/:id/reset", s.handleAffinityReset)
```

- [x] **Step 4: Implement API handlers**

Create `services/webui/api.go`:

```go
package webui

import (
	"net/http"
	"strconv"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	systemplugin "github.com/Yuelioi/yueling-go/plugins/system"
	"github.com/gin-gonic/gin"
)

func parseInt64Param(c *gin.Context, name string) (int64, bool) {
	v, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || v == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid " + name})
		return 0, false
	}
	return v, true
}

func parseIntParam(c *gin.Context, name string) (int, bool) {
	v, err := strconv.Atoi(c.Param(name))
	if err != nil || v == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid " + name})
		return 0, false
	}
	return v, true
}

func (s *Server) liveAPI(c *gin.Context) *bot.BotAPI {
	api := s.current.Load()
	if api == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "error": "bot not connected"})
		return nil
	}
	return api
}
```

Add `bot` import to the file:

```go
	"github.com/Yuelioi/yueling-go/bot"
```

Then implement handlers:

```go
func (s *Server) handleGroups(c *gin.Context) {
	api := s.liveAPI(c)
	if api == nil {
		return
	}
	groups, err := api.GetGroupList()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "groups": groups})
}

func (s *Server) handlePlugins(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "plugins": systemplugin.Catalog()})
}

func (s *Server) handleGroupPlugins(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	disabled, err := db.GetDisabledPlugins(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "disabled": disabled})
}

func (s *Server) handleSetGroupPlugin(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	pluginID, ok := parseIntParam(c, "pluginID")
	if !ok {
		return
	}
	var req struct {
		Disabled bool `json:"disabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid json"})
		return
	}
	if err := db.SetGroupPluginDisabled(groupID, pluginID, req.Disabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleApplyPluginAll(c *gin.Context) {
	pluginID, ok := parseIntParam(c, "pluginID")
	if !ok {
		return
	}
	var req struct {
		GroupIDs  []int64 `json:"group_ids"`
		Disabled bool    `json:"disabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.GroupIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "group_ids required"})
		return
	}
	if err := db.SetPluginDisabledForGroups(pluginID, req.GroupIDs, req.Disabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
```

For affinity handlers, normalize config through existing AI helper:

```go
func affinityBounds() (initial, minScore, maxScore int) {
	cfg := ai.NormalizeAffinityConfig(config.C.AI.Affinity)
	return cfg.Initial, cfg.Min, cfg.Max
}

func (s *Server) handleAffinityList(c *gin.Context) {
	groupID, _ := strconv.ParseInt(c.Query("group_id"), 10, 64)
	rows, err := db.ListAIAffinityAdmin(groupID, c.Query("q"), 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok": true,
		"affinity": rows,
		"block_below": ai.NormalizeAffinityConfig(config.C.AI.Affinity).BlockBelow,
	})
}

func (s *Server) handleAffinitySetScore(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid id"})
		return
	}
	var req struct {
		Score int `json:"score"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid json"})
		return
	}
	_, minScore, maxScore := affinityBounds()
	row, err := db.SetAIAffinityScore(uint(id64), req.Score, minScore, maxScore, "webui_set")
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "affinity": row})
}

func (s *Server) handleAffinityAdjust(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid id"})
		return
	}
	var req struct {
		Delta int `json:"delta"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Delta == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "delta required"})
		return
	}
	_, minScore, maxScore := affinityBounds()
	row, err := db.AdjustAIAffinityScore(uint(id64), req.Delta, minScore, maxScore, "webui_adjust")
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "affinity": row})
}

func (s *Server) handleAffinityReset(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid id"})
		return
	}
	initial, minScore, maxScore := affinityBounds()
	row, err := db.ResetAIAffinityScore(uint(id64), initial, minScore, maxScore, "webui_reset")
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "affinity": row})
}
```

- [x] **Step 5: Run service tests**

Run:

```bash
go test ./services/webui -v
```

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add services/webui
git commit -m "feat(webui): add admin api"
```

---

### Task 8: Wire WebUI Into Bot Startup

**Files:**
- Modify: `cmd/bot/main.go`

- [x] **Step 1: Wire plugin gate after bot construction**

In `cmd/bot/main.go`, add import:

```go
	"github.com/Yuelioi/yueling-go/services/webui"
```

After `b := bot.New()`, add:

```go
	b.SetPluginGate(db.IsGroupPluginDisabled)
```

- [x] **Step 2: Start WebUI when enabled**

After the external HTTP API block and before connecting to NapCat, add:

```go
	if config.C.WebUI.Enabled {
		srv := webui.New(config.C.WebUI)
		srv.BindBot(b)
		go srv.Start(config.C.WebUI.Addr)
		logx.Infof("[webui] enabled on %s", config.C.WebUI.Addr)
	}
```

This uses `logx`, not stdlib `log`.

- [x] **Step 3: Run compile tests**

Run:

```bash
go test ./cmd/bot ./bot ./services/webui -v
```

Expected: PASS.

- [x] **Step 4: Commit**

```bash
git add cmd/bot/main.go
git commit -m "feat(bot): start webui with bot"
```

---

### Task 9: Frontend Scaffold

**Files:**
- Modify: `.gitignore`
- Create: `webui/package.json`
- Create: `webui/pnpm-lock.yaml`
- Create: `webui/index.html`
- Create: `webui/vite.config.ts`
- Create: `webui/tsconfig.json`
- Create: `webui/tsconfig.app.json`
- Create: `webui/src/main.ts`
- Create: `webui/src/App.vue`
- Create: `webui/src/router.ts`
- Create: `webui/src/assets/main.css`
- Create: `webui/src/api.ts`

- [x] **Step 1: Scaffold Vue TypeScript app**

Run:

```bash
pnpm create vite webui --template vue-ts
cd webui
pnpm add @nuxt/ui tailwindcss vue-router @iconify-json/tabler
pnpm install
```

If `pnpm create vite` asks for confirmation, choose the Vue + TypeScript template. Keep the generated TypeScript config files.

- [x] **Step 2: Configure Nuxt UI for Vue/Vite**

In `webui/vite.config.ts`:

```ts
import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import ui from '@nuxt/ui/vite'

export default defineConfig({
  plugins: [vue(), ui()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    proxy: {
      '/api/webui': 'http://127.0.0.1:9080',
    },
  },
})
```

Create `webui/src/assets/main.css`:

```css
@import "tailwindcss";
@import "@nuxt/ui";

html,
body,
#app {
  min-height: 100%;
}

body {
  margin: 0;
}
```

In `webui/src/main.ts`:

```ts
import './assets/main.css'

import { createApp } from 'vue'
import ui from '@nuxt/ui/vue-plugin'
import App from './App.vue'
import { router } from './router'

const app = createApp(App)
app.use(router)
app.use(ui)
app.mount('#app')
```

- [x] **Step 3: Add typed API client**

Create `webui/src/api.ts`:

```ts
export interface GroupInfo {
  group_id: number
  group_name: string
}

export interface PluginEntry {
  id: number
  name: string
  group: string
  desc: string
  usage: string
  commands: string[]
}

export interface AffinityRow {
  ID: number
  UserID: number
  GroupID: number
  Nickname: string
  Score: number
  LastReason: string
  UpdatedAt: number
}

async function request<T>(url: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(url, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...(init.headers || {}) },
    ...init,
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error(data.error || `HTTP ${res.status}`)
  }
  return data as T
}

export const api = {
  login(password: string) {
    return request<{ ok: true }>('/api/webui/auth/login', {
      method: 'POST',
      body: JSON.stringify({ password }),
    })
  },
  me() {
    return request<{ ok: true; authenticated: boolean }>('/api/webui/auth/me')
  },
  logout() {
    return request<{ ok: true }>('/api/webui/auth/logout', { method: 'POST' })
  },
  groups() {
    return request<{ ok: true; groups: GroupInfo[] }>('/api/webui/groups')
  },
  plugins() {
    return request<{ ok: true; plugins: PluginEntry[] }>('/api/webui/plugins')
  },
  groupPlugins(groupID: number) {
    return request<{ ok: true; disabled: Record<string, boolean> }>(`/api/webui/groups/${groupID}/plugins`)
  },
  setGroupPlugin(groupID: number, pluginID: number, disabled: boolean) {
    return request<{ ok: true }>(`/api/webui/groups/${groupID}/plugins/${pluginID}`, {
      method: 'PUT',
      body: JSON.stringify({ disabled }),
    })
  },
  applyPluginAll(pluginID: number, groupIDs: number[], disabled: boolean) {
    return request<{ ok: true }>(`/api/webui/plugins/${pluginID}/apply-all`, {
      method: 'POST',
      body: JSON.stringify({ group_ids: groupIDs, disabled }),
    })
  },
  affinity(groupID: number | null, q: string) {
    const params = new URLSearchParams()
    if (groupID) params.set('group_id', String(groupID))
    if (q) params.set('q', q)
    return request<{ ok: true; affinity: AffinityRow[]; block_below: number }>(`/api/webui/affinity?${params}`)
  },
  setAffinityScore(id: number, score: number) {
    return request<{ ok: true; affinity: AffinityRow }>(`/api/webui/affinity/${id}/score`, {
      method: 'PUT',
      body: JSON.stringify({ score }),
    })
  },
  adjustAffinity(id: number, delta: number) {
    return request<{ ok: true; affinity: AffinityRow }>(`/api/webui/affinity/${id}/adjust`, {
      method: 'POST',
      body: JSON.stringify({ delta }),
    })
  },
  resetAffinity(id: number) {
    return request<{ ok: true; affinity: AffinityRow }>(`/api/webui/affinity/${id}/reset`, { method: 'POST' })
  },
}
```

- [x] **Step 4: Add router shell**

Create `webui/src/router.ts`:

```ts
import { createRouter, createWebHistory } from 'vue-router'
import LoginView from './views/LoginView.vue'
import PluginGroupsView from './views/PluginGroupsView.vue'
import AffinityView from './views/AffinityView.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: LoginView },
    { path: '/', component: PluginGroupsView },
    { path: '/affinity', component: AffinityView },
  ],
})
```

Create the view files in the next task before building.

- [x] **Step 5: Update `.gitignore`**

Append:

```gitignore
webui/node_modules/
webui/dist/
```

The Go server can serve `webui/dist` when present locally or in a release image. It does not need to be tracked in git.

- [x] **Step 6: Commit scaffold**

```bash
git add .gitignore webui
git commit -m "feat(webui): scaffold vue admin app"
```

---

### Task 10: Frontend Screens

**Files:**
- Create: `webui/src/views/LoginView.vue`
- Create: `webui/src/views/PluginGroupsView.vue`
- Create: `webui/src/views/AffinityView.vue`
- Modify: `webui/src/App.vue`

- [x] **Step 1: Create login view**

Create `webui/src/views/LoginView.vue`:

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'

const router = useRouter()
const password = ref('')
const loading = ref(false)
const error = ref('')

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await api.login(password.value)
    await router.push('/')
  } catch (err) {
    error.value = err instanceof Error ? err.message : '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="min-h-screen flex items-center justify-center bg-neutral-950 px-4">
    <form class="w-full max-w-sm space-y-4 rounded-lg border border-neutral-800 bg-neutral-900 p-6" @submit.prevent="submit">
      <div>
        <h1 class="text-xl font-semibold text-white">月灵 WebUI</h1>
        <p class="mt-1 text-sm text-neutral-400">输入管理密码继续</p>
      </div>
      <UInput v-model="password" type="password" autofocus placeholder="Password" icon="i-tabler-lock" />
      <p v-if="error" class="text-sm text-red-400">{{ error }}</p>
      <UButton type="submit" block :loading="loading" icon="i-tabler-login-2">登录</UButton>
    </form>
  </main>
</template>
```

- [x] **Step 2: Create app shell**

Replace `webui/src/App.vue`:

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, RouterLink, RouterView } from 'vue-router'

const route = useRoute()
const inLogin = computed(() => route.path === '/login')
</script>

<template>
  <RouterView v-if="inLogin" />
  <div v-else class="min-h-screen bg-neutral-950 text-neutral-100">
    <header class="border-b border-neutral-800 bg-neutral-900/80">
      <div class="mx-auto flex max-w-7xl items-center gap-3 px-4 py-3">
        <div class="font-semibold">月灵 WebUI</div>
        <nav class="ml-4 flex gap-2 text-sm">
          <RouterLink class="rounded px-3 py-1.5 hover:bg-neutral-800" to="/">群插件</RouterLink>
          <RouterLink class="rounded px-3 py-1.5 hover:bg-neutral-800" to="/affinity">AI 好感度</RouterLink>
        </nav>
      </div>
    </header>
    <main class="mx-auto max-w-7xl px-4 py-5">
      <RouterView />
    </main>
  </div>
</template>
```

- [x] **Step 3: Create group plugins page**

Create `webui/src/views/PluginGroupsView.vue`:

```vue
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type GroupInfo, type PluginEntry } from '../api'

const groups = ref<GroupInfo[]>([])
const plugins = ref<PluginEntry[]>([])
const selectedGroupID = ref<number | null>(null)
const disabled = ref<Record<string, boolean>>({})
const loading = ref(false)
const saving = ref<Record<number, boolean>>({})
const saved = ref<Record<number, boolean>>({})
const error = ref('')

const groupedPlugins = computed(() => {
  const map = new Map<string, PluginEntry[]>()
  for (const plugin of plugins.value) {
    if (!map.has(plugin.group)) map.set(plugin.group, [])
    map.get(plugin.group)!.push(plugin)
  }
  return Array.from(map.entries())
})

async function load() {
  error.value = ''
  loading.value = true
  try {
    const [groupRes, pluginRes] = await Promise.all([api.groups(), api.plugins()])
    groups.value = groupRes.groups
    plugins.value = pluginRes.plugins
    selectedGroupID.value = groups.value[0]?.group_id ?? null
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadDisabled() {
  if (!selectedGroupID.value) return
  const res = await api.groupPlugins(selectedGroupID.value)
  disabled.value = res.disabled
}

async function setPlugin(pluginID: number, value: boolean) {
  if (!selectedGroupID.value) return
  saving.value[pluginID] = true
  saved.value[pluginID] = false
  try {
    await api.setGroupPlugin(selectedGroupID.value, pluginID, value)
    disabled.value[String(pluginID)] = value
    saved.value[pluginID] = true
    window.setTimeout(() => (saved.value[pluginID] = false), 1200)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value[pluginID] = false
  }
}

async function applyAll(pluginID: number, value: boolean) {
  saving.value[pluginID] = true
  try {
    await api.applyPluginAll(pluginID, groups.value.map((group) => group.group_id), value)
    if (selectedGroupID.value) disabled.value[String(pluginID)] = value
    saved.value[pluginID] = true
    window.setTimeout(() => (saved.value[pluginID] = false), 1200)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '批量操作失败'
  } finally {
    saving.value[pluginID] = false
  }
}

watch(selectedGroupID, loadDisabled)
onMounted(load)
</script>

<template>
  <section class="space-y-4">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-lg font-semibold">群插件</h1>
        <p class="text-sm text-neutral-400">禁用后该群静默跳过对应插件</p>
      </div>
      <UButton icon="i-tabler-refresh" :loading="loading" @click="load">刷新</UButton>
    </div>

    <UAlert v-if="error" color="error" icon="i-tabler-alert-circle" :description="error" />

    <div class="grid gap-4 lg:grid-cols-[280px_1fr]">
      <aside class="rounded-lg border border-neutral-800 bg-neutral-900 p-3">
        <div v-for="group in groups" :key="group.group_id">
          <button class="w-full rounded px-3 py-2 text-left text-sm hover:bg-neutral-800"
            :class="{ 'bg-neutral-800': selectedGroupID === group.group_id }"
            @click="selectedGroupID = group.group_id">
            <div>{{ group.group_name || group.group_id }}</div>
            <div class="text-xs text-neutral-500">{{ group.group_id }}</div>
          </button>
        </div>
      </aside>

      <div class="space-y-4">
        <section v-for="[groupName, items] in groupedPlugins" :key="groupName" class="rounded-lg border border-neutral-800 bg-neutral-900 p-4">
          <h2 class="mb-3 text-sm font-semibold text-neutral-300">{{ groupName }}</h2>
          <div class="divide-y divide-neutral-800">
            <div v-for="plugin in items" :key="plugin.id" class="flex items-center gap-3 py-3">
              <div class="min-w-0 flex-1">
                <div class="font-medium">{{ plugin.name }}</div>
                <div class="text-sm text-neutral-400">{{ plugin.desc }}</div>
                <div v-if="saved[plugin.id]" class="mt-1 text-xs text-emerald-400">已保存</div>
              </div>
              <UButton size="xs" variant="ghost" :loading="saving[plugin.id]" @click="applyAll(plugin.id, true)">全部禁用</UButton>
              <UButton size="xs" variant="ghost" :loading="saving[plugin.id]" @click="applyAll(plugin.id, false)">全部启用</UButton>
              <USwitch :model-value="!disabled[String(plugin.id)]" :disabled="saving[plugin.id]" @update:model-value="setPlugin(plugin.id, !$event)" />
            </div>
          </div>
        </section>
      </div>
    </div>
  </section>
</template>
```

- [x] **Step 4: Create affinity page**

Create `webui/src/views/AffinityView.vue`:

```vue
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, type AffinityRow, type GroupInfo } from '../api'

const groups = ref<GroupInfo[]>([])
const groupID = ref<number | null>(null)
const q = ref('')
const rows = ref<AffinityRow[]>([])
const blockBelow = ref(10)
const loading = ref(false)
const saving = ref<Record<number, boolean>>({})
const error = ref('')

async function loadGroups() {
  try {
    const res = await api.groups()
    groups.value = res.groups
  } catch (err) {
    error.value = err instanceof Error ? err.message : '群列表加载失败'
  }
}

async function loadRows() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.affinity(groupID.value, q.value)
    rows.value = res.affinity
    blockBelow.value = res.block_below
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载失败'
  } finally {
    loading.value = false
  }
}

async function setScore(row: AffinityRow) {
  const raw = window.prompt('设置分数', String(row.Score))
  if (raw === null) return
  const score = Number(raw)
  if (!Number.isFinite(score)) return
  saving.value[row.ID] = true
  try {
    const res = await api.setAffinityScore(row.ID, score)
    Object.assign(row, res.affinity)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value[row.ID] = false
  }
}

async function adjust(row: AffinityRow, delta: number) {
  saving.value[row.ID] = true
  try {
    const res = await api.adjustAffinity(row.ID, delta)
    Object.assign(row, res.affinity)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value[row.ID] = false
  }
}

async function reset(row: AffinityRow) {
  saving.value[row.ID] = true
  try {
    const res = await api.resetAffinity(row.ID)
    Object.assign(row, res.affinity)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '重置失败'
  } finally {
    saving.value[row.ID] = false
  }
}

onMounted(async () => {
  await loadGroups()
  await loadRows()
})
</script>

<template>
  <section class="space-y-4">
    <div>
      <h1 class="text-lg font-semibold">AI 好感度</h1>
      <p class="text-sm text-neutral-400">查看和修正隐藏好感度分数</p>
    </div>
    <UAlert v-if="error" color="error" icon="i-tabler-alert-circle" :description="error" />
    <div class="flex flex-wrap gap-2">
      <USelectMenu v-model="groupID" class="w-64" :items="groups.map(g => ({ label: `${g.group_name || g.group_id}`, value: g.group_id }))" value-key="value" placeholder="全部群" />
      <UInput v-model="q" class="w-64" icon="i-tabler-search" placeholder="QQ 或昵称" @keyup.enter="loadRows" />
      <UButton icon="i-tabler-search" :loading="loading" @click="loadRows">查询</UButton>
    </div>
    <div class="overflow-hidden rounded-lg border border-neutral-800 bg-neutral-900">
      <table class="w-full text-left text-sm">
        <thead class="bg-neutral-900 text-neutral-400">
          <tr>
            <th class="px-3 py-2">群</th>
            <th class="px-3 py-2">QQ</th>
            <th class="px-3 py-2">昵称</th>
            <th class="px-3 py-2">分数</th>
            <th class="px-3 py-2">最近原因</th>
            <th class="px-3 py-2">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-neutral-800">
          <tr v-for="row in rows" :key="row.ID">
            <td class="px-3 py-2">{{ row.GroupID }}</td>
            <td class="px-3 py-2">{{ row.UserID }}</td>
            <td class="px-3 py-2">{{ row.Nickname || '-' }}</td>
            <td class="px-3 py-2">
              <UBadge :color="row.Score < blockBelow ? 'error' : 'neutral'">{{ row.Score }}</UBadge>
            </td>
            <td class="px-3 py-2">{{ row.LastReason || '-' }}</td>
            <td class="px-3 py-2">
              <div class="flex gap-1">
                <UButton size="xs" variant="ghost" :loading="saving[row.ID]" @click="adjust(row, -5)">-5</UButton>
                <UButton size="xs" variant="ghost" :loading="saving[row.ID]" @click="adjust(row, 5)">+5</UButton>
                <UButton size="xs" variant="ghost" :loading="saving[row.ID]" @click="setScore(row)">设置</UButton>
                <UButton size="xs" variant="ghost" :loading="saving[row.ID]" @click="reset(row)">重置</UButton>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
```

- [x] **Step 5: Build frontend**

Run:

```bash
pnpm --dir webui build
```

Expected: PASS. If Nuxt UI component names or props have changed, adjust only to match the installed package API while preserving the same page behavior.

- [x] **Step 6: Commit**

```bash
git add webui
git commit -m "feat(webui): add admin screens"
```

---

### Task 11: Docker Build And Release Packaging

**Files:**
- Modify: `Dockerfile`

- [ ] **Step 1: Add Node build stage**

Modify `Dockerfile` to build frontend before the Go binary:

```dockerfile
# ── WebUI stage ───────────────────────────────────────────────────────────────
FROM node:22-alpine AS webui

WORKDIR /webui
COPY webui/package.json webui/pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY webui/ ./
RUN pnpm build

 # ── Build stage ────────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=webui /webui/dist ./webui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bot ./cmd/bot/
```

Keep the existing runtime stage. The final image only needs the Go binary because `webui/dist` is copied into the Go build context before compilation and then served from the runtime filesystem only if included next to the binary. If the server reads `webui/dist` from disk at runtime, also copy it:

```dockerfile
COPY --from=webui /webui/dist ./webui/dist
```

in the runtime stage after `COPY --from=builder /build/bot .`.

- [ ] **Step 2: Run local build commands**

Run:

```bash
pnpm --dir webui build
go test ./...
go vet ./...
go build ./cmd/bot
```

Expected: all pass.

- [ ] **Step 3: Commit**

```bash
git add Dockerfile
git commit -m "build: include webui assets"
```

---

### Task 12: Final Verification

**Files:**
- Modify only files needed to fix failures discovered in verification.

- [ ] **Step 1: Run full Go verification**

Run:

```bash
go test ./...
go vet ./...
```

Expected: PASS.

- [ ] **Step 2: Run frontend verification**

Run:

```bash
pnpm --dir webui build
```

Expected: PASS.

- [ ] **Step 3: Manual startup checks**

With a test `config.toml` where:

```toml
[webui]
enabled = false
```

Run:

```bash
go run ./cmd/bot/
```

Expected: no `[webui] enabled` log line.

Then set:

```toml
[webui]
enabled = true
addr = ":9080"
password = "secret"
```

Run:

```bash
go run ./cmd/bot/
```

Expected: log line includes `[webui] enabled on :9080`; visiting `http://127.0.0.1:9080/login` shows the login page after `pnpm --dir webui build`.

- [ ] **Step 4: Manual behavior checks**

1. Log in with wrong password. Expected: login stays on page and shows inline error.
2. Log in with correct password. Expected: navigates to group plugin page.
3. If NapCat is disconnected, group plugin page shows an inline offline/error state.
4. If NapCat is connected, select a group and disable one harmless plugin such as `帮助`; send that command in the group. Expected: no reply.
5. Re-enable the plugin; send the command again. Expected: normal reply.
6. Open AI affinity page, search a seeded user, adjust score by +5, then reset. Expected: row updates in place, no success toast.

- [ ] **Step 5: Commit final fixes**

If verification required fixes:

```bash
git add <fixed-files>
git commit -m "fix(webui): close verification gaps"
```

If no fixes were needed, do not create an empty commit.

---

## Self-Review

Spec coverage:
- Password WebUI config: Task 1.
- Gin server started with bot: Tasks 6 and 8.
- SQLite runtime management: Task 2.
- Per-group plugin disable with silent dispatch behavior: Tasks 4 and 5.
- Help-system plugin catalog: Task 3.
- NapCat group list source: Tasks 4 and 7.
- AI affinity score management only: Tasks 2, 7, and 10.
- Vue + TypeScript + Vite 8 + Nuxt UI + Tabler icons: Tasks 9 and 10.
- In-place success feedback and error-only attention feedback: Task 10.
- Verification: Tasks 11 and 12.

Placeholder-marker scan:
- No task uses unfinished-work markers or fill-in instructions.
- Code snippets define the referenced functions and types in the task where they first appear.

Type consistency:
- Backend uses `config.WebUIConfig`, `db.GroupPluginDisabled`, `bot.PluginGate`, `plugins/system.PluginCatalogEntry`, and `bot.GroupInfo`.
- Frontend API types mirror current Go JSON tags where existing structs expose capitalized fields from Gorm models.
