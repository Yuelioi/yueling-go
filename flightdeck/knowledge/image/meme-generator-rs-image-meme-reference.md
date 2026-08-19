---
kind: note
summary: "meme-generator-rs 的模板模式、图片处理与 GIF 管线调研，以及 yueling-go 自有 Go 头像表情插件和共享 imaging 模块方案。"
activation: action
read_when: "新增头像表情包、无状态图片处理命令、roll GIF 随机抽帧，或扩展 services/meme 客户端协议时。"
recheck_when: "本地模板接口、素材布局、共享 imaging API、图片来源优先级、GIF 资源限制或参考上游版本改变时。"
---

# meme-generator-rs 图片处理与头像表情包参考

## 研究范围与结论

本文研究日期为 2026-08-19，上游源码固定在 `MemeCrafters/meme-generator-rs@067da9fef80a6364018b7fd682622b655e2cddae`，避免 `main` 后续变化使结论失效。

结论先行：

1. **新增头像表情内容做成 yueling-go 自己的内置 Go 插件。** 参考上游的模板元数据、显式注册、同名素材目录和逐帧渲染模式，但模板、素材、解码与编码都在 Go 进程内，不修改 Rust 仓库，也不依赖它的动态扩展机制或 HTTP 服务。
2. **无状态图片处理不要塞进现有 `plugins/image`。** 该包现在承担的是图片素材库、上传、随机抽取、网格与外链类目；翻转、镜像、旋转等是“输入一张图、即时生成输出”的另一种职责，建议新建 `plugins/imageops/`。
3. **建立纯 Go 的 `services/imaging` 共享层。** 头像模板、图片处理命令和 `roll` 抽帧共用解码、完整 GIF 显示帧合成、缩放/裁剪/蒙版/叠图、时间轴对齐与 PNG/GIF 编码；上游 `/tools/image_operations/*` 仅作为行为参考或可选的既有远端能力，不是本地头像插件的依赖。
4. **`roll` 的 GIF 随机抽帧留在 `plugins/random/roll.go` 做命令分流，抽帧细节下沉到独立 service。** 不要把 GIF disposal 合成和资源限制写死在 handler；上游同名 `roll` 模板是“让头像滚动”的表情包，并不是随机抽帧。
5. **本地插件拥有自己的模板注册表和元数据。** 不扩充或复用 `services/meme.MemeInfo`；本地 `TemplateSpec` 负责 key、命令词、图片/文字数量、默认文字、typed options 和帮助信息，避免本地模板协议被远端服务 DTO 绑住。

## 推荐放置位置

建议的职责边界如下；这是设计建议，不表示这些文件已经存在。

```text
plugins/
├── avatar_meme/                 # 新增：yueling-go 自有头像表情插件（纯 Go）
│   ├── plugin.go                # Register、命令注册、catalog 绑定
│   ├── handler.go               # 图片/头像/文字/options 收集，调用本地 engine
│   ├── registry.go              # 显式组装模板；检查 key/命令词冲突
│   └── internal/
│       ├── engine/              # Template/Spec/Request/Animation、校验、统一编码
│       └── templates/
│           ├── register.go      # All() 显式返回全部内置模板
│           ├── coupon/          # 单头像静图示例
│           │   ├── template.go
│           │   └── assets/...
│           ├── petpet/          # 单头像 GIF 示例
│           │   ├── template.go
│           │   └── assets/0.png ...
│           └── rub/             # 多头像 GIF 示例
│               ├── template.go
│               └── assets/0.png ...
├── funny/
│   └── memes.go                 # 现有远端 meme 插件；与 avatarmeme 相互独立
├── image/                       # 保留：素材库、随机图、网格、external；不放图像变换
├── imageops/                    # 新增：翻转/镜像/旋转/滤镜/GIF 工具的命令层
│   ├── plugin.go                # 注册与 help 元数据
│   ├── basic.go                 # 翻转、镜像、旋转、缩放、裁剪、灰度、反色
│   └── gif.go                   # 倒放、变速、拆帧等命令（第二阶段）
├── internal/
│   └── imageinput/              # 可选：附件/引用/@头像/自己头像的统一解析策略
└── random/
    └── roll.go                  # 保留：检测图片模式，否则沿用数字/选项 roll

services/
├── imaging/                     # 新增：纯 Go、无 Bot 语义的共享图像内核
│   ├── source.go                # Source、Frame、Animation 与资源限额
│   ├── decode.go                # PNG/JPEG/WebP/GIF 解码；GIF disposal 合成
│   ├── encode.go                # 单帧 PNG、多帧 GIF
│   ├── transform.go             # 缩放、裁剪、翻转、旋转、圆形/蒙版
│   ├── composite.go             # 画布、定位、图层叠加
│   └── timeline.go              # 动图取样和固定模板/动态头像对齐
└── meme/                        # 现有远端客户端；本地 avatar_meme 不导入它
```

边界理由：

- `plugins/*` 只处理 Bot 语义：命令匹配、图片来源优先级、参数提示、发送结果。
- `services/imaging` 只处理图像数据和资源限制，不知道 QQ、命令、模板关键词或远端 meme server。
- `plugins/avatar_meme/internal/engine` 负责模板领域语义；它依赖 `services/imaging`，但不能依赖 `services/meme`。
- `services/meme` 继续只服务现有 `plugins/funny/memes.go`。两个插件可以同时存在；本地模板通过独立 catalog ID 和独立注册函数启动，远端服务不可用时也应照常工作。
- `plugins/image` 的现有知识契约是“配置表驱动的素材类目”，与无状态变换的生命周期、权限、错误与资源成本都不同；合并只会让包名看似相同、内部职责却更散。

启动注册仍须遵守本仓库铁律：所有注册发生在 `b.Start()` 之前，并为新命令补 `plugins/catalog` 与 `plugins/system/help.go`。

## 上游模块组织

上游使用 Rust workspace 分层：`meme_generator_core` 定义模板接口和元数据，`meme_generator_utils` 提供图片/GIF/文字/Builder，`meme_generator_memes` 放内置模板，`meme_generator` 提供门面和通用工具，server/node/py 则是不同绑定层。[workspace 成员](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/Cargo.toml#L1-L12)

内置模板基本采用“一个模板一个 `.rs`”的组织方式，集中在 `meme_generator_memes/src/memes/`；`register_meme!` 将模板声明提交到 inventory，启动时统一注册。[模板注册宏与收集](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_memes/src/registry.rs#L3-L36)

模板素材与代码分离，默认从 `MEME_HOME/resources/images` 读取；例如 `petpet` 使用 `resources/images/petpet/{0..4}.png`。上游还有资源清单、哈希校验和并发下载，说明二进制素材与模板代码是不同生命周期。[图片目录配置](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_utils/src/config.rs#L82-L91) [资源下载与哈希校验](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator/src/resources.rs#L80-L143)

对 yueling-go 来说，应借鉴这种**职责划分**，不复制 Rust 多 crate、inventory 或动态库 ABI。Go 版本使用编译期显式注册和 `go:embed`：素材随二进制发布，不在运行时下载，也不要求部署 Rust 服务。

## 模板、参数和输入输出抽象

## yueling-go 自有 Go 插件方案

本地扩展采用 `plugins/avatar_meme`，它是与 `plugins/funny/memes.go` 并列的独立插件。前者包含 yueling-go 自己维护的 Go 模板和素材；后者仍是现有 Rust HTTP 服务的客户端。`avatar_meme` 不导入 `services/meme`，不读取 `MemeInfo`，不受 `MemeServer` 是否可用影响。

图片来源解析放到 `plugins/internal/imageinput`：统一执行“当前/引用图片 > @头像 > 发送者头像”的策略、安全下载和数量截断，供 `avatar_meme`、`imageops` 和 `random/roll` 复用。它只返回图片 bytes、来源名称等 Bot 输入，不承担解码或渲染。

模板最小契约建议为：

```go
type Template interface {
    Spec() TemplateSpec
    Render(context.Context, RenderRequest) (imaging.Animation, error)
}

type TemplateSpec struct {
    Key, Description string
    Keywords         []string
    MinImages, MaxImages int
    MinTexts, MaxTexts   int
    DefaultTexts     []string
    Options          []OptionSpec
}

type RenderRequest struct {
    Images  []imaging.Source
    Texts   []string
    Options map[string]any
}
```

`engine` 在调用 `Render` 前统一检查图片数、文字数、option 类型/default/choices/min/max，并在调用后统一编码：一帧输出 PNG，多帧输出 GIF。模板只返回完整 RGBA 帧和逐帧 delay，不自行处理 QQ 消息、网络下载或输出文件。这样模板实现只关注裁剪、缩放、定位、蒙版、叠层和文字绘制。

`services/imaging` 是共享的纯 Go 图像内核：解码后把 GIF disposal 应用成完整显示帧；提供缩放、cover/contain、裁方/裁圆、翻转、旋转、蒙版和叠图；按时间采样动态输入；最后编码 PNG/GIF。固定模板动画用 `FrameCount + Delay + RenderFrame(index, sampledInputs)` 表达，动态头像按输出时间循环或保持首/尾帧，不让每个模板重复实现时间轴。

每个模板使用独立子包和同名素材目录，并用 `go:embed` 随程序发布：

```text
plugins/avatar_meme/internal/templates/
├── register.go                 # 显式 All()；无 init/inventory 魔法
├── coupon/
│   ├── template.go             # 单头像静图 + 可选文字
│   └── assets/0.png
├── petpet/
│   ├── template.go             # 单头像固定帧 GIF + circle option
│   └── assets/0.png ... 4.png
└── rub/
    ├── template.go             # 双头像固定帧 GIF
    └── assets/0.png ... 5.png
```

`registry.go` 显式组装 `templates.All()`，启动时拒绝重复 key/keyword。命令注册延续当前仓库惯例：新增独立 `catalog.PluginAvatarMeme`，在 `b.Start()` 和 `system.RegisterHelp` 前调用 `avatar_meme.Register(b)`。如果本地关键词可能和远端模板重名，应使用本地独有关键词或统一的 `头像表情 <模板>` 入口，不能靠注册顺序隐式覆盖。

上游代表模板可直接转译为四类 Go 写法：

- `coupon`：读取一张底图，把头像裁圆、缩放、旋转后叠加，再绘制可选文字，输出单帧；[源码](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_memes/src/memes/coupon.rs#L12-L54)
- `petpet`：5 张前景帧配 5 组头像矩形，逐帧先画头像再盖手掌，`circle` 是 typed boolean option；[源码](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_memes/src/memes/petpet.rs#L13-L58)
- `rub`：6 帧中分别按两套坐标表处理两个头像，其中一个还逐帧旋转，体现多头像和图层顺序；[源码](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_memes/src/memes/rub.rs#L11-L60)
- `gif_subtitle`：解码素材 GIF，用 `(start, end)` 帧区间选择文字并逐帧绘制，体现文字数量由时间段数量决定；[源码](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_memes/src/memes/gif_subtitle.rs#L17-L84)
- `look_flat`：同时包含默认文字、float option 的默认值和范围，并用公共逐帧管线自然支持动态输入；[源码](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_memes/src/memes/look_flat.rs#L12-L73)

首批本地模板只需选 2～3 个自有或授权明确的素材验证接口，不必照搬上游素材。上游代码的 MIT 许可也不自动覆盖其网络素材的再分发权。

### 上游契约

上游核心接口是：

```text
Meme.generate(images, texts, options) -> encoded bytes
```

输入图片抽象只有 `name + data`；模板元数据 `MemeInfo` 包含：

- `key`
- 图片数与文字数的 min/max
- 默认文字
- typed options
- keywords
- shortcuts
- tags
- 创建/修改日期

option 支持 boolean/string/integer/float，并可声明默认值、choices、数值上下界、说明及 short/long aliases。[MemeOption、MemeParams、MemeInfo 与 Meme trait](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_core/src/meme.rs#L27-L184)

`MemeBuilder` 在执行模板前先验证图片/文字数量，将松散的 option map 反序列化成模板自己的强类型 options，再解码图片并调用模板函数。[Builder 校验与执行](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_utils/src/builder.rs#L170-L245)

`InputImage` 同时保存第一帧 `image` 和完整 `codec`，使模板既能方便地把输入当静图，又能通过公共 GIF pipeline 保留动画。[InputImage](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_utils/src/builder.rs#L60-L77)

### 当前 Go 侧缺口

`services/meme/client.go` 当前只镜像了 `key`、`keywords`、图片/文字 min/max 和 `default_texts`，`Generate` 的 `options` 参数也始终由现有 handler 传 `nil`。直接影响是：

- 无法在“表情详情”中列出模板选项、choices、上下界与别名；
- 无法解析 `petpet` 的圆形开关、`symmetric` 的方向、`pixelate` 的强度等上游能力；
- 无法用 shortcuts/tags 做更精确的帮助、搜索和命令别名；
- Go 侧对 `/meme/infos` 的兼容模型不完整。

建议扩充 `MemeInfo`，至少镜像 `options / shortcuts / tags`，并让参数解析结果进入现有 `Generate(..., options)`。不建议为每个模板在 Go 中硬编码一套 option 结构；上游元数据已经是事实来源。

输出方面，上游核心 trait 只返回裸 bytes，没有格式/MIME/尺寸信息；HTTP server 最终通过文件内容推断 MIME。yueling-go 的 service 层应继续返回类似 `{Bytes, ContentType}` 的结果，不要让 plugin 再猜格式。[核心输出是裸 bytes](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_core/src/meme.rs#L163-L184) [HTTP 取图时推断 Content-Type](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_server/src/server.rs#L222-L230)

## 头像表情包怎样制作

### 固定帧动画模板

`petpet` 是最典型的头像模板：

1. 准备 5 张透明前景手掌图；
2. 每帧配置一组头像的 `(x, y, width, height)`；
3. 将头像裁成正方形，可选裁圆，再缩放到当前帧尺寸；
4. 先画头像，再覆盖手掌前景；
5. 以 0.06 秒每帧编码为循环 GIF。

源码见 [petpet 模板](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_memes/src/memes/petpet.rs#L13-L58)。

双头像/多头像模板仍是同一模式：每帧分别变换、定位头像，再叠加素材；`rub` 维护两套头像坐标并将头像裁圆。[rub 双头像模板](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_memes/src/memes/rub.rs#L11-L60)

因此，如果未来自制模板，最小模板描述应包括：

- 稳定 `key`、关键词和别名；
- min/max 图片数、min/max 文字数、默认文字；
- typed options 及默认值/约束；
- 帧数、逐帧 duration；
- 每个图片槽在每帧的变换：裁方/裁圆、目标矩形、旋转、透视或蒙版；
- 图层顺序；
- 同名素材目录。

不过，模板代码与素材应继续归上游/扩展 meme pack 管理。yueling-go 只维护 QQ 图片来源、用户参数与生成结果发送，不应该出现一份平行模板引擎。

### 字幕动画和镜像模板

GIF 字幕模板会解码模板动画，按帧区间选择文字，再逐帧绘制和编码。[gif_subtitle](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_memes/src/memes/gif_subtitle.rs#L17-L84)

“镜像”与“翻转”应在命令语义上区分：

- **水平翻转**：整张图左右互换；
- **垂直翻转**：整张图上下互换；
- **左/右/上/下镜像**：保留指定半边，把这一半反射复制到另一半。

上游 `/tools/image_operations` 公开的是前两种翻转；四方向镜像可参考 `symmetric`：裁一半、翻转对应区域、再拼回整图。[symmetric](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_memes/src/memes/symmetric.rs#L13-L119)

## 上游现成的图片处理能力

当前 server 已公开以下路由，Go 侧可以复用现有“上传图片 -> 调操作 -> 获取结果图片”的两阶段协议。[server 路由表](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_server/src/server.rs#L442-L490) [请求 DTO 与 handler](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_server/src/tools/image_operations.rs#L13-L258)

| 能力 | API | 主要参数 | 动图行为 |
|---|---|---|---|
| 图片信息 | `inspect` | `image_id` | 返回是否多帧、帧数、平均间隔 |
| 水平翻转 | `flip_horizontal` | `image_id` | 逐帧处理，保持 GIF |
| 垂直翻转 | `flip_vertical` | `image_id` | 逐帧处理，保持 GIF |
| 旋转 | `rotate` | `image_id`, `degrees?` | 默认 90°，逐帧处理，扩大画布 |
| 缩放 | `resize` | `width?`, `height?` | 单边等比，双边强制尺寸，逐帧 |
| 裁剪 | `crop` | `left?`, `top?`, `right?`, `bottom?` | 坐标夹到图片边界，逐帧 |
| 灰度 | `grayscale` | `image_id` | 逐帧处理 |
| 反色 | `invert` | `image_id` | 逐帧处理 |
| 横向拼接 | `merge_horizontal` | `image_ids` | 按最小高度等比缩放后拼接 |
| 纵向拼接 | `merge_vertical` | `image_ids` | 按最小宽度等比缩放后拼接 |
| GIF 拆帧 | `gif_split` | `image_id` | 每帧输出 PNG image id |
| GIF 合成 | `gif_merge` | `image_ids`, `duration?` | 默认 0.1 秒；帧缩到共同最小尺寸 |
| GIF 倒放 | `gif_reverse` | `image_id` | 逆序重编码 |
| GIF 变速 | `gif_change_duration` | `image_id`, `duration` | 为全部帧设置同一间隔 |

具体像素实现见 [image_operations.rs](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator/src/tools/image_operations.rs#L25-L253)。内部 `ImageExt` 还有 contain/cover、裁方、圆形、圆角、蒙版、保留/裁切画布旋转、透视、着色、透明度、亮度和高斯模糊；这些尚未全部暴露为 HTTP tools。[ImageExt 能力](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_utils/src/image.rs#L18-L78)

### 建议开放给 Bot 的能力

第一阶段优先做高频、参数简单、GIF 语义自然的命令：

- 水平翻转、垂直翻转；
- 左/右/上/下镜像；
- 旋转 90/180/270，另允许有界的任意角度；
- 等比缩放；
- 灰度、反色；
- GIF 随机抽帧（作为 `roll` 图片模式）。

第二阶段再开放：

- 裁剪（需要确定像素坐标、百分比还是“居中裁方”的用户语法）；
- 横/纵拼接（需要多图顺序规则）；
- GIF 倒放、变速、拆帧、合成；
- 圆形/圆角、模糊、亮度、透明度、像素化等效果。

不要把所有内部 `ImageExt` 一次性映射成命令。先确定稳定的中文命令、参数错误提示、图片来源与资源上限，再扩展能力表。

## GIF 管线与 `roll` 随机抽帧

### 上游怎样处理 GIF

上游 `CodecExt` 提供多帧判断、首帧和指定 index 取帧；帧时长用于动画重编码时会取平均值，并钳制到至少 0.02 秒。[CodecExt](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_utils/src/decoder.rs#L5-L49)

`make_png_or_gif` 的规则是：全静图输出 PNG；单 GIF 对每帧应用同一个闭包；多 GIF 选择最短平均间隔作为目标时间轴并循环对齐。固定模板动画与输入 GIF 的组合则使用 `NoExtend / ExtendLoop / ExtendFirst / ExtendLast` 对齐策略。[静图/GIF 共用管线](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_utils/src/encoder.rs#L236-L320) [组合 GIF 管线](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_utils/src/encoder.rs#L322-L389)

上游模板 `roll` 直接取 `InputImage.image`，也就是初始化时的第一帧，再生成 8 帧旋转动画；它没有随机选择输入 GIF 帧。[上游 roll 模板](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_memes/src/memes/roll.rs#L11-L45)

### yueling-go 的命令分流建议

`plugins/random/roll.go` 可采用以下优先级：

1. 当前消息附件或引用消息中有图片：进入图片模式；
2. 图片确为多帧 GIF：随机抽一个**显示帧**并发送 PNG；
3. 没有图片：完全保留现有 `roll`、`roll N`、`roll N M`、`roll A B C` 行为；
4. 静图或不支持格式给明确提示，不静默改变原有文本 roll 语义。

建议把“均匀按帧序号抽取”作为首版定义；如果希望更接近暂停动画的视觉概率，可以改成按每帧 delay 加权抽取，但需要在帮助中写清楚。

### 两种实现路径

**路径 A：复用上游 `gif_split`。** 上传 GIF，调用 split，从返回的 PNG IDs 中随机选一个再取回。优点是实现小、与现有 meme client 同一后端；缺点是服务端会解码并编码**所有帧**，还为每帧创建临时文件，随机只用一帧却支付全部输出成本。它只适合先加严格帧数/像素限制的首版。

**路径 B：本地 `services/imageframe`。** 使用 Go `image/gif.DecodeAll` 读取帧、delay、disposal 和全局画布，只合成到随机目标帧并编码一次 PNG。优点是不依赖 `meme_server`，也避免把全部帧编码成 PNG；缺点是必须自行正确实现显示帧合成和内存限制。

### disposal 不能忽略

Go 标准库 `gif.GIF` 返回的是逐帧 `*image.Paletted`、`Delay`、`Disposal` 和全局 `Config`；每帧 bounds 只保证位于全局画布内，disposal 包含 none/background/previous。[Go 官方 image/gif 文档](https://pkg.go.dev/image/gif#GIF)

因此不能直接选择 `g.Image[i]` 后编码。这些帧可能只是局部更新块；正确“随机显示帧”需要从第 0 帧重放到目标帧，在全尺寸画布上按透明色叠加，并在相邻帧之间应用 disposal：保留、清除前一帧区域，或恢复绘制前画布。否则抽到的 PNG 可能缺背景、缺前序内容或残留不该存在的像素。这是根据 Go 暴露的 frame bounds/disposal 语义得出的实现要求。

### 必须先设资源上限

压缩 GIF 很小并不代表解码后成本小。风险至少由 `width × height × frame_count × 每像素字节` 决定，还要考虑合成画布、previous 快照和最终 PNG。必须在进入昂贵处理前检查：

- 输入下载字节数；
- 宽、高及单帧像素数；
- 帧数；
- 累计解码像素或估算 RGBA 内存；
- 输出字节数；
- 单任务超时；
- 全局并发数。

本仓库现有 meme 图片下载上限是 16 MiB，可继续作为输入 bytes 的初始基线。上游当前 server 默认请求体 20 MiB、最多 16 个并发图像任务，encoder 的 `gif_max_frames` 默认 200；但后者主要限制 GIF 对齐延长，并**不等价于**为 `gif_split` 或任意输入解码提供完整的帧数/像素保护。[server 默认限制](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_server/src/config.rs#L23-L40) [encoder 默认帧数](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_utils/src/config.rs#L38-L49) [gif_split 会遍历全部帧](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator/src/tools/image_operations.rs#L175-L187)

这些限制应进入统一配置或 service 常量，不要散落在各 handler 中。具体数值要按部署内存与并发压测确定。

## 可借鉴与不适合照搬

### 值得借鉴

- 通用图片原语、GIF 解码/编码管线、模板实现、素材和 HTTP 绑定清晰分层。
- 一个头像模板一个模块，并用同名素材目录管理逐帧前景。
- 模板元数据统一描述图片数、文字数、默认文字、typed options、关键词、快捷方式和标签。
- 普通像素变换统一走逐帧 pipeline，使同一实现自然覆盖静图和 GIF。
- 固定动画用“帧素材 + 坐标/尺寸表 + duration”实现，便于逐帧校准。
- HTTP server 对 CPU 密集图像任务使用 blocking worker，并设置请求体与并发限制。[并发限制与 blocking 调用](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_server/src/server.rs#L55-L58)

### 不适合直接照搬

- Rust 多 crate、inventory 和动态库 ABI 对 Go Bot 命令层过重；借鉴 seam，不复制机制。
- 上游核心输出只有裸 bytes；Go service 应携带 MIME/格式信息。
- 上游用平均帧时长重编码，变时长 GIF 会失去逐帧 delay；`gif_reverse` 同样使用平均间隔。[gif_reverse](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator/src/tools/image_operations.rs#L210-L235)
- 不是所有模板都会自动保留动态头像：直接读取 `InputImage.image` 的老模板只取首帧，只有显式走 GIF pipeline 的模板才逐帧处理。
- 上游图片上传协议支持 `url`、`path` 和 base64 `data`。yueling-go 不应把 `url/path` 透传给可被用户控制的请求，否则会把 SSRF 或服务端本地文件读取面带入 Bot；继续由 Go 的公网 URL 安全下载器取 bytes，再使用 `data` 上传。[上游 ImageData 三种来源](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/meme_generator_server/src/server.rs#L148-L195)
- 上游 README 明示模板素材来自网络、侵权可联系删除；代码 MIT 不代表每张素材都具有清晰的再分发授权。自制或新增素材必须单独确认来源与授权。[上游素材声明](https://github.com/MemeCrafters/meme-generator-rs/blob/067da9fef80a6364018b7fd682622b655e2cddae/README.md#L59-L62)

## 推荐实施顺序（只作为后续计划输入）

1. 补齐 `services/meme` 的 `MemeInfo`、typed options 和图片 operation DTO；把私有 upload/fetch 能力整理成可复用的显式方法。
2. 新建 `plugins/imageops`，先接水平/垂直翻转、四方向镜像、90/180/270 旋转、等比缩放、灰度、反色。
3. 为图片来源建立一个统一 resolver，复用附件/引用图片和头像优先级；避免 `memes.go`、`imageops`、`roll.go` 各写一套。
4. 扩展 `roll` 图片模式；在“远端 gif_split 首版”与“本地 disposal-aware 抽帧”之间做一次明确选择，并先落资源限制测试。
5. 最后再开放复杂参数和 GIF 拆合/倒放/变速，同时补 help、超时、并发、尺寸/帧数/输出限制与错误提示。
