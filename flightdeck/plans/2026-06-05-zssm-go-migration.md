---
status: done
summary: 实现 plugins/tools/zssm.go：VL 配置、prompt 内嵌、网页正文抓取、图片描述、JSON 输出解析、命令注册
last_updated: 2026-06-05
implements: specs/2026-06-05-zssm-go-migration.md
---

# zssm 迁移到 Go 框架 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 nonebot-plugin-zssm 迁移为 yueling-go 的原生 Go 插件 `plugins/tools/zssm.go`：`zssm` 命令对回复内容/链接/图片做 AI 解释。

**Architecture:** 命令插件归入既有 `tools` 包，复用 `link_analysis.go` 同包 HTML helpers 与 `ai.NewClient`；新增可选 `ai.vl` 视觉模型配置；网页用轻量 HTTP+HTML 解析（不用无头浏览器），图片下载转 base64 走 VL 模型，文本走 deepseek 并解析 `{output,keyword,block}` JSON。

**Tech Stack:** Go 1.25, `github.com/sashabaranov/go-openai`, `golang.org/x/net/html`, `spf13/viper`, 项目内 `bot` / `ai` / `config` / `services/httpclient` 包。

**Conventions:** 项目铁律——不写注释（除非 WHY 不明显）；所有 `Register*` 必须在 `b.Start()` 前调用；handler 签名为四种之一。项目此前无测试文件，本计划仅对**纯函数** helper 写单元测试，命令 handler 靠 `go build` + 手动验证。

---

### Task 1: 新增 VL 视觉模型配置

**Files:**
- Modify: `config/config.go`（`AIConfig` 结构 + `Load` 默认值）
- Modify: `config.example.toml`（加 `[ai.vl]` 示例）

- [ ] **Step 1: 给 AIConfig 增加 VL 子配置**

修改 `config/config.go` 的 `AIConfig` 结构体（当前在第 34-38 行），改为：

```go
type AIConfig struct {
	DeepSeekKey string   `mapstructure:"deepseek_key"`
	BaseURL     string   `mapstructure:"base_url"`
	Model       string   `mapstructure:"model"`
	VL          VLConfig `mapstructure:"vl"`
}

type VLConfig struct {
	Key     string `mapstructure:"key"`
	BaseURL string `mapstructure:"base_url"`
	Model   string `mapstructure:"model"`
}
```

- [ ] **Step 2: 补 VL 默认值**

在 `config/config.go` 的 `Load` 函数里，紧跟现有 `viper.SetDefault("ai.base_url", ...)` 之后加两行：

```go
	viper.SetDefault("ai.vl.base_url", "https://api.siliconflow.cn/v1")
	viper.SetDefault("ai.vl.model", "Qwen/Qwen2.5-VL-72B-Instruct")
```

（不给 `ai.vl.key` 设默认值——空 key 表示未启用图片识别，是合法状态，`validate()` 不校验它。）

- [ ] **Step 3: config.example.toml 加示例**

在 `config.example.toml` 的 `[ai]` 段之后追加：

```toml
# 图片识别（可选）。不配 key 则 zssm 遇到图片回复「未配置图片识别」。
[ai.vl]
key = ""
base_url = "https://api.siliconflow.cn/v1"
model = "Qwen/Qwen2.5-VL-72B-Instruct"
```

- [ ] **Step 4: 验证编译**

Run: `go build ./...`
Expected: 通过，无报错。

- [ ] **Step 5: Commit**

```bash
git add config/config.go config.example.toml
git commit -m "feat(config): add optional ai.vl vision model config"
```

---

### Task 2: 拷贝并内嵌 system prompt

**Files:**
- Create: `plugins/tools/zssm_prompt.txt`（从源插件原样拷贝）
- Create: `plugins/tools/zssm.go`（先只放 embed 与包声明）

- [ ] **Step 1: 拷贝 prompt.txt**

把 `flightdeck/charts/nonebot-plugin-zssm/nonebot_plugin_zssm/prompt.txt` 的内容**原样**写入 `plugins/tools/zssm_prompt.txt`（内容不改一字，末尾保留它以「当前随机数为：」结尾的那行）。

- [ ] **Step 2: 创建 zssm.go 骨架 + embed**

Create `plugins/tools/zssm.go`：

```go
package tools

import (
	_ "embed"
)

//go:embed zssm_prompt.txt
var zssmSystemPrompt string
```

- [ ] **Step 3: 验证编译**

Run: `go build ./...`
Expected: 通过（包级 var 即使未使用也不报 unused，安全）。

- [ ] **Step 4: Commit**

```bash
git add plugins/tools/zssm_prompt.txt plugins/tools/zssm.go
git commit -m "feat(zssm): embed system prompt"
```

---

### Task 3: 网页正文抓取 helper（TDD）

**Files:**
- Modify: `plugins/tools/zssm.go`
- Test: `plugins/tools/zssm_test.go`

- [ ] **Step 1: 写失败测试**

Create `plugins/tools/zssm_test.go`：

```go
package tools

import (
	"strings"
	"testing"
)

func TestExtractVisibleText(t *testing.T) {
	htmlDoc := `<html><head><title>T</title>
<style>.x{color:red}</style><script>var a=1;</script></head>
<body><h1>标题</h1><p>正文内容</p></body></html>`
	got := extractVisibleText([]byte(htmlDoc))
	if !strings.Contains(got, "标题") || !strings.Contains(got, "正文内容") {
		t.Fatalf("缺正文: %q", got)
	}
	if strings.Contains(got, "color:red") || strings.Contains(got, "var a=1") {
		t.Fatalf("未去除 script/style: %q", got)
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./plugins/tools/ -run TestExtractVisibleText -v`
Expected: 编译失败 / FAIL —— `undefined: extractVisibleText`。

- [ ] **Step 3: 实现 extractVisibleText 与 fetchPageText**

在 `plugins/tools/zssm.go` 追加（imports 增加 `bytes`、`strings`、`golang.org/x/net/html`、`github.com/Yuelioi/yueling-go/services/httpclient`）：

```go
const zssmMaxPageChars = 8000

func extractVisibleText(body []byte) string {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return ""
	}
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "noscript") {
			return
		}
		if n.Type == html.TextNode {
			if t := strings.TrimSpace(n.Data); t != "" {
				sb.WriteString(t)
				sb.WriteByte('\n')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	out := sb.String()
	if len([]rune(out)) > zssmMaxPageChars {
		out = string([]rune(out)[:zssmMaxPageChars])
	}
	return out
}

func fetchPageText(url string) (string, error) {
	body, err := httpclient.Direct.GetBytes(url, "User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	if err != nil {
		return "", err
	}
	return extractVisibleText(body), nil
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./plugins/tools/ -run TestExtractVisibleText -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add plugins/tools/zssm.go plugins/tools/zssm_test.go
git commit -m "feat(zssm): add web page text extraction"
```

---

### Task 4: AI 响应解析与格式化（TDD）

**Files:**
- Modify: `plugins/tools/zssm.go`
- Test: `plugins/tools/zssm_test.go`

- [ ] **Step 1: 写失败测试**

在 `plugins/tools/zssm_test.go` 追加：

```go
func TestFormatZssmResponse(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{`{"output":"地球是圆的","keyword":["地球"],"block":false}`, "关键词：地球\n\n地球是圆的"},
		{"```json\n{\"output\":\"x\",\"keyword\":[],\"block\":false}\n```", "x"},
		{`{"output":"","keyword":[],"block":true}`, "（抱歉, 我现在还不会这个）"},
	}
	for _, c := range cases {
		got, err := formatZssmResponse(c.raw)
		if err != nil {
			t.Fatalf("raw=%q err=%v", c.raw, err)
		}
		if got != c.want {
			t.Fatalf("raw=%q got=%q want=%q", c.raw, got, c.want)
		}
	}
	if _, err := formatZssmResponse("not json"); err == nil {
		t.Fatalf("非 JSON 应当报错")
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./plugins/tools/ -run TestFormatZssmResponse -v`
Expected: FAIL —— `undefined: formatZssmResponse`。

- [ ] **Step 3: 实现 formatZssmResponse**

在 `plugins/tools/zssm.go` 追加（imports 增加 `encoding/json`、`fmt`、`regexp`）：

```go
var reCodeFence = regexp.MustCompile("(?s)^```[a-zA-Z]*\\s*|\\s*```$")

type zssmOutput struct {
	Output  string   `json:"output"`
	Keyword []string `json:"keyword"`
	Block   bool     `json:"block"`
}

func formatZssmResponse(raw string) (string, error) {
	data := reCodeFence.ReplaceAllString(strings.TrimSpace(raw), "")
	var out zssmOutput
	if err := json.Unmarshal([]byte(data), &out); err != nil {
		return "", err
	}
	if out.Block {
		return "（抱歉, 我现在还不会这个）", nil
	}
	if out.Output == "" {
		return "（AI回复内容异常，请重试）", nil
	}
	if len(out.Keyword) > 0 {
		return fmt.Sprintf("关键词：%s\n\n%s", strings.Join(out.Keyword, " | "), out.Output), nil
	}
	return out.Output, nil
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./plugins/tools/ -run TestFormatZssmResponse -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add plugins/tools/zssm.go plugins/tools/zssm_test.go
git commit -m "feat(zssm): parse and format AI JSON output"
```

---

### Task 5: 图片描述 helper（VL 模型）

**Files:**
- Modify: `plugins/tools/zssm.go`

- [ ] **Step 1: 实现图片下载转 base64**

在 `plugins/tools/zssm.go` 追加（imports 增加 `encoding/base64`、`net/http`）：

```go
const zssmMaxImageBytes = 8 * 1024 * 1024

func imageToDataURL(url string) (string, error) {
	body, err := httpclient.Direct.GetBytes(url)
	if err != nil {
		return "", err
	}
	if len(body) > zssmMaxImageBytes {
		return "", fmt.Errorf("图片过大")
	}
	mime := http.DetectContentType(body)
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(body), nil
}
```

- [ ] **Step 2: 实现 describeImage（调 VL 模型）**

继续在 `plugins/tools/zssm.go` 追加（imports 增加 `context`、`github.com/Yuelioi/yueling-go/ai`、`github.com/Yuelioi/yueling-go/config`、openai `github.com/sashabaranov/go-openai`）：

```go
func describeImage(url string) (string, error) {
	vl := config.C.AI.VL
	dataURL, err := imageToDataURL(url)
	if err != nil {
		return "", err
	}
	client := ai.NewClient(vl.Key, vl.BaseURL)
	resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: vl.Model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{Type: openai.ChatMessagePartTypeImageURL, ImageURL: &openai.ChatMessageImageURL{URL: dataURL}},
					{Type: openai.ChatMessagePartTypeText, Text: "请你作为文本模型的眼睛，告诉她这张图片的内容"},
				},
			},
		},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("VL 无响应")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}
```

- [ ] **Step 3: 验证编译**

Run: `go build ./...`
Expected: 通过。若报 `ChatMessagePartTypeImageURL` 等常量名不符，按 go-openai 实际 API 调整（`go doc github.com/sashabaranov/go-openai ChatMessagePartType` 确认；该库这些常量与 `ChatMessageImageURL` 类型在 v1.20+ 均存在）。

- [ ] **Step 4: Commit**

```bash
git add plugins/tools/zssm.go
git commit -m "feat(zssm): add VL image description"
```

---

### Task 6: 命令 handler 与注册

**Files:**
- Modify: `plugins/tools/zssm.go`（加 `RegisterZssm`）
- Modify: `cmd/bot/main.go`（调用注册）

- [ ] **Step 1: 实现 RegisterZssm**

在 `plugins/tools/zssm.go` 追加（imports 增加 `math/rand`、`github.com/Yuelioi/yueling-go/bot`）：

```go
const zssmMaxImages = 2

func RegisterZssm(b *bot.Bot) {
	b.OnCommand("zssm").Handle(func(ctx *bot.CommandContext) error {
		var userPrompt strings.Builder
		var rawInput strings.Builder

		if replyID, ok := ctx.Message().ReplyID(); ok {
			var mid int32
			fmt.Sscan(replyID, &mid)
			if mid != 0 {
				if replied, err := ctx.GetMsg(mid); err == nil {
					t := replied.Text()
					userPrompt.WriteString("<type: text>\n" + t + "\n</type: text>")
					rawInput.WriteString(t)
				}
			}
		}

		argText := strings.TrimSpace(strings.Join(ctx.Args, " "))
		if argText != "" {
			userPrompt.WriteString("<type: interest>\n" + argText + "\n</type: interest>")
			rawInput.WriteString(" " + argText)
		}

		images := ctx.CollectImageURLs()

		if userPrompt.Len() == 0 && len(images) == 0 {
			return ctx.Reply("请回复或输入内容")
		}

		if len(images) > 0 {
			if config.C.AI.VL.Key == "" {
				return ctx.Reply("未配置图片识别")
			}
			if len(images) > zssmMaxImages {
				return ctx.Reply("图片数量超过限制, 最多 2 张")
			}
			for i, u := range images {
				desc, err := describeImage(u)
				if err != nil {
					return ctx.Reply("图片识别失败")
				}
				userPrompt.WriteString(fmt.Sprintf("\n<type: image, id: %d>\n%s\n</type: image, id: %d>", i, desc, i))
			}
		}

		if u := extractURL(rawInput.String()); u != "" {
			if page, err := fetchPageText(u); err == nil && page != "" {
				userPrompt.WriteString(fmt.Sprintf("\n<type: web_page, url: %s>\n%s\n</type: web_page>", u, page))
			}
		}

		randomNumber := fmt.Sprintf("%08d", rand.Intn(100000000))
		systemPrompt := zssmSystemPrompt + randomNumber
		finalUser := fmt.Sprintf("<random number: %s>\n%s\n</random number: %s>", randomNumber, userPrompt.String(), randomNumber)

		result, err := zssmGenerate(systemPrompt, finalUser)
		if err != nil {
			result, err = zssmGenerate(systemPrompt, finalUser)
		}
		if err != nil {
			return ctx.Reply("AI 回复解析失败, 请重试")
		}
		return ctx.Reply(result)
	})
}

func zssmGenerate(systemPrompt, userPrompt string) (string, error) {
	cfg := config.C.AI
	client := ai.NewClient(cfg.DeepSeekKey, cfg.BaseURL)
	resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: cfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("无响应")
	}
	return formatZssmResponse(resp.Choices[0].Message.Content)
}
```

注：`extractURL` 已存在于同包 `link_analysis.go`（`plugins/tools/link_analysis.go:563`），直接调用，**不要**重复定义。

- [ ] **Step 2: 在 main.go 注册**

修改 `cmd/bot/main.go`，在 `// ── Tools ──` 段（约第 122-125 行），在 `tools.RegisterSearchAE(b)` 之后加一行：

```go
	tools.RegisterZssm(b)
```

- [ ] **Step 3: 验证编译**

Run: `go build ./...`
Expected: 通过。

- [ ] **Step 4: 跑全部单测确保没回归**

Run: `go test ./plugins/tools/ -v`
Expected: `TestExtractVisibleText`、`TestFormatZssmResponse` 均 PASS。

- [ ] **Step 5: Commit**

```bash
git add plugins/tools/zssm.go cmd/bot/main.go
git commit -m "feat(zssm): wire zssm command handler and register in main"
```

---

### Task 7: 手动验证

**Files:** 无（运行验证）

- [ ] **Step 1: 最终编译 + 全量测试**

Run: `go build ./... && go test ./...`
Expected: 编译通过；测试全过（其它包本就无测试，`no test files` 属正常）。

- [ ] **Step 2: 启动并实测**

Run: `go run ./cmd/bot/`（需有效的 `config.toml` 与 NapCat 连接）

在群里验证三种场景：
1. 回复一段纯文本消息发 `zssm` → 返回解释。
2. 回复/附带一个网页链接发 `zssm` → 返回结合网页内容的解释。
3. （配了 `ai.vl.key`）回复一张图片发 `zssm` → 返回图片解释；未配 key 时返回「未配置图片识别」。

- [ ] **Step 3: 更新看板并 landing**

实现完成后通过 `/flightdeck:landing` 收尾（更新 cockpit、提交）。本计划 `status` 在 landing 时置 `done`。

---

## 验证清单（self-review 对照 spec）

- [x] VL 配置 → Task 1
- [x] prompt 内嵌 → Task 2
- [x] 网页正文抓取 → Task 3
- [x] JSON 输出契约解析 → Task 4
- [x] 图片解释(VL) → Task 5
- [x] 命令 handler + 收集回复/参数/图片 + 注册 → Task 6
- [x] 手动验证三场景 → Task 7
- [x] 非目标（PDF/审查/无头浏览器/reaction）均未出现在任务中
