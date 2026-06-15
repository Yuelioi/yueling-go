---
status: done
summary: 设精命令实现计划——bot.SetEssenceMsg 封装 + plugins/group/essence.go + 注册，2 任务（build+手验，无单测面）
last_updated: 2026-06-15
---

# 设精命令 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用户回复一条消息发 `设精`（别名 `加精`），bot 调 NapCat `set_essence_msg` 把该消息加入群精华；不限管理员，普通用户可用。

**Architecture:** 与既有 `RegisterRevoke`（`plugins/group/ban.go`）完全同构——从 `ReplyID()` 取被回复消息 id，调 API。新增 `bot.SetEssenceMsg` 封装 `set_essence_msg`，新增 `plugins/group/essence.go` 注册命令（不挂 `perm.Admin`），在 `cmd/bot/main.go` 注册。

**Tech Stack:** Go，OneBot v11 / NapCat，项目内 `bot` 包。

> **测试说明**：本功能是框架粘合（取 reply id → 调 API），唯一非网络逻辑是 stdlib `strconv.ParseInt`。`plugins/group` 现有命令（ban/revoke 等）与 `bot/api.go` 的 action 封装均无单测，端到端依赖 NapCat。因此本计划用 `go build` 保证编译、`go vet` 保证正确性，功能验证走 Task 3 的部署后手动清单——不写"测 mock 自己"的空测试。

---

### Task 1: `bot.SetEssenceMsg` 封装

**Files:**
- Modify: `bot/api.go`（紧跟 `DeleteMsg`，约 133-136 行）

- [ ] **Step 1: 实现**

在 `bot/api.go` 的 `DeleteMsg` 之后追加：

```go
func (a *BotAPI) SetEssenceMsg(msgID int32) error {
	_, err := a.call("set_essence_msg", map[string]any{"message_id": msgID})
	return err
}
```

- [ ] **Step 2: 构建确认**

Run: `go build ./...`
Expected: 无输出（编译通过）

- [ ] **Step 3: 提交**

```
git add bot/api.go
git commit -m "feat(bot): SetEssenceMsg 封装 set_essence_msg"
```
（提交信息用 `-m` 普通引号，勿用 bash here-string `@'...'@`。）

---

### Task 2: `essence` 插件 + 注册

**Files:**
- Create: `plugins/group/essence.go`
- Modify: `cmd/bot/main.go`（group 注册区，`group.RegisterRevoke(b)` 附近）

- [ ] **Step 1: 新建插件**

`plugins/group/essence.go`：

```go
package group

import (
	"strconv"

	"github.com/Yuelioi/yueling-go/bot"
)

func RegisterEssence(b *bot.Bot) {
	b.OnCommand("设精", "加精").
		Handle(func(ctx *bot.CommandContext) error {
			replyID, ok := ctx.Message().ReplyID()
			if !ok {
				return ctx.Reply("请回复要设精的消息后使用 /设精")
			}
			msgID64, err := strconv.ParseInt(replyID, 10, 32)
			if err != nil {
				return ctx.Reply("无法解析消息ID")
			}
			if err := ctx.SetEssenceMsg(int32(msgID64)); err != nil {
				return ctx.Reply("设精失败：" + err.Error())
			}
			return nil
		})
}
```

注意：**不挂** `perm.Admin`（普通用户可用），因此本文件不 import `bot/perm`。

- [ ] **Step 2: 注册**

在 `cmd/bot/main.go` 找到 `group.RegisterRevoke(b)`，在其后加一行：

```go
	group.RegisterEssence(b)
```

- [ ] **Step 3: 构建 + vet + 全量测试**

Run: `go build ./...`
Expected: 无输出

Run: `go vet ./plugins/group/ ./bot/`
Expected: 无输出

Run: `go test ./...`
Expected: 全部 PASS（无新增测试，确认未破坏现有）

- [ ] **Step 4: 提交**

```
git add plugins/group/essence.go cmd/bot/main.go
git commit -m "feat(essence): 设精命令（回复消息加群精华，普通用户可用）"
```

---

### Task 3: 手动验证（部署后，依赖 NapCat）

- [ ] 普通成员回复一条消息发 `设精` → 该消息进群精华（前提：bot 是群管理员）。
- [ ] 不回复直接发 `设精` → 回「请回复要设精的消息后使用 /设精」。
- [ ] bot 非管理员时设精 → 回「设精失败：…」（QQ 服务端拒绝）。
- [ ] 别名 `加精` 同样生效。
- [ ] 更新 `flightdeck/cockpit.md`。

---

## Self-Review

- **Spec 覆盖**：普通用户可用（Task2 不挂 perm.Admin）✓；回复取 message_id（Task2 ReplyID→ParseInt）✓；调 set_essence_msg（Task1 SetEssenceMsg）✓；成功静默 return nil（Task2）✓；无回复用法提示 + 失败回显（Task2）✓；别名 加精（Task2 OnCommand 第二参）✓；注册（Task2 Step2）✓；bot-admin 前提（Task3 手验覆盖）✓。
- **占位符**：无 TBD/TODO；每步完整代码/命令。
- **类型一致**：`SetEssenceMsg(int32) error`（Task1 定义，Task2 `ctx.SetEssenceMsg(int32(msgID64))` 调用一致；ctx 是 *CommandContext，经 GroupContext 嵌入 *BotAPI，故可直接调）。`OnCommand(...).Handle(func(*bot.CommandContext) error)` 与 revoke 同构。
