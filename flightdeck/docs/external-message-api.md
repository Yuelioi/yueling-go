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
addr = ":9080"        # 监听地址
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
| 400 | JSON 非法 / `message` 为空 / `message_type` 非法 / 必填 id 缺失 |
| 401 | key 缺失或错误 |
| 405 | 方法非 POST |
| 502 | 协议端（NapCat）调用失败或超时 |
| 503 | bot 尚未连上 NapCat（无可用连接） |

## 示例

### 发群文本

```bash
curl -s -X POST http://127.0.0.1:9080/api/send \
  -H "Authorization: Bearer your-secret" \
  -H "Content-Type: application/json" \
  -d '{"message_type":"group","group_id":123456,"message":[{"type":"text","data":{"text":"hello"}}]}'
```

### 发私聊文本 + 图片

```bash
curl -s -X POST http://127.0.0.1:9080/api/send \
  -H "Authorization: Bearer your-secret" \
  -H "Content-Type: application/json" \
  -d '{"message_type":"private","user_id":10001,"message":[{"type":"text","data":{"text":"看图"}},{"type":"image","data":{"file":"https://example.com/x.png"}}]}'
```

### @某人 + 文本

```bash
curl -s -X POST http://127.0.0.1:9080/api/send \
  -H "Authorization: Bearer your-secret" \
  -H "Content-Type: application/json" \
  -d '{"message_type":"group","group_id":123456,"message":[{"type":"at","data":{"qq":"10001"}},{"type":"text","data":{"text":" 在吗"}}]}'
```

## 部署

- 同一 docker-compose 网络内的调用方：直接用服务名 `http://bot:9080`，**无需**端口映射。
- 需从宿主机 / 外网访问：在 `docker-compose.yml` 的 bot 服务取消注释 `- "9080:9080"`。
- 不提供 HTTPS；如需，交给前置反向代理。

## 注意

- 同步语义：HTTP 响应等到协议端回执，`message_id` 即实际发出的消息 id。
- key 为唯一凭证，泄露即可任意发消息；务必走内网或反代 + 强 key。
- bot 重连 NapCat 时 `BotAPI` 会刷新，API 无需重启即可继续工作。
