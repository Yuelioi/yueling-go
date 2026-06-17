---
status: active
summary: 分 8 任务实现每群加群审核：db 模型+CRUD、decideJoin 纯函数、命令解析、join_review 注册、退役 manager、help 更新、删搜ae、README
last_updated: 2026-06-18
implements: specs/2026-06-18-group-join-review.md
---

# 每群加群审核 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 每个群用 db 配置的白名单/黑名单关键词自动审核加群申请（拒绝优先、命中通过、其余留人工），管理员命令维护，退役全局 join_keywords。

**Architecture:** 新 gorm 模型 `GroupJoinRule`（一行一词）+ CRUD；`plugins/group/join_review.go` 持内存 cache、`OnRequest("group")` 决策、三条管理员命令；删 `manager.go` 全局逻辑。顺带删搜ae、README 补 pack。

**Tech Stack:** Go, gorm + glebarez/sqlite, 项目自有 bot 框架（`OnRequest`/`OnCommand`/`perm.Admin`）。

**约定（CLAUDE.md）：** 不写注释除非 WHY 不明显；所有注册在 `b.Start()` 前；改插件命令后同步 `help.go`。

---

### Task 1: db 模型 + CRUD

**Files:**
- Modify: `db/db.go`（加模型、进 `allModels`、加 3 个 CRUD 函数）
- Test: `db/join_rule_test.go`（Create）

- [ ] **Step 1: 写失败测试** — `db/join_rule_test.go`

```go
package db

import (
	"path/filepath"
	"testing"
)

func TestGroupJoinRuleCRUD(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	const g = int64(123)

	added, err := AddGroupJoinRule(g, JoinActionAllow, "交流")
	if err != nil || !added {
		t.Fatalf("add1: added=%v err=%v", added, err)
	}
	// 幂等：重复添加不新增
	added, err = AddGroupJoinRule(g, JoinActionAllow, "交流")
	if err != nil || added {
		t.Fatalf("add dup: added=%v err=%v", added, err)
	}
	if _, err := AddGroupJoinRule(g, JoinActionDeny, "广告"); err != nil {
		t.Fatalf("add deny: %v", err)
	}
	// 另一个群互不干扰
	if _, err := AddGroupJoinRule(456, JoinActionAllow, "你好"); err != nil {
		t.Fatalf("add other group: %v", err)
	}

	rows, err := GetAllGroupJoinRules()
	if err != nil || len(rows) != 3 {
		t.Fatalf("getall: n=%d err=%v", len(rows), err)
	}

	removed, err := DeleteGroupJoinRule(g, JoinActionAllow, "交流")
	if err != nil || !removed {
		t.Fatalf("del: removed=%v err=%v", removed, err)
	}
	removed, err = DeleteGroupJoinRule(g, JoinActionAllow, "交流")
	if err != nil || removed {
		t.Fatalf("del again should be false: removed=%v err=%v", removed, err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败** — `go test ./db/ -run TestGroupJoinRuleCRUD`，预期 FAIL（未定义 `GroupJoinRule`/`AddGroupJoinRule`…）。

- [ ] **Step 3: 加模型 + 进 allModels** — `db/db.go`

在模型定义区加：
```go
type GroupJoinRule struct {
	ID      uint   `gorm:"primarykey;autoIncrement"`
	GroupID int64  `gorm:"index"`
	Action  string `gorm:"size:8"`
	Keyword string `gorm:"size:128"`
}

const (
	JoinActionAllow = "allow"
	JoinActionDeny  = "deny"
)
```
把 `&GroupJoinRule{}` 加进 `allModels`：
```go
var allModels = []any{
	&AutoReply{}, &UserGameRecord{}, &Reminder{},
	&SemanticMemory{}, &EpisodicMemory{}, &ProceduralMemory{},
	&UserTag{}, &TodoItem{}, &UserProfile{},
	&GroupJoinRule{},
}
```

- [ ] **Step 4: 加 CRUD 函数** — `db/db.go`（用 Count 判存在，避免引入 `errors` 包）

```go
func AddGroupJoinRule(groupID int64, action, keyword string) (bool, error) {
	var count int64
	if err := DB.Model(&GroupJoinRule{}).
		Where("group_id = ? AND action = ? AND keyword = ?", groupID, action, keyword).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	if err := DB.Create(&GroupJoinRule{GroupID: groupID, Action: action, Keyword: keyword}).Error; err != nil {
		return false, err
	}
	return true, nil
}

func DeleteGroupJoinRule(groupID int64, action, keyword string) (bool, error) {
	res := DB.Where("group_id = ? AND action = ? AND keyword = ?", groupID, action, keyword).
		Delete(&GroupJoinRule{})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func GetAllGroupJoinRules() ([]GroupJoinRule, error) {
	var rows []GroupJoinRule
	err := DB.Find(&rows).Error
	return rows, err
}
```

- [ ] **Step 5: 跑测试确认通过** — `go test ./db/ -run TestGroupJoinRuleCRUD`，预期 PASS。

- [ ] **Step 6: 提交**
```bash
git add db/db.go db/join_rule_test.go
git commit -m "feat(db): GroupJoinRule 模型 + CRUD（每群加群审核规则）"
```

---

### Task 2: decideJoin 决策纯函数

**Files:**
- Create: `plugins/group/join_review.go`（先只放 decideJoin + 类型）
- Test: `plugins/group/join_review_test.go`

- [ ] **Step 1: 写失败测试** — `plugins/group/join_review_test.go`

```go
package group

import "testing"

func TestDecideJoin(t *testing.T) {
	allow := []string{"交流", "学习"}
	deny := []string{"广告"}
	cases := []struct {
		name    string
		comment string
		allow   []string
		deny    []string
		want    joinDecision
	}{
		{"命中通过词", "我想来交流技术", allow, deny, decisionApprove},
		{"命中拒绝词", "代理广告招商", allow, deny, decisionReject},
		{"拒绝优先", "交流广告", allow, deny, decisionReject},
		{"通配任意非空", "随便写点", []string{"*"}, nil, decisionApprove},
		{"空comment留人工", "", []string{"*"}, deny, decisionNone},
		{"都不命中留人工", "你好啊", allow, deny, decisionNone},
		{"无配置留人工", "交流", nil, nil, decisionNone},
	}
	for _, c := range cases {
		if got := decideJoin(c.comment, c.allow, c.deny); got != c.want {
			t.Errorf("%s: decideJoin=%d want %d", c.name, got, c.want)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败** — `go test ./plugins/group/ -run TestDecideJoin`，预期 FAIL（未定义）。

- [ ] **Step 3: 实现** — `plugins/group/join_review.go`

```go
package group

import "strings"

type joinDecision int

const (
	decisionNone joinDecision = iota
	decisionApprove
	decisionReject
)

func decideJoin(comment string, allow, deny []string) joinDecision {
	if comment == "" {
		return decisionNone
	}
	for _, kw := range deny {
		if kw != "" && strings.Contains(comment, kw) {
			return decisionReject
		}
	}
	for _, kw := range allow {
		if kw == "*" || (kw != "" && strings.Contains(comment, kw)) {
			return decisionApprove
		}
	}
	return decisionNone
}
```

- [ ] **Step 4: 跑测试确认通过** — `go test ./plugins/group/ -run TestDecideJoin`，预期 PASS。

- [ ] **Step 5: 提交**
```bash
git add plugins/group/join_review.go plugins/group/join_review_test.go
git commit -m "feat(group): decideJoin 加群审核决策（拒绝优先/通配/留人工）"
```

---

### Task 3: parseKeywordArg 命令参数解析

**Files:**
- Modify: `plugins/group/join_review.go`（加 `parseKeywordArg`）
- Test: `plugins/group/join_review_test.go`（加测试）

- [ ] **Step 1: 写失败测试** — 追加到 `plugins/group/join_review_test.go`

```go
import "reflect"  // 加到已有 import

func TestParseKeywordArg(t *testing.T) {
	cases := []struct {
		raw      string
		wantAdd  bool
		wantKws  []string
		wantOK   bool
	}{
		{"+交流", true, []string{"交流"}, true},
		{"-广告", false, []string{"广告"}, true},
		{"+交流,学习", true, []string{"交流", "学习"}, true},
		{"+交流，学习", true, []string{"交流", "学习"}, true}, // 中文逗号
		{"+大写ABC", true, []string{"大写abc"}, true},          // 统一小写
		{"+*", true, []string{"*"}, true},
		{"交流", false, nil, false},  // 无 +/-
		{"+", false, nil, false},     // 空词
		{"+ , ", false, nil, false},  // 全空
		{"", false, nil, false},
	}
	for _, c := range cases {
		add, kws, ok := parseKeywordArg(c.raw)
		if ok != c.wantOK || add != c.wantAdd || !reflect.DeepEqual(kws, c.wantKws) {
			t.Errorf("%q: got (add=%v kws=%v ok=%v) want (add=%v kws=%v ok=%v)",
				c.raw, add, kws, ok, c.wantAdd, c.wantKws, c.wantOK)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败** — `go test ./plugins/group/ -run TestParseKeywordArg`，预期 FAIL。

- [ ] **Step 3: 实现** — 追加到 `plugins/group/join_review.go`

```go
func parseKeywordArg(raw string) (add bool, keywords []string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false, nil, false
	}
	switch raw[0] {
	case '+':
		add = true
	case '-':
		add = false
	default:
		return false, nil, false
	}
	rest := strings.ReplaceAll(raw[1:], "，", ",")
	for _, p := range strings.Split(rest, ",") {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
			keywords = append(keywords, p)
		}
	}
	if len(keywords) == 0 {
		return false, nil, false
	}
	return add, keywords, true
}
```

- [ ] **Step 4: 跑测试确认通过** — `go test ./plugins/group/ -run TestParseKeywordArg`，预期 PASS。

- [ ] **Step 5: 提交**
```bash
git add plugins/group/join_review.go plugins/group/join_review_test.go
git commit -m "feat(group): parseKeywordArg 解析 +/-/逗号多词"
```

---

### Task 4: cache + 注册（handler + 命令）+ 退役 manager + 接线

**Files:**
- Modify: `plugins/group/join_review.go`（cache + `RegisterJoinReview`）
- Delete: `plugins/group/manager.go`
- Modify: `cmd/bot/main.go:78`（`RegisterManager` → `RegisterJoinReview`）
- Modify: `config/config.go`（删 `BotConfig.JoinKeywords`）

- [ ] **Step 1: cache + 注册实现** — 追加到 `plugins/group/join_review.go`

补 import（整文件 import 改成）：
```go
import (
	"fmt"
	"strings"
	"sync"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/bot/perm"
	"github.com/Yuelioi/yueling-go/db"
)
```
追加：
```go
const joinDenyReason = "申请未通过审核"

type joinRule struct {
	allow []string
	deny  []string
}

type joinCache struct {
	mu   sync.RWMutex
	data map[int64]*joinRule
}

var jcache = &joinCache{data: map[int64]*joinRule{}}

func (c *joinCache) load() {
	rows, err := db.GetAllGroupJoinRules()
	if err != nil {
		return
	}
	m := make(map[int64]*joinRule)
	for _, r := range rows {
		jr := m[r.GroupID]
		if jr == nil {
			jr = &joinRule{}
			m[r.GroupID] = jr
		}
		switch r.Action {
		case db.JoinActionAllow:
			jr.allow = append(jr.allow, r.Keyword)
		case db.JoinActionDeny:
			jr.deny = append(jr.deny, r.Keyword)
		}
	}
	c.mu.Lock()
	c.data = m
	c.mu.Unlock()
}

func (c *joinCache) get(groupID int64) *joinRule {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data[groupID]
}

func joinListHandler(action, label string) func(*bot.CommandContext) error {
	return func(ctx *bot.CommandContext) error {
		raw := strings.Join(ctx.Args, " ")
		if strings.TrimSpace(raw) == "" {
			return ctx.Reply(formatJoinList(ctx.GroupID()))
		}
		add, keywords, ok := parseKeywordArg(raw)
		if !ok {
			return ctx.Reply("用法：加群" + label + " +词1,词2  添加；加群" + label + " -词  删除")
		}
		n := 0
		for _, kw := range keywords {
			var changed bool
			var err error
			if add {
				changed, err = db.AddGroupJoinRule(ctx.GroupID(), action, kw)
			} else {
				changed, err = db.DeleteGroupJoinRule(ctx.GroupID(), action, kw)
			}
			if err != nil {
				return ctx.Reply("操作失败：" + err.Error())
			}
			if changed {
				n++
			}
		}
		jcache.load()
		verb := "添加"
		if !add {
			verb = "删除"
		}
		return ctx.Reply(fmt.Sprintf("已%s %d 个%s关键词", verb, n, label))
	}
}

func formatJoinList(groupID int64) string {
	rule := jcache.get(groupID)
	var allow, deny []string
	if rule != nil {
		allow, deny = rule.allow, rule.deny
	}
	show := func(s []string) string {
		if len(s) == 0 {
			return "（空）"
		}
		return strings.Join(s, "、")
	}
	return fmt.Sprintf("加群审核（本群）\n通过词：%s\n拒绝词：%s\n用法：加群白名单 +词1,词2 / -词；加群黑名单 +词 / -词；白名单加 * 表示任意理由放行",
		show(allow), show(deny))
}

func RegisterJoinReview(b *bot.Bot) {
	jcache.load()

	b.OnRequest("group").Handle(func(ctx *bot.RequestContext) error {
		e := ctx.Event
		if e.SubType != "add" {
			return nil
		}
		rule := jcache.get(e.GroupID)
		if rule == nil {
			return nil
		}
		switch decideJoin(strings.ToLower(e.Comment), rule.allow, rule.deny) {
		case decisionReject:
			return ctx.BotAPI.SetGroupAddRequest(e.Flag, e.SubType, false, joinDenyReason)
		case decisionApprove:
			return ctx.BotAPI.SetGroupAddRequest(e.Flag, e.SubType, true, "")
		}
		return nil
	})

	b.OnCommand("加群审核").Where(perm.Admin).Handle(func(ctx *bot.CommandContext) error {
		return ctx.Reply(formatJoinList(ctx.GroupID()))
	})
	b.OnCommand("加群白名单").Where(perm.Admin).Handle(joinListHandler(db.JoinActionAllow, "白名单"))
	b.OnCommand("加群黑名单").Where(perm.Admin).Handle(joinListHandler(db.JoinActionDeny, "黑名单"))
}
```

- [ ] **Step 2: 删全局 manager** — 删除文件 `plugins/group/manager.go`。

- [ ] **Step 3: 接线 main.go** — `cmd/bot/main.go`，把
```go
	group.RegisterManager(b)
```
改为
```go
	group.RegisterJoinReview(b)
```

- [ ] **Step 4: 删 config 字段** — `config/config.go`，从 `BotConfig` 删除这一行：
```go
	JoinKeywords []string `mapstructure:"join_keywords"`
```

- [ ] **Step 5: 编译 + 全测** — `go build ./...` 预期 OK；`go test ./plugins/group/ ./db/` 预期 PASS。
  （若编译报 `RegisterManager` 未定义残留，检查 main.go；报 `JoinKeywords` 残留引用，grep 清掉。）

- [ ] **Step 6: 提交**
```bash
git add plugins/group/join_review.go cmd/bot/main.go config/config.go
git rm plugins/group/manager.go
git commit -m "feat(group): 每群加群审核（OnRequest+命令），退役全局 join_keywords"
```

---

### Task 5: help.go —「入群审批」改「加群审核」

**Files:**
- Modify: `plugins/system/help.go`（id 3 条目）

- [ ] **Step 1: 替换条目** — `plugins/system/help.go`，把现有 id 3「入群审批」整条：
```go
	{3, "入群审批", "群管",
		"含关键词的入群申请自动通过",
		"  在 config.toml 中设置 join_keywords 列表\n" +
			"  申请消息含关键词 → 自动同意，否则拒绝",
		[]string{}},
```
替换为：
```go
	{3, "加群审核", "群管",
		"每群独立配置：加群申请理由命中关键词自动通过 / 拒绝（拒绝优先，其余留人工）",
		"  加群审核              查看本群通过词 / 拒绝词\n" +
			"  加群白名单 +词1,词2    添加通过词（可逗号多词；+* 表示任意理由放行）\n" +
			"  加群白名单 -词         删除通过词\n" +
			"  加群黑名单 +词 / -词   添加 / 删除拒绝词",
		[]string{"加群审核", "加群白名单", "加群黑名单"}},
```

- [ ] **Step 2: 编译** — `go build ./...` 预期 OK。

- [ ] **Step 3: 提交**
```bash
git add plugins/system/help.go
git commit -m "docs(help): 入群审批 → 加群审核（新命令）"
```

---

### Task 6: 删除 搜ae 插件

**Files:**
- Delete: `plugins/tools/search_ae.go`
- Modify: `cmd/bot/main.go`（删 `RegisterSearchAE`）
- Modify: `plugins/system/help.go`（删 id 27「搜AE插件」）

- [ ] **Step 1: 删文件** — 删除 `plugins/tools/search_ae.go`（其内 `ctx.React(bot.EmojiProcessing)` 随文件消失）。

- [ ] **Step 2: 删接线** — `cmd/bot/main.go`，删除这一行：
```go
	tools.RegisterSearchAE(b)
```

- [ ] **Step 3: 删 help 条目** — `plugins/system/help.go`，删除整条 id 27：
```go
	{27, "搜AE插件", "工具",
		"在 lookae.com 搜索 After Effects 插件/脚本",
		"  搜ae插件 <关键词>\n" +
			"  搜ae脚本 <关键词>",
		[]string{"搜ae插件", "搜ae脚本"}},
```

- [ ] **Step 4: 编译 + vet** — `go build ./...` 与 `go vet ./plugins/tools/ ./plugins/system/` 预期 OK（确认无 `searchLookAE`/`RegisterSearchAE` 残留引用）。

- [ ] **Step 5: 提交**
```bash
git rm plugins/tools/search_ae.go
git add cmd/bot/main.go plugins/system/help.go
git commit -m "chore: 删除 搜ae插件/搜ae脚本 功能"
```

---

### Task 7: README — 补 pack、加群审核节、删搜ae

**Files:**
- Modify: `README.md`

- [ ] **Step 1: 工具区补 pack 行 + 删搜ae 行** — `README.md` 的「## 工具」表格里，删除这两行：
```
| `搜ae插件 <关键词>` | 命令 | 搜索 After Effects 插件 |
| `搜ae脚本 <关键词>` | 命令 | 搜索 After Effects 脚本 |
```
并在「发送平台链接」行**之前**加一行：
```
| `pack` + 引用消息 | 命令 | 把被引用消息（含合并转发）里的所有图片打包成 zip 传到群文件 |
```

- [ ] **Step 2: 加「加群审核」小节** — 在「## 群文件」小节之后插入：
```markdown
## 加群审核

> 需要 **管理员 / 群主** 权限，每个群单独配置

加群申请的**验证理由**命中关键词时自动处理：命中拒绝词→拒绝，命中通过词→通过，**拒绝优先**，都不命中则留管理员人工处理。

| 命令 | 匹配 | 说明 |
|------|------|------|
| `加群审核` | 命令 | 查看本群通过词 / 拒绝词 |
| `加群白名单 +词1,词2` | 命令 | 添加通过词（可逗号加多个；`+*` 表示任意理由放行） |
| `加群白名单 -词` | 命令 | 删除通过词 |
| `加群黑名单 +词` / `-词` | 命令 | 添加 / 删除拒绝词 |

---
```

- [ ] **Step 3: 提交**
```bash
git add README.md
git commit -m "docs(readme): 补 pack、加群审核；删搜ae"
```

---

### Task 8: 全量验证

- [ ] **Step 1: 全量 build + test + vet**
```bash
go build ./... && go test ./... && go vet ./...
```
预期：build OK、全部测试 PASS、vet 无输出。

- [ ] **Step 2: 残留检查** — grep 确认无遗留：`JoinKeywords`、`RegisterManager`、`RegisterSearchAE`、`searchLookAE`、`join_keywords` 在 Go 代码中均无引用（README/spec 里的说明文字不算）。

- [ ] **Step 3: 手验清单写入 cockpit Pending Review**（landing 时）：部署后造一条加群申请 →（a）理由含某群白名单词→自动通过；（b）含黑名单词→自动拒绝；（c）`加群白名单 +xx,yy`、`加群审核`、`加群黑名单 -xx` 命令回执正确。

---

## 备注

- 端到端加群审批依赖真实 NapCat 事件，单测覆盖到 `decideJoin`/`parseKeywordArg`/db CRUD；审批链路手动验证。
- db 测试用 `Init(filepath.Join(t.TempDir(), "test.db"))` 独立临时库（避开 sqlite `:memory:` 多连接不可见的坑），不污染真实库。
- 命令均 `perm.Admin`，群聊场景；`OnRequest("group")` 只处理 `SubType=="add"`。
