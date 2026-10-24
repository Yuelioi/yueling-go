# 六职业设计预览

职业候选数据、数值验证摘要与幽影、千蛊、弦歌技能代表原画，不属于当前可获得物品或正式战斗内容。

- `shadow-eternity.webp` — 刹那无间，短刃残影的传世武技提案。
- `venom-abyss.webp` — 万蛊归墟，收拢蛊器与蛊心的传世武技提案。
- `chord-ocean.webp` — 沧海龙吟，古琴与音浪的传世武技提案。
- `prompts.json` — 内置 imagegen 提示词、样式参考与生成源位置。

采用[器物制作规范](../equipment-v2/production-rules.md)与[武学美术规则](../martial-art/index.md)，透明主体供大图与小图共同使用。与已认可素材分开保存，不覆盖正式主体。

- [统一预览](../wuxia-preview.html#professions) — 在职业与成长标签查看原画大小对照、兵器、武技与成长；对战验证在同页标签。

- `profession-data.json` — 可执行候选目录的职业、兵器、武技镜像，测试防止预览数值漂移。
- 展示组件为 `webui/src/components/WuxiaProfessionDesign.vue`，随统一预览入口加载；旧职业设计链接自动跳转。
- `balance-summary.json` — 完整报告的独立种子汇总与适用范围。

- `completion-prompts.json` — 补齐兵器与武技的内置image_gen逐件提示词、源文件及Web/QQ副本。
