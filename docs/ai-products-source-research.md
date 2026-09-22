# AstrBot / LangBot：月灵 AI 架构选型的一手资料附录

调研日期：2026-09-22。只查官方文档、官方仓库源码；未安装、运行或部署这些产品，未改月灵业务代码或配置。此文件支撑选型，不构成上线验收，也不以功能数量、Star 数或产品自称的“生产级”证明可靠性。

## 版本与证据等级

- **AstrBot**：官方仓库固定提交 [`95e98b8aed75d56713666eff39e31bafffd95426`](https://github.com/AstrBotDevs/AstrBot/tree/95e98b8aed75d56713666eff39e31bafffd95426)，提交时间 2026-09-21 16:27 +08:00。
- **LangBot**：官方仓库固定提交 [`6b83089535c1b71bae45f42bdb522d67f20be3eb`](https://github.com/langbot-app/LangBot/tree/6b83089535c1b71bae45f42bdb522d67f20be3eb)，提交时间 2026-09-22 00:43 +08:00。
- 下文 **源码确认** 指上述提交中看到了具体实现；**文档确认** 指官方说明支持该能力，但未验证运行行为；**推断** 是面向月灵的工程判断；**未确认** 不代表产品绝对没有，只表示本次查阅未获得足够依据。
- 两个固定点均为调研时的 `master`，不是已经验收的发布版本。选型实验必须再固定可部署版本，并核对所依赖 API 是否已经发布。

## 先回答“必须自己重写吗”

**不必预设继续自研，也不必预设整机迁移。两个产品都有程序调用入口，可以保留月灵的 QQ 接入、权限和既有业务，把 AI 运行交给独立服务。**

| 方案 | 官方依据 | 对月灵的真实含义 |
| --- | --- | --- |
| AstrBot 全量承接 QQ 与 AI | OneBot v11 反向 WebSocket 适配，官方教程明确列出 NapCat。[接入文档](https://docs.astrbot.app/platform/aiocqhttp.html) | 可替换消息入口；月灵专有插件、权限、定时任务和历史数据仍需迁移或适配，不能因“支持 QQ”就认定等价替换。 |
| AstrBot 只承接 AI | 文档声明 OpenAPI 可调用内置 Agent/插件/工具；`POST /api/v1/chat` 返回 SSE，支持 API Key、`username`、`session_id`；源码另有 `/chat/ws`。[固定版本 OpenAPI 文档](https://github.com/AstrBotDevs/AstrBot/blob/95e98b8aed75d56713666eff39e31bafffd95426/docs/en/dev/openapi.md)、[API 实现](https://github.com/AstrBotDevs/AstrBot/blob/95e98b8aed75d56713666eff39e31bafffd95426/astrbot/dashboard/api/open_api.py) | 是值得实测的 AI 服务候选。调用者是 WebChat 身份，QQ 群身份、群历史、业务权限需显式传递和校验；不能假设自动继承 NapCat 场景。 |
| LangBot 全量承接 QQ 与 AI | 官方提供 NapCat 接入与 Bot→Pipeline 绑定。[NapCat 文档](https://docs.langbot.app/en/usage/platforms/qq/aiocqhttp/napcat)、[Pipeline 文档](https://docs.langbot.app/en/usage/pipelines/readme) | 可以使用其管理面板、模型和知识库配置；现有 Go 插件仍不能直接装入它的插件系统。 |
| LangBot 只承接 AI | HTTP Bot 接受签名消息并进入配置的 Pipeline；异步回调带 `reply_to`、`sequence`、`is_final`，另有 `/sync`。[HTTP Bot 文档](https://docs.langbot.app/en/usage/platforms/http-bot)、[固定版本适配器](https://github.com/langbot-app/LangBot/blob/6b83089535c1b71bae45f42bdb522d67f20be3eb/src/langbot/pkg/platform/sources/http_bot.py) | 是另一个可实测的 AI 服务候选。月灵可做唯一 QQ 收发端，同时保留业务权限；需实现会话映射、回调验签、重复回调处理和取消/超时约定。 |

**双接入风险是本次推断，不是已复现的产品缺陷：**若同一个 NapCat 消息同时交给旧月灵与新产品，且两边触发条件重叠，就可能重复回复、重复执行业务。试验应使用独立测试群/账号，或让月灵保持唯一消息入口，仅调用上述 HTTP 服务。消息入口不能靠双方提示词约定所有权。

## AstrBot：值得看具体机制的地方

| 维度 | 已确认机制与依据 | 对月灵的借鉴与边界 |
| --- | --- | --- |
| Agent 与输出渠道 | **源码确认**：Agent 产生 `tool_call`、`tool_call_result`、`llm_result` 等事件；运行层分别记录 trace，再按 WebChat/普通渠道以及 `show_tool_use`、`show_reasoning` 等开关决定展示。[事件消费](https://github.com/AstrBotDevs/AstrBot/blob/95e98b8aed75d56713666eff39e31bafffd95426/astrbot/core/astr_agent_run_util.py#L193) | 月灵应区分执行事件与用户回复，不能把任意内部字符串当正文发送。该机制仍不能证明能拦住模型自己在正文中生成的伪 `tool call` 文本。 |
| 工具协议适配 | **源码确认**：OpenAI 适配器处理空 assistant、孤立/重复 tool 消息，解析结构化工具调用，保留推理字段及工具额外字段。[OpenAI 适配器](https://github.com/AstrBotDevs/AstrBot/blob/95e98b8aed75d56713666eff39e31bafffd95426/astrbot/core/provider/sources/openai_source.py#L451) | 兼容逻辑属于 Provider 边界；“都兼容 OpenAI”不能消除差异。月灵当前模型仍需单独做多轮协议回放测试。 |
| 超时、重试与回退 | **源码确认**：提供统一请求重试，识别连接/超时及部分 HTTP 状态，指数等待；OpenAI SDK 自带重试被关闭，避免层层叠加。[重试模块](https://github.com/AstrBotDevs/AstrBot/blob/95e98b8aed75d56713666eff39e31bafffd95426/astrbot/core/provider/sources/request_retry.py)、[SDK 设置](https://github.com/AstrBotDevs/AstrBot/blob/95e98b8aed75d56713666eff39e31bafffd95426/astrbot/core/provider/sources/openai_source.py#L361) | 可借鉴“由一处拥有重试策略”。不能直接照搬默认 5 次尝试：月灵要先定整个用户请求的时间与成本上限。模型重试也不等于业务动作可以重试。 |
| 工具执行边界 | **源码确认**：独立工具执行器统一接受本地 handler/MCP 工具；本地异步调用包在超时中，并将返回值转为 `CallToolResult`。[工具执行器](https://github.com/AstrBotDevs/AstrBot/blob/95e98b8aed75d56713666eff39e31bafffd95426/astrbot/core/astr_agent_tool_exec.py#L650) | 可以借鉴独立执行边界与超时传播。但这里的包装不能证明工具副作用被事务回滚，也不能推定超时后远端动作一定没完成。 |
| 群聊感知与总结 | **源码确认**：`GroupChatContext` 按 UMO 保存内存 `deque`、限制数量；下一次模型请求注入此前记录，并消费已处理部分。[群上下文](https://github.com/AstrBotDevs/AstrBot/blob/95e98b8aed75d56713666eff39e31bafffd95426/astrbot/builtin_stars/astrbot/group_chat_context.py#L44)。**文档确认**：隔离对话是可配置项。[配置文档](https://docs.astrbot.app/dev/astrbot-config.html) | “感知群聊”与“查询昨天全部群消息并给出可追溯总结”不是同一能力。未确认内核提供完整的按日期群历史总结、覆盖率报告或月灵 PostgreSQL 记录的导入。 |
| 会话与压缩 | **源码确认**：会话锁单独管理；上下文可按完整轮次截断或做摘要，保留最近完整轮次。[会话锁](https://github.com/AstrBotDevs/AstrBot/blob/95e98b8aed75d56713666eff39e31bafffd95426/astrbot/core/utils/session_lock.py)、[摘要压缩](https://github.com/AstrBotDevs/AstrBot/blob/95e98b8aed75d56713666eff39e31bafffd95426/astrbot/core/agent/context/compressor.py#L115) | 可借鉴“不截断工具调用与结果的配对”。压缩会话不等于可靠的长期事实记忆，后者还需来源、纠错与删除策略。 |
| 知识库 | **源码确认**：检索层协调稠密向量、BM25、RRF 融合及可选 Rerank，并返回文档/片段标识。[检索管理器](https://github.com/AstrBotDevs/AstrBot/blob/95e98b8aed75d56713666eff39e31bafffd95426/astrbot/core/knowledge_base/retrieval/manager.py) | 是月灵词面检索之外的现成候选，但需要以本群真实问题测召回与引用正确率；不能以组件存在代替效果。 |
| 观察与调试 | **文档确认**：WebUI 有原始会话 JSON、工具状态、Token/耗时、日志和 trace；文档明确追踪目前只覆盖部分主 Agent 路径。[WebUI 文档](https://docs.astrbot.app/use/webui.html) | 借鉴每个请求能追到模型、工具与发送环节。不可宣称全链路都已覆盖；启用详细记录前仍需核对记录内容和访问范围。 |
| 部署 | **源码确认**：基础 Compose 为 AstrBot 容器、持久化 `data`、WebUI 6185 和可选 OneBot 6199；NapCat 另有组合部署链接。[Compose](https://github.com/AstrBotDevs/AstrBot/blob/95e98b8aed75d56713666eff39e31bafffd95426/compose.yml) | 采用现成服务可省掉一部分 AI 内核维护，但新增 Python 服务、数据卷、插件和版本升级管理。不是零运维。 |

补充：仓库存在重试、工具循环、群上下文和 API Key 等专项测试，例如 [request retry 测试](https://github.com/AstrBotDevs/AstrBot/blob/95e98b8aed75d56713666eff39e31bafffd95426/tests/test_request_retry.py) 与 [tool loop 测试](https://github.com/AstrBotDevs/AstrBot/blob/95e98b8aed75d56713666eff39e31bafffd95426/tests/test_tool_loop_agent_runner.py)。本次仅阅读，未运行，也未以测试文件存在宣称覆盖充分。

## LangBot：值得看具体机制的地方

| 维度 | 已确认机制与依据 | 对月灵的借鉴与边界 |
| --- | --- | --- |
| Pipeline / Runner 边界 | **文档确认**：Bot 绑定 Pipeline；Pipeline 选择内置 Agent 或 Dify/n8n 等外部 Runner。使用外部 Runner 时，模型、提示词、工具由对应外部平台提供。[Pipeline 文档](https://docs.langbot.app/en/usage/pipelines/readme) | 借鉴“可替换 AI 执行后端”的边界。不能理解成任意 Runner 都自动继承 LangBot 内置工具、知识库和权限。 |
| 工具协议 | **源码确认**：Local Agent 聚合流式工具片段和 Provider 特有字段；LiteLLM 适配层处理 reasoning 与工具调用格式。[本地 Agent](https://github.com/langbot-app/LangBot/blob/6b83089535c1b71bae45f42bdb522d67f20be3eb/src/langbot/pkg/provider/runners/localagent.py#L52)、[模型适配层](https://github.com/langbot-app/LangBot/blob/6b83089535c1b71bae45f42bdb522d67f20be3eb/src/langbot/pkg/provider/modelmgr/requesters/litellmchat.py#L700) | 可以直接评估它承担兼容维护的收益。仍需用月灵的模型和请求回放；不能因为依赖 LiteLLM 就认为所有特殊字段已可靠支持。 |
| 回退位置 | **源码确认**：先按主模型/备用模型尝试；流式返回首个 chunk 后提交模型选择；后续工具循环固定使用该模型，代码明确避免中途换模型解释既有工具结果。[回退实现](https://github.com/langbot-app/LangBot/blob/6b83089535c1b71bae45f42bdb522d67f20be3eb/src/langbot/pkg/provider/runners/localagent.py#L295)、[工具循环](https://github.com/langbot-app/LangBot/blob/6b83089535c1b71bae45f42bdb522d67f20be3eb/src/langbot/pkg/provider/runners/localagent.py#L592) | “失败后换模型”必须有阶段边界。月灵不能在已经产生副作用后无条件从头重跑请求。该源码还有 128 轮工具调用硬上限，不宜照搬到普通 QQ 问答。 |
| 请求预算与错误出口 | **源码确认**：LiteLLM requester 默认配置为 timeout 120、num_retries 0；聊天处理器把用户提示、错误和调试信息分开，允许 show-hint/show-error/hide。[requester 配置](https://github.com/langbot-app/LangBot/blob/6b83089535c1b71bae45f42bdb522d67f20be3eb/src/langbot/pkg/provider/modelmgr/requesters/litellmchat.py#L198)、[异常出口](https://github.com/langbot-app/LangBot/blob/6b83089535c1b71bae45f42bdb522d67f20be3eb/src/langbot/pkg/pipeline/process/handlers/chat.py#L193) | 可借鉴错误面向不同受众输出。上述默认字段不证明存在整个 Agent 回合的统一截止时间；本次也未确认所有插件/MCP 使用同一工具超时策略。 |
| 工具与观测 | **源码确认**：ToolManager 分发 native/plugin/MCP/skill；记录工具来源、耗时、结果/异常及关联消息。监控失败不会替代工具执行结果。[工具管理器](https://github.com/langbot-app/LangBot/blob/6b83089535c1b71bae45f42bdb522d67f20be3eb/src/langbot/pkg/provider/tools/toolmgr.py#L199) | 这比散落日志更便于诊断。但监控中的 success 是调用未抛异常，不等于业务已验证成功；仍需要月灵工具定义明确结果。参数/结果会传给监控层，不能默认没有敏感内容。 |
| 会话隔离 | **源码确认**：会话索引包含 instance、workspace、generation、bot、launcher type/id；会话有并发控制与过期回收。[会话管理器](https://github.com/langbot-app/LangBot/blob/6b83089535c1b71bae45f42bdb522d67f20be3eb/src/langbot/pkg/provider/session/sessionmgr.py#L34) | 现成多 Bot 隔离有价值，但默认 launcher 粒度不一定等于月灵的“群+用户”；采用前要明确群共享、用户独立和后台摘要的会话键。 |
| 群总结、记忆与知识库 | **文档确认**：知识库由 Knowledge Engine 插件提供索引/检索，绑定 Local Agent，支持可选 rerank；外部 Runner 使用自己的机制。[知识库文档](https://docs.langbot.app/en/usage/knowledge/readme)。**未确认**：查阅核心 Runner、session 和 pipeline 后，未找到与月灵等价的按日期完整群历史总结或长期用户事实记忆闭环。 | 知识库可直接评估，群总结仍可能需要自定义工具或工作流；不能将“多轮会话”“RAG”“完整群历史”混为一种记忆。 |
| 独立接入协议 | **源码确认**：HTTP Bot 验证请求签名、有限容量内存幂等缓存、每会话回调队列；回调失败有重试，`/sync` 收集多段回复。[HTTP Bot 实现](https://github.com/langbot-app/LangBot/blob/6b83089535c1b71bae45f42bdb522d67f20be3eb/src/langbot/pkg/platform/sources/http_bot.py) | 对保留 Go 业务很有实用价值。内存幂等键不是持久化事务：重启后的去重、回调确认丢失、跨进程顺序仍需验证；不能宣称 exactly-once。 |
| 部署与插件边界 | **源码确认**：官方 Compose 包含 LangBot 和独立 plugin runtime；Box 是可选 profile，提供 sandbox/部分 Skills/stdio MCP 能力。[Compose](https://github.com/langbot-app/LangBot/blob/6b83089535c1b71bae45f42bdb522d67f20be3eb/docker/docker-compose.yaml) | 可利用成熟插件接口，但有额外进程与版本协调成本。月灵若只需要总结、检索和业务工具，不应把代码沙箱作为必选依赖。 |

仓库有针对工具返回内容、重复输出、模型转换和会话隔离的专项测试；例如 [工具消息内容测试](https://github.com/langbot-app/LangBot/blob/6b83089535c1b71bae45f42bdb522d67f20be3eb/tests/unit_tests/provider/test_localagent_tool_content.py)、[重复回复测试](https://github.com/langbot-app/LangBot/blob/6b83089535c1b71bae45f42bdb522d67f20be3eb/tests/unit_tests/provider/test_localagent_no_duplicate.py)。这说明这些兼容问题也需要持续修复，不能推断月灵迁入后自动免疫。

## 迁移成本：三条路线应公平比较

以下是根据上述接口与月灵现有职责做出的**推断**，不是已经测得的工期或成本。

| 迁移项目 | 直接采用现成机器人 | 保留月灵入口，接现成 AI 服务 | 继续自建 AI 内核 |
| --- | --- | --- | --- |
| QQ 接入和普通命令 | 迁移入口与触发规则；已有适配器可省维护 | 继续使用现有入口，新增 HTTP/SSE/回调适配 | 沿用当前代码 |
| 月灵权限、确认与群开关 | 重新映射，逐项验证行为等价 | 月灵维持最终鉴权，外部工具经受控接口调用；避免仅靠提示词授权 | 自己维护鉴权与执行边界 |
| 既有 Go 插件 | 重写插件或封装成 HTTP/MCP 服务 | 可逐项暴露为明确工具接口，保留业务实现 | 原生复用，但继续承担注册/协议/循环维护 |
| 群历史与用户记忆 | 导出/导入或建立读接口；要核对数据语义 | 提供有权限的查询接口，显式传群和时间范围 | 继续使用现有数据库，补数据范围与状态模型 |
| 模型兼容、RAG、界面 | 获得项目已有模块并跟随升级 | 同样可复用其 AI 能力，需确认 API 路径暴露程度 | 维护成本最大，但控制最直接 |
| 故障排查与运行 | 一个新平台替代旧核心，迁移风险集中 | 多一个服务边界，需要关联请求 ID 和端到端超时 | 部署变化较少，但所有故障机制由自己负责 |

合理的下一步是先把 **AstrBot OpenAPI、LangBot HTTP Bot、月灵现有实现**放进同一验收表，必要时再纳入其他产品；不先宣布其中任何一条获胜。重点比较：明确群总结、带日期追问、模型异常、工具协议、重复消息、执行后模型失败、取消、跨群隔离和既有权限，而不是“能聊天”一次。

采用独立 AI 服务并非为了永久保留 Go；它也可以是验证全量迁移的过渡路线。反过来，如果实测发现接口无法承载月灵的上下文和工具权限，或者额外运维成本超过收益，再选择自建也有依据。

## 本次尚不能回答的事

1. 哪个产品在月灵真实模型、NapCat 版本、群消息量下成功率和延迟更好：未运行比较试验。
2. 原始“总结群聊不可用”的线上根因：源码调研不能替代月灵当次请求日志。
3. 第三方总结/记忆插件是否足够可靠：本次未逐个审计插件，不能用插件市场列表充当结论。
4. 这些 `master` 能力在选定稳定发布中是否全部存在：需在实验前核对版本。
5. 动作执行是否具有持久化幂等、崩溃恢复及完整审计：本次未获得可覆盖所有工具的保证，任何路线都要按月灵具体业务测试。
