# 月灵 AI：当前代码验证记录

日期：2026-09-22。测试执行于 v1.22.0 发布批次提交前，生产群尚未验收。架构选择见 [对照总报告](ai-evaluation.md)；候选产品不是本报告的测试对象。

## 执行结果

| 检查 | 实际结果 | 边界 |
| --- | --- | --- |
| `go test -json ./... -count=1` | 退出码 0；517 个测试/子测试通过，99 个跳过；37 个包通过，17 个包无测试 | 99 个跳过项均因未设置 `YUELING_TEST_DATABASE_DSN`。图片测试临时使用本机 PingFang 字体链接，已移除 |
| 相关 9 个包的 `go test -race` | 退出码 0；289 个测试/子测试通过，30 个数据库项跳过 | AI、工具、入口、聊天资料、模型、Bot、配置、订阅、图文解释路径；不是全仓库压力测试 |
| `go vet ./...`、`git diff --check` | 通过 | 静态检查不能代替运行验收 |
| `go run ./cmd/ai-check -config config.toml` | 结构化调用 → 新 ToolResult 包络 → 整理 → 跨轮事实保留通过；无 Tools 的纯文本总结通过 | 当前配置的真实模型，仅两条虚构资料；不访问 QQ/数据库，不输出原始回复/凭证，不代表真实长群聊质量 |
| WebSocket 收发集成 | `TestSummaryThroughWebSocketAndModelRecovery` 通过 | 真实 Bot 处理器、注册工具及 Dispatch；OneBot/模型为本地模拟，包含 503、伪工具正文纠正、连续总结/待办 |
| 带数据库的日期收发集成 | 已保留用例，本次跳过 | `TestDatedSummaryThroughWebSocketAndDatabase` 含跨群资料与插件关闭断言，但没有本次执行通过证据 |

计数来自 `go test -json` 的 pass/skip 事件，包含父测试和子测试，不能当作独立用户场景数量。汇总见 [机器可读执行记录](../scripts/ai-eval/yueling/results.json)。上述统计来自最后一轮取消时序修复之后的完整复跑。

本机 Docker 管理接口持续超时；本地 PostgreSQL 端口可达，但此前专用测试库未取得有效连接。没有重启 Docker、修改其他容器或改连非本机数据库。更早改动带独立 PostgreSQL/zhparser 通过的记录仍保留在 [ai-rebuild.md](ai-rebuild.md)，不冒充当前最终代码的数据库验证。

## 场景覆盖

| 场景 | 本次证据 | 尚未覆盖或仍有的限制 |
| --- | --- | --- |
| 群聊总结 | 固定流程先取真实注册入口提供的资料，再请求纯文本；模拟 WebSocket 收发通过 | 真实 NapCat 历史权限、真实群长文本质量 |
| 日期/类型连续追问 | recent → yesterday → actions 每次重新读取；today → 详细解释 → yesterday 保留任务；新主题/引用/附件结束任务 | 仅明确语法进入固定流程，复杂自然语言仍依赖通用模型；日期 SQL 本次未跑 |
| 空记录、接口断开 | 空资料、读取错误、取消分别处理，未拿到有效资料不调用模型 | 实际网络断开、数据库重启的联调 |
| 超长资料 | 按实际 JSON 编码预算截断、保留 ID；`<` 和控制字符导致转义膨胀的回归通过 | 前 N 条取样，未实现整日分页/分段聚合 |
| 429/503/超时/401 | 临时错误有界重试、认证错误不重试、取消停止等待、原始提供商正文不出现在错误提示；收发集成注入 503 | 真实限流策略与高负载排队 |
| 畸形参数、截断调用、伪工具正文 | 参数验证先于 handler；不完整调用不执行；模型格式纠正有界；异常正文不直接投递 | 更多提供商样本；未来流式输出需独立验证 |
| 动作执行后生成失败 | 503 重试不重放工具；永久失败区分只读与可能有副作用，保留调用/结果配对 | 跨用户新请求、进程重启后的持久业务幂等 |
| 等价动作去重 | 真实提醒注册经 Dispatch，省略 repeat 与 none、空白和等价时区共享回执；不同内容/时刻仍分别执行；update 缺省含义不变 | 使用计数 handler，未实际落库；仅一次性提醒有业务归一化，其余工具按解析后 JSON |
| 权限、禁用、隔离 | 工具权限/暴露集合门禁、确认绑定群、会话按群/用户分离；总结无权限不读资料 | 数据库群开关与日期跨群集成本次跳过 |
| 同会话竞争与重置 | 活跃/排队取消、过期替换、驱逐、父请求取消、新会话独立及 Get/acquire/Delete race 通过 | 未测生产吞吐、排队上限体验 |
| 生成/确认/发送时取消 | Dispatch 的模型请求及发送 callback 继承取消；确认预取消不消耗码；迟到成功不投递/不写历史 | 已发送 QQ 包和已落地外部副作用不能撤回；部分旧 DB 业务不支持 context 取消 |
| 重复入站 | 同一消息执行中及完成后抑制重复，重置会话不清除去重记录，其他群独立 | 进程内 5 分钟；消息 ID 为 0 不去重；没有跨重启动作账本或外接回调系统 |
| 模型回退 | 未实现自动备用模型切换；单提供商历史 reasoning/tool 配对回归通过 | 不标为已具备能力；不能用 HTTP 重试冒充模型回退 |
| 知识/长期记忆 | 相关代码编译，自动记忆继承取消，源码检查作用域保持不变 | SQL 检索与记忆删除用例本次跳过；个人记忆仍按 userID 共享，群历史按 groupID 隔离 |

关键测试入口：[运行时](../ai/runtime_test.go)、[会话生命周期](../ai/session_lifecycle_test.go)、[确认](../ai/confirmation_control_test.go)、[总结流程](../ai/summary_workflow_test.go)、[资料预算](../services/chatsummary/summary_test.go)、[工具与参数](../ai/registry_tooling_test.go)、[提醒动作](../ai/tools/reminder_test.go)、[WebSocket 集成](../plugins/ai_dispatch/integration_test.go)、[模型策略](../services/llm/client_test.go)。

## 先复现再修复的缺口

1. 原 Route 对“总结群聊”暴露总结工具，对“昨天呢”“只列待办”不暴露，导致模型即使知道上文任务，也不能重新取数。现把明确总结流程及任务 Query 独立于关键词路由。
2. 原 Session 删除只移除 map；已持有旧对象的执行和等待者仍继续。新增排队取消测试先失败，再以 lifetime 绑定活跃回合、确认、发送和自动记忆。
3. 独立复查发现“详细解释一下”之后清空总结任务；回归先失败，现只为明确简短追问保留任务。
4. 独立复查发现相同一次性提醒通过省略/显式默认参数绕过去重，消耗调用预算且可能创建两次。现使用注册级 ActionKey，执行器无需硬编码提醒业务。
5. 独立复查发现首条特殊文本转义超预算时被整条删掉，仍返回成功空资料。现按实际编码长度截短，最终无有效记录时返回明确错误。
6. 最后复查发现普通预检/确认鉴权之后才绑定会话，期间重置会让旧请求拿到新会话继续；引用读取也没有绑定取消。先以 GORM DryRun/连接池夹具和真实本机 WebSocket 复现，再把会话绑定前移，所有后续提示沿原会话发送。未改为 context 查询的既有数据库预检仍需等待调用返回，返回后不能恢复已取消请求。

以上均是代码或测试复现的缺口。用户原始“AI 暂时不可用”的生产日志尚未取得，不能把这些问题中的任意一项说成那次故障的唯一根因。

## 复现

```sh
go test ./... -count=1
go test -race ./ai/... ./plugins/ai_dispatch ./services/chatsummary ./services/llm ./bot ./config ./services/feed ./plugins/tools -count=1
go vet ./...
git diff --check
go run ./cmd/ai-check -config config.toml

# 仅使用专用测试数据库；测试创建/删除独立 schema，并需要 zhparser/pg_trgm。
YUELING_TEST_DATABASE_DSN='<dedicated-test-database-url>' go test ./... -count=1
YUELING_TEST_DATABASE_DSN='<dedicated-test-database-url>' go test -race ./ai/... ./plugins/ai_dispatch ./services/chatsummary ./services/llm ./bot ./config ./services/feed ./plugins/tools -count=1
```

本机执行使用 `GOCACHE=/private/tmp/yueling-go-build`；原始 Go JSON 日志存于 `/private/tmp/yueling-ai-final-full.jsonl` 和 `/private/tmp/yueling-ai-final-race.jsonl`。可移植的命令不依赖这些临时路径。候选复现脚本和结果另见各自报告。
