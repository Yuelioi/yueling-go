---
status: done
summary: 把 nonebot-plugin-zssm 迁移为 Go 插件：保留文本解释核心 + 网页正文抓取 + 图片解释(VL)，砍掉 PDF/审查模型/无头浏览器
last_updated: 2026-06-05
---

# zssm 迁移到 Go 框架

## 背景

`flightdeck/charts/nonebot-plugin-zssm` 是一个 nonebot2 的「AI 解释」插件：对着想了解的内容回复 `zssm`，AI 把复杂概念讲成大白话。现在要把它迁移成 yueling-go 的原生 Go 插件。

源插件能力（Python）：
1. `zssm` 命令，对回复消息或附带内容做 AI 解释
2. 收集「回复消息 + 命令附带」的文本与图片
3. 图片 → 视觉模型(VL, Qwen-VL) 转文字描述（最多 2 张）
4. URL → playwright 无头浏览器抓网页正文；PDF → 解析文本
5. 文本模型(deepseek) 用「防注入 + 随机数标记」system prompt 生成解释，输出 JSON(`output/keyword/block`)
6. 可选审查模型检测 system prompt 泄露
7. reaction 表情表示处理进度

## 范围

**保留**：纯文本解释（核心）、网页正文抓取、图片解释(VL)。

**砍掉**：
- PDF 解析（依赖重、收益窄）
- 审查模型（YAGNI）
- playwright 无头浏览器 → 改用项目已有的轻量 HTTP + HTML 解析
- reaction 进度表情（bot 层无对应 API）
- PIL 图片压缩 → 改简单大小上限

## 框架现状（迁移复用点）

- `b.OnCommand("zssm")` —— 命令注册（见 `bot/matcher.go` CommandMatcher，支持可选 `CmdPrefix`）
- `func(*bot.CommandContext) error` —— handler 签名，含 `ctx.Args`
- `ctx.CollectImageURLs()` (`bot/context.go`) —— 已自动合并「当前消息 + 回复消息」的图片 URL
- `ctx.GetMsg(msgID)` (`bot/api.go`) —— 取回复消息的 `Message`，可 `.Text()` / `.ImageURLs()`
- `ai.NewClient(key, baseURL)` + go-openai —— 一次性 LLM 调用（见 `plugins/tools/translate.go`）
- go-openai 的 `MultiContent`(`ChatMessagePart` + `ChatMessageImageURL`) —— 多模态调用
- `plugins/tools/link_analysis.go` 同包 helpers：`extractURL`、`htmlTitle`、`htmlMeta`、`golang.org/x/net/html`
- `config.C.AI` —— 当前仅单文本模型(deepseek)，**无 VL 配置**

## 设计

### 1. 配置改动 (`config/config.go`)

给 `AIConfig` 加可选 VL 子配置：

```toml
[ai]
deepseek_key = "..."
base_url = "https://api.deepseek.com/v1"
model = "deepseek-chat"

[ai.vl]                                          # 新增，可选
key = "..."
base_url = "https://api.siliconflow.cn/v1"
model = "Qwen/Qwen2.5-VL-72B-Instruct"
```

- 结构：`AIConfig` 增加 `VL VLConfig` 字段；`VLConfig{ Key, BaseURL, Model }`（mapstructure tag）。
- 给 `viper.SetDefault` 补 `ai.vl.base_url`、`ai.vl.model` 默认值。
- 未配置 `ai.vl.key` 时：带图片的 zssm 回复「未配置图片识别」；纯文本/链接仍可用。
- `config.example.toml` 同步加 `[ai.vl]` 注释示例。

### 2. 文件与注册

- 新增 `plugins/tools/zssm.go`，归入既有 `tools` 包（可直接复用 link_analysis 的 HTML helpers，无需导出）。
- 用 `//go:embed prompt.txt` 内嵌系统提示词；`prompt.txt` 从源插件原样拷贝到 `plugins/tools/`。
- 导出 `RegisterZssm(b *bot.Bot)`，在 main 中与其它 `tools.RegisterXxx` 一起调用（注册必须在 `b.Start()` 之前）。

### 3. 处理流程

`b.OnCommand("zssm").Handle(func(ctx *bot.CommandContext) error { ... })`：

```
1. 收集输入
   - 回复消息：取 ReplyID → GetMsg → 文本
   - 命令本身：strings.Join(ctx.Args, " ") 文本
   - 图片：ctx.CollectImageURLs()（已合并 当前+回复）
   - 文本与图片都为空 → ctx.Reply("请回复或输入内容")

2. 图片处理（最多 2 张；仅当配置了 ai.vl.key）
   - 超过 2 张 → 回复「图片数量超过限制, 最多 2 张」
   - 下载图片字节 → base64 data URL（设大小上限，超限跳过/报错）
   - 调 VL 模型（MultiContent：image_url + "请描述这张图片的内容"）→ 文字描述
   - 拼进 user_prompt：<type: image, id: N> {描述} </type: image, id: N>
   - 未配置 ai.vl.key 且存在图片 → 回复「未配置图片识别」

3. 链接处理（取原始文本第一个 http(s) URL）
   - fetchPageText(url)：httpclient GetBytes + 解析 HTML 提取可见正文纯文本（去 script/style，截断到上限）
   - 拼进 user_prompt：<type: web_page, url: U> {正文} </type: web_page>

4. 组装
   - random := 8 位随机数
   - system := embed(prompt.txt) + random
   - user := "<random number: R>\n{...各段}\n</random number: R>"

5. 文本模型生成（ai.NewClient + config.C.AI）
   - CreateChatCompletion(system, user)
   - 清理可能的 ```json 包裹，json.Unmarshal 到 {Output string; Keyword []string; Block bool}
   - block=true → "（抱歉，我现在还不会这个）"
   - 有 keyword → "关键词：a | b\n\n{output}"，否则 output
   - 解析失败重试一次；再失败 → "AI 回复解析失败, 请重试"

6. ctx.Reply(结果)
```

### 4. 新增 helper

- `fetchPageText(url string) (string, error)`：用已引入的 `golang.org/x/net/html` 遍历 DOM，跳过 `<script>/<style>`，收集文本节点拼接并截断到字符上限。link_analysis 现有的只取 title/meta，没有正文提取，故需新写——放在 `zssm.go` 内（同 `tools` 包，无需导出）。

### 5. 系统提示词与输出契约

- `prompt.txt` 原样内嵌，保留「防注入 + 随机数标记 + JSON 输出」的人格设定。
- JSON 输出契约 `{output, keyword, block}` 原样保留——这是 block/keyword 行为的来源。

## 非目标

- 不引入无头浏览器（chromedp 等）。
- 不解析 PDF。
- 不做 system prompt 泄露审查。
- 不实现 reaction 进度表情。
- 不复用 link_analysis 的站点专属卡片逻辑（zssm 走通用正文抓取）。

## 验证

- `go build ./...` 通过。
- 配置/未配置 `ai.vl` 两种情况下，纯文本 / 含链接 / 含图片的 zssm 各跑通一次（手动）。
