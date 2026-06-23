---
status: active
when_to_read: 新增/修改图片类目（调用或添加命令）、配置 [[image.entry]]、或排查 single/grid/external 行为时
applies_to: [image, config, plugins/image, plugins/quotation, plugins/emoticon, config/config.go, cmd/bot/main.go]
when_to_update: image 配置 schema（字段/kind）、命名策略、或 help 生成方式改变时
last_updated: 2026-06-23
graduate: true
---

# 图片类目配置表（[[image.entry]]）

图片类目（调用命令 + 添加命令 + 文件名策略）由 `config.C.Image.Entry`（TOML `[[image.entry]]`）驱动，在 `plugins/image` 一处注册。语录、表情因检索逻辑特殊，抽成独立插件。

## 三种 kind

| kind | 调用方式 | 添加 / 文件名 | 默认例子 |
|---|---|---|---|
| `single`（缺省） | 随机发一张 | hash 命名，忽略参数 | 龙图/福瑞/老公/老婆/沙雕图/杂鱼/美少女/ba |
| `grid` | 挑 4 张拼 2×2 网格 + 按文件名列菜单 | **名字命名（文件名=参数），名字必填** | 吃的/喝的/玩的/水果 |
| `external` | 发外链 URL（可从 JSON 取图） | 无文件 | 猫猫 |

## 配置 schema（`config/config.go`）

```go
type ImageEntry struct {
    Folder string   // 素材子目录；external 可空
    Call   []string // 调用命令（FullMatch）
    Add    string   // 添加命令（OnCommand）；external/无添加可空
    Kind   Kind     // single(默认) / grid / external
    URL    string   // 仅 external：请求地址
    Pick   string   // 仅 external：JSON 取图点路径；空=响应本身即图
}
```

`config.C.Image.Entry` 为空时用 `plugins/image/entries.go` 的 `defaultEntries`（照搬重构前行为）；**非空即整体覆盖**默认表（不 merge）。`plugins/image/image.go` 的 `Register` 在启动时 `validateEntries` 校验（kind 合法、single/grid 需 folder+call、grid 需 add、external 需 url、命令全局不重复），非法即 `logx.Fatalf`。

TOML 示例见 `config.example.toml` 的 `[image]` 段注释。

## external 的 `pick` 路径求值

`pick` 为点分隔路径，左到右求值，**遇数组自动随机抽一个**（`plugins/image/external.go` 的 `ExtractImageURL`，纯函数，有单测）：

| 响应 JSON | `pick` | 结果 |
|---|---|---|
| `{"data":["u1","u2"]}` | `data` | 随机一个 |
| `{"data":{"url":"x"}}` | `data.url` | `x` |
| `{"data":[{"url":"a"}]}` | `data.url` | 先随机取项 → `.url` |
| 直接返回图片字节 | 空 | URL 本身即图 |

## 命名策略

通用上传内核 `image.Upload(ctx, folder, nameFn)`（`plugins/image/upload.go`）下载/去重(sha256)/转JPEG/探测扩展名，文件名由 `nameFn(hash, arg, gid)` 决定：

- single → `nameByHash`：`{hash}`
- grid → `nameByArg`：`{名字}_{hash}`（名字必填即菜名，hash 保证去重 + 同名不同图不互相覆盖；4合1 网格显示时 `displayLabel` 去掉 `_{hash}` 后缀只显示名字）
- quotation → `{gid}_{arg}_{hash}`（群隔离，`语录 [昵称]` 按群+昵称检索，白名单 玉米/甜甜 跨群）
- emoticon → `{arg}_{hash}`（`添加表情 [关键词]`，空格触发关键词检索）

## 包结构

- `plugins/image/`：配置表驱动的 single/grid/external（调用+添加），`grid.go` 渲染网格，`external.go` 取图，`upload.go` 上传内核，`entries.go` 默认表+校验+命名，`help.go` 生成帮助。
- `plugins/quotation/`、`plugins/emoticon/`：各自独立，复用 `image.Upload`。
- 注册顺序：`cmd/bot/main.go` 中 `image.Register(b)` 必须早于 `system.RegisterHelp(b)`——后者 `finalizeRegistry()` 据 `image.activeEntries` 生成 help #18/#32 的用法与命令清单（满足「改命令同步 help」铁律，加类目只改配置）。

## 注意

- 加图片类目 = 改配置（或改 `defaultEntries`），help 自动跟随，无需手改 `help.go`。
- grid（吃喝玩乐）添加必须带名字；重构前存量文件是 hash 命名，grid 调用对旧文件仍显示 hash（无意义），新加的才有菜名——旧文件不自动清理。
