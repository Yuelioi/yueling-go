# ⚠ 主动发言不要重复处理直接 AI 触发消息

SUMMARY: 主动发言这类 catch-all 群消息处理器若也更新 AI 聊天副作用，必须跳过 @bot / bot 名开头的直接 AI 触发消息，避免同一条消息被 Dispatch 和 Proactive 双计分。
READ WHEN: 修改 AI dispatch、主动发言、catch-all 群消息处理器，或给群消息链路增加隐藏计分/记忆/限流等副作用前。

---

`ai_dispatch` 处理 `@月灵` 或消息以 bot 名开头的直接 AI 对话；`ai_proactive`
是低优先级 catch-all，也会看到同一条群消息。

如果主动发言也调用隐藏好感度、记忆写入、限流或其他副作用，同一条直接 AI 消息会被处理两次：

1. `ai.Dispatch` 先执行直接对话链路。
2. dispatch 继续落到 catch-all `ai.Proactive.Feed`。
3. catch-all 再执行一次副作用。

做法：主动发言入口先复用直接 AI 触发语义过滤：

- 消息 `@` 目标包含 `SelfID`。
- `strings.TrimSpace(text)` 以 `config.C.Bot.Name` 开头，且 bot 名非空。

这些消息只由 `ai.Dispatch` 计分/记忆/限流；主动发言不积热、不触发、不重复写副作用。
