---
status: active
when_to_read: 需要从外部系统通过 HTTP 让 bot 发群/私聊消息、或排查 /api/send 调用时
applies_to: [httpapi, api, integration]
last_updated: 2026-06-06
---

# 外部 HTTP API 发消息调用指南

实现见 `services/httpapi/`，接线见 `cmd/bot/main.go`。

## 开启

在 `config.toml` 加 `[http_api]` 段（`addr` 留空 = 关闭；`addr` 非空时 `key` 必填，
否则启动报错）：

```toml
[http_api]
addr = ":9078"        # 监听地址
key  = "your-secret"  # Bearer 鉴权 key
```

bot 启动时若 `addr` 非空，会在该地址拉起 HTTP 服务。

## 端点

`POST /api/send`

| | |
|---|---|
| 鉴权 | Header `Authorization: Bearer <key>` |
| Content-Type | `application/json` |
| 请求体 | 见下 |
| 成功 | `200 {"ok": true, "message_id": 12345}` |
| 失败 | `{"ok": false, "error": "<reason>"}` + 对应状态码 |

### 请求体

```json
{
  "message_type": "group",
  "group_id": 123456,
  "user_id": 0,
  "message": [
    {"type": "text",  "data": {"text": "hi"}},
    {"type": "image", "data": {"file": "https://example.com/x.png"}}
  ]
}
```

- `message_type`：`"group"` 或 `"private"`。
- `group_id`：`message_type=group` 时必填。
- `user_id`：`message_type=private` 时必填。
- `message`：完整 OneBot v11 段数组（`[{type, data}, ...]`），原样转交协议端。
  常见段：`text` / `image`（`file` 支持 url、`base64://...`、NapCat 可达的本地路径）/
  `at`（`{"qq":"123"}` 或 `{"qq":"all"}`）/ `reply`（`{"id":"<消息id>"}`）/ `face`。

## 状态码

| 状态码 | 含义 |
|---|---|
| 200 | 发送成功，返回 `message_id` |
| 400 | JSON 非法 / `message` 为空或含非法 UTF-8 / `message_type` 非法 / 必填 id 缺失 |
| 401 | key 缺失或错误 |
| 405 | 方法非 POST |
| 502 | 协议端（NapCat）调用失败或超时 |
| 503 | bot 尚未连上 NapCat（无可用连接） |

## 编码要求（务必合法 UTF-8，重要）

`message` 里的所有文本**必须是合法 UTF-8**。服务端在发送前会校验各段，含非法字节
直接返回 `400 {"ok":false,"error":"message contains invalid utf-8"}`，不会转发给 NapCat。

> 历史背景：早期版本不校验、原样透传，非法字节会让 NapCat 断开整条 WebSocket
> （close 1007），调用方表现为 `502 response timeout`、bot 对所有请求离线；现已在
> 服务端拦截。详见 `incidents/2026-06-06-httpapi-invalid-utf8-1007`。

调用方仍应保证 JSON body 以 UTF-8 编码发出（否则会拿到 400）。常见坑：

- **命令行内联非 ASCII**：在非 UTF-8 终端（如 Windows 默认 GBK 代码页）里
  `curl -d '{...中文...}'`，shell 会用本地代码页编码，发出去就是非法 UTF-8。
  → 把 body 写进 **UTF-8 文件**再 `curl --data @body.json`（见下方示例），
  或先把终端切到 UTF-8（Windows：`chcp 65001`）。
- **程序化调用**：用语言自带的 JSON 序列化（Go `encoding/json`、Python
  `json.dumps(..., ensure_ascii=False)` + UTF-8、Node `JSON.stringify`）默认即 UTF-8，
  正常无需特殊处理——**这是推荐的接入方式**，比拼命令行稳。

## 示例

### 发群文本

```bash
curl -s -X POST http://127.0.0.1:9078/api/send \
  -H "Authorization: Bearer your-secret" \
  -H "Content-Type: application/json" \
  -d '{"message_type":"group","group_id":123456,"message":[{"type":"text","data":{"text":"hello"}}]}'
```

### 发私聊文本 + 图片

```bash
curl -s -X POST http://127.0.0.1:9078/api/send \
  -H "Authorization: Bearer your-secret" \
  -H "Content-Type: application/json" \
  -d '{"message_type":"private","user_id":10001,"message":[{"type":"text","data":{"text":"看图"}},{"type":"image","data":{"file":"https://example.com/x.png"}}]}'
```

### @某人 + 文本

```bash
curl -s -X POST http://127.0.0.1:9078/api/send \
  -H "Authorization: Bearer your-secret" \
  -H "Content-Type: application/json" \
  -d '{"message_type":"group","group_id":123456,"message":[{"type":"at","data":{"qq":"10001"}},{"type":"text","data":{"text":" 在吗"}}]}'
```

### 发中文（推荐：UTF-8 文件，避开 shell 编码）

把请求体写进一个 UTF-8 文件 `body.json`：

```json
{"message_type":"group","group_id":123456,"message":[{"type":"text","data":{"text":"中文消息没问题 ✅"}}]}
```

再用 `--data @` 引用（curl 按字节发送文件内容，不经 shell 编码）：

```bash
curl -s -X POST http://127.0.0.1:9078/api/send \
  -H "Authorization: Bearer your-secret" \
  -H "Content-Type: application/json" \
  --data @body.json
```

## 部署

- 同一 docker-compose 网络内的调用方：直接用服务名 `http://bot:9078`，**无需**端口映射。
- 需从宿主机 / 外网访问：在 `docker-compose.yml` 的 bot 服务取消注释 `- "9078:9078"`。
- 不提供 HTTPS；如需，交给前置反向代理。

## 注意

- 同步语义：HTTP 响应等到协议端回执，`message_id` 即实际发出的消息 id。
- key 为唯一凭证，泄露即可任意发消息；务必走内网或反代 + 强 key。
- bot 重连 NapCat 时 `BotAPI` 会刷新，API 无需重启即可继续工作。
