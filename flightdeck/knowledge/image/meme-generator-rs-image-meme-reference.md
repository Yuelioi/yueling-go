# 图片处理与头像表情保持分层

图片能力按职责分层，避免把素材库、无状态变换、本地模板和远端表情服务混成一个插件：

- `plugins/avatar_meme` 拥有本地 Go 模板、素材、模板元数据和命令注册；模板显式组装并随程序发布。
- `plugins/imageops` 处理翻转、镜像、旋转、缩放、灰度和反色等“输入图片、立即输出”的命令。
- `plugins/internal/imageinput` 统一解析当前消息、引用消息、@头像和发送者头像，插件不重复实现下载优先级。
- `services/imaging` 负责解码、完整 GIF 帧、资源限制、变换、时间轴与 PNG/GIF 编码，不感知 QQ 命令。
- `plugins/funny/memes.go` 与 `services/meme` 继续作为远端 meme 服务客户端；远端不可用时不影响本地头像模板。
- `plugins/image` 只保留配置表驱动的素材类目、上传、随机抽取和网格，不承载无状态图像变换。

新增本地模板时，在独立子包中定义稳定 key、关键词、图片/文字数量、默认文字和 typed options；通过 `go:embed` 携带权利清晰的素材，在注册表中拒绝重复 key 或关键词。模板只返回图像帧，图片来源、Bot 回复和最终编码分别留在命令层与 `services/imaging`。

所有命令在 Bot 启动前注册，并绑定独立 catalog ID；帮助目录同步维护，精确命令使用 `OnCommand` 或 `OnFullMatch`。
