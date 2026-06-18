---
status: obsolete
when_to_read: 排查 bot↔NapCat 莫名断连(close 1007)、或外部 API 发送 502 response timeout 时
applies_to: [httpapi, websocket, napcat, utf8]
last_updated: 2026-06-06
resolved_by: services/httpapi/httpapi.go (utf8.Valid 校验)
---

# httpapi 非法 UTF-8 拖垮 NapCat 连接（close 1007）

## Signature
- symptom: `websocket: close 1007 (invalid payload data)`（bot 侧）/ `[OneBot] [WebSocket Server] Client Error: Invalid WebSocket frame: invalid UTF-8 sequence`（NapCat 侧）
- error_type: WS close 1007 / API 502 response timeout
- where: services/httpapi handleSend → bot.BotAPI.call → bot sendLoop WriteMessage
- trigger: 外部 API 调用方在 message 文本里发了非法 UTF-8 字节（如 GBK 编码的中文）

## 症状/复现

外部 `POST /api/send` 发带中文的消息，调用方拿到 `502 {"ok":false,"error":"response
timeout: send_group_msg"}`；bot 日志周期性 `disconnected: websocket: close 1007`，
NapCat 日志 `Client Error: Invalid WebSocket frame: invalid UTF-8 sequence`。
群里收不到消息，且断连影响 bot **全部**收发（普通插件此刻也发不出）。

复现：用 Windows GBK 终端 `curl -d '{...中文...}'`（shell 按本地代码页编码，发出非法
UTF-8）。纯 ASCII 内容 / 从 UTF-8 文件 `--data @file` 发送则正常。

## 根因

`bot.Message` 的段是 `Segment{Type string, Data json.RawMessage}`。Go 的 `encoding/json`
**不校验 `json.RawMessage` 内容的 UTF-8**，坏字节被原样保留。httpapi 把它直接转发给
NapCat（`WriteMessage(TextMessage, ...)`）；NapCat 作为 WS server 按 RFC6455 校验文本帧
UTF-8，发现非法即以 close 1007 断开**整条连接** → 该次 API 调用收不到回执（10s 超时 →
502），且连接抖动期间所有收发受影响。

不是 httpapi 逻辑 bug，也不是 NapCat 的 bug（它按规范行事）——是**未校验就透传非法字节**。

## 修法

httpapi 发送前对 `message` 各段 `utf8.Valid(seg.Data)` 校验，非法直接返回
`400 message contains invalid utf-8`，绝不转发给 NapCat。见
`services/httpapi/httpapi.go` 与测试 `TestInvalidUTF8Rejected`。

调用方侧根治：用语言自带 JSON 序列化（默认 UTF-8），不要在非 UTF-8 终端内联拼 body。

## Cases
- 2026-06-06 首次：外部 API 冒烟时用 GBK 终端发中文触发，定位后加服务端 UTF-8 校验根治。
