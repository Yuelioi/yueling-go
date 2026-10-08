# LangBot 组件实测（2026-09-22）

结论：LangBot 的 HTTP 入口、模型回退和工具循环可作为接入候选，但本次实测**不支持把 `/sync` 直接当成可靠的远程 AI 调用**。复现了超时后旧回复进入下一轮同步请求的问题；同参数副作用工具仍需要业务侧去重；模型把工具协议写进正文时，回复包装层仍会发送该正文。

这不是对 LangBot 全产品稳定性的排名，也不是完整部署验收。未修改候选源码来使测试通过。

## 固定对象与实验边界

- 源码：[LangBot `6b83089535c1b71bae45f42bdb522d67f20be3eb`](https://github.com/langbot-app/LangBot/tree/6b83089535c1b71bae45f42bdb522d67f20be3eb)，包版本 `4.10.11`。
- SDK：`langbot-plugin==0.6.0b5`；Python `3.13.2`，macOS arm64。实际组件依赖已冻结到 [`requirements-components.txt`](../scripts/ai-eval/langbot/requirements-components.txt)。这些依赖属于这次组件环境，**不是完整 `uv.lock` 安装**。
- HTTP 实验：真实 `HttpBotAdapter.handle_unified_webhook`、签名校验、任务调度、同步收集器、真实本机 HTTP 回调；入站由 Quart test client 进入。流水线 listener 是可控合成函数，以精确制造迟到与取消。
- Runner 实验：真实 `LocalAgentRunner.run`、`ResponseWrapper.process`、`SessionManager.get_session`；模型 provider 边界通过 HTTPX 访问本机 OpenAI 格式假服务。工具执行器只记录计数和返回虚构回执，不调用业务系统。
- Runner 和上游单测为避免导入全部平台/数据库驱动，仅隔离了 `core.app` 的启动模块，并注入测试 Application；**没有替换被测 Runner、Wrapper、SessionManager 或 HTTP adapter 的方法**。
- 没有启动完整 LangBot、真实 LiteLLM requester、插件运行时、持久化数据库或真实 QQ。没有使用真实 API key、群聊或现有月灵配置。统计不代表真实模型理解质量、性能或生产上线结果。

## 观察结果

表中的“复现”表示探针确认了行为；脚本退出成功并不表示每个行为满足月灵的验收要求。

| 场景 | 实际结果 | 对接含义 |
|---|---|---|
| 入站签名 | 缺失、错误、超过 300 秒窗口的签名均 `401`；正确签名 `202` | HMAC 入口验证有效 |
| 同一入站重发 | 带同一个幂等键：`202 → 409`；不带键：`202 → 202` | 网关必须传稳定的请求键，不能依赖签名实现去重 |
| 无效消息修正重发 | 首次参数非法 `400`，同一个键修正后 `409` | 键在消息解析前已被占用；不要把“键已占用”等同于流水线已执行 |
| 回调鉴权与重试 | 合成接收端先 `503` 后 `200`，收到两份完全相同、签名均有效的回调；`429` 仅发送一次 | 接收端需去重；限流恢复不能假设所有非成功状态都会被重试 |
| 回调轮次标识 | 单轮结束后 `sequence` 从 `1` 重新开始 | 去重至少需要 `session_id + reply_to + sequence`，仅会话和序号会冲突 |
| `/sync` 等待超时 | `callback_timeout=1` 时约 4 秒后返回 `HTTP 200 / code=0 / message=[]`，原 listener 仍在运行 | HTTP 成功不能表示任务完成；超时不等于取消 |
| 超时后立即同会话追问 | 第二轮 `/sync` 收到了“第一轮迟到答案：周五发布”；真正第二轮“周四测试”随后走了 callback | **已复现跨轮回复错配**，当前版本直接接 `/sync` 存在阻塞性缺口 |
| 调用方取消等待 | 取消 Quart 请求等待后，原 listener 仍存活并最终完成 | 不能依据 HTTP 取消推断工具没有执行 |
| 正常群聊总结 | `summarize_chat` 执行一次；模型再次请求含 `user/assistant/tool`；可见正文为“周五发布，周四测试” | 正常工具往返可运行；资料和模型均为合成夹具 |
| 首轮模型 `503` | primary 调用一次失败，fallback 调用一次成功 | 首轮模型回退生效 |
| 提醒已执行后模型 `503` | `create_reminder` 计数 `1`，回执 `fixture-1`；primary 共两次请求，fallback `0`，错误向上抛出 | 本次 Runner 没有自动重放动作；完整流水线如何呈现“已执行但回复失败”尚未实测 |
| 同回合相同副作用调用 | 两个不同 call ID、相同 `create_reminder({})`，执行器收到两次调用 | **Runner 层没有按业务参数去重**；本实验没有验证每种真实插件工具是否自行幂等 |
| 正常工具字段与正文分离 | 关闭 `track-function-calls`，原生 `tool_calls` 和 `role=tool` 数据没有成为 Wrapper 输出正文 | 结构化协议在这条配置路径下可分离 |
| 模型将协议放入 `content` | `<tool_call>summarize_chat({})</tool_call>` 原样成为 Wrapper 可见正文 | 仍需要月灵侧最终输出校验，关闭工具跟踪并不能解决此类泄漏 |
| 同群不同用户 | 相同 `launcher_id=group-100`、不同 sender 得到同一个会话 | 与月灵“群 + 用户”的习惯不同，属于会话策略差异 |
| 不同群、复合会话键 | 不同群隔离；`group-100:user-1` 与 `group-100:user-2` 隔离 | 网关必须自行定义可信复合键，不能只提交群号 |

正常工具分离使用了 `track-function-calls=false`。这不表示打开工具跟踪、不同插件修改回复、流式推理内容或错误直出配置也满足相同性质。`reasoning_content` 的真实提供商处理、本次注入的 `secret-provider-body` 是否经过完整错误阶段泄漏，均**未在完整流水线验证**。

## 上游已有测试

在同一组件环境下运行了原仓库四个文件，共 **35 passed**：

- `tests/unit_tests/platform/test_http_bot_tenancy.py`：7 项。
- `tests/unit_tests/provider/test_session_manager.py`：24 项。
- `tests/unit_tests/provider/test_localagent_tool_content.py`：2 项。
- `tests/unit_tests/provider/test_localagent_no_duplicate.py`：2 项。

这里的 `no_duplicate` 验证流式开头不重复，并不覆盖同参数副作用工具的幂等。测试运行有 7 条上游警告；未把它表述为全项目测试通过。

## 可复现入口

脚本和本次合成结果都保存在 [`scripts/ai-eval/langbot`](../scripts/ai-eval/langbot)，临时目录不是唯一证据。

```bash
# 在月灵仓库根目录运行。工作目录完全隔离，无需月灵 config.toml。
git clone https://github.com/langbot-app/LangBot.git /tmp/langbot-ai-eval-source
git -C /tmp/langbot-ai-eval-source checkout 6b83089535c1b71bae45f42bdb522d67f20be3eb
python3 -m venv /tmp/langbot-ai-eval-venv
/tmp/langbot-ai-eval-venv/bin/python -m pip install --no-deps \
  -r scripts/ai-eval/langbot/requirements-components.txt
/tmp/langbot-ai-eval-venv/bin/python -m pip install --no-deps \
  -e /tmp/langbot-ai-eval-source
scripts/ai-eval/langbot/run.sh \
  /tmp/langbot-ai-eval-source \
  /tmp/langbot-ai-eval-venv/bin/python \
  /tmp/langbot-ai-eval-results
```

本次环境中官方 wheel 下载异常缓慢，最终安装相同固定版本时使用了 `--index-url https://pypi.tuna.tsinghua.edu.cn/simple`。这属于依赖获取选择，不是修改候选运行行为。最小环境故意没有安装完整 SDK 终端界面、云沙箱等依赖，不能拿它运行完整 LangBot 服务。

`run.sh` 检查固定 commit 和被测源码无已跟踪改动，生成 HTTP/Runner JSON、上游 JUnit XML 与日志。全部监听仅使用 `127.0.0.1` 随机端口，探针结束关闭任务、连接及监听。JSON 中的 `completed: true` 表示观察断言完成。

## 对月灵选型的影响

保留 LangBot 为候选，但先采用它的可取边界：会话作用域、独立 Runner、结构化消息、首轮回退和工具执行后的模型固定。若以后接入 LangBot，优先异步提交/回调并由月灵持久记录请求与动作状态，逐条校验回调所属轮次；补齐取消、迟到消息丢弃和副作用幂等，再讨论迁移。

当前证据不足以认定“换成 LangBot 就能消除月灵现有问题”。特别是用户反馈的工具协议泄漏和失败后的操作状态，都仍需要明确的业务与回复契约。
