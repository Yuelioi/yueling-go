# 武学插画

按稳定技能ID保存384px透明WebP，供侠客槽位与战报收尾复用；`skill_art.go`嵌入并缓存解码。原始高清图保留在Web图鉴，QQ不解码整套高清图。

- `sword-*.webp` — 剑类五档主体，传世保留开天门与万剑归宗。
- `blade-*.webp` — 刀类五档主体。
- `fist-*.webp` — 拳类五档主体。
- `mind-*.webp` — 全职业通用心法五档主体。
- [制作规则与提示词入口](../../../../../webui/public/martial-art/index.md) — 高清图鉴、意象规范与生成来源。

交锋小图标由 `skill_icons.go` 单独绘制，保留主体辨识特征，不直接缩小完整插画。

- `shadow-*.webp`、`venom-*.webp`、`chord-*.webp` — 新三职业五档武技的同源384px副本，来源见[Web武学索引](../../../../../webui/public/martial-art/index.md)。
