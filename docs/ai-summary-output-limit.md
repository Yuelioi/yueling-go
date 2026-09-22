# 群总结取数成功但生成不完整：诊断与修复

日期：2026-09-22。基线：v1.22.0 / `6566861`；补丁版本：v1.22.1。已复现并修复一种与用户提示、等待时间相同的生成失败路径；仍缺线上模型配置及完整结束日志，**不能确认线上那次请求的唯一根因**。

## 现场证据与判断范围

用户日志显示 `summary_source status=ok duration_ms=25`，之后收到“AI 回复生成不完整，请重试”。这证明取资料阶段成功，不代表总结已经生成。v1.22.0 的该提示会合并多种响应校验失败，而 `Text` 在重试结束时还会丢失具体原因。原日志因此不足以区分额度耗尽、空正文和协议正文。

诊断使用真实配置的模型和50条虚构 AE 表达式/脚本讨论，不读取生产群资料、不发送 QQ 消息。模型响应只记录结束类别、token 和字符数，未保存或输出正文、推理内容、凭证。

## 对照实验

固定模型 `deepseek-v4-pro` 和同一合成资料，每步只改变表中条件。以下都是单次观察，不是生产延迟或成功率保证。

| 条件 | 观察 |
| --- | --- |
| v1.22.0，4096 token，默认 low，未约束回复长度 | 一次成功，正文703字符，completion 1580 token；约21.32秒 |
| 仅把预算改为300 | 连续两次 `finish=length`，每次300 token，正文均为0；约9.93秒返回与用户相同的“不完整”提示 |
| 在300预算下仅把推理改为 none | 正文出现，但没有长度要求；连续两次被截断，仍失败，约8.51秒 |
| 再加入已有配置的200字符回复要求 | 一次成功，正文117字符，69 token；约2.17秒 |
| 最终修复代码，原300预算复现命令，不再额外覆盖推理和提示词 | 一次成功，正文133字符，85 token，无显式推理内容；约2.47秒 |
| 可交付的 `ai-check -summary-only -summary-records 50`，300预算 | 保留“周四测试、周五发布”两个事实，正文95字符；约2.60秒 |

因此已确认代码层有三个缺口：文本整理默认也开启显式推理；固定总结流程漏用 `reply_max_chars`；额度耗尽后仍按原预算重复生成并丢掉错误细节。这些问题可在本机复现，但线上具体配置尚未确认。

DeepSeek 官方说明思考内容与正文分别通过 `reasoning_content` 和 `content` 返回，支持控制思考模式；Chat Completions 的 `reasoning_effort=none` 可关闭思考模式，`finish_reason=length` 表示长度限制导致截断。[思考模式](https://api-docs.deepseek.com/guides/thinking_mode/)、[Chat Completions 参数](https://api-docs.deepseek.com/api/create-chat-completion/)

## 修复内容

- `Client.Text` 对已知 DeepSeek V4 / deepseek-flash 默认使用 `none`。调用方或配置显式指定的推理强度仍优先；其他模型、通用对话和工具往返保留原策略。
- 群总结使用配置中的回复字符上限，范围说明和来源 ID 也计入；这是给模型的长度要求，不是截断已生成正文。生成 token 预算保持配置值。
- 纯文本生成因长度耗尽而失败时，不再按相同额度重复请求；格式校验问题仍保留一次有界修正。
- 保留 `output_limit`、`reasoning_only`、`empty_content`、`empty_choices`、`protocol_markup` 等分类。日志附上经过类别限制的结束原因、有效预算、消耗 token、正文/推理长度及格式尝试次数，不记录内容。
- 实际总结和诊断命令共用 `SummaryRequest`，避免此前两条短记录、另一套提示词的探针漏掉真实长度策略。

## 验证入口

先增加回归，确认原代码会丢失错误原因、重复请求和漏掉回复长度；修复后回归通过。新增收发测试从真实 WebSocket 处理器进入，在300预算下验证最终总结正文被送出。模型和 OneBot 为可控替身，不能当作真实 QQ 投递。

```sh
go test ./services/llm ./ai ./plugins/ai_dispatch -run 'TestText|TestDeepSeek|TestSummary' -count=1
go test -race ./ai/... ./plugins/ai_dispatch ./services/chatsummary ./services/llm ./bot ./config ./services/feed ./plugins/tools ./services/knowledge -count=1

# 使用当前配置，只发50条虚构资料；会消耗少量模型额度。
YUELING_AI_MAX_TOKENS=300 go run ./cmd/ai-check -config config.toml -summary-only -summary-records 50
go run ./cmd/ai-check -config config.toml
```

完整仓库、并发检查与最终模型探针的结果记录在 [补丁验证数据](../scripts/ai-eval/yueling/output-limit-results.json)。数据库用例仍因专用测试连接缺失而跳过，本次未改数据库逻辑。

最终完整测试退出码0，533个测试/子测试通过、99个数据库项跳过；相关10个包的 race 检查309个通过、33个数据库项跳过；`go vet ./...` 和差异格式检查通过。完整真实模型探针通过工具调用、多轮事实保留与50条虚构资料的生产总结请求检查。计数包含父测试和子测试，不能当作独立用户场景数量。

## 线上复核

需要补同一条消息后续的 `model_text`、`summary`、`delivery` 日志，以及 `ai.model`、`ai.max_tokens`、`ai.reasoning_effort` 的非敏感值。更新后若再失败，`detail` 和结束元数据可直接区分原因；不能只根据“取数成功”推断模型已返回正文。没有要求或授权输出真实聊天与密钥。
