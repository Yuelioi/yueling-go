# 分品级装备美术

兵器、通用护具、通用饰品按五个品级分组，主体与品质底图分层，用于图鉴与抽取展示。护具与饰品不设职业专属。由内置imagegen生成，WebP编码保留透明通道。

- [制作规范](production-rules.md) — 新增或重绘装备前必读：品级、命名、主体、底图、文案与验收标准。
- `*-subject.webp` — 透明器物主体，同一件的缩略图与详情共用。
- `quality-*-background.webp` — 当前五档共用的低对比无实景底图。
- 其他 `*-background.webp` — 已撤回的旧实景背景，只留作参考，不再使用。
- [底图提示词](background-prompts.json) — 当前五档底图的独立生成提示词。
- [提示词](prompts.json) — 每层独立生成提示词，重绘时阅读。
- [兵器图鉴数据](../../src/data/weapon-gallery.json) — 兵器品级、名称、说明及素材路径。
- [通用装备图鉴数据](../../src/data/universal-equipment-gallery.json) — 护具与饰品品级、名称、来历及素材路径。
- [通用装备提示词](universal-prompts.json) — 护具与饰品独立透明主体的生成提示词。

旧版黑底美术保留在[旧装备目录](../equipment/index.md)，不作新图鉴素材。展示不包含武器Lv。

其他游戏界面见[卡片预览目录](../wuxia-preview/index.md)，可在本地总预览切换查看。
