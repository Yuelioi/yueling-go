# 武学图鉴素材

武技、心法的独立图鉴与 QQ 卡片共用稳定技能 ID。技能名称、品级和规则以 `services/wuxia/internal/content/content.json` 为准。

- `sword-heaven.webp` — 一剑开天门：银剑劈开青玉古门印，裂口为视觉中心。
- `sword-unity.webp` — 万剑归宗：剑阵汇入玉质剑心，汇聚轮廓区别单剑。
- `blade-frost.webp` — 傲寒六诀：厚重刀身与层叠冰刃，银白与靛青为主体。
- `fist-dragon.webp` — 降龙十八掌：盘龙环抱玉掌，掌印与龙首构成双重识别。
- `sword-basic.webp`、`sword-rain.webp`、`sword-swallow.webp`、`sword-solitude.webp` — 剑类入门至绝学主体。
- `blade-basic.webp`、`blade-rift.webp`、`blade-tide.webp`、`blade-moon.webp` — 刀类入门至绝学主体。
- `fist-basic.webp`、`fist-mountain.webp`、`fist-tiger.webp`、`fist-taichi.webp` — 拳类入门至绝学主体。
- `mind-*.webp` — 吐纳、混元、铁衣、小无相与九阳五档通用心法。
- [扩充提示词](expansion-prompts.json) — 各品级原画的内置 image_gen 提示词及来源。
- [提示词](prompts.json) — 内置 image_gen 的逐件提示词及原始生成文件。

## 制作规则

遵循[装备制作规范](../equipment-v2/production-rules.md)的材质、品质配色、透明主体和无实景底图要求。武学是招式意象，可以表现剑阵、掌印和内息，不将所有武学画成相同秘籍换色。详情复用装备的五档底图，主体不得自带文字、品级、数值或界面边框。

传世武技使用独立插画主体和两行七言。图鉴、主页槽位与收尾招式使用同一主体；交锋轨迹使用同一原画，去除透明留白后等比居中，辅以品级细框；不使用另一套单色抽象符号。QQ 侧384px副本位于 `services/wuxia/cards/assets/martial`。修改主体时从高清图重新编码QQ副本，不改变构图或透明通道。不维护独立实战图标素材；网页直接使用原画，QQ 从同一原画副本按实际尺寸渲染。

入口为[武学图鉴](../../wuxia-preview.html#martial)。

## 六职业扩充

- `shadow-stab.webp`、`shadow-raven.webp`、`shadow-pursuit.webp`、`shadow-formless.webp`、`shadow-eternity.webp` — 幽影五档武技。
- `venom-coat.webp`、`venom-beetle.webp`、`venom-bone.webp`、`venom-soul.webp`、`venom-abyss.webp` — 千蛊五档武技。
- `chord-pluck.webp`、`chord-clear.webp`、`chord-rain.webp`、`chord-guangling.webp`、`chord-ocean.webp` — 弦歌五档武技。
- [补齐提示词及源文件](../wuxia-concepts/completion-prompts.json) — 新增入门至绝学主体的独立生成记录；传世沿用[已确认概念稿](../wuxia-concepts/prompts.json)。

统一预览读取six_professions.json候选规则，图片映射在webui/src/data/wuxia-art.json。候选美术完整不表示正式职业已经开放；QQ副本由同一原图编码。
