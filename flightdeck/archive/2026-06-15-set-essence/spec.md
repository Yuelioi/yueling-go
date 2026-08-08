---
status: done
summary: 设精命令——回复一条消息发「设精/加精」把它加入群精华，普通用户即可用（不加 perm.Admin）；新增 bot.SetEssenceMsg 封装 set_essence_msg + plugins/group/essence.go
last_updated: 2026-06-15
---

# 设精 — 把消息加入群精华

## 背景 / 目标

群友想把某条消息设为群精华，且**不限管理员**——普通成员也能用。当前没有精华相关功能。
QQ/NapCat 提供 `set_essence_msg`（参数 `message_id`）。

目标：用户**回复**目标消息并发 `设精`（别名 `加精`），bot 将该消息加入群精华。
触发命令的人无需任何群权限。

成功标准：

- 普通成员回复一条消息发 `设精` → 该消息进入群精华。
- 不加任何用户权限门槛（无 `perm.Admin` / `perm.Owner`）。
- 未回复任何消息时给用法提示；失败回显原因。

## 已知前提（QQ 限制，非本功能可绕过）

`set_essence_msg` 在服务端要求**机器人本身是群管理员**。机器人非管理员时 API 报错，
经失败分支回显给用户。这是 QQ 的限制，本功能只负责不对**调用者**设权限门槛。

## 改动

**新增 API** `bot/api.go`（紧跟 `DeleteMsg`）：

```go
func (a *BotAPI) SetEssenceMsg(msgID int32) error {
	_, err := a.call("set_essence_msg", map[string]any{"message_id": msgID})
	return err
}
```

**新增插件** `plugins/group/essence.go`（与 `RegisterRevoke` 同构，但**不挂** `perm.Admin`）：

```go
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

**注册**：`cmd/bot/main.go` 的 group 区加 `group.RegisterEssence(b)`。

## 行为

- 触发：回复目标消息 + 发 `设精`（别名 `加精`）；`message_id` 取自 `ReplyID()`。
- 成功**静默**（与 `revoke` 一致；QQ 自带「xx 设置了精华消息」系统提示）。
- 失败回 `设精失败：<原因>`；无回复回用法提示。

## 不做（YAGNI）

- 取消精华（`delete_essence_msg`）。
- 对当前消息直接设精（精华必须针对目标消息，故必须回复）。
- 注册处的权限/确认包装。

## 测试

纯框架粘合（取 reply id → 调 API），逻辑与既有 `revoke` 完全一致；`plugins/group` 现有命令
均无单测，端到端依赖 NapCat。采**部署后手动验证**：

- 普通成员回复一条消息发 `设精` → 该消息进群精华（前提 bot 是管理员）。
- 不回复直接发 `设精` → 回用法提示。
- bot 非管理员时设精 → 回 `设精失败：…`。
