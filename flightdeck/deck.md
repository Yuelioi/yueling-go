---
format: 4
project: yueling-go
focus: webui-admin
---

# Flightdeck

## Conventions

### 项目协作规则

- 日志统一用 `services/logx`，不用 stdlib `log`；修改日志前读取 `knowledge/logging/logx.md`。
- 本地 commit 由 agent 按任务落地情况自决，可 reset/amend；**push 必须先询问用户**。
- 任务彻底完成后更新 topic、分类知识并运行 `flightdeck_checkpoint`；整个主题完成时运行 `flightdeck_finish`。需要落地时按项目约定创建本地 commit，push 仍须先询问。

### 核心工程铁律

- 所有注册（插件、AI工具）必须在 `b.Start()` 之前完成，不得动态修改 `b.regs`
- AI 工具通过 `ai.Register()` + `init()` 注册，插件通过 `Register(b *bot.Bot)` 注册
- handler 签名必须是四种类型之一：`func(*CommandContext)error` / `func(*GroupContext)error` / `func(*NoticeContext)error` / `func(*RequestContext)error`
- 高风险操作（禁言/踢人）必须设置 `ConfirmRequired: true` 或加 `AdminOnly{}` 条件
- 新增/修改插件命令后，必须同步更新 `plugins/system/help.go` 的 `pluginRegistry`（命令清单、用法 Usage、`Commands` 列表），否则 `help`/`帮助` 看不到该命令
- 精确命令必须用 `OnCommand` / `OnFullMatch` 注册（不要用 `OnKeyword`/`Any` 兜底实现精确命令）。dispatcher 仅对 `Command`/`FullMatch` 命中置 `commandMatched`，复读插件据此自动跳过命令；用 Keyword/Any 实现的「命令」不会被识别，会被复读。无需再手动维护复读黑名单

### Flightdeck 4 路由规则

- 先用 `flightdeck_inspect` 查看 focus、活动主题和知识 frontmatter；恢复主题使用 `flightdeck_resume`。
- topic index 严格保持 State、Next、Read now、Read if、Progress、Open questions 六节；设计与计划作为 package 内显式依赖。
- 不要批量读取 `knowledge/`；按 `read_when` 只加载与任务匹配的知识。
- 完成主题使用 `flightdeck_finish` 归档到仓库内 `archive/<topic>/`，不要手工移动。

### 本地化协作知识

- 修改源码注释前读取 `knowledge/coding/comments.md`。
- 暂存、提交或准备 PR 前读取 `knowledge/git/commits.md`。

## Open questions

### Cockpit Next

当前开发：WebUI 管理后台 spec review。

待重新部署后线上手验四项（均已过单测/构建，仅缺线上验证）：

1. **AI 频率限制** — 配 `[ai.ratelimit] user_per_min=5, group_per_min=15`，同一人 1 分钟内连发 >5 次群聊AI/zssm/翻译 → 回「你发消息太频繁了…」；群内多人累计 >15 次/分钟 → 回「本群 AI 用得太频繁了…」；改 0 则不限。
2. **加群审核新命令（覆盖语义）** — `加群审核` 展示白名单/黑名单；`加群白名单 交流,学习` 直接覆盖；`加群白名单`（空）清空；命中通过词自动通过、命中拒绝词自动拒绝、其余留人工。
3. **AI 上下文工具默认条数可配** — 配 `[ai.context] chat_history=15, summary=50`；@月灵 对话不自动带群记录，模型按需调 get_chat_history（默认取 chat_history、上限仍 30）；总结调 summarize_chat（默认取 summary=50、上限仍 100）。改默认值后让模型「看看刚才聊了啥」「总结一下」验证条数生效。
4. **图片类目配置表重构（v1.9.0）** — 默认表不配则行为不变；验证：龙图/福瑞等随机一张、吃啥/喝啥等 4合1 网格（添加须带名字 `添加吃的 麻辣烫`、4合1 显示名字、同名不覆盖）、随机猫猫外链、添加语录/表情/各类图、`帮助 龙图`/`帮助 添加语录` 能查到；新配的 `随机插画`（external pick=`data[*].url` + base 取 pln 画站）能发图。契约见 `knowledge/image/categories-config-table.md`。

其余五项（pack 上传 / 嵌套转发 / 进度表情 / 设精 / 复读不复读命令）此前已手验通过。

### Cockpit Open questions

待手验四项的具体未验点（阻塞于「重新部署后」线上验证）：

- **AI 频率限制** — aiLimiter 双窗口逻辑已过单测（ai/ratelimit_test.go：个人/群超限、0=不限、被拦不占名额、私聊跳群窗），但真实群里触发提示 + config 默认 5/15 是否符合预期未线上验证。
- **加群审核（覆盖语义）** — joinListHandler 改 db.SetGroupJoinRules 一次性覆盖、parseKeywords 去掉 +/-，已过单测（TestParseKeywords + db TestGroupJoinRuleCRUD），但整条 OnRequest 审批链路 + 新命令交互未线上验证。
- **AI 上下文条数可配** — 新增 [ai.context]，两个 handler 改用纯函数 ai.ResolveCount(provided, def, min, max)（已过 ai/count_test.go 8 例），硬上限 30/100 不变。但「配置默认值真生效、模型按需拉取条数符合预期」未线上验证。
- **图片类目配置表重构** — single/grid/external 由 [[image.entry]] 驱动（默认表照搬旧行为、整体覆盖语义），语录/表情抽成 plugins/quotation、plugins/emoticon，help #18/#32 由配置运行期生成；已过 build/vet/test（image 包 pick 求值/校验/命名/扩展名探测单测）。但随机发图/4合1/外链/添加各类/help 查询的真实群行为未线上验证。
