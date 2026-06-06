---
name: notify-task-done
description: Use when a task or work cycle finishes and you need to push a "✅ 已完成" notice to a QQ user or group via the yueling-go bot's external HTTP message API. Sends a completion notification to the configured owner (default) or a given group. Triggered explicitly by the caller — e.g. a cycle-end detector skill, or when the user asks to be pinged on completion.
---

# notify-task-done

任务 / 周期完成后，通过月灵 Bot 的「外部 HTTP 发消息 API」（`POST /api/send`）给指定 QQ 推一条 `✅ … 已完成` 通知。

本 skill 只负责「发通知」，**不判断任务是否结束** —— 由调用方（如周期检测 skill 或用户指令）决定何时调用。职责单一，便于被其他 skill 组合。

## 前置配置（环境变量）

本 skill 不内置任何端点 / 密钥，全部从环境变量读取：

| 变量 | 说明 | 示例 |
|---|---|---|
| `YUELING_API_URL` | 发消息端点完整 URL | `https://api.example.com/bot/api/send` |
| `YUELING_API_KEY` | Bearer 鉴权 key | `your-secret-key` |
| `YUELING_OWNER_ID` | 默认私聊目标 QQ 号 | `10001` |

- 未设置 `YUELING_API_URL` / `YUELING_API_KEY` → 脚本直接报错退出。
- 私聊模式（默认）还需 `YUELING_OWNER_ID`；用 `--group/<群号>` 发群时则不需要。

## 用法

Windows / 跨平台（PowerShell 7+）：

```powershell
pwsh skills/notify-task-done/send.ps1 -Message "图床迁移"
pwsh skills/notify-task-done/send.ps1 -Message "构建" -Group 123456
```

Linux / macOS（bash，需 `python3`）：

```bash
bash skills/notify-task-done/send.sh "图床迁移"
bash skills/notify-task-done/send.sh "构建" 123456   # 第二个参数 = 群号
```

行为：

- 不带消息参数 → 文本为 `✅ 任务 已完成`
- 带消息 `X` → `✅ X 已完成`
- 指定群号 → 发到该群；否则私聊 `YUELING_OWNER_ID`

成功打印 `{"message_id":...,"ok":true}`；失败打印错误与状态码，便于调用方判断是否需要重试。

## 实现说明

- 文本先经 JSON 安全转义、再以 UTF-8 发送，避免非法 UTF-8 被服务端 `400` 拒绝（Windows 非 UTF-8 终端里内联中文是常见坑）。
- `message` 是 OneBot v11 段数组，本 skill 只发 `text` 段；要发图 / @ / 回复，直接参考 API 文档自行拼段。
- API 完整规格见月灵仓库 `flightdeck/docs/external-message-api.md`。
