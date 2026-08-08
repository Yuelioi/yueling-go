---
status: done
summary: 分阶段落地图片配置表重构：config schema → image 包(upload/external/grid/entries/register/help) → 抽离 quotation/emoticon → main 重连 + 删旧 → help 集成。
last_updated: 2026-06-23
implements: specs/2026-06-23-image-categories-config-table.md
---

# 图片类目配置表驱动重构 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development 或 superpowers:executing-plans 逐任务实现。步骤用 `- [ ]` 复选框跟踪。

**Goal:** 把图片类目（调用+添加+文件名策略）收敛成 `[image.entry]` 配置表（single/grid/external 三 kind），语录/表情抽成独立插件，help 由配置生成。

**Architecture:** 新建 `plugins/image` 包用配置表驱动注册；上传内核抽成带 `nameFn` 参数的通用函数供三插件共用；`config` 包持纯数据结构 `ImageEntry`/`Kind` + 内置默认表兜底（覆盖语义）。先加新代码（旧代码仍在、可编译），最后一步重连 `main.go` 并删旧。

**Tech Stack:** Go，viper（config），现有 `bot` 框架（`OnFullMatch`/`OnCommand`/`Handle`），`services`（`GetRandomImage`/`ListImageNames`/`ShrinkToJPEG`/`DataPath`），`httpclient.Direct`。

## Global Constraints

- module path: `github.com/Yuelioi/yueling-go`
- 所有注册必须在 `b.Start()` 之前完成，不得动态改 `b.regs`。
- handler 签名必须是四种之一：`func(*CommandContext)error` / `func(*GroupContext)error` / `func(*NoticeContext)error` / `func(*RequestContext)error`。
- 精确命令用 `OnCommand`/`OnFullMatch`（不要 `OnKeyword`/`Any` 兜底）。
- 日志统一 `services/logx`，不用 stdlib `log`。
- 改命令后必须让 `help`/`帮助` 能看到——本计划用「help 由配置生成」满足。
- `config.ImageEntry`/`Kind` 定义在 `config` 包（纯数据），`plugins/*` 引用它，禁止 `config` 反向 import `plugins`。

---

### Task 1: config 包加 ImageEntry / Kind / Entry 字段

**Files:**
- Modify: `config/config.go`（`ImageConfig` 结构 + 新增类型）
- Test: `config/imageentry_test.go`（新建）

**Interfaces:**
- Produces: `config.Kind`（`KindSingle`/`KindGrid`/`KindExternal`）、`config.ImageEntry{Folder,Call,Add,Kind,URL,Pick}`、`config.ImageConfig.Entry []ImageEntry`。

- [ ] **Step 1: 写失败测试** —— `config/imageentry_test.go`：

```go
package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestImageEntryUnmarshal(t *testing.T) {
	viper.Reset()
	viper.SetConfigType("toml")
	toml := `
[[image.entry]]
folder = "龙图"
call   = ["龙图", "龙图攻击"]
add    = "添加龙图"

[[image.entry]]
call = ["随机猫猫"]
kind = "external"
url  = "http://edgecats.net/"
pick = "data.url"
`
	if err := viper.ReadConfig(strings.NewReader(toml)); err != nil {
		t.Fatal(err)
	}
	var c Config
	if err := viper.Unmarshal(&c); err != nil {
		t.Fatal(err)
	}
	if len(c.Image.Entry) != 2 {
		t.Fatalf("want 2 entries, got %d", len(c.Image.Entry))
	}
	e0 := c.Image.Entry[0]
	if e0.Folder != "龙图" || len(e0.Call) != 2 || e0.Add != "添加龙图" {
		t.Fatalf("entry0 mismatch: %+v", e0)
	}
	e1 := c.Image.Entry[1]
	if e1.Kind != KindExternal || e1.URL != "http://edgecats.net/" || e1.Pick != "data.url" {
		t.Fatalf("entry1 mismatch: %+v", e1)
	}
}
```

- [ ] **Step 2: 跑测试确认失败** —— `go test ./config/ -run TestImageEntryUnmarshal -v`，预期 FAIL（`Entry` / `KindExternal` 未定义，编译不过）。

- [ ] **Step 3: 实现** —— `config/config.go`，在 `ImageConfig` 上加 `Entry`，并新增类型（放 `ImageConfig` 定义附近）：

```go
type ImageConfig struct {
	Convert        bool         `mapstructure:"convert"`
	ConvertMinKB   int          `mapstructure:"convert_min_kb"`
	ConvertQuality int          `mapstructure:"convert_quality"`
	Entry          []ImageEntry `mapstructure:"entry"` // [[image.entry]] 配置表；空则用插件内置默认表
}

// Kind 图片类目行为：随机一张 / 4合1网格 / 外链。
type Kind string

const (
	KindSingle   Kind = "single"
	KindGrid     Kind = "grid"
	KindExternal Kind = "external"
)

// ImageEntry 一条图片类目配置。kind 隐含带不带参与文件名策略。
type ImageEntry struct {
	Folder string   `mapstructure:"folder"` // 素材子目录；external 可空
	Call   []string `mapstructure:"call"`   // 调用命令（FullMatch）
	Add    string   `mapstructure:"add"`    // 添加命令（OnCommand）；external/无添加可空
	Kind   Kind     `mapstructure:"kind"`   // 缺省视为 single
	URL    string   `mapstructure:"url"`    // 仅 external：请求地址
	Pick   string   `mapstructure:"pick"`   // 仅 external：JSON 取图路径；空=响应本身就是图
}
```

- [ ] **Step 4: 跑测试确认通过** —— `go test ./config/ -run TestImageEntryUnmarshal -v`，预期 PASS。

- [ ] **Step 5: 提交** ——

```bash
git add config/config.go config/imageentry_test.go
git commit -m "feat(config): 新增 [image.entry] 图片类目配置表结构"
```

---

### Task 2: plugins/image 的 pick 路径求值（纯函数 + TDD）

**Files:**
- Create: `plugins/image/external.go`
- Test: `plugins/image/external_test.go`

**Interfaces:**
- Produces: `func ExtractImageURL(jsonBody []byte, path string) (string, error)`（path 非空；为空表示「响应本身即图」，由调用方处理）。

- [ ] **Step 1: 写失败测试** —— `plugins/image/external_test.go`：

```go
package image

import "testing"

func TestExtractImageURL(t *testing.T) {
	cases := []struct {
		name, body, path, want string
		wantErr                bool
	}{
		{"object field", `{"data":{"url":"x"}}`, "data.url", "x", false},
		{"list of strings random", `{"data":["only"]}`, "data", "only", false},
		{"list of objects random", `{"data":[{"url":"a"}]}`, "data.url", "a", false},
		{"nested", `{"a":{"b":{"c":"deep"}}}`, "a.b.c", "deep", false},
		{"missing key", `{"data":{}}`, "data.url", "", true},
		{"not a string", `{"data":{"url":123}}`, "data.url", "", true},
		{"bad json", `not json`, "data", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ExtractImageURL([]byte(c.body), c.path)
			if c.wantErr {
				if err == nil {
					t.Fatalf("want error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: 跑测试确认失败** —— `go test ./plugins/image/ -run TestExtractImageURL -v`，预期 FAIL（`ExtractImageURL` 未定义）。

- [ ] **Step 3: 实现** —— `plugins/image/external.go`：

```go
package image

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
)

// ExtractImageURL 按点路径从 JSON 取图片 URL；遇数组自动随机抽一个。
// path 必须非空（path 为空表示「响应本身即图」，由调用方在外层处理）。
func ExtractImageURL(jsonBody []byte, path string) (string, error) {
	var root any
	if err := json.Unmarshal(jsonBody, &root); err != nil {
		return "", fmt.Errorf("解析 JSON 失败: %w", err)
	}
	cur := root
	for _, key := range strings.Split(path, ".") {
		if key == "" {
			continue
		}
		if arr, ok := cur.([]any); ok {
			if len(arr) == 0 {
				return "", fmt.Errorf("路径 %q 处数组为空", path)
			}
			cur = arr[rand.Intn(len(arr))]
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return "", fmt.Errorf("路径 %q 在 %q 处不是对象", path, key)
		}
		v, ok := m[key]
		if !ok {
			return "", fmt.Errorf("路径 %q 缺少键 %q", path, key)
		}
		cur = v
	}
	if arr, ok := cur.([]any); ok {
		if len(arr) == 0 {
			return "", fmt.Errorf("路径 %q 结果数组为空", path)
		}
		cur = arr[rand.Intn(len(arr))]
	}
	s, ok := cur.(string)
	if !ok {
		return "", fmt.Errorf("路径 %q 结果不是字符串", path)
	}
	return s, nil
}
```

- [ ] **Step 4: 跑测试确认通过** —— `go test ./plugins/image/ -run TestExtractImageURL -v`，预期 PASS。

- [ ] **Step 5: 提交** ——

```bash
git add plugins/image/external.go plugins/image/external_test.go
git commit -m "feat(image): pick 路径求值（JSON 取图，数组随机）"
```

---

### Task 3: 通用上传内核（从 system/image.go 抽出，加 nameFn）

**Files:**
- Create: `plugins/image/upload.go`
- Test: `plugins/image/upload_test.go`（仅测纯函数 `detectImageExt`）

**Interfaces:**
- Consumes: `config.C.Image.Convert/ConvertMinKB/ConvertQuality`、`services.DataPath`/`ShrinkToJPEG`、`httpclient.Direct.GetBytes`、`bot.CommandContext`（`CollectImageURLs`/`Args`/`GroupID`/`Reply`）。
- Produces: `func Upload(ctx *bot.CommandContext, folder string, nameFn func(hash, arg string, gid int64) string) error`、`func detectImageExt([]byte) string`。

> 把 `plugins/system/image.go` 的 `uploadImages`/`hashExistsInDir`/`fetchImageBytes`/`detectImageExt` 逻辑搬来。改动：`uploadImages`→`Upload`，签名加 `nameFn`，内部 `buildImageFilename(...)` 调用换成 `nameFn(hash, arg, gid)`；`fetchImageBytes` 内联为 `httpclient.Direct.GetBytes`。`buildImageFilename` 不搬（分支下放到各插件 nameFn）。

- [ ] **Step 1: 写失败测试** —— `plugins/image/upload_test.go`：

```go
package image

import "testing"

func TestDetectImageExt(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"png", []byte{0x89, 'P', 'N', 'G', 0, 0, 0, 0, 0, 0, 0, 0}, "png"},
		{"gif", []byte("GIF89a______"), "gif"},
		{"webp", []byte("RIFF____WEBP"), "webp"},
		{"jpg default", []byte{0xFF, 0xD8, 0xFF, 0, 0, 0, 0, 0, 0, 0, 0, 0}, "jpg"},
		{"too short", []byte{1, 2, 3}, "jpg"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := detectImageExt(c.data); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: 跑测试确认失败** —— `go test ./plugins/image/ -run TestDetectImageExt -v`，预期 FAIL（`detectImageExt` 未定义）。

- [ ] **Step 3: 实现** —— `plugins/image/upload.go`（完整代码）：

```go
package image

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services"
	"github.com/Yuelioi/yueling-go/services/httpclient"
	"github.com/Yuelioi/yueling-go/services/logx"
)

// Upload 下载附带图片入库到 <folder>，文件名由 nameFn 决定。相同图片(hash)不重复收录。
func Upload(ctx *bot.CommandContext, folder string, nameFn func(hash, arg string, gid int64) string) error {
	urls := ctx.CollectImageURLs()
	if len(urls) == 0 {
		return ctx.Reply("请附带图片")
	}

	arg := strings.TrimSpace(strings.Join(ctx.Args, " "))
	dir := services.DataPath("images", folder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ctx.Reply("目录创建失败")
	}

	var lines []string
	for i, imgURL := range urls {
		label := fmt.Sprintf("图片%d", i+1)
		data, err := httpclient.Direct.GetBytes(imgURL)
		if err != nil {
			logx.Warnf("[image] fetch %s: %v", label, err)
			lines = append(lines, label+" 下载失败")
			continue
		}

		if config.C.Image.Convert {
			data = services.ShrinkToJPEG(data, config.C.Image.ConvertMinKB*1024, config.C.Image.ConvertQuality)
		}

		h := sha256.Sum256(data)
		hash := fmt.Sprintf("%x", h)[:16]

		if hashExistsInDir(dir, hash) {
			lines = append(lines, label+" 已收录（重复）")
			continue
		}

		ext := detectImageExt(data)
		name := nameFn(hash, arg, ctx.GroupID())
		if err := os.WriteFile(filepath.Join(dir, name+"."+ext), data, 0o644); err != nil {
			lines = append(lines, label+" 保存失败")
			continue
		}
		logx.Infof("[image] saved %s/%s.%s", folder, name, ext)
		lines = append(lines, label+" 上传成功")
	}

	return ctx.Reply(strings.Join(lines, "\n"))
}

func hashExistsInDir(dir, hash string) bool {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), hash) {
			return true
		}
	}
	return false
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

- [ ] **Step 4: 跑测试确认通过** —— `go test ./plugins/image/ -run TestDetectImageExt -v`，预期 PASS。

- [ ] **Step 5: 提交** ——

```bash
git add plugins/image/upload.go plugins/image/upload_test.go
git commit -m "feat(image): 通用上传内核 Upload（命名策略参数化）"
```

---

### Task 4: grid 网格渲染（从 random/daily.go 搬入 image 包）

**Files:**
- Create: `plugins/image/grid.go`

**Interfaces:**
- Consumes: `services.DataPath`、`bot.GroupContext`、`bot.Msg()`。
- Produces: `func renderGrid(ctx *bot.GroupContext, folder string) error`。

> 把 `plugins/random/daily.go` 的 `dailyReplies`/`dailyNums`/`pickFiles`/`buildGrid`/`decodeImage`/`coverResize` 原样搬来（含全部 import：`bytes`/`image`/`image/color`/`image/draw`/`image/gif`/`image/jpeg`/`image/png`(下划线)/`math/rand`/`os`/`path/filepath`/`strings`/`golang.org/x/image/draw`(xdraw)/`golang.org/x/image/webp`(下划线)/`bot`/`services`/`logx`），package 改 `image`。`RegisterDaily`/`dailyHandler` 不搬，改写成 `renderGrid`。

- [ ] **Step 1: 实现 grid.go** —— 搬运上述辅助函数，并新增 `renderGrid`：

```go
// renderGrid 挑 4 张拼 2×2 网格，按文件名(stem)列出菜单。
func renderGrid(ctx *bot.GroupContext, folder string) error {
	picks, err := pickFiles(services.DataPath("images", folder), 4)
	if err != nil || len(picks) == 0 {
		return ctx.Reply("暂无素材")
	}

	replies, ok := dailyReplies[folder]
	if !ok {
		replies = []string{"随手摇了几个出来", "看上哪个挑哪个", "难选就闭眼点一个", "这几个都不错"}
	}
	hint := replies[rand.Intn(len(replies))]

	var parts []string
	for i, p := range picks {
		stem := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		parts = append(parts, fmt.Sprintf("%s %s", dailyNums[i], stem))
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n%s", hint, strings.Join(parts, "  "))

	imgData, err := buildGrid(picks)
	if err != nil {
		return ctx.Reply(sb.String())
	}
	_, err = ctx.SendGroupMsg(ctx.GroupID(), bot.Msg().Text(sb.String()+"\n").ImageBytes(imgData).Build())
	return err
}
```

> `renderGrid` 用到 `fmt`，确保 import 含 `fmt`（daily.go 原本就有）。

- [ ] **Step 2: 编译确认** —— `go build ./plugins/image/`，预期 PASS（包级未导出函数即使暂无引用，Go 不报错）。

- [ ] **Step 3: 提交** ——

```bash
git add plugins/image/grid.go
git commit -m "feat(image): grid 网格渲染（搬自 daily.go）"
```

---

### Task 5: 默认表 + 启动校验 + 命名函数（TDD）

**Files:**
- Create: `plugins/image/entries.go`
- Test: `plugins/image/entries_test.go`

**Interfaces:**
- Consumes: `config.ImageEntry`/`config.Kind`。
- Produces: `var defaultEntries []config.ImageEntry`、`func kindOf(config.ImageEntry) config.Kind`、`func validateEntries([]config.ImageEntry) error`、`func nameByHash(hash, arg string, gid int64) string`、`func nameByArg(hash, arg string, gid int64) string`。

- [ ] **Step 1: 写失败测试** —— `plugins/image/entries_test.go`：

```go
package image

import (
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

func TestValidateEntries(t *testing.T) {
	ok := []config.ImageEntry{
		{Folder: "龙图", Call: []string{"龙图"}, Add: "添加龙图"},
		{Folder: "吃的", Call: []string{"随机吃的"}, Add: "添加吃的", Kind: config.KindGrid},
		{Call: []string{"猫猫"}, Kind: config.KindExternal, URL: "http://x/"},
	}
	if err := validateEntries(ok); err != nil {
		t.Fatalf("valid set rejected: %v", err)
	}

	bad := []struct {
		name    string
		entries []config.ImageEntry
	}{
		{"bad kind", []config.ImageEntry{{Folder: "a", Call: []string{"a"}, Kind: "weird"}}},
		{"single no folder", []config.ImageEntry{{Call: []string{"a"}}}},
		{"single no call", []config.ImageEntry{{Folder: "a"}}},
		{"grid no add", []config.ImageEntry{{Folder: "a", Call: []string{"a"}, Kind: config.KindGrid}}},
		{"external no url", []config.ImageEntry{{Call: []string{"a"}, Kind: config.KindExternal}}},
		{"dup command", []config.ImageEntry{
			{Folder: "a", Call: []string{"x"}, Add: "添加a"},
			{Folder: "b", Call: []string{"x"}, Add: "添加b"},
		}},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			if err := validateEntries(c.entries); err == nil {
				t.Fatalf("expected error for %s", c.name)
			}
		})
	}
}

func TestNameFns(t *testing.T) {
	if got := nameByHash("HH", "ignored", 123); got != "HH" {
		t.Fatalf("nameByHash = %q", got)
	}
	if got := nameByArg("HH", "麻辣烫", 123); got != "麻辣烫" {
		t.Fatalf("nameByArg = %q", got)
	}
	if got := nameByArg("HH", "", 123); got != "HH" {
		t.Fatalf("nameByArg empty arg = %q, want HH", got)
	}
}

func TestDefaultEntriesValid(t *testing.T) {
	if err := validateEntries(defaultEntries); err != nil {
		t.Fatalf("defaultEntries invalid: %v", err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败** —— `go test ./plugins/image/ -run 'TestValidateEntries|TestNameFns|TestDefaultEntriesValid' -v`，预期 FAIL（未定义）。

- [ ] **Step 3: 实现** —— `plugins/image/entries.go`：

```go
package image

import (
	"fmt"

	"github.com/Yuelioi/yueling-go/config"
)

// defaultEntries 现网不配 [[image.entry]] 时使用，逐条照搬重构前行为。
var defaultEntries = []config.ImageEntry{
	{Folder: "龙图", Call: []string{"龙图", "龙图攻击"}, Add: "添加龙图"},
	{Folder: "福瑞", Call: []string{"福瑞", "来点福瑞"}, Add: "添加福瑞"},
	{Folder: "老公", Call: []string{"我老公呢", "老公"}, Add: "添加老公"},
	{Folder: "老婆", Call: []string{"我老婆呢", "老婆"}, Add: "添加老婆"},
	{Folder: "沙雕图", Call: []string{"沙雕图"}, Add: "添加沙雕图"},
	{Folder: "杂鱼", Call: []string{"杂鱼"}, Add: "添加杂鱼"},
	{Folder: "美少女", Call: []string{"美少女"}, Add: "添加美少女"},
	{Folder: "ba", Call: []string{"ba", "来点ba"}, Add: "添加ba"},
	{Folder: "吃的", Call: []string{"随机吃的", "吃啥", "吃什么", "来点吃的"}, Add: "添加吃的", Kind: config.KindGrid},
	{Folder: "喝的", Call: []string{"随机喝的", "喝啥", "喝什么", "来点喝的"}, Add: "添加喝的", Kind: config.KindGrid},
	{Folder: "玩的", Call: []string{"随机玩的", "玩啥", "玩什么", "来点玩的"}, Add: "添加玩的", Kind: config.KindGrid},
	{Folder: "水果", Call: []string{"随机水果", "来点水果"}, Add: "添加水果", Kind: config.KindGrid},
	{Folder: "猫猫", Call: []string{"随机猫猫", "来点猫猫"}, Kind: config.KindExternal, URL: "http://edgecats.net/"},
}

func kindOf(e config.ImageEntry) config.Kind {
	if e.Kind == "" {
		return config.KindSingle
	}
	return e.Kind
}

// validateEntries 启动时校验配置表，非法即返回错误（fail-fast）。
func validateEntries(entries []config.ImageEntry) error {
	seen := map[string]bool{}
	mark := func(cmd string) error {
		if cmd == "" {
			return nil
		}
		if seen[cmd] {
			return fmt.Errorf("命令重复: %q", cmd)
		}
		seen[cmd] = true
		return nil
	}
	for i, e := range entries {
		switch kindOf(e) {
		case config.KindSingle, config.KindGrid:
			if e.Folder == "" {
				return fmt.Errorf("entry[%d] %s 缺少 folder", i, kindOf(e))
			}
			if len(e.Call) == 0 {
				return fmt.Errorf("entry[%d] %s 缺少 call", i, kindOf(e))
			}
			if kindOf(e) == config.KindGrid && e.Add == "" {
				return fmt.Errorf("entry[%d] grid 缺少 add", i)
			}
		case config.KindExternal:
			if e.URL == "" {
				return fmt.Errorf("entry[%d] external 缺少 url", i)
			}
		default:
			return fmt.Errorf("entry[%d] 非法 kind: %q", i, e.Kind)
		}
		for _, c := range e.Call {
			if err := mark(c); err != nil {
				return err
			}
		}
		if err := mark(e.Add); err != nil {
			return err
		}
	}
	return nil
}

func nameByHash(hash, _ string, _ int64) string { return hash }

func nameByArg(hash, arg string, _ int64) string {
	if arg == "" {
		return hash
	}
	return arg
}
```

- [ ] **Step 4: 跑测试确认通过** —— `go test ./plugins/image/ -v`，预期全部 PASS。

- [ ] **Step 5: 提交** ——

```bash
git add plugins/image/entries.go plugins/image/entries_test.go
git commit -m "feat(image): 默认表 + 启动校验 + 命名函数"
```

---

### Task 6: image.Register + 三 kind 分派

**Files:**
- Create: `plugins/image/image.go`

**Interfaces:**
- Consumes: `defaultEntries`/`validateEntries`/`kindOf`/`nameByHash`/`nameByArg`/`Upload`/`renderGrid`/`ExtractImageURL`、`config.C.Image.Entry`、`bot` API、`services.GetRandomImage`、`httpclient.Direct.GetBytes`、`logx`。
- Produces: `func Register(b *bot.Bot)`、包级 `var activeEntries []config.ImageEntry`（供 help.go 读取）。

- [ ] **Step 1: 实现** —— `plugins/image/image.go`：

```go
package image

import (
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services"
	"github.com/Yuelioi/yueling-go/services/httpclient"
	"github.com/Yuelioi/yueling-go/services/logx"
)

// activeEntries 实际生效的配置表（默认表或配置覆盖），help.go 据此生成帮助。
var activeEntries []config.ImageEntry

func Register(b *bot.Bot) {
	entries := config.C.Image.Entry
	if len(entries) == 0 {
		entries = defaultEntries
	}
	if err := validateEntries(entries); err != nil {
		logx.Fatalf("[image] 配置校验失败: %v", err)
	}
	activeEntries = entries

	for _, e := range entries {
		e := e
		switch kindOf(e) {
		case config.KindSingle:
			registerSingle(b, e)
		case config.KindGrid:
			registerGrid(b, e)
		case config.KindExternal:
			registerExternal(b, e)
		}
	}
}

func registerSingle(b *bot.Bot, e config.ImageEntry) {
	folder := e.Folder
	b.OnFullMatch(e.Call...).Handle(func(ctx *bot.GroupContext) error {
		path, err := services.GetRandomImage(folder, "")
		if err != nil {
			return ctx.Reply("图片不存在，请先放入素材")
		}
		return ctx.SendGroupLocalImage(ctx.GroupID(), path)
	})
	if e.Add != "" {
		b.OnCommand(e.Add).Handle(func(ctx *bot.CommandContext) error {
			return Upload(ctx, folder, nameByHash)
		})
	}
}

func registerGrid(b *bot.Bot, e config.ImageEntry) {
	folder, add := e.Folder, e.Add
	b.OnFullMatch(e.Call...).Handle(func(ctx *bot.GroupContext) error {
		return renderGrid(ctx, folder)
	})
	b.OnCommand(add).Handle(func(ctx *bot.CommandContext) error {
		if joinArgs(ctx.Args) == "" {
			return ctx.Reply("请带上名字，如：" + add + " 麻辣烫")
		}
		return Upload(ctx, folder, nameByArg)
	})
}

func registerExternal(b *bot.Bot, e config.ImageEntry) {
	url, pick := e.URL, e.Pick
	b.OnFullMatch(e.Call...).Handle(func(ctx *bot.GroupContext) error {
		if pick == "" {
			_, err := ctx.SendGroupMsg(ctx.GroupID(), bot.Msg().Image(url).Build())
			return err
		}
		body, err := httpclient.Direct.GetBytes(url)
		if err != nil {
			logx.Warnf("[image] external GET %s: %v", url, err)
			return ctx.Reply("获取失败")
		}
		imgURL, err := ExtractImageURL(body, pick)
		if err != nil {
			logx.Warnf("[image] external pick %q: %v", pick, err)
			return ctx.Reply("解析失败")
		}
		_, err = ctx.SendGroupMsg(ctx.GroupID(), bot.Msg().Image(imgURL).Build())
		return err
	})
}

func joinArgs(args []string) string {
	out := ""
	for _, a := range args {
		out += strings.TrimSpace(a)
	}
	return out
}
```

> 实现前先核对环境（这两点在 Step 2 编译时会暴露，按实际调整）：
> - `logx.Fatalf` 是否存在。若无，用 `logx.Errorf(...)` + `panic(err)`（启动期 fail-fast，等价）。
> - `joinArgs` 需要 `strings`，记得 import；或若 `bot` 已有等价「参数是否为空」helper 则用现成的，删掉 `joinArgs`。

- [ ] **Step 2: 编译确认** —— `go build ./plugins/image/`，预期 PASS（按上面注记修正 `logx.Fatalf`/`strings` import 后）。

- [ ] **Step 3: 提交** ——

```bash
git add plugins/image/image.go
git commit -m "feat(image): Register 按 kind 分派 single/grid/external"
```

---

### Task 7: image 包的 help 生成

**Files:**
- Create: `plugins/image/help.go`

**Interfaces:**
- Consumes: `activeEntries`、`kindOf`、`config.Kind`。
- Produces: `func HelpCallUsage() string`、`func HelpAddUsage() string`、`func HelpCallCommands() []string`、`func HelpAddCommands() []string`。

- [ ] **Step 1: 实现** —— `plugins/image/help.go`：

```go
package image

import (
	"strings"

	"github.com/Yuelioi/yueling-go/config"
)

// HelpCallUsage 列出 single/grid/external 的调用命令（每类一组）。
func HelpCallUsage() string {
	var single, grid, external []string
	for _, e := range activeEntries {
		switch kindOf(e) {
		case config.KindSingle:
			single = append(single, e.Call...)
		case config.KindGrid:
			if len(e.Call) > 0 {
				grid = append(grid, e.Call[0]) // grid 取首命令做代表
			}
		case config.KindExternal:
			external = append(external, e.Call...)
		}
	}
	var lines []string
	if len(single) > 0 {
		lines = append(lines, "  "+strings.Join(single, " / "))
	}
	if len(external) > 0 {
		lines = append(lines, "  "+strings.Join(external, " / "))
	}
	if len(grid) > 0 {
		lines = append(lines, "  "+strings.Join(grid, " / ")+"（4合1，发 2×2 网格）")
	}
	return strings.Join(lines, "\n")
}

// HelpAddUsage 列出所有添加命令。
func HelpAddUsage() string {
	var adds []string
	for _, e := range activeEntries {
		if e.Add != "" {
			adds = append(adds, e.Add)
		}
	}
	return "  " + strings.Join(adds, " / ") + "  + 图片"
}

func HelpCallCommands() []string {
	var cmds []string
	for _, e := range activeEntries {
		cmds = append(cmds, e.Call...)
	}
	return cmds
}

func HelpAddCommands() []string {
	var cmds []string
	for _, e := range activeEntries {
		if e.Add != "" {
			cmds = append(cmds, e.Add)
		}
	}
	return cmds
}
```

- [ ] **Step 2: 编译确认** —— `go build ./plugins/image/`，预期 PASS。

- [ ] **Step 3: 提交** ——

```bash
git add plugins/image/help.go
git commit -m "feat(image): 由配置表生成 help 用法/命令清单"
```

---

### Task 8: 抽离 plugins/quotation（语录）

**Files:**
- Create: `plugins/quotation/quotation.go`

**Interfaces:**
- Consumes: `image.Upload`、`services.GetRandomImage`、`bot` API。
- Produces: `func Register(b *bot.Bot)`、`func HelpCommands() []string`（`["语录","添加语录"]`）。

> 合并现 `random/quotation.go`（调用检索）+ `system/image.go` 中 `语录` 命名分支（添加）。命名 `nameQuotation`：arg 非空→`{gid}_{arg}_{hash}`，否则 `{gid}_{hash}`。白名单 `玉米`/`甜甜` 与原 `quotationWhitelist` 一致。

- [ ] **Step 1: 实现** —— `plugins/quotation/quotation.go`：

```go
package quotation

import (
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/plugins/image"
	"github.com/Yuelioi/yueling-go/services"
)

var whitelist = []string{"玉米", "甜甜"}

func Register(b *bot.Bot) {
	b.OnCommand("语录").Handle(func(ctx *bot.CommandContext) error {
		nickname := strings.TrimSpace(strings.Join(ctx.Args, " "))
		var keyword string
		switch {
		case nickname == "":
			keyword = fmt.Sprintf("%d_", ctx.GroupID())
		case isWhitelisted(nickname):
			keyword = nickname
		default:
			keyword = fmt.Sprintf("%d_%s", ctx.GroupID(), nickname)
		}
		path, err := services.GetRandomImage("语录", keyword)
		if err != nil {
			return ctx.Reply("尚未添加此人语录")
		}
		return ctx.SendGroupLocalImage(ctx.GroupID(), path)
	})

	b.OnCommand("添加语录").Handle(func(ctx *bot.CommandContext) error {
		return image.Upload(ctx, "语录", nameQuotation)
	})
}

func nameQuotation(hash, arg string, gid int64) string {
	if arg != "" {
		return fmt.Sprintf("%d_%s_%s", gid, arg, hash)
	}
	return fmt.Sprintf("%d_%s", gid, hash)
}

func isWhitelisted(name string) bool {
	for _, w := range whitelist {
		if name == w {
			return true
		}
	}
	return false
}

func HelpCommands() []string { return []string{"语录", "添加语录"} }
```

- [ ] **Step 2: 编译确认** —— `go build ./plugins/quotation/`，预期 PASS。

- [ ] **Step 3: 提交** ——

```bash
git add plugins/quotation/quotation.go
git commit -m "feat(quotation): 语录抽成独立插件（调用+添加+群隔离命名）"
```

---

### Task 9: 抽离 plugins/emoticon（表情）

**Files:**
- Create: `plugins/emoticon/emoticon.go`

**Interfaces:**
- Consumes: `image.Upload`、`services.ListImageNames/GetRandomImage`、`bot`/`rule` API。
- Produces: `func Register(b *bot.Bot)`、`func HelpCommands() []string`（`["添加表情"]`）。

> 合并现 `random/emoticon.go`（空格触发）+ `system/image.go` 中 `表情` 命名分支（添加）。命名 `nameEmoticon`：arg 非空→`{arg}_{hash}`，否则 `{hash}`。空格触发逻辑（三空格忽略/双空格列名/单空格随机）原样照搬。

- [ ] **Step 1: 实现** —— `plugins/emoticon/emoticon.go`：

```go
package emoticon

import (
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/bot/rule"
	"github.com/Yuelioi/yueling-go/plugins/image"
	"github.com/Yuelioi/yueling-go/services"
)

func Register(b *bot.Bot) {
	b.OnCommand("添加表情").Handle(func(ctx *bot.CommandContext) error {
		return image.Upload(ctx, "表情", nameEmoticon)
	})

	b.OnGroupMessage().When(rule.NoReply, rule.NoAt).Handle(func(ctx *bot.GroupContext) error {
		text := ctx.MsgCtx.Event.Message.Text()

		if strings.HasPrefix(text, "   ") {
			return nil
		}
		if keyword, ok := strings.CutPrefix(text, "  "); ok {
			names, err := services.ListImageNames("表情", keyword)
			if err != nil || len(names) == 0 {
				return ctx.Reply(fmt.Sprintf("没有找到包含「%s」的表情", keyword))
			}
			preview := names
			if len(preview) > 10 {
				preview = preview[:10]
			}
			return ctx.Reply(fmt.Sprintf("共找到%d个:\n%s", len(names), strings.Join(preview, "\n")))
		}
		if keyword, ok := strings.CutPrefix(text, " "); ok {
			path, err := services.GetRandomImage("表情", keyword)
			if err != nil {
				return nil
			}
			return ctx.SendGroupLocalImage(ctx.GroupID(), path)
		}
		return nil
	})
}

func nameEmoticon(hash, arg string, _ int64) string {
	if arg != "" {
		return arg + "_" + hash
	}
	return hash
}

func HelpCommands() []string { return []string{"添加表情"} }
```

- [ ] **Step 2: 编译确认** —— `go build ./plugins/emoticon/`，预期 PASS。

- [ ] **Step 3: 提交** ——

```bash
git add plugins/emoticon/emoticon.go
git commit -m "feat(emoticon): 表情抽成独立插件（空格触发+添加+关键词命名）"
```

---

### Task 10: 重连 main.go、删旧文件、集成 help、更新示例配置

**Files:**
- Modify: `cmd/bot/main.go`
- Delete: `plugins/random/image.go`、`plugins/random/daily.go`、`plugins/random/quotation.go`、`plugins/random/emoticon.go`、`plugins/system/image.go`
- Modify: `plugins/system/help.go`（#17/#18/#19/#32 条目按下述调整）
- Modify: `config.example.toml`

**Interfaces:**
- Consumes: `image.Register/HelpCallUsage/HelpAddUsage/HelpCallCommands/HelpAddCommands`、`quotation.Register/HelpCommands`、`emoticon.Register/HelpCommands`。

- [ ] **Step 1: 改 main.go** —— 加 import `plugins/image`、`plugins/quotation`、`plugins/emoticon`；删 random 段 `RegisterEmoticon`/`RegisterImage`/`RegisterQuotation`/`RegisterDaily` 四行 + system 段 `system.RegisterImage(b)` 一行；加：

```go
	image.Register(b)
	quotation.Register(b)
	emoticon.Register(b)
```

- [ ] **Step 2: 删旧文件** ——

```bash
git rm plugins/random/image.go plugins/random/daily.go plugins/random/quotation.go plugins/random/emoticon.go plugins/system/image.go
```

- [ ] **Step 3: 改 help.go** —— `plugins/system/help.go` 顶部加 import `plugins/image`、`plugins/quotation`、`plugins/emoticon`。pluginRegistry 现为包级静态 slice 字面量；把它改为在 `init()` 内构造（其余 entry 原样），仅下述 4 条的动态字段用生成函数填：
  - **#18「随机图片」**：`Usage` = `image.HelpCallUsage() + "\n  语录 [名字]    群友语录，可按名字筛选"`；`Commands` = `append(image.HelpCallCommands(), "语录")`。（grid 命令已含在 `HelpCallUsage` 中）
  - **#19「日常随机」**：删除该条（grid 已并入 #18，避免重复）。
  - **#32「素材上传」**：`Usage` = `image.HelpAddUsage() + "\n  添加表情 [关键词] + 图片   按关键词索引，用于空格触发\n  添加语录 [昵称]   + 图片   按群+昵称索引，语录命令可查\n  支持同时上传多张；引用含图片的消息也可触发"`；`Commands` = `append(append(image.HelpAddCommands(), quotation.HelpCommands()...), emoticon.HelpCommands()...)`。
  - **#17「表情包」**：不动（空格触发说明，非命令清单）。

> 若 `pluginRegistry` 被其他启动逻辑（如预渲染列表图的 goroutine，见 help.go:303 附近）在 `init` 之外读取，确认改成 `init()` 构造后读取时机仍在其后；Go 包级 `init()` 早于 `main`，安全。

- [ ] **Step 4: 改 config.example.toml** —— 在 `[image]` 块的转换项之后补注释示例：

```toml
# ── 图片类目配置表（可选）────────────────────────────────────────────
# 不配则用内置默认表（龙图/福瑞/老公/老婆/沙雕图/杂鱼/美少女/ba +
# 吃喝玩乐 4合1 + 随机猫猫）。一旦填写则【整体覆盖】默认表。
# kind: single=随机一张(默认) / grid=4合1网格 / external=外链
#   single/grid 需 folder+call；grid 的添加必须带名字；external 需 url。
#   external 的 pick=从返回 JSON 按点路径取图(遇数组随机)，空=响应本身即图。
# [[image.entry]]
# folder = "龙图"
# call   = ["龙图", "龙图攻击"]
# add    = "添加龙图"
# [[image.entry]]
# folder = "吃的"
# call   = ["随机吃的", "吃啥", "吃什么", "来点吃的"]
# add    = "添加吃的"
# kind   = "grid"
# [[image.entry]]
# call = ["随机猫猫", "来点猫猫"]
# kind = "external"
# url  = "http://edgecats.net/"
# [[image.entry]]
# call = ["来只狗"]
# kind = "external"
# url  = "https://api.example.com/dog"
# pick = "data.url"
```

- [ ] **Step 5: 全量编译 + vet + 测试** ——

```bash
go build ./... && go vet ./... && go test ./...
```

预期：编译通过、vet 干净、所有测试 PASS（含 image 包新测）。若有引用残留（旧 Register 名、`random` 包遗留符号），按报错修正。

- [ ] **Step 6: 提交** ——

```bash
git add cmd/bot/main.go plugins/system/help.go config.example.toml
git commit -m "refactor(image): 配置表驱动重构落地——重连 main、删旧、help 由配置生成"
```

---

## 收尾

- 全部 task 完成后跑 `/flightdeck:landing`：把本 spec/plan 标 done、更新 cockpit、分类知识（如 pick 路径语义值得记则入 checklist/incident）、本地 commit（push 先问）。
- 向用户复述行为变更：grid（吃喝玩乐）添加从此必须带名字；旧 hash 文件不动（grid 调用对旧文件仍显示 hash，新加的才有菜名）。
