---
status: active
when_to_read: 新增或改造一个会发网络/AI/外部 API 调用的耗时命令时
applies_to: [command, react, emoji, ux, bot/context.go]
last_updated: 2026-06-18
---

# 耗时命令加进度表情提示

耗时命令（下载、AI 调用、外部 API）在开始干活时给触发消息贴一个表情回应，
让用户知道「命令已收到、正在执行中」，不用干等。

## 怎么做

handler 里、**过了用法/参数校验之后、真正开始耗时操作之前**加一行：

```go
ctx.React(bot.EmojiProcessing)
```

- `bot.EmojiProcessing`（="424"）是约定的「处理中」表情 id。
- `GroupContext.React` 是 best-effort：贴失败只 `logx.Warnf`，**不返回 error、不打断命令**。
- 底层是 `BotAPI.SetMsgEmojiLike(messageID, emojiID, set)` 包 `set_msg_emoji_like`。

## 注意

- **放在早返回校验之后**——否则「用法错误」的回复也会被贴上「处理中」，违和。
- 当前只在开始点、保留不动（不切「完成」表情、不撤）。要做完成态再说，别提前加复杂度。
- 已接入：pack / zssm / 翻译 / 场景识别。新增同类慢命令照此办理。
