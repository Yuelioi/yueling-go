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

## 六职业扩充主体

- `dagger-1.webp`～`dagger-5.webp` — 铁匕、淬锋双匕、乌金双刃、寒鸦双刺、鱼肠五档短刃。
- `gu-1.webp`～`gu-5.webp` — 陶蛊盅、铜纹蛊罐、碧玉蛊盅、五仙蛊鼎、万蛊归墟盅五档蛊器。
- `qin-1.webp`～`qin-5.webp` — 桐木琴、漆纹琴、冰弦琴、绿绮、伏羲琴五档古琴。
- `weapon-25-cutout.webp`～`weapon-27-cutout.webp` — 旧精制兵器的同名透明原画复用，非新造型。
- [扩充提示词及源文件](../wuxia-concepts/completion-prompts.json) — 内置image_gen逐件提示词、原始生成路径与Web/QQ副本位置。

图鉴读取候选目录和共享美术映射；只展示品级，不将武器养成等级绘入图片。

- [通用装备扩充提示词](expansion-prompts.json) — 新增护具与饰品逐件设计、生成提示词与原图来源。
