---
status: active
summary: 回复一条消息发 pack，把该消息（含递归展开的合并转发）里的所有图片下载打成 zip 上传到群文件；新增独立插件 plugins/tools/pack.go + bot.GetForwardMsg API 封装
last_updated: 2026-06-15
---

# pack — 打包消息里的图片为 zip

## 背景 / 目标

群友常对一条「图很多」的消息（尤其合并转发）想一次性拿到所有图。当前只有
`添加X`（按分类入库，复用 `CollectImageURLs`，不处理合并转发）和 `zssm`（取当前+被回复
消息的图，同样不处理合并转发）。两者都不能把「一组图」整体下发，也都看不进合并转发内部。

目标：用户**回复**一条目标消息并发 `pack`，bot 把该消息里的一组图片打成一个 zip 上传到群
文件。目标消息可能是普通图文消息，也可能是**合并转发**（可嵌套多层）。

成功标准：

- 回复一条带图的消息发 `pack` → 群文件里出现一个含这些图的 zip。
- 被回复的是合并转发时，**递归**抽干所有层每条消息的图，全部进同一个 zip。
- 无图 / 全部下载失败 → 明确文字提示，不静默。
- 对超大合并转发有上限保护，不会把进程拖死。

## 触发 & 取图

- `b.OnCommand("pack").Handle(func(ctx *bot.CommandContext) error)`，群聊。
- 取图范围（去重后合并）：
  1. **pack 当条消息自带的图**（用户可在发 pack 时直接附图）。
  2. **被回复消息**：`ctx.Message().ReplyID()` → `ctx.GetMsg(mid)` → 递归收图。
- 递归收图（`collectImages`）遍历一条 `bot.Message` 的段：
  - `image` 段 → 收其 url（复用 `Message.ImageURLs()` 的取值逻辑：优先 `url`，回退 `file`）。
  - `forward` 段 → 取其 `data.id` → `GetForwardMsg(id)` → 对返回的每条消息再递归。
  - **防爆**：`depth ≤ 5`；维护 `visited map[string]bool` 记录已展开的 forward id，命中即跳过（防嵌套循环）。

## 新增 API

`bot/api.go`：

```go
func (a *BotAPI) GetForwardMsg(id string) ([]Message, error)
```

封装 `get_forward_msg`，解析返回的 `messages` 数组，每条取其 `message` 字段（segments），
返回 `[]Message`。解析要对 NapCat 的返回结构容错（messages 缺失时返回空切片而非报错）。

## 打包 & 下发

- 逐张 `httpclient.Direct.GetBytes(url)` 下载；`detectImageExt(data)` 定扩展名
  （该函数现在 `plugins/system/image.go`，pack 在另一个包，需要一份可复用的实现——
  见「实现注意」）。
- zip 内文件按收集顺序命名：`001.jpg`、`002.png` …（3 位补零 + 探测出的扩展名）。
- 用 `archive/zip` 写到本地临时文件：`services.DataPath("tmp")` 下
  `pack_<groupID>_<unixts>.zip`（目录不存在则 `os.MkdirAll`）。
- `UploadGroupFile(groupID, zipPath, "图片打包_<时间>.zip", "")` 上传到群文件根目录
  （复用已有 API；沿用 `files.go` 的本地路径约定——部署里 NapCat 能读到 bot 的
  `data/` 路径）。
- 上传成功后删临时 zip（`defer os.Remove`）。回复「已打包 N 张图片」。

## 边界 & 防滥用

- 无图 → 回复「未找到可打包的图片」。
- 上限常量：最多 **100 张**、累计下载 ≤ **100MB**；达到上限即停止收集/下载并在回复里提示「已达上限，仅打包前 N 张」。
- 单张下载失败 → 跳过并计数，不中断整体；全部失败 → 回复「图片下载失败」。
- 日志统一走 `services/logx`（项目铁律），前缀 `[pack]`。

## 实现注意

- `detectImageExt` 目前是 `plugins/system/image.go` 的包内私有函数。pack 在 `plugins/tools`，
  不能直接引用。处理：在 pack.go 内放一份等价实现（与现有逻辑一致：png/gif/webp/默认 jpg）。
  不为这点小重复去动 system 包（避免无关重构）。
- 高风险无（只读消息 + 上传文件），无需 `ConfirmRequired` / `AdminOnly`。
- 所有注册在 `b.Start()` 前，通过插件的 `Register(b)` 完成。

## 不做（YAGNI）

- 自定义 zip 文件名 / 密码 / 按分类归档。
- 私聊场景（仅群聊）。
- 把 zip 同时存进本地图库。

## 测试

- `GetForwardMsg` 返回解析：给定一段 NapCat 风格 `messages` JSON，断言解出的 `[]Message`
  段正确（含 image / 嵌套 forward）。
- `collectImages` 递归：构造「forward 里套 forward 里有图」的桩，断言递归抽图 + depth/visited
  防循环（mock `GetForwardMsg`）。
- 扩展名探测：复用现有 `detectImageExt` 的判定（png/gif/webp/jpg）。
- 端到端上传链路依赖 NapCat，手动验证。
