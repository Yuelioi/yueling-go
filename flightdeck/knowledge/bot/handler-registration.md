# Bot 功能按统一注册契约接入

新增 Bot 功能时，在 `b.Start()` 之前完成全部注册。普通插件暴露 `Register(*bot.Bot)` 并由 [`cmd/bot/main.go`](../../../cmd/bot/main.go) 显式调用；AI 工具通过 `ai.Register` 注册，工具包由入口的空导入触发初始化。

Handler 只使用框架支持的四种签名：`func(*bot.CommandContext) error`、`func(*bot.GroupContext) error`、`func(*bot.NoticeContext) error`、`func(*bot.RequestContext) error`。注册时会校验签名，不能用相似但未支持的函数类型绕过。

受群级开关管理的命令必须调用 `.Plugin(catalog.<稳定ID>)`，并同步维护 [`plugins/system/help.go`](../../../plugins/system/help.go) 的目录。命令或精确短语使用 `OnCommand` / `OnFullMatch`；不要用 `OnKeyword` 或 catch-all 模拟命令，因为 dispatcher 只把前两者标记为 `commandMatched`，复读等兜底逻辑依赖这个标记。

有外部副作用或高风险的 AI 工具应设置与风险相符的权限，并在需要用户确认时启用 `ConfirmRequired`；不要仅靠提示词约束危险操作。
