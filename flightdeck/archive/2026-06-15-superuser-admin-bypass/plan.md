---
status: done
summary: 超级管理员免群权限门槛实现计划——改 cond.Admin/Owner 放行 SuperUsers，单任务 TDD
last_updated: 2026-06-15
---

# 超级管理员免群权限门槛 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让配置的超级管理员（`config.C.Bot.SuperUsers`）无条件满足 `cond.Admin` 与 `cond.Owner` 条件，从而能用所有受管理权限把关的命令（禁言、撤回等）。

**Architecture:** 单文件改动。`bot/cond/cond.go` 里给 `Admin`/`Owner` 两个 `Condition` 的判定追加「或该用户是配置的超管」分支，复用一个私有 `isSuperUser` helper。一处改动即覆盖所有 `.Where(perm.Admin)`/`.Where(perm.Owner)` 命令，无需动命令注册处。

**Tech Stack:** Go，项目内 `bot` / `config` 包。

---

### Task 1: Admin/Owner 条件放行超级管理员

**Files:**
- Modify: `bot/cond/cond.go`
- Test: `bot/cond/cond_test.go`（新建，package cond）

`cond.Admin` 目前只认 `msg.Role()` 为 `admin`/`owner`，`cond.Owner` 只认 `owner`。
追加 `isSuperUser` 分支，超管来源 `config.C.Bot.SuperUsers`（与 reboot 同源）。
`cond` import `config` 无循环依赖（config 仅依赖 viper）。

- [ ] **Step 1: 写失败测试** `bot/cond/cond_test.go`

```go
package cond

import (
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
)

func mkMsg(userID int64, role string) *bot.MsgCtx {
	return &bot.MsgCtx{Event: &bot.GroupMessageEvent{
		UserID: userID,
		Sender: bot.Sender{UserID: userID, Role: role},
	}}
}

func TestAdminOwnerSuperUser(t *testing.T) {
	config.C.Bot.SuperUsers = []int64{999}

	cases := []struct {
		name      string
		userID    int64
		role      string
		wantAdmin bool
		wantOwner bool
	}{
		{"群管", 1, "admin", true, false},
		{"群主", 2, "owner", true, true},
		{"超管普通成员", 999, "member", true, true},
		{"普通成员", 3, "member", false, false},
	}
	for _, c := range cases {
		msg := mkMsg(c.userID, c.role)
		if got := Admin.Check(nil, msg); got != c.wantAdmin {
			t.Errorf("%s: Admin = %v, want %v", c.name, got, c.wantAdmin)
		}
		if got := Owner.Check(nil, msg); got != c.wantOwner {
			t.Errorf("%s: Owner = %v, want %v", c.name, got, c.wantOwner)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./bot/cond/ -run TestAdminOwnerSuperUser -v`
Expected: FAIL（「超管普通成员」用例：Admin/Owner 当前返回 false，断言不通过）

- [ ] **Step 3: 实现**

`bot/cond/cond.go`。将 import 块改为：

```go
import (
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
)
```

新增 helper（放在 `Admin` 定义之前）：

```go
func isSuperUser(msg *bot.MsgCtx) bool {
	for _, id := range config.C.Bot.SuperUsers {
		if msg.UserID() == id {
			return true
		}
	}
	return false
}
```

把现有 `Admin`、`Owner` 两个 var 改成：

```go
var Admin bot.Condition = bot.CondFn(func(_ *bot.BotAPI, msg *bot.MsgCtx) bool {
	r := msg.Role()
	return r == "admin" || r == "owner" || isSuperUser(msg)
})

var Owner bot.Condition = bot.CondFn(func(_ *bot.BotAPI, msg *bot.MsgCtx) bool {
	return msg.Role() == "owner" || isSuperUser(msg)
})
```

`SuperUser(ids ...int64)`、`NoReply`、`NoAt`、`NoCommand` 保持不变。

- [ ] **Step 4: 运行测试确认通过 + 全量构建**

Run: `go test ./bot/cond/ -run TestAdminOwnerSuperUser -v`
Expected: PASS（全部 4 个用例）

Run: `go build ./...`
Expected: 无输出（编译通过）

- [ ] **Step 5: 提交**

```
git add bot/cond/cond.go bot/cond/cond_test.go
git commit -m "feat(perm): 超级管理员满足 Admin/Owner 条件"
```
（提交信息用 `-m` 普通引号字符串，勿用 bash here-string `@'...'@`。）

---

## Self-Review

- **Spec 覆盖**：超管满足 Admin(Step3) ✓；超管满足 Owner(Step3) ✓；群管/群主不变(Step3 保留原 role 判定) ✓；普通成员不变 ✓；超管来源 config.C.Bot.SuperUsers(Step3) ✓；SuperUser(ids) 不变 ✓；测试矩阵 4 用例(Step1) ✓。
- **占位符**：无 TBD/TODO；每步含完整代码与命令。
- **类型一致**：`isSuperUser(*bot.MsgCtx) bool`、`Admin`/`Owner` 仍是 `bot.Condition`、`Check(nil, msg)` 签名 `Check(*BotAPI, *MsgCtx) bool`、测试构造 `bot.MsgCtx{Event: *bot.GroupMessageEvent{UserID, Sender: bot.Sender{UserID, Role}}}` 与 `bot/event.go`、`bot/context.go` 字段一致。
