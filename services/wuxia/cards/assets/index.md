# 江湖静态美术

界面分区参考《逸剑风云决》官方角色装备页，使用独立的靛蓝、暖白与朱砂配色、雨夜山城底图和银白框线。人物取自用户已认可版本，运行时使用Go绘制真实数据。

- `menu-backdrop-v6.png` — 靛蓝雨夜山城菜单背景。
- `menu-frame-v6.png` — 可九宫格拉伸的透明靛蓝漆边、银白双线与朱砂角饰。
- `ui-v6-prompts.json` — 底图与边框的内置imagegen提示词、生成源文件与编辑参考。
- `menu-backdrop-v5.png`、`menu-frame-v5.png` — 中性水墨灰与蚀金边框的来源参考，不参与当前构建。
- `menu-logo-v5.png` — 「月灵江湖」透明书法字标。
- `stat-glyphs-v5.png` — 攻击、防御、身法与武学的透明徽记。
- `ui-v5-prompts.json`、`ui-v5-logo-prompt.json` — 视觉目标与静态部件的内置imagegen原始提示词、参考来源与作用。
- `ui-display.otf` — Noto Serif CJK SC SemiBold的标题字形子集，字体名为Yueling UI Display。
- `OFL.txt` — 原字体版权声明与许可证。
- `display-glyphs.txt` — 字形子集清单。
- `build_display_font.py` — 根据官方原字体与许可证重建内嵌标题字体的开发脚本；依赖fontTools，运行时不需要Python。

- `room-v4.png` — 上一版练功房参考，不参与当前构建。
- `scenes-v4.png` — 竹林、雨街、山院和渡口的同画风历练卡片场景。
- `ui-v4-prompts.json` — 练功房与历练场景的内置imagegen提示词。

- `ink-shell-v3.png` — 旧纸山水界面的来源参考，不参与当前构建。
- `ink-character-sword-v3.png`、`ink-character-blade-v3.png`、`ink-character-boxer-v3.png` — 独立透明职业角色，避免图集边界残片。
- `ink-items-v3.png` — 游戏物品的透明线描图标。
- `ink-sprites-v3.png` — 职业样式参考图集，独立角色制作参考。
- `ui-v3-prompts.json` — 人物认可版本的底图、角色、物品及角色拆分的内置imagegen提示词。

旧版美术保留作来源参考，是否参与构建由`render.go`的嵌入清单及`regions.json`决定。


- `scenes.png` — 场景图集：竹径、雨街、山院、渡口；本地渲染按固定网格读取。
- `portraits.png` — 旧版侠客图集：剑客、刀客、拳师；形象可独立于职业切换。
- `courtyard-v2.png` — 客栈庭院主界面场景，预留角色站位与装备面板空间。
- `sprites-v2.png` — 三职业透明角色图集，用于角色融入场景。
- `items-v2.png` — 武器、护具、玉佩、银两、武学与行囊的透明物品图标。
- `ui-v2-prompts.json` — 游戏主界面新增场景、角色抠图和物品图标的内置imagegen原始提示词。
- `regions.json` — 稳定形象／场景ID到文件、网格和区域的映射；新增模板时阅读。
- `prompts.json` — 使用内置 imagegen 生成上述美术的提示词与来源说明。

当前使用的图集随程序嵌入，运行时不访问生成服务。新增场景或形象时增加图文件与区域映射，内容定义引用稳定ID，不重排已有ID。

字体来源：[官方Noto Serif仓库](https://github.com/notofonts/noto-cjk/tree/main/Serif/OTF/SimplifiedChinese)、[官方许可证](https://github.com/notofonts/noto-cjk/blob/main/Serif/LICENSE)。构建时向`build_display_font.py`传入下载好的原始OTF与许可证路径，输出本目录字体、版权和许可证；字形清单变更后需要重新构建。动态昵称缺少标题字形时仍使用正文中文字体。

- [现有兵器原画](equipment/index.md) — 侠客页按物件名称绑定的大幅兵器素材及来源。
