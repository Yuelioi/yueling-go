# Cockpit — yueling-go

**Last updated**: 2026-06-23 by 月离 (图片类目配置表驱动重构已实现+单测：single/grid/external 三 kind 由 [[image.entry]] 驱动，语录/表情抽成独立插件，help 由配置生成；契约见 docs/image-categories-config-table.md)
**Active focus**: 图片类目改配置表驱动（single/grid/external + 语录/表情抽离）已实现并过 build/vet/test，待重新部署后手验；连同 v1.7.0/v1.8.0 三项共四项待手验（见 Pending Review / 下一步）；无进行中开发任务

## 进行中

<!-- AUTO:inprogress -->

<!-- /AUTO -->

## 下一步

无进行中开发任务。待重新部署后手验四项：

1. **AI 频率限制** — 配 `[ai.ratelimit] user_per_min=5, group_per_min=15`，同一人 1 分钟内连发 >5 次群聊AI/zssm/翻译 → 回「你发消息太频繁了…」；群内多人累计 >15 次/分钟 → 回「本群 AI 用得太频繁了…」；改 0 则不限。
2. **加群审核新命令（覆盖语义）** — `加群审核` 展示白名单/黑名单；`加群白名单 交流,学习` 直接覆盖；`加群白名单`（空）清空；命中通过词自动通过、命中拒绝词自动拒绝、其余留人工。
3. **AI 上下文工具默认条数可配** — 配 `[ai.context] chat_history=15, summary=50`；@月灵 对话不自动带群记录，模型按需调 get_chat_history（默认现取 chat_history、上限仍 30）；总结调 summarize_chat（默认现取 summary=50、上限仍 100）。改默认值后让模型「看看刚才聊了啥」「总结一下」验证条数生效。
4. **图片类目配置表重构** — 默认表不配则行为不变；验证：龙图/福瑞等随机一张、吃啥/喝啥等 4合1 网格（添加吃喝玩乐须带名字、grid 显示菜名）、随机猫猫外链、添加语录/表情/各类图、`帮助 龙图`/`帮助 添加语录` 能查到；可选配 `[[image.entry]]`（external 的 pick 取 JSON 图）。契约见 `docs/image-categories-config-table.md`。

其余五项（pack 上传/嵌套转发/进度表情/设精/复读不复读命令）此前已手验通过。

## Pending Review

- ⚠待手验: AI 频率限制 — aiLimiter 双窗口逻辑已过单测（ai/ratelimit_test.go：个人/群超限、0=不限、被拦不占名额、私聊跳群窗），但真实群里触发提示 + config 默认 5/15 是否符合预期未在线上验证；重新部署后手验「下一步 1」。
- ⚠待手验: 加群审核（覆盖语义）— joinListHandler 改为 db.SetGroupJoinRules 一次性覆盖、parseKeywords 去掉 +/-，已过单测（TestParseKeywords + db TestGroupJoinRuleCRUD），但整条 OnRequest 审批链路 + 新命令交互未在线上验证；重新部署后手验「下一步 2」确认查看/覆盖/清空/自动通过拒绝。
- ⚠待手验: AI 上下文工具默认条数可配 — 新增 [ai.context] chat_history/summary，两个 handler 改用纯函数 ai.ResolveCount(provided, def, min, max) 取默认+钳制（已过单测 ai/count_test.go 8 例），硬上限 30/100 不变。但「配置默认值真生效、模型按需拉取条数符合预期」未在线上验证；重新部署后手验「下一步 3」。
- ⚠待手验: 图片类目配置表重构 — single/grid/external 由 [[image.entry]] 驱动（默认表照搬旧行为，整体覆盖语义），语录/表情抽成 plugins/quotation、plugins/emoticon，help #18/#32 由配置运行期生成；已过 build/vet/test（image 包 pick 求值/校验/命名/扩展名探测单测）。但随机发图/4合1/外链/添加各类/help 查询的真实群行为未在线上验证；重新部署后手验「下一步 4」。契约 docs/image-categories-config-table.md。

## Hanging tasks

- (none)
