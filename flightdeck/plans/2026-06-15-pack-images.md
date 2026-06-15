---
status: active
summary: pack 功能实现计划——GetForwardMsg 封装 + 递归抽图 collectImages + zip 打包上传，5 个 TDD 任务
last_updated: 2026-06-15
---

# pack 打包消息图片为 zip — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 群友回复一条消息发 `pack`，bot 把该消息（含递归展开的合并转发各层）里的所有图片下载打成一个 zip 上传到群文件。

**Architecture:** 新增独立插件 `plugins/tools/pack.go`（命令 `pack`，群聊）。在 `bot` 包补一个 `GetForwardMsg` 封装抽合并转发内容。取图用纯函数 `collectImages` 递归遍历段，遇 `image` 收 url、遇 `forward` 调注入的取转发函数再递归（depth≤5 + visited 去重防循环）。下载后用 `archive/zip` 打包写本地临时文件，复用已有 `UploadGroupFile` 上传，传完删临时文件。

**Tech Stack:** Go，OneBot v11 / NapCat，标准库 `archive/zip`，项目内 `bot` / `services/httpclient` / `services/logx`。

---

### Task 1: `bot.GetForwardMsg` 封装 + 解析

**Files:**
- Modify: `bot/api.go`（在 `GetMsg` 之后追加）
- Test: `bot/forward_test.go`（新建，package bot）

把 `get_forward_msg` 返回的 `data.messages[]` 解析成 `[]Message`。解析逻辑抽成纯函数
`parseForwardMsg(raw)` 便于单测；网络部分 `GetForwardMsg` 薄封装 `a.call`。

- [ ] **Step 1: 写失败测试**

`bot/forward_test.go`：

```go
package bot

import "testing"

func TestParseForwardMsg(t *testing.T) {
	raw := []byte(`{"messages":[
		{"message":[{"type":"text","data":{"text":"hi"}},{"type":"image","data":{"url":"http://a/1.jpg"}}]},
		{"message":[{"type":"forward","data":{"id":"999"}}]},
		{"content":[{"type":"image","data":{"url":"http://a/2.jpg"}}]}
	]}`)
	msgs := parseForwardMsg(raw)
	if len(msgs) != 3 {
		t.Fatalf("want 3 messages, got %d", len(msgs))
	}
	if got := msgs[0].ImageURLs(); len(got) != 1 || got[0] != "http://a/1.jpg" {
		t.Fatalf("msg0 images = %v", got)
	}
	if msgs[1][0].Type != "forward" {
		t.Fatalf("msg1 seg0 type = %q", msgs[1][0].Type)
	}
	if got := msgs[2].ImageURLs(); len(got) != 1 || got[0] != "http://a/2.jpg" {
		t.Fatalf("msg2 images (content fallback) = %v", got)
	}
	if parseForwardMsg([]byte(`not json`)) != nil {
		t.Fatalf("非法 JSON 应返回 nil")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./bot/ -run TestParseForwardMsg -v`
Expected: 编译失败 `undefined: parseForwardMsg`

- [ ] **Step 3: 实现 `parseForwardMsg` + `GetForwardMsg`**

`bot/api.go`，在 `GetMsg`（约 150 行）之后追加：

```go
func parseForwardMsg(raw json.RawMessage) []Message {
	var resp struct {
		Messages []struct {
			Message Message `json:"message"`
			Content Message `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil
	}
	out := make([]Message, 0, len(resp.Messages))
	for _, m := range resp.Messages {
		switch {
		case len(m.Message) > 0:
			out = append(out, m.Message)
		case len(m.Content) > 0:
			out = append(out, m.Content)
		}
	}
	return out
}

func (a *BotAPI) GetForwardMsg(id string) ([]Message, error) {
	raw, err := a.call("get_forward_msg", map[string]any{"message_id": id})
	if err != nil {
		return nil, err
	}
	return parseForwardMsg(raw), nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./bot/ -run TestParseForwardMsg -v`
Expected: PASS

- [ ] **Step 5: 提交**

```
git add bot/api.go bot/forward_test.go
git commit -m "feat(bot): GetForwardMsg 封装 get_forward_msg"
```

---

### Task 2: `collectImages` 递归抽图（纯函数）

**Files:**
- Create: `plugins/tools/pack.go`（先只放常量 + collectImages）
- Test: `plugins/tools/pack_test.go`（新建，package tools）

递归遍历消息段：`image` 收 url（优先 `url` 回退 `file`），`forward` 取 `id` 调注入的取转发函数再递归。
depth>5 或已达 100 张即停；visited 记已展开 forward id 防嵌套循环。

- [ ] **Step 1: 写失败测试**

`plugins/tools/pack_test.go`：

```go
package tools

import (
	"encoding/json"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
)

func img(u string) bot.Segment {
	return bot.Segment{Type: "image", Data: json.RawMessage(`{"url":"` + u + `"}`)}
}
func fwd(id string) bot.Segment {
	return bot.Segment{Type: "forward", Data: json.RawMessage(`{"id":"` + id + `"}`)}
}

func TestCollectImages(t *testing.T) {
	// f1 内含一张图 + 指向 f2；f2 含一张图 + 指回 f1（制造循环）
	store := map[string][]bot.Message{
		"f1": {{img("http://a/inner1.jpg"), fwd("f2")}},
		"f2": {{img("http://a/inner2.jpg"), fwd("f1")}},
	}
	getForward := func(id string) ([]bot.Message, error) { return store[id], nil }

	root := bot.Message{img("http://a/top.jpg"), fwd("f1")}
	var out []string
	visited := map[string]bool{}
	collectImages(root, getForward, 0, visited, &out)

	want := []string{"http://a/top.jpg", "http://a/inner1.jpg", "http://a/inner2.jpg"}
	if len(out) != len(want) {
		t.Fatalf("got %v want %v", out, want)
	}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("out[%d]=%q want %q (full=%v)", i, out[i], want[i], out)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./plugins/tools/ -run TestCollectImages -v`
Expected: 编译失败 `undefined: collectImages`

- [ ] **Step 3: 实现 `pack.go` 常量 + collectImages**

`plugins/tools/pack.go`：

```go
package tools

import (
	"encoding/json"

	"github.com/Yuelioi/yueling-go/bot"
)

const (
	packMaxImages = 100
	packMaxBytes  = 100 * 1024 * 1024
	packMaxDepth  = 5
)

// collectImages 递归收集一条消息里的图片 url（含展开的合并转发）。
// getForward 注入以便单测；visited 记已展开 forward id 防循环。
func collectImages(msg bot.Message, getForward func(string) ([]bot.Message, error), depth int, visited map[string]bool, out *[]string) {
	if depth > packMaxDepth {
		return
	}
	for _, s := range msg {
		if len(*out) >= packMaxImages {
			return
		}
		switch s.Type {
		case "image":
			var d struct {
				File string `json:"file"`
				URL  string `json:"url"`
			}
			if json.Unmarshal(s.Data, &d) == nil {
				if d.URL != "" {
					*out = append(*out, d.URL)
				} else if d.File != "" {
					*out = append(*out, d.File)
				}
			}
		case "forward":
			var d struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(s.Data, &d) == nil && d.ID != "" && !visited[d.ID] {
				visited[d.ID] = true
				if inner, err := getForward(d.ID); err == nil {
					for _, im := range inner {
						collectImages(im, getForward, depth+1, visited, out)
					}
				}
			}
		}
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./plugins/tools/ -run TestCollectImages -v`
Expected: PASS（循环被 visited 拦住，只收到 3 张）

- [ ] **Step 5: 提交**

```
git add plugins/tools/pack.go plugins/tools/pack_test.go
git commit -m "feat(pack): collectImages 递归抽图 + 防循环"
```

---

### Task 3: 扩展名探测 + zip 字节打包（纯函数）

**Files:**
- Modify: `plugins/tools/pack.go`
- Test: `plugins/tools/pack_test.go`

`detectImageExt` 与 `plugins/system/image.go` 同逻辑（包内私有不可跨包引用，按 spec 放一份等价实现）。
`writeZipBytes` 把若干 (name,data) 打成 zip 字节。

- [ ] **Step 1: 写失败测试**

追加到 `plugins/tools/pack_test.go`：

```go
import (
	"archive/zip"
	"bytes"
)

func TestDetectImageExt(t *testing.T) {
	cases := []struct {
		head []byte
		want string
	}{
		{[]byte{0x89, 'P', 'N', 'G', 0, 0, 0, 0, 0, 0, 0, 0}, "png"},
		{[]byte{'G', 'I', 'F', '8', '9', 'a', 0, 0, 0, 0, 0, 0}, "gif"},
		{[]byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P'}, "webp"},
		{[]byte{0xFF, 0xD8, 0xFF, 0, 0, 0, 0, 0, 0, 0, 0, 0}, "jpg"},
		{[]byte{1, 2}, "jpg"},
	}
	for _, c := range cases {
		if got := detectImageExt(c.head); got != c.want {
			t.Fatalf("head=%v got=%q want=%q", c.head, got, c.want)
		}
	}
}

func TestWriteZipBytes(t *testing.T) {
	items := []packItem{
		{name: "001.jpg", data: []byte("aaa")},
		{name: "002.png", data: []byte("bb")},
	}
	raw, err := writeZipBytes(items)
	if err != nil {
		t.Fatalf("writeZipBytes: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	if len(zr.File) != 2 {
		t.Fatalf("want 2 files, got %d", len(zr.File))
	}
	if zr.File[0].Name != "001.jpg" || zr.File[1].Name != "002.png" {
		t.Fatalf("names = %q,%q", zr.File[0].Name, zr.File[1].Name)
	}
	rc, _ := zr.File[0].Open()
	got, _ := io.ReadAll(rc)
	rc.Close()
	if string(got) != "aaa" {
		t.Fatalf("file0 content = %q", got)
	}
}
```

（同时在测试 import 块补 `"io"`。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./plugins/tools/ -run 'TestDetectImageExt|TestWriteZipBytes' -v`
Expected: 编译失败 `undefined: detectImageExt` / `packItem` / `writeZipBytes`

- [ ] **Step 3: 实现**

追加到 `plugins/tools/pack.go`（顶部 import 补 `"archive/zip"`、`"bytes"`）：

```go
type packItem struct {
	name string
	data []byte
}

func writeZipBytes(items []packItem) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, it := range items {
		w, err := zw.Create(it.name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(it.data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func detectImageExt(data []byte) string {
	if len(data) < 12 {
		return "jpg"
	}
	switch {
	case data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G':
		return "png"
	case data[0] == 'G' && data[1] == 'I' && data[2] == 'F':
		return "gif"
	case string(data[8:12]) == "WEBP":
		return "webp"
	default:
		return "jpg"
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./plugins/tools/ -run 'TestDetectImageExt|TestWriteZipBytes' -v`
Expected: PASS

- [ ] **Step 5: 提交**

```
git add plugins/tools/pack.go plugins/tools/pack_test.go
git commit -m "feat(pack): detectImageExt + writeZipBytes 辅助函数"
```

---

### Task 4: 下载装包 + RegisterPack 命令处理

**Files:**
- Modify: `plugins/tools/pack.go`
- Test: `plugins/tools/pack_test.go`（只测纯函数 `downloadItems`；handler 依赖 NapCat 手动验证）

`downloadItems` 把 url 列表逐个下载（注入下载函数便于测），带张数/字节上限，命名 `001.ext`、
单张失败跳过。`RegisterPack` 串起：收 url（当前消息 + 被回复消息递归）→ 去重 → 下载 → zip →
写临时文件 → 上传群文件 → 删临时文件 → 回复。

- [ ] **Step 1: 写失败测试**

追加到 `plugins/tools/pack_test.go`：

```go
func TestDownloadItems(t *testing.T) {
	data := map[string][]byte{
		"u1": {0x89, 'P', 'N', 'G', 0, 0, 0, 0, 0, 0, 0, 0}, // png
		"u2": {0xFF, 0xD8, 0xFF, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // jpg
		"u3": nil,                                            // 下载失败
	}
	get := func(u string) ([]byte, error) {
		if data[u] == nil {
			return nil, errTest
		}
		return data[u], nil
	}
	items, total := downloadItems([]string{"u1", "u2", "u3"}, get)
	if len(items) != 2 {
		t.Fatalf("want 2 ok items, got %d", len(items))
	}
	if items[0].name != "001.png" || items[1].name != "002.jpg" {
		t.Fatalf("names = %q,%q", items[0].name, items[1].name)
	}
	if total != 24 {
		t.Fatalf("total bytes = %d", total)
	}
}

var errTest = errors.New("fail")
```

（测试 import 块补 `"errors"`。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./plugins/tools/ -run TestDownloadItems -v`
Expected: 编译失败 `undefined: downloadItems`

- [ ] **Step 3: 实现 downloadItems + RegisterPack**

`plugins/tools/pack.go` 顶部 import 补全为：

```go
import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services"
	"github.com/Yuelioi/yueling-go/services/httpclient"
	"github.com/Yuelioi/yueling-go/services/logx"
)
```

追加：

```go
// downloadItems 逐个下载 url，命名 NNN.ext，带字节上限，单张失败跳过。
// get 注入便于单测。
func downloadItems(urls []string, get func(string) ([]byte, error)) ([]packItem, int64) {
	var items []packItem
	var total int64
	for _, u := range urls {
		if len(items) >= packMaxImages || total >= packMaxBytes {
			break
		}
		data, err := get(u)
		if err != nil {
			logx.Warnf("[pack] 下载失败 %s: %v", u, err)
			continue
		}
		total += int64(len(data))
		name := fmt.Sprintf("%03d.%s", len(items)+1, detectImageExt(data))
		items = append(items, packItem{name: name, data: data})
	}
	return items, total
}

func RegisterPack(b *bot.Bot) {
	b.OnCommand("pack").Handle(func(ctx *bot.CommandContext) error {
		visited := map[string]bool{}
		var urls []string
		collectImages(ctx.Message(), ctx.GetForwardMsg, 0, visited, &urls)
		if replyID, ok := ctx.Message().ReplyID(); ok {
			var mid int32
			fmt.Sscan(replyID, &mid)
			if mid != 0 {
				if replied, err := ctx.GetMsg(mid); err == nil {
					collectImages(replied, ctx.GetForwardMsg, 0, visited, &urls)
				}
			}
		}

		// 去重，保序
		seen := map[string]bool{}
		uniq := urls[:0]
		for _, u := range urls {
			if !seen[u] {
				seen[u] = true
				uniq = append(uniq, u)
			}
		}
		urls = uniq

		if len(urls) == 0 {
			return ctx.Reply("未找到可打包的图片")
		}

		items, _ := downloadItems(urls, func(u string) ([]byte, error) {
			return httpclient.Direct.GetBytes(u)
		})
		if len(items) == 0 {
			return ctx.Reply("图片下载失败")
		}

		zipBytes, err := writeZipBytes(items)
		if err != nil {
			logx.Errorf("[pack] 打包失败: %v", err)
			return ctx.Reply("打包失败")
		}

		dir := services.DataPath("tmp")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return ctx.Reply("打包失败")
		}
		ts := time.Now().Format("20060102_150405")
		zipPath := filepath.Join(dir, fmt.Sprintf("pack_%d_%s.zip", ctx.GroupID(), ts))
		if err := os.WriteFile(zipPath, zipBytes, 0o644); err != nil {
			logx.Errorf("[pack] 写临时文件失败: %v", err)
			return ctx.Reply("打包失败")
		}
		defer os.Remove(zipPath)

		if err := ctx.UploadGroupFile(ctx.GroupID(), zipPath, fmt.Sprintf("图片打包_%s.zip", ts), ""); err != nil {
			logx.Errorf("[pack] 上传群文件失败: %v", err)
			return ctx.Reply("上传失败")
		}

		msg := fmt.Sprintf("已打包 %d 张图片", len(items))
		if len(items) >= packMaxImages {
			msg += fmt.Sprintf("（已达上限 %d 张）", packMaxImages)
		}
		return ctx.Reply(msg)
	})
}
```

- [ ] **Step 4: 运行测试确认通过 + 全包构建**

Run: `go test ./plugins/tools/ -run TestDownloadItems -v`
Expected: PASS

Run: `go build ./...`
Expected: 无输出（编译通过）。

- [ ] **Step 5: 提交**

```
git add plugins/tools/pack.go plugins/tools/pack_test.go
git commit -m "feat(pack): downloadItems + RegisterPack 命令处理"
```

---

### Task 5: 注册命令 + 全量验证

**Files:**
- Modify: `cmd/bot/main.go`（tools 注册区，`tools.RegisterZssm(b)` 附近，约 127 行）

- [ ] **Step 1: 注册**

在 `cmd/bot/main.go` 的 `tools.RegisterZssm(b)` 之后加一行：

```go
	tools.RegisterPack(b)
```

- [ ] **Step 2: 全量构建 + 测试**

Run: `go build ./...`
Expected: 无输出

Run: `go test ./...`
Expected: 全部 PASS（bot 与 plugins/tools 新测试在内）

- [ ] **Step 3: 提交**

```
git add cmd/bot/main.go
git commit -m "feat(pack): 注册 pack 命令"
```

- [ ] **Step 4: 手动验证（依赖 NapCat，部署后）**

1. 群里发一条带多张图的消息，回复它发 `pack` → 群文件出现 `图片打包_<时间>.zip`，含全部图，bot 回「已打包 N 张图片」。
2. 回复一条**合并转发**（内含多条带图消息，最好有嵌套）发 `pack` → zip 含递归抽出的所有图。
3. 回复一条无图消息发 `pack` → 回「未找到可打包的图片」。

> 前提：部署里 NapCat 能读到 bot 的 `data/tmp` 路径（与现有 `group/files.go` 上传群文件同款约定）。
> 同时更新 `flightdeck/cockpit.md` 与（若有）功能清单 `board`。

---

## Self-Review

- **Spec 覆盖**：触发(Task4 RegisterPack) ✓；取图含当前+被回复递归(Task2+4) ✓；GetForwardMsg(Task1) ✓；
  递归 depth≤5 + visited 防循环(Task2) ✓；zip→临时文件→UploadGroupFile→删除(Task4) ✓；
  无图/全失败提示(Task4) ✓；100 张/100MB 上限(Task2 张数 + Task4 字节)✓；detectImageExt 独立一份(Task3) ✓；
  注册在 Start 前(Task5) ✓；日志走 logx(Task4) ✓；测试(Task1/2/3/4) ✓。
- **占位符**：无 TBD/TODO；每个代码步骤含完整代码。
- **类型一致**：`packItem{name,data}`、`collectImages(msg,getForward,depth,visited,*out)`、
  `downloadItems(urls,get)->([]packItem,int64)`、`writeZipBytes([]packItem)->([]byte,error)`、
  `detectImageExt([]byte)->string`、`GetForwardMsg(string)->([]Message,error)` 跨任务一致。
