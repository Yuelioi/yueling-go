# 江湖界面预览样张

由当前 Go 卡片渲染器及测试示例角色生成，供本地界面验收，不连接玩家数据。新装备图鉴素材并未自动接入这些玩法卡片。

- `martial-icons.webp` — 武技与心法图标对照。
- `martial-profile-*.webp`、`martial-duel-*.webp` — 三职业武技、心法、兵器联动与真实伤害战报。
- `profile.webp`、`upgrade-*.webp` — 侠客属性及职业成长界面。
- `inventory-profile.webp` — 当前护具、饰品与武学状态。
- `adventure-variety.webp` — 高人指点、钱袋与破财事件的奖惩展示。
- `onboarding.webp`、`adventure*.webp`、`replay.webp` — 入门、历练、操作回执及重复进入状态。
- `duel*.webp` — 群友切磋与跨职业对战。
- `casino-*.webp` — 赌坊顺子、落空、十倍豹子、退半及对子回本结算。
- `draw-five.webp` — 当前五连抽结算。
- [预览页面](../../wuxia-preview.html) — 分类浏览、原尺寸图片及装备图鉴入口。

刷新样张：设置 `YUELING_WUXIA_PREVIEW_DIR` 后运行 `go test ./services/wuxia/cards -count=1`，将同名 PNG 编码为 WebP；保留原尺寸，不重新绘制界面。只选择测试生成的样张，不复制真实群聊截图。

## QQ卡片排版约束

图片里的操作入口是群聊指令提示，不是可点击网页按钮。底部独立深色区域，说明发送方式；侠客和切磋使用低权重的单行指令栏，其余指令块横向内边距至少24px。主要内容与指令区分开留白。玩家正文不放操作保存、回执等内部术语，不声明未发生的自动换装；重播需以旧事回顾明确区分。

侠客页以身份、战力、当前兵刃为首要信息；次数及签到只在顶部。行囊收藏不混入当前装备槽。切磋卡以胜负、双方气血及关键招式为主，普通交手按时序以头像、效果图标和伤害压缩展示，每行四招；制胜一击单独突出招式名与伤害。

侠客以连续背景承载人物与信息，身份在左上，人物为主体，兵器完整展示于右侧；不裁切剑尖、刀锋或挂穗。空装备位用简短状态行，不扩大空槽。切磋使用双方头像，胜者与制胜一击为主；普通交锋以紧凑图标条呈现，不写逐招日志。

交锋图标沿起到收的回折剑路阅读，箭头及跨行弧线指示顺序，无编号；连续同方出手标注连攻次数。结尾调侃按战况分类并由战斗种子固定，同场回看保持一致；只有满血胜利称完胜，旧记录不推断优势。

- [武学图鉴素材](../martial-art/index.md) — 招式插画、品级表现与生成提示词；顶部武学图鉴入口按职业浏览。

- `pk-basic-eight.webp`、`pk-legend-eight.webp`、`pk-legend-long.webp` — 由实际引擎结算的低阶/传世短战报及加厚气血长战报布局样例，展示简形武技、回血与受击方心法。
- `pk-symbol-comparison.webp` — 武学原画与48px/24px交锋符号的对照。

- `profile-empty.webp` — 初始兵器及四个空槽，空装备不画成已佩戴物件。
- `profile-full-swordsman.webp`、`profile-full-blade.webp`、`profile-full-boxer.webp` — 三职业护具、饰品、职业武技及通用心法均已装配的角色页。
- `pk-empty.webp`、`pk-full.webp` — 双方无武学/武学齐备的切磋对照。头像下始终展示携带武技与心法；交锋只显示真实触发。

## 当前候选页面

`candidate-profile-*`、`candidate-empty-*`、`candidate-duel-*`、`candidate-adventure-*`、`candidate-draw-*`由六职业候选真实数据渲染；`candidate-casino`展示赌场结果。入口清单按候选职业定义生成，旧样张仅作历史参考。重新生成使用 `YUELING_WUXIA_PREVIEW_DIR` 配合 `TestSixCandidateScreens`，PNG编码WebP后放入本目录。
