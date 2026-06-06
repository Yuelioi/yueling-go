---
status: active
summary: 实现 services/httpapi：Bearer 鉴权 POST /api/send，群/私聊，OneBot 段，atomic BotAPI 刷新；config [http_api]；SendPrivateMsg 返 message_id
last_updated: 2026-06-06
implements: specs/2026-06-06-external-message-api.md
---

# 外部 HTTP API 发消息 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增一个独立 HTTP 服务，外部系统凭 Bearer key `POST /api/send` 让 bot 发群/私聊消息（OneBot v11 段数组），同步返回 message_id。

**Architecture:** 新建 `services/httpapi` 包，持有 `atomic.Pointer[bot.BotAPI]`，经 `b.OnConnect` 在每次（重）连接时刷新；handler 通过 `resolve func() Sender` 取当前发送器（生产读 atomic 指针，测试注入桩）。在 `cmd/bot/main.go` 启动前接线，遵守"启动前完成注册"铁律。

**Tech Stack:** Go 标准库 `net/http` + `net/http/httptest`；模块路径 `github.com/Yuelioi/yueling-go`；构建 `go build ./...`，测试 `go test ./...`。

---

## File Structure

- Modify `bot/api.go` — `SendPrivateMsg` 返回 `(int32, error)`，与 `SendGroupMsg` 对齐。
- Create `services/httpapi/httpapi.go` — `Sender` 接口、`Server`、`New`/`BindBot`/`Handler`/`Start`、`handleSend`。
- Create `services/httpapi/httpapi_test.go` — 桩 `Sender` + `httptest` 覆盖鉴权/分发/错误码。
- Modify `config/config.go` — 新增 `HTTPAPIConfig` + 校验。
- Modify `cmd/bot/main.go` — 接线 httpapi。
- Modify `config.example.toml` — 新增 `[http_api]` 段。
- Modify `docker-compose.yml` — 可选端口映射注释。

---

### Task 1: `SendPrivateMsg` 返回 message_id

**Files:**
- Modify: `bot/api.go:58-64`

`httpapi` 的 `Sender` 接口需要 group/private 都返回 `message_id`。`SendPrivateMsg` 当前丢弃了它，且无其他调用方（已 grep 确认），可安全改签名。无法对它做单测（需活的 WS 连接）——覆盖在 Task 2 经桩 `Sender` 完成。

- [ ] **Step 1: 改签名与实现**

把 `bot/api.go` 中：

```go
func (a *BotAPI) SendPrivateMsg(userID int64, msg Message) error {
	_, err := a.call("send_private_msg", map[string]any{
		"user_id": userID,
		"message": msg,
	})
	return err
}
```

替换为：

```go
func (a *BotAPI) SendPrivateMsg(userID int64, msg Message) (int32, error) {
	var resp struct {
		MessageID int32 `json:"message_id"`
	}
	raw, err := a.call("send_private_msg", map[string]any{
		"user_id": userID,
		"message": msg,
	})
	if err != nil {
		return 0, err
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, err
	}
	return resp.MessageID, nil
}
```

- [ ] **Step 2: 构建确认无破坏**

Run: `go build ./...`
Expected: 无输出，退出码 0（确认没有遗漏的旧调用方）。

- [ ] **Step 3: Commit**

```bash
git add bot/api.go
git commit -m "refactor(bot): SendPrivateMsg 返回 message_id，与 SendGroupMsg 对齐"
```

---

### Task 2: `services/httpapi` 包（TDD）

**Files:**
- Create: `services/httpapi/httpapi.go`
- Test: `services/httpapi/httpapi_test.go`

依赖 Task 1（`Sender` 接口里 `SendPrivateMsg` 返回 `(int32, error)`）。

- [ ] **Step 1: 先写测试（会编译失败）**

创建 `services/httpapi/httpapi_test.go`：

```go
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
)

type stubSender struct {
	calledGroup, calledPrivate bool
	groupID, userID            int64
	msg                        bot.Message
	retID                      int32
	retErr                     error
}

func (s *stubSender) SendGroupMsg(groupID int64, msg bot.Message) (int32, error) {
	s.calledGroup = true
	s.groupID = groupID
	s.msg = msg
	return s.retID, s.retErr
}

func (s *stubSender) SendPrivateMsg(userID int64, msg bot.Message) (int32, error) {
	s.calledPrivate = true
	s.userID = userID
	s.msg = msg
	return s.retID, s.retErr
}

func newTestServer(key string, sender Sender) *Server {
	s := New(key)
	s.resolve = func() Sender { return sender }
	return s
}

func do(s *Server, method, auth, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/send", strings.NewReader(body))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

const groupBody = `{"message_type":"group","group_id":123,"message":[{"type":"text","data":{"text":"hi"}}]}`

func TestSendGroupOK(t *testing.T) {
	stub := &stubSender{retID: 555}
	s := newTestServer("k", stub)
	rec := do(s, http.MethodPost, "Bearer k", groupBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !stub.calledGroup || stub.groupID != 123 {
		t.Fatalf("stub not called correctly: %+v", stub)
	}
	if len(stub.msg) != 1 || stub.msg[0].Type != "text" {
		t.Fatalf("message not passed through: %+v", stub.msg)
	}
}

func TestSendPrivateOK(t *testing.T) {
	stub := &stubSender{retID: 7}
	s := newTestServer("k", stub)
	body := `{"message_type":"private","user_id":42,"message":[{"type":"text","data":{"text":"yo"}}]}`
	rec := do(s, http.MethodPost, "Bearer k", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !stub.calledPrivate || stub.userID != 42 {
		t.Fatalf("stub not called correctly: %+v", stub)
	}
}

func TestMissingKey(t *testing.T) {
	s := newTestServer("k", &stubSender{})
	rec := do(s, http.MethodPost, "", groupBody)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestWrongKey(t *testing.T) {
	s := newTestServer("k", &stubSender{})
	rec := do(s, http.MethodPost, "Bearer nope", groupBody)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	s := newTestServer("k", &stubSender{})
	rec := do(s, http.MethodGet, "Bearer k", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestBadJSON(t *testing.T) {
	s := newTestServer("k", &stubSender{})
	rec := do(s, http.MethodPost, "Bearer k", "{not json")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestMissingGroupID(t *testing.T) {
	s := newTestServer("k", &stubSender{})
	body := `{"message_type":"group","message":[{"type":"text","data":{"text":"hi"}}]}`
	rec := do(s, http.MethodPost, "Bearer k", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestInvalidType(t *testing.T) {
	s := newTestServer("k", &stubSender{})
	body := `{"message_type":"channel","group_id":1,"message":[{"type":"text","data":{"text":"hi"}}]}`
	rec := do(s, http.MethodPost, "Bearer k", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestEmptyMessage(t *testing.T) {
	s := newTestServer("k", &stubSender{})
	body := `{"message_type":"group","group_id":1,"message":[]}`
	rec := do(s, http.MethodPost, "Bearer k", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestNotConnected(t *testing.T) {
	s := newTestServer("k", nil) // resolve 返回 nil 接口
	rec := do(s, http.MethodPost, "Bearer k", groupBody)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestSendError(t *testing.T) {
	stub := &stubSender{retErr: http.ErrServerClosed}
	s := newTestServer("k", stub)
	rec := do(s, http.MethodPost, "Bearer k", groupBody)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("code=%d", rec.Code)
	}
}
```

- [ ] **Step 2: 运行测试，确认编译失败**

Run: `go test ./services/httpapi/`
Expected: FAIL — `undefined: New` / `undefined: Server` / `undefined: Sender`（包还没实现）。

- [ ] **Step 3: 实现 `httpapi.go`**

创建 `services/httpapi/httpapi.go`：

```go
package httpapi

import (
	"encoding/json"
	"net/http"
	"sync/atomic"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services/logx"
)

// Sender is the subset of *bot.BotAPI the HTTP API needs.
type Sender interface {
	SendGroupMsg(groupID int64, msg bot.Message) (int32, error)
	SendPrivateMsg(userID int64, msg bot.Message) (int32, error)
}

// Server exposes an authenticated HTTP endpoint for sending messages.
// It holds the latest per-connection BotAPI, refreshed on every (re)connect.
type Server struct {
	key     string
	current atomic.Pointer[bot.BotAPI]
	resolve func() Sender
}

func New(key string) *Server {
	s := &Server{key: key}
	s.resolve = func() Sender {
		api := s.current.Load()
		if api == nil { // avoid a typed-nil interface
			return nil
		}
		return api
	}
	return s
}

// BindBot wires the server to the bot so each (re)connection refreshes the live BotAPI.
// Must be called before bot.Start (registration-before-start rule).
func (s *Server) BindBot(b *bot.Bot) {
	b.OnConnect(func(a *bot.BotAPI) { s.current.Store(a) })
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/send", s.handleSend)
	return mux
}

func (s *Server) Start(addr string) {
	logx.Infof("[httpapi] serving on %s", addr)
	if err := http.ListenAndServe(addr, s.Handler()); err != nil {
		logx.Fatalf("[httpapi] server error: %v", err)
	}
}

type sendRequest struct {
	MessageType string      `json:"message_type"`
	GroupID     int64       `json:"group_id"`
	UserID      int64       `json:"user_id"`
	Message     bot.Message `json:"message"`
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			logx.Errorf("[httpapi] panic: %v", rec)
			writeErr(w, http.StatusInternalServerError, "internal error")
		}
	}()

	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+s.key {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.Message) == 0 {
		writeErr(w, http.StatusBadRequest, "message is empty")
		return
	}
	switch req.MessageType {
	case "group":
		if req.GroupID == 0 {
			writeErr(w, http.StatusBadRequest, "group_id required")
			return
		}
	case "private":
		if req.UserID == 0 {
			writeErr(w, http.StatusBadRequest, "user_id required")
			return
		}
	default:
		writeErr(w, http.StatusBadRequest, "invalid message_type")
		return
	}

	sender := s.resolve()
	if sender == nil {
		writeErr(w, http.StatusServiceUnavailable, "bot not connected")
		return
	}

	var (
		msgID int32
		err   error
	)
	if req.MessageType == "group" {
		msgID, err = sender.SendGroupMsg(req.GroupID, req.Message)
	} else {
		msgID, err = sender.SendPrivateMsg(req.UserID, req.Message)
	}
	if err != nil {
		logx.Warnf("[httpapi] send failed: %v", err)
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message_id": msgID})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"ok": false, "error": msg})
}
```

- [ ] **Step 4: 运行测试，确认全部通过**

Run: `go test ./services/httpapi/`
Expected: PASS（ok github.com/Yuelioi/yueling-go/services/httpapi）。

- [ ] **Step 5: Commit**

```bash
git add services/httpapi/httpapi.go services/httpapi/httpapi_test.go
git commit -m "feat(httpapi): 外部发消息 HTTP 服务（Bearer 鉴权 + OneBot 段）"
```

---

### Task 3: 配置 `[http_api]`

**Files:**
- Modify: `config/config.go:9-14`（Config 结构体）、`config/config.go:75-86`（validate）
- Modify: `config.example.toml`

- [ ] **Step 1: 加配置结构体**

在 `config/config.go` 的 `Config` 结构体里加一行字段：

```go
type Config struct {
	Bot     BotConfig     `mapstructure:"bot"`
	NapCat  NapCatConfig  `mapstructure:"napcat"`
	AI      AIConfig      `mapstructure:"ai"`
	Tools   ToolsConfig   `mapstructure:"tools"`
	HTTPAPI HTTPAPIConfig `mapstructure:"http_api"`
}
```

并在 `ToolsConfig` 定义之后新增：

```go
// HTTPAPIConfig configures the external send-message HTTP API.
// Addr empty = disabled. When Addr is set, Key is required.
type HTTPAPIConfig struct {
	Addr string `mapstructure:"addr"`
	Key  string `mapstructure:"key"`
}
```

- [ ] **Step 2: 加校验**

在 `validate()` 的 `return nil` 之前插入：

```go
	if c.HTTPAPI.Addr != "" && c.HTTPAPI.Key == "" {
		return fmt.Errorf("http_api.key is required when http_api.addr is set")
	}
```

- [ ] **Step 3: 更新示例配置**

在 `config.example.toml` 末尾追加：

```toml

[http_api]
# 外部系统凭 key 通过 HTTP 让 bot 发消息（POST /api/send）。addr 留空 = 关闭。
addr = ""          # 如 ":9080"；设置后必须同时设置 key
key  = ""          # Bearer 鉴权 key，addr 非空时必填
```

- [ ] **Step 4: 构建确认**

Run: `go build ./...`
Expected: 退出码 0。

- [ ] **Step 5: Commit**

```bash
git add config/config.go config.example.toml
git commit -m "feat(config): 新增 [http_api] 段（addr + key，强制要 key）"
```

---

### Task 4: 在 `cmd/bot/main.go` 接线

**Files:**
- Modify: `cmd/bot/main.go:10-31`（import）、`cmd/bot/main.go:135-145`（connect 之前）

- [ ] **Step 1: 加 import**

在 import 块里加（与其他 `services/*` 并列）：

```go
	"github.com/Yuelioi/yueling-go/services/httpapi"
```

- [ ] **Step 2: 接线（在 connect 之前，遵守启动前注册铁律）**

在 `// ── Connect ──` 注释那一行之前插入：

```go
	// ── External HTTP API ─────────────────────────────────────────────────────
	if config.C.HTTPAPI.Addr != "" {
		srv := httpapi.New(config.C.HTTPAPI.Key)
		srv.BindBot(b) // 注册 OnConnect 钩子，刷新活的 BotAPI
		go srv.Start(config.C.HTTPAPI.Addr)
		logx.Infof("[httpapi] enabled on %s", config.C.HTTPAPI.Addr)
	}
```

- [ ] **Step 3: 构建确认**

Run: `go build ./...`
Expected: 退出码 0。

- [ ] **Step 4: 全量测试**

Run: `go test ./...`
Expected: 全部 PASS（含 `services/httpapi`）。

- [ ] **Step 5: Commit**

```bash
git add cmd/bot/main.go
git commit -m "feat(bot): 启动时按配置拉起外部发消息 HTTP API"
```

---

### Task 5: docker-compose 端口（可选）+ 手动冒烟

**Files:**
- Modify: `docker-compose.yml`（bot 服务的 `ports`）

仅在需要从 compose 网络外（宿主机/外网）访问 API 时才映射端口；若调用方也在同一 compose 网络内，用服务名 `http://bot:9080` 直连，**无需**映射。

- [ ] **Step 1: （按需）加端口映射**

在 `docker-compose.yml` 的 bot 服务 `ports:` 下，仿照现有 `9077` 的写法加一行（注释说明，默认注释掉）：

```yaml
      # - "9080:9080"   # 外部发消息 HTTP API（启用 [http_api] 且需宿主机访问时取消注释）
```

- [ ] **Step 2: 手动冒烟（本地，需 bot 已连上 NapCat）**

临时在 `config.toml` 设 `[http_api] addr=":9080"` `key="testkey"`，`go run ./cmd/bot/`，另开终端：

```bash
curl -s -X POST http://127.0.0.1:9080/api/send \
  -H "Authorization: Bearer testkey" \
  -H "Content-Type: application/json" \
  -d '{"message_type":"group","group_id":<你的群号>,"message":[{"type":"text","data":{"text":"httpapi 冒烟"}}]}'
```

Expected: 返回 `{"ok":true,"message_id":...}`，群里收到消息。错 key → `401 {"ok":false,...}`。

- [ ] **Step 3: Commit（若改了 compose）**

```bash
git add docker-compose.yml
git commit -m "docs(docker): 外部发消息 API 端口映射示例（默认注释）"
```

---

## 完成标准

- `go build ./...` 与 `go test ./...` 全绿。
- `addr` 留空时行为与现状完全一致（服务不启动）。
- `addr` 非空 + `key` 空时启动报错。
- 冒烟：正确 key 能发群/私聊并返回 message_id；错/缺 key 返回 401。
