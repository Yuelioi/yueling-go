---
status: done
summary: 让 cond.Admin / cond.Owner 条件对 config.C.Bot.SuperUsers 也放行，超管无需群管/群主身份即可用禁言/撤回等管理命令；一处改动覆盖所有 .Where(perm.Admin) 命令
last_updated: 2026-06-15
---

# 超级管理员免群权限门槛

## 背景 / 目标

管理类命令（禁言、撤回、踢人、群文件、规则等）通过 `.Where(perm.Admin)` 把关，
`perm.Admin` = `cond.Admin`，仅当 `msg.Role()` 为 `admin`/`owner`（QQ 群管理/群主）时通过。
配置里的「超级管理员」(`config.C.Bot.SuperUsers`) 若本身不是该群的群管/群主，就用不了这些命令。

目标：超级管理员无条件满足 `Admin` 与 `Owner` 条件，从而能用所有受管理权限把关的命令。
心智模型：超管 ≥ 群主 ≥ 群管。

成功标准：

- 配置在 `[bot] superusers` 里的人，即使在群里是普通成员，也能用 `.Where(perm.Admin)`
  / `.Where(perm.Owner)` 的命令（禁言、撤回 等）。
- 群管/群主行为不变（仍通过）。
- 普通成员（非超管）行为不变（仍拒绝）。

## 改动

仅 `bot/cond/cond.go`：

- 新增私有 `isSuperUser(msg *bot.MsgCtx) bool`，遍历 `config.C.Bot.SuperUsers` 比对 `msg.UserID()`。
- `Admin` 条件：`role == "admin" || role == "owner" || isSuperUser(msg)`。
- `Owner` 条件：`role == "owner" || isSuperUser(msg)`。
- `cond` 包新增 import `github.com/Yuelioi/yueling-go/config`（无循环依赖：config 仅依赖 viper）。
- `SuperUser(ids ...int64)` 保持不变（reboot 仍用显式列表）。

一处改动即覆盖所有引用 `perm.Admin`/`perm.Owner` 的命令（ban.go 的禁言/撤回/踢人、
files.go、member_backup.go、rules.go），无需改各命令注册处。

## 不做（YAGNI）

- 不新增权限层级、不改 `.Where` API、不动命令注册处。
- 不把 `SuperUser(ids...)` 改成读全局配置。

## 测试

新增 `bot/cond/cond_test.go`（package cond），设 `config.C.Bot.SuperUsers = []int64{<超管ID>}`，
构造不同 `Role` + `UserID` 的 `MsgCtx`（`bot.MsgCtx{Event: &bot.GroupMessageEvent{...}}`），
`Admin.Check(nil, msg)` / `Owner.Check(nil, msg)` 断言：

- 群管（role=admin，非超管）→ Admin 通过、Owner 拒绝。
- 群主（role=owner）→ Admin 通过、Owner 通过。
- 超管（role=member）→ Admin 通过、Owner 通过。
- 普通成员（role=member，非超管）→ Admin 拒绝、Owner 拒绝。
