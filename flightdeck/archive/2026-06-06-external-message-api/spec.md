---
status: done
summary: HTTP 服务凭 Bearer key 让外部系统发群/私聊消息，body 为 OneBot v11 段数组，同步返回 message_id；独立 services/httpapi 包，OnConnect 刷新活的 BotAPI
last_updated: 2026-06-06
---

# 外部 HTTP API 发消息

## 背景 / 目标

外部系统需要凭一个 key 通过 HTTP 让 bot 主动发消息（群 / 私聊）。当前 bot 只通过
WebSocket 与 NapCat 收发，没有对外的 HTTP 入口。本设计新增一个独立的 HTTP 服务，
不改动现有 WS 收发链路。

成功标准：

- 外部 `POST` 一个 OneBot v11 段数组即可发群 / 私聊消息。
- 无有效 key 一律拒绝（401）。
- 同步返回 `message_id` 或明确的错误状态码。

## 架构

新建包 `services/httpapi`（与 `logx` / `httpclient` / `meme` 并列）。导入 `bot`，
无循环依赖——`bot` 只依赖 `services/logx`。

**拿到活的 `BotAPI`**：`BotAPI` 是每条 WS 连接新建一个的（见 `bot/bot.go` 的
`handleConn`）。Server 内持有 `atomic.Pointer[bot.BotAPI]`，通过 `b.OnConnect(...)`
钩子在每次（重）连接时刷新指针。连接建立前指针为 nil → 返回 503。

**可测性**：handler 通过 `resolve func() Sender` 取当前发送器（生产读 atomic 指针；
测试注入桩）。`Sender` 是只含发送方法的小接口，`*bot.BotAPI` 天然满足：

```go
type Sender interface {
    SendGroupMsg(groupID int64, msg bot.Message) (int32, error)
    SendPrivateMsg(userID int64, msg bot.Message) (int32, error)
}
```

**Server 形态**：

```go
type Server struct {
    key     string
    resolve func() Sender // 返回 nil 表示未连接
}

func New(key string) *Server
func (s *Server) BindBot(b *bot.Bot) // 注册 OnConnect，刷新内部 atomic 指针
func (s *Server) Start(addr string)  // ListenAndServe + 鉴权中间件 + 路由
```

**接入点**（`cmd/bot/main.go`，在 `b.Start` 前，遵守"启动前完成注册"铁律）：

```go
if config.C.HTTPAPI.Addr != "" {
    srv := httpapi.New(config.C.HTTPAPI.Key)
    srv.BindBot(b)            // 注册 OnConnect 钩子
    go srv.Start(config.C.HTTPAPI.Addr)
}
```

## HTTP 接口

统一端点，镜像 OneBot `send_msg` 语义：

`POST /api/send`，Header `Authorization: Bearer <key>`

请求体：

```json
{
  "message_type": "group",
  "group_id": 123456,
  "user_id": 0,
  "message": [
    {"type":"text","data":{"text":"hi"}},
    {"type":"image","data":{"file":"https://..."}}
  ]
}
```

- `message_type`: `"group"` | `"private"`。
- `message_type=group` 时 `group_id` 必填；`=private` 时 `user_id` 必填。
- `message`: 完整 OneBot v11 段数组，直接 unmarshal 进 `bot.Message`（即
  `[]Segment{Type, Data json.RawMessage}`）。

成功响应 `200`：

```json
{"ok": true, "message_id": 12345}
```

错误响应统一 `{"ok": false, "error": "<reason>"}`，状态码：

| 情况 | 状态码 |
|---|---|
| key 缺失 / 错误 | 401 |
| 方法非 POST | 405 |
| JSON 非法 / message_type 非法 / 必填 id 缺失 / message 空 | 400 |
| bot 未连接（resolve 返回 nil） | 503 |
| NapCat 调用失败 / 超时 | 502 |

## 数据流

外部 client → `POST /api/send` → 鉴权中间件校验 Bearer → 解析 body 进
`bot.Message` → `resolve()` 取当前 `Sender` → 按 `message_type` 调
`SendGroupMsg` / `SendPrivateMsg` → 拿到 `message_id` → 200 返回。

## 配置

`config.go` 新增 `HTTPAPIConfig`，section `[http_api]`：

```toml
[http_api]
addr = ":9080"     # 留空 / 缺省 = 关闭该功能
key  = "your-secret"
```

校验：`addr` 非空但 `key` 空 → `Load` 报错（强制要 key，防裸奔）。
同步更新 `config.example.toml`；docker-compose 里按需加端口映射并注释说明。

## `bot/api.go` 改动

`SendPrivateMsg(userID, msg) error` → `(int32, error)`，与 `SendGroupMsg` 对齐，
解析返回的 `message_id`。无其他调用方（已 grep 确认），改动安全。

## 错误处理

- 鉴权失败、解析失败在进入发送逻辑前短路。
- `resolve()` 为 nil（未连上 NapCat）时不调用发送，直接 503。
- 发送层错误（含 `bot/api.go` 的 5s/10s 超时）透传为 502，body 带 error 文案。
- handler 顶层 recover，避免单请求 panic 拖垮 server。

## 测试

`services/httpapi` 单测，注入桩 `Sender`，用 `httptest`：

- 正确 key + group → 200，message_id 正确，桩收到对应 group_id 与段。
- 正确 key + private → 200。
- 缺 key / 错 key → 401。
- 坏 JSON / 缺 group_id / message_type 非法 → 400。
- 非 POST → 405。
- resolve 返回 nil → 503。
- 桩返回 error → 502。

不需要起真 WS / NapCat。

## 明确不做（YAGNI）

限流、多 key / 权限分级、消息模板、收消息回调 / webhook、HTTPS（交给前置反代）。
