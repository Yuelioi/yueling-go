---
kind: note
summary: "Go QQ Bot 中文两列帮助图的渲染方案比较：现有 x/image 两遍布局优先，HTML+chromedp 作为复杂排版升级路线；不选 gofpdf、gg 或 go-chart 充当表格引擎。"
activation: action
read_when: "重构帮助图片、生成中文表格 PNG、选择 Go 文字/表格渲染库，或考虑引入 headless browser 时。"
recheck_when: "帮助图片发展出富文本/多主题/复杂响应式布局，部署镜像已经包含 Chromium，或候选库维护状态明显变化时。"
---

# 中文两列表格帮助图渲染方案调研

调研日期：2026-08-20。外部事实仅采用官方文档、官方仓库及官方源码。

## 结论

**当前项目不应为“两列帮助表格”新增通用图片或表格库。** 最合适的实现是沿用 Go 原生 `image.RGBA` 绘制，把现有代码整理成一个小型、语义明确的“两遍 table layout”：

1. 第一遍按列宽逐 rune 测量和换行，得到每个单元格的行数；行高取同一行两个单元格高度的最大值，再累计画布高度。
2. 第二遍按计算结果画表头、交替行背景、列分隔线和文字。
3. 字体栈从旧 `github.com/golang/freetype/truetype` 迁到项目已经依赖的 `golang.org/x/image/font/opentype`；后者官方定位就是 TTF/OTF 的字形栅格化器，并提供实现 `font.Face` 的高层 API。[官方 `opentype` 文档](https://pkg.go.dev/golang.org/x/image/font/opentype)
4. 输出从 JPEG Q88 改为标准库 `image/png`。Go 标准库直接提供 `png.Encode(io.Writer, image.Image)`，[`image/png` 官方文档](https://pkg.go.dev/image/png)；PNG 规范定义它是无损栅格格式，因此文字边缘和 1 px 表格线不会再引入 JPEG 有损压缩伪影。[W3C PNG 第三版规范](https://www.w3.org/TR/png-3/)

这条路线不是“继续堆坐标常量”，而是把布局抽象为 `Row{Command, Description}` 和可测试的 `Measure -> Layout -> Paint` 三步。它恰好覆盖当前需求，新增运行时依赖为零，部署也不变。

如果未来帮助页发展成富文本卡片、复杂徽章、主题、多字号字体回退或多语言响应式布局，第二选择才是 **HTML/CSS + chromedp 截取目标元素为 PNG**。浏览器原生表格排版最完整，但为当前这张启动时缓存的静态帮助图引入 Chromium，运行和部署成本明显过高。

## 落地状态

本轮已按推荐完成：帮助详情使用结构化命令/说明行、两遍测量与绘制、独立列换行和自动行高；字体改用 `x/image/font/opentype`，输出改为 PNG。实现与回归测试见 [`plugins/system/help_image.go`](../../../plugins/system/help_image.go) 和 [`plugins/system/help_image_test.go`](../../../plugins/system/help_image_test.go)。

## 改造前现状与迁移成本

- 当前帮助渲染固定宽度 720 px，自行创建 `image.RGBA`、测量文本和绘制背景/文字。[`plugins/system/help_image.go`](../../../plugins/system/help_image.go#L26)
- 当前字体来自数据目录的 TTF，通过旧 `golang/freetype/truetype` 解析；文本宽度已使用 `font.MeasureString`，逐 rune 换行也已存在。因此表格真正缺的是稳定的“列/行布局模型”，不是新的字形绘制 API。[字体加载与测量](../../../plugins/system/help_image.go#L105) [中文换行](../../../plugins/system/help_image.go#L193)
- 当前编码是 JPEG、Quality 88。[当前编码](../../../plugins/system/help_image.go#L184)
- `go.mod` 已有 `golang.org/x/image v0.40.0` 和旧 `github.com/golang/freetype`，候选库均未引入。[`go.mod`](../../../go.mod#L10)
- 最终 Docker runtime 是 `alpine:3.20`，只安装 `ca-certificates` 与 `tzdata`，没有 Chromium/Chrome。[`Dockerfile`](../../../Dockerfile#L22)

所以迁移成本从低到高是：

1. 现有 renderer + `x/image/font/opentype` + `image/png`；
2. `fogleman/gg`，但仍需保留自制 CJK 换行和表格布局；
3. HTML + chromedp，需要修改 runtime 镜像并管理浏览器进程；
4. Playwright Go，需要 driver、浏览器下载和版本配套；
5. gofpdf 还要再接 PDF rasterizer 才能得到 PNG。

## 候选方案总表

| 方案 | 中文字体 | 自动换行与行高 | 表格布局 | PNG | 维护状态（2026-08-20） | 依赖重量 | 结论 |
|---|---|---|---|---|---|---|---|
| 现有 renderer + `x/image` | 指定包含中文字形的 TTF/OTF；`opentype` 提供 `font.Face` | 项目已有逐 rune 测量；两遍布局可精确控制 | 自建很小的两列模型 | 标准库直接编码 | `x/image` 为 Go 官方扩展仓库 | 无新增依赖 | **当前推荐** |
| `fogleman/gg` | `LoadFontFace` 读取 TTF | 有行距参数，但内置 `WordWrap` 不适合连续中文 | 无 table primitive，仍要自建 | 直接保存 PNG | 未归档，但仓库 `pushed_at` 为 2023-12-14 | 复用 freetype/x/image，较轻 | 只简化画图调用，不解决核心问题 |
| `wcharczuk/go-chart/v2` | 可注入 `*truetype.Font` | 有逐 rune wrap 和 line spacing | 面向图表，不提供表格组件 | 原生 PNG renderer | 2024-08-23 已归档 | freetype + x/image | 不选 |
| 当前 `codeberg.org/go-pdf/fpdf` | UTF-8 TrueType，可用 Noto Sans SC | `MultiCell` 自动断行并显式传行高 | 可用 cell 手工拼表格 | **只输出 PDF** | 活跃 fork；v0.12.0 发布于 2026-05-18 | 官方称无第三方依赖，但 PNG 需额外 PDF rasterizer | 不选 |
| HTML/CSS + chromedp | `@font-face` 可加载随包字体 | 浏览器原生换行、`overflow-wrap`、`line-height` | 原生 `<table>` / CSS Grid | 元素截图明确输出 PNG | 活跃，`pushed_at` 为 2026-07-14 | Go 依赖中等，另需 Chrome/headless-shell | 复杂排版时推荐 |
| HTML/CSS + Playwright Go | 同浏览器方案 | 同浏览器方案 | 同浏览器方案 | `Page.Screenshot` 返回图片 bytes | 活跃，`pushed_at` 为 2026-08-17 | driver 约 50 MB，另下载浏览器和系统依赖 | 对单一截图过重 |

维护时间来自各项目的官方仓库状态或官方仓库 API：[gg repository API](https://api.github.com/repos/fogleman/gg)、[chromedp repository API](https://api.github.com/repos/chromedp/chromedp)、[Playwright Go repository API](https://api.github.com/repos/mxschmitt/playwright-go)。归档项目的仓库页直接显示归档状态，详见下文。

## 1. gofpdf：不要把 PDF 生成器当 PNG 表格引擎

需要区分三代仓库：

- `github.com/jung-kurt/gofpdf` 是原始仓库，2021-11-13 被归档，README 明示停止维护。[原仓库归档页](https://github.com/jung-kurt/gofpdf)
- 后来的 `github.com/go-pdf/fpdf` 也已于 2025-03-04 归档，README 指向 Codeberg。[GitHub fork 归档页](https://github.com/go-pdf/fpdf)
- 当前延续维护的是 `codeberg.org/go-pdf/fpdf`；Go 官方包索引显示 v0.12.0 于 2026-05-18 发布。[当前包文档与版本](https://pkg.go.dev/codeberg.org/go-pdf/fpdf)

当前 fork 对中文文档生成本身是合格的：官方说明支持 UTF-8 TrueType，并明确建议中日韩使用包含对应字形的专用字体，例如 Noto Sans SC。[UTF-8/CJK 字体说明](https://pkg.go.dev/codeberg.org/go-pdf/fpdf#readme-features) `AddUTF8FontFromBytes` 也允许从静态 bytes 加载字体。[字体 API](https://pkg.go.dev/codeberg.org/go-pdf/fpdf#Fpdf.AddUTF8FontFromBytes)

`MultiCell(w, h, ...)` 会在内容到达右边界时自动换行，`h` 明确表示每行高度，适合做 PDF 单元格。[`MultiCell` 官方文档](https://pkg.go.dev/codeberg.org/go-pdf/fpdf#Fpdf.MultiCell)

但它的输出契约就是 PDF：`Output` 将 PDF document 写入 `io.Writer`。[`Output` 官方文档](https://pkg.go.dev/codeberg.org/go-pdf/fpdf#Fpdf.Output) QQ 帮助需要 PNG 时，还要增加 PDF rasterizer 或外部二进制，字体、抗锯齿和目标像素尺寸又会经历第二次渲染。对只需一张 720 px 两列表格的 Bot，这是方向不匹配，而不是库能力不足。

## 2. fogleman/gg：绘图 API 友好，但不是中文表格布局器

`gg` 是纯 Go 2D 绘图库。README 列出了矩形、圆角矩形、文字测量、`DrawStringWrapped`、`LoadFontFace` 和 `SavePNG`，所以它能把现有低层绘制代码写得更短。[官方 README](https://github.com/fogleman/gg)

字体支持与当前项目本质相同：官方源码读取字体 bytes 后调用 `truetype.Parse` 和 `truetype.NewFace`。[`LoadFontFace` 源码](https://github.com/fogleman/gg/blob/master/util.go#L131-L145) PNG 也是直接调用标准库 `png.Encode`。[`SavePNG` 源码](https://github.com/fogleman/gg/blob/master/util.go#L76-L88)

关键限制是中文换行。`wordWrap` 只用 `unicode.IsSpace` 分段；当某一整段超宽而当前行为空时，会把这段原样加入结果，并不会继续逐 rune 拆分。[`wrap.go` 源码](https://github.com/fogleman/gg/blob/master/wrap.go#L15-L54) 连续中文通常没有空格，因此 README 所说的 word wrap 不能直接满足本任务。虽然 `DrawStringWrapped` 能按 `fontHeight * lineSpacing` 画多行，[绘制与行高源码](https://github.com/fogleman/gg/blob/master/context.go#L749-L774)，调用者仍要先实现 CJK-safe wrap、计算左右单元格高度、再画每行背景和分隔线。

因此引入 `gg` 只会替换 `hFill`/`put` 等绘图表层，不会减少最需要维护的表格布局逻辑。它未被归档，但官方仓库 API 的 `pushed_at` 是 2023-12-14，维护活跃度也不适合作为新核心抽象。[官方仓库 API](https://api.github.com/repos/fogleman/gg)

## 3. go-chart：具备文字和 PNG 原语，但领域不匹配且已归档

这里的 `go-chart` 指 `github.com/wcharczuk/go-chart/v2`。其官方仓库已于 2024-08-23 归档，README 明确建议寻找 fork 或其他图表库。[官方归档说明](https://github.com/wcharczuk/go-chart)

它的 raster renderer 确实创建 `image.RGBA` 并最终 `png.Encode`，也能设置 `*truetype.Font`。[PNG renderer 源码](https://github.com/wcharczuk/go-chart/blob/main/raster_renderer.go#L18-L44) [字体与编码源码](https://github.com/wcharczuk/go-chart/blob/main/raster_renderer.go#L135-L191) 文字工具甚至同时提供按 word 和按 rune 的换行，后一种适合连续中文，并能计算多行间距。[文字换行源码](https://github.com/wcharczuk/go-chart/blob/main/text.go#L35-L126)

但仓库的核心对象是折线、柱状、饼图、坐标轴和 series；没有 table/cell/row 抽象。为帮助表格使用它，需要绕过图表层直接调用 renderer，结果仍是手工表格，却额外绑定到一个已归档的图表模型。其 `go.mod` 依赖也正是项目已经拥有的 freetype 与 x/image。[官方 `go.mod`](https://github.com/wcharczuk/go-chart/blob/main/go.mod)

## 4. HTML/CSS -> screenshot：排版最强，但运行时最重

HTML/CSS 是候选中唯一真正拥有成熟表格布局算法的方案：`table-layout` 控制表格、行和列，自动布局会根据单元格内容计算列宽。[W3C CSS 2.1 表格宽度算法](https://www.w3.org/TR/CSS2/tables.html#width-layout) `overflow-wrap: anywhere` 可以为原本不可断开的字符串增加软换行机会，[W3C CSS Text Level 3](https://www.w3.org/TR/css-text-3/#overflow-wrap-property)；`line-height` 控制行盒计算所用的最小高度，[W3C CSS 2.1 行高定义](https://www.w3.org/TR/CSS2/visudet.html#line-height)；`@font-face` 可声明外部字体资源及其来源，[W3C CSS Fonts Level 4](https://www.w3.org/TR/css-fonts-4/#font-face-rule)。

实现时应把随程序发布的中文字体写成 data URL 或本地受控 URL，等待 `document.fonts.ready` fulfilled 后再截图；规范定义该 Promise 会在文档完成字体加载和布局操作后完成。[W3C CSS Font Loading Level 3](https://www.w3.org/TR/css-font-loading/#font-face-set-ready)

### chromedp

`chromedp.Screenshot` 截取指定元素时明确使用 `CaptureScreenshotFormatPng`，并启用 beyond viewport；`FullScreenshot` 在 quality=100 时也选择 PNG。[官方截图源码](https://github.com/chromedp/chromedp/blob/main/screenshot.go#L25-L115) 官方示例展示了元素截图和整页截图写入 `.png`。[官方 screenshot example](https://github.com/chromedp/examples/blob/main/screenshot/main.go#L14-L58)

Go 侧依赖不算巨大，但仍有 `cdproto`、WebSocket 和若干间接依赖。[官方 `go.mod`](https://github.com/chromedp/chromedp/blob/main/go.mod) 更重要的是运行时必须有 Chrome/Chromium/headless-shell：源码会在各操作系统查找浏览器可执行文件，官方对 headless 环境也建议使用包含 `headless-shell` 的镜像。[浏览器查找源码](https://github.com/chromedp/chromedp/blob/main/allocate.go#L331-L379) [headless 部署说明](https://github.com/chromedp/chromedp#frequently-asked-questions)

若未来确实选择浏览器路线，当前任务只需要 Chromium 和截图，`chromedp` 比 Playwright Go 更贴合。

### Playwright Go

Playwright Go 同样能先 `SetContent(html)` 再 `Page.Screenshot`；截图 API 返回 bytes，并能依据 path/option 决定图片格式。[`SetContent` 与截图源码](https://github.com/mxschmitt/playwright-go/blob/main/page.go#L1139-L1146) [Screenshot 源码](https://github.com/mxschmitt/playwright-go/blob/main/page.go#L1338-L1365)

它适合跨 Chromium、Firefox、WebKit 的自动化，但当前任务用不到跨浏览器能力。官方安装文档要求安装与模块 minor version 匹配的 Playwright driver 和浏览器，还可能安装 OS dependencies。[官方安装说明](https://github.com/mxschmitt/playwright-go#installation) README 进一步说明 Go bridge 自带 Node.js runtime 和 Playwright，约 50 MB，并会下载预编译浏览器。[官方架构说明](https://github.com/mxschmitt/playwright-go#how-does-it-work)

所以 Playwright Go 能生成目标图片，但它的优势与本任务不重合，部署重量却完整保留。

## 推荐实现边界

建议为帮助图只建立以下小接口，不建设“通用报表引擎”：

```go
type HelpRow struct {
    Command     string
    Description string
}

type LaidOutRow struct {
    CommandLines     []string
    DescriptionLines []string
    Height           int
}
```

固定流程：

1. 从结构化帮助元数据产生 `[]HelpRow`，不要再依赖视觉空格对齐。
2. 给命令列固定或受限宽度，说明列使用剩余宽度。
3. 用同一个 `font.Face` 的 `font.MeasureString` 对 rune 序列做宽度约束；对英文优先在空格断行，对连续中文和 URL 回退到 rune 断行。
4. 行高为 `max(len(CommandLines), len(DescriptionLines)) * lineHeight + paddingY*2`。
5. 预计算完整高度后只分配一次 `image.RGBA`；第二遍按布局绘制。
6. 使用 `png.Encode` 输出，并保留启动预渲染缓存。

这层只负责“帮助内容 -> 两列静态图片”，不会泄漏到头像表情、素材网格或其他图片业务。未来若需求跨过以下任一阈值，再重新评估 HTML + chromedp：

- 单元格需要混排多个字体、图标、链接样式或复杂 badge；
- 需要响应式宽度、多主题或大量国际化排版；
- 部署镜像本来就已经稳定携带 Chromium；
- 同一个 HTML 模板还要复用到 Web UI 或打印输出。
