# AstrBot 运行验证（2026-09-22）

结论：AstrBot 已实际运行，不能直接据此替换月灵 AI。它的 Runner、模型重试、工具超时和事件类型边界有可验证的基础，但本轮发现三处不满足月灵当前验收要求：相同副作用工具重复执行、上游错误细节进入回复文本、模型伪工具文本原样输出。全部测试都使用虚构消息与本地假模型，没有调用真实模型或 QQ。

## 版本与验证层次

- 上游仓库：<https://github.com/AstrBotDevs/AstrBot>。
- 源码版本：`95e98b8aed75d56713666eff39e31bafffd95426`，版本号 4.28.1。
- CPython 3.12.9，macOS arm64。
- 未修改上游源码；运行后上游 `git status --short` 为空。
- 上游此版本没有提交 `uv.lock`。本轮从官方 `requirements.txt` 安装，具体安装版本冻结在 `scripts/ai-eval/astrbot/requirements.lock`，这是本次评测的依赖快照，不是上游发布锁文件。
- 自定义夹具直接运行真实 `ToolLoopAgentRunner`、`ProviderOpenAIOfficial`、`FunctionToolExecutor`，通过 127.0.0.1 HTTP 与脚本模型通信。模型与业务工具是测试替身，框架运行逻辑未替换。
- 上游 Runner、ChatService 测试实际运行：**72 passed, 1 warning**。其中部分持久化依赖使用上游测试自带 mock，不能称为完整持久化链验证。
- 上游 API Key / OpenAPI 测试实际运行：**14 passed, 3 warnings**，真实初始化 CoreLifecycle、FastAPI 测试适配器和隔离 SQLite，覆盖 API scope、撤销、管理员限制及会话 username 归属。测试中的对话流生成被上游 fixture 替换，不能当作完整模型对话验证。
- 尚未跑通外部 HTTP OpenAPI → 完整消息管道 → 实际 QQ 的全程测试，也未评估真实模型摘要质量、并发容量、进程重启恢复。

## 共同夹具与实测结果

群聊仅含“甲：周五发布”“乙：周四测试”；读工具为 `summarize_chat`。副作用工具 `create_reminder` 仅增加内存计数并返回 `receipt=fixture-1`。

| 场景 | 实际结果 | 判断 |
| --- | --- | --- |
| 流式响应包含 `reasoning-only`、工具调用和正文 | `reasoning`、`tool_call`、`tool_call_result` 与普通正文分开；读工具执行 1 次 | 组件边界通过，调用方必须按事件类型消费 |
| 副作用执行后，合成请求首次 503，随后恢复 | HTTP 共 3 次；副作用 1 次；重试带相同历史 | 通过，无工具重放 |
| 副作用执行后，合成请求持续 503 | HTTP 共 3 次；副作用 1 次；Runner 进入 ERROR | 无重放，但错误输出不满足要求 |
| 持续 503 的响应体含 `secret-provider-body` | 普通 `err` 消息正文包含完整标记与错误类型 | 不通过，边界需统一错误映射 |
| 同回合两次同名同参数 `create_reminder`（不同 call ID） | 副作用计数 2 | 不通过，不能依赖框架保证业务幂等 |
| 模型把 `<tool_call>summarize_chat({})</tool_call>` 放进 content | 作为普通 `llm_result` 文本原样返回 | 不通过，事件分流不等于正文协议污染过滤 |
| OpenAPI 访问其他 username 的会话 | 上游 API 测试实际拒绝，返回 `session_id belongs to another username` | API 归属检查通过；真实 QQ 身份映射仍由接入方负责 |
| 使用上游历史序列化续接“周几测试？” | 下一请求包含既有群聊；独立 Runner 未提供历史时不含前群内容 | 仅验证调用方传入上下文隔离，不代表服务端会话隔离 |
| 模型 HTTP 一直等待，调用 `runner.request_stop()` | 约 1 ms 内返回 aborted，HTTP 请求只发 1 次 | 组件取消通过；不等于远端服务撤销 |
| 工具等待 30 秒，框架超时设 1 秒 | 约 1.015 秒取消工具协程，工具计数 1 | 组件超时通过；外部已完成副作用不能靠取消撤销 |

本轮将 `request_max_retries` 设置为 2 次总尝试，使故障场景有固定时限；不能把本轮 HTTP 次数当作默认配置行为。

原始结构化证据：`scripts/ai-eval/astrbot/results.json`。探针正常退出表示场景完成，负面观察仍明确记录为 false/true；**不表示候选全项通过**。

## SSE 生命周期的接入约束

上游测试 `test_chat_stream_disconnect_does_not_own_run_lifecycle` 实际通过：关闭 SSE 订阅后后台 run 继续，并能保存后续结果。它是为刷新/断线恢复设计的行为。

因此月灵端的 HTTP 超时、连接中断或取消，不能直接视作 AstrBot 已停止执行，更不能自动重发有副作用的整轮任务。接入时要保存稳定的任务标识、区分订阅中止与执行中止，并验证停止接口及结果查询的会话归属。此轮只验证了 Runner 的 `request_stop()`，没有把停止 API 的完整外部接入判为通过。

## 可复现命令

源码与临时环境可换路径；所有运行数据写到独立临时目录。不要指向部署中的 AstrBot 数据目录。

```sh
git clone https://github.com/AstrBotDevs/AstrBot.git /private/tmp/yueling-research-astrbot-20260922
git -C /private/tmp/yueling-research-astrbot-20260922 checkout 95e98b8aed75d56713666eff39e31bafffd95426
uv venv --python 3.12 /private/tmp/yueling-eval-astrbot-20260922/venv
uv pip install --python /private/tmp/yueling-eval-astrbot-20260922/venv/bin/python -r scripts/ai-eval/astrbot/requirements.lock
ASTRBOT_SOURCE=/private/tmp/yueling-research-astrbot-20260922 ASTRBOT_EVAL_OUTPUT=/private/tmp/yueling-eval-astrbot-20260922 /private/tmp/yueling-eval-astrbot-20260922/venv/bin/python scripts/ai-eval/astrbot/probe.py
cd /private/tmp/yueling-research-astrbot-20260922
ASTRBOT_ROOT=/private/tmp/yueling-eval-astrbot-20260922/upstream-tests /private/tmp/yueling-eval-astrbot-20260922/venv/bin/python -m pytest tests/test_tool_loop_agent_runner.py tests/test_chat_route.py -q --disable-warnings
ASTRBOT_ROOT=/private/tmp/yueling-eval-astrbot-20260922/openapi-tests /private/tmp/yueling-eval-astrbot-20260922/venv/bin/python -m pytest tests/test_api_key_open_api.py -q --disable-warnings
```

本机官方 PyPI 大包下载约 25 KB/s，首次安装因此改用清华 PyPI 镜像并成功。复现如遇同样下载速度，可给 `uv pip install` 添加 `--index-url https://pypi.tuna.tsinghua.edu.cn/simple`。依赖版本仍以本次快照为准。

## 对月灵选型的影响

AstrBot 可继续作为候选后台，但接入仍需月灵保留群权限、群数据范围、确认流程、业务幂等和最终正文处理。不能把所有 SSE `data` 拼接发群，也不能让候选后台自行替代月灵已有业务权限。是否值得新增一个 Python 服务，应结合完整接入验证和运维成本决定；本轮不能给出直接切换生产的结论。
