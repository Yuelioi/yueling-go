# 主动群消息副作用跳过直接 AI 触发

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
